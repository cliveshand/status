package main

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// had to dedupe package conflicts as everything is in main
// we might be reaching the point where we need to breaking these out into other modules
// that should be part of a refactor work
var (
	tracerQuerier = otel.Tracer("querier")
	meterQuerier  = otel.Meter("querier")
	querierTime   metric.Int64Counter
)

func init() {
	var err error
	querierTime, err = meterQuerier.Int64Counter("querier.time",
		metric.WithDescription("The seconds for performing mimir queryier"),
		metric.WithUnit("{ti}"))
	if err != nil {
		panic(err)
	}
}

func querier(w http.ResponseWriter, r *http.Request) {

	ctx, span := tracerQuerier.Start(r.Context(), "querier")
	defer span.End()

	startTime := time.Now()

	mimirQuerier := newPromQuerier()

	mimirQuerier.config = &appConfig
	// TODO: The info block is pointless since our Query Results contain the same data from the looks of it
	// in the intermediate representation
	log.Printf("attempting %v:%v", mimirQuerier.config.Defaults.Host, mimirQuerier.config.Defaults.Port)
	results, err := mimirQuerier.queryRuleset(mimirQuerier.config.Rules)
	if err != nil {
		fmt.Println("error querying ruleset:", err)
	}

	for _, rule := range results {
		if len(rule.QueryResults) < 1 {
			fmt.Printf("rule query: %s has empty results", rule.Name)
			return
		}
	}

	// now we need to transform the intermediate modified rules to the simpler form
	finalRes, err := transformToFinalOutput(results)
	if err != nil {
		fmt.Println("failed to transform data:", finalRes, " with error:", err)
		return
	}

	jRes, err := json.MarshalIndent(finalRes, "", "\t")
	if err != nil {
		fmt.Println("error marshalling json:", err)
		return
	}

	createDirectoryIfNotExists()
	err = os.WriteFile(appConfig.WriteTo, jRes, 0644)
	if err != nil {
		log.Printf("failed to write json response to file location: %s", appConfig.WriteTo)
		return
	}

	// fmt.Println(results)

	// cmd := exec.Command("hugo")
	// result := "Success"
	// _, err := cmd.Output()
	// if err != nil {
	// 	result = "Failed to build"
	// }
	duration := time.Since(startTime)

	// Add the custom attribute to the span and counter.
	querierValueAttr := attribute.Int("querier.time", int(duration.Minutes()))
	span.SetAttributes(querierValueAttr)
	querierTime.Add(ctx, 1, metric.WithAttributes(querierValueAttr))

	_, err = http.Get("http://localhost:8080/build")
	if err != nil {
		log.Printf("failed to call build with error: %v", err)
		return
	}
}

// uses the app config, but probably should be passed in as argument
func createDirectoryIfNotExists() {
	// because writeTo includes the file, go to -1
	splitPath := strings.Split(appConfig.WriteTo, "/")
	pathJoin := strings.Join(splitPath[:len(splitPath)-1], "/")

	if _, err := os.Stat(pathJoin); os.IsNotExist(err) {
		err := os.MkdirAll(pathJoin, 0755)
		if err != nil {
			fmt.Println("error creating directory:", err)
			return
		}
	}
}

type Rule struct {
	name      string
	condition string
	scores    map[string]int
	// I don't think this should be using interface, but I don't know the type of this data coming back yet
	info    map[string]interface{}
	queries []QueryResult
}

// RuleBuilder is a builder for the Rule struct.
type RuleBuilder struct {
	rule Rule
}

// NewRuleBuilder creates a new RuleBuilder instance.
func NewRuleBuilder() *RuleBuilder {
	return &RuleBuilder{}
}

// SetName sets the name field of the Rule struct.
func (rb *RuleBuilder) SetName(name string) *RuleBuilder {
	rb.rule.name = name
	return rb
}

// SetCondition sets the condition field of the Rule struct.
func (rb *RuleBuilder) SetCondition(condition string) *RuleBuilder {
	rb.rule.condition = condition
	return rb
}

// SetScores sets the scores field of the Rule struct.
func (rb *RuleBuilder) SetScores(scores map[string]int) *RuleBuilder {
	rb.rule.scores = scores
	return rb
}

// SetInfo sets the info field of the Rule struct.
func (rb *RuleBuilder) SetInfo(info map[string]interface{}) *RuleBuilder {
	rb.rule.info = info
	return rb
}

// SetQueries sets the queries field of the Rule struct.
func (rb *RuleBuilder) SetQueries(queries []QueryResult) *RuleBuilder {
	rb.rule.queries = queries
	return rb
}

// Build returns the final Rule instance.
func (rb *RuleBuilder) Build() Rule {
	return rb.rule
}

type PromQuerier struct {
	// TODO: This is going to get updated with the config types
	config *Config
	qCount int
}

func newPromQuerier() *PromQuerier {
	return &PromQuerier{qCount: 0}
}

func (pq *PromQuerier) getQueryCount() int {
	return pq.qCount
}

func (pq *PromQuerier) queryRuleset(rules []RuleConfig) ([]ModifiedRule, error) {
	// I don't know if things start erroring out if this creates a weird
	// state issue
	// ruleBuilder := NewRuleBuilder()
	// ruleBuilders := make(map[string]RuleBuilder, 0)

	// to aggregate all the modified rules
	mRules := make([]ModifiedRule, 0)

	for _, rule := range rules {
		// ruleBuilder.SetName(rule.Name)
		// ruleBuilder.SetCondition(rule.Condition)
		mrb := NewModifiedRuleBuilder().WithName(rule.Name).WithCondition(rule.Condition)

		tempArray := make([]ModifiedPrometheusQueryResult, 0)

		for _, query := range rule.Queries {
			pq.qCount += 1
			qr, err := pq.queryMimir(query)
			if err != nil {
				fmt.Println(err)
			}

			for _, metric := range qr.metrics {
				for key, value := range metric.Pqr.Metric {
					if _, ok := mrb.Scores[metric.Instance]; ok {
						temp := mrb.Scores[metric.Instance]
						temp.Value += 1
						mrb.Scores[metric.Instance] = temp
					} else {
						temp := mrb.Scores[metric.Instance]
						temp.Value = 1
						mrb.Scores[metric.Instance] = temp
						tempMap := map[string]string{
							key: value,
						}
						// since mrb.Infos[metric.Instance] can panic
						if mrb.Infos[metric.Instance] == nil {
							mrb.Infos[metric.Instance] = tempMap
						} else {
							maps.Copy(mrb.Infos[metric.Instance], tempMap)
						}
					}
				}
				// for key, value := range metric {
				// 	if _, found := ruleBuilder.rule.scores[key]; found {
				// 		ruleBuilder.rule.scores[key] += 1
				// 	} else {
				// 		ruleBuilder.rule.scores[key] = 1
				// 		ruleBuilder.rule.info[key] = value
				// 	}
				// }
			}
			// mrb.WithQueryResults(qr.metrics)
			// ruleBuilder.rule.queries = append(ruleBuilder.rule.queries, *qr)
			mrb.WithStatus(rule.Name)
			tempArray = append(tempArray, qr.metrics...)
		}
		// build the queryresults with the temp
		mrb.WithQueryResults(tempArray)
		mRules = append(mRules, *mrb.Build())
		// ruleBulders[ruleBuilder.rule.name] = *ruleBuilder
	}

	// if len(ruleBuilders) < 1 {
	// 	return nil, fmt.Errorf("no metrics for provided rules")
	// }

	// return ruleBuilders, nil
	return mRules, nil
}

type TimeDelta struct {
	start time.Time
	end   time.Time
}

// TODO: this function should be done wrapping a new type that is parsed from the read JSON config; but for now, it will just parse the string
func getTimeWindow(timeWindow string) (time.Duration, error) {
	// last letter
	lastLetter := len(timeWindow) - 1

	// unit of time
	unitOfTime := timeWindow[lastLetter:]

	value, err := strconv.Atoi(timeWindow[:lastLetter])
	if err != nil {
		return 0, err
	}

	switch unitOfTime {
	case "s":
		return time.Duration(value) * time.Second, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "M":
		// Note: This is a simplified example; months can vary in length
		// and need a more complex calculation.
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported unit: %s", unitOfTime)
	}
}

func getStartEnd(t string) (*TimeDelta, error) {
	endTime := time.Now()

	timeWindow, err := getTimeWindow(t)
	if err != nil {
		return &TimeDelta{}, err
	}

	// this is a bit confusing; Sub() returns a time duration,
	// which is what we don't want; Add() returns a time,
	// so we have to substract the value from time window
	startTime := endTime.Add(-timeWindow)

	return &TimeDelta{
		start: startTime,
		end:   endTime,
	}, nil
}

// TODO: right now, we're going to assume that we won't call params as we are only going off of the config
func (pq *PromQuerier) queryMimir(query string) (*QueryResult, error) {
	timeframe, err := getStartEnd(pq.config.Defaults.TimeWindow)
	if err != nil {
		return &QueryResult{}, err
	}

	fullQuery := fmt.Sprint(pq.config.Protocol, "://", pq.config.Defaults.Host, ":", pq.config.Defaults.Port, pq.config.Defaults.Api, "?query=", query, "&start=", timeframe.start.Unix(), "&end=", timeframe.end.Unix(), "&step=", pq.config.Defaults.Step, "&")

	res, err := http.Get(fullQuery)
	if err != nil {
		return &QueryResult{}, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return &QueryResult{}, fmt.Errorf("error received http status %s", res.Status)
	}

	var data PrometheusQueryResponse

	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&data); err != nil {
		return &QueryResult{}, fmt.Errorf("failed to decode json response")
	}

	// // here is where the magic happens for flattening it down
	// flatResults := make([]interface{}, 0)
	// // this reaches into the Prometheus data from the HTTP api
	// // TODO: this really should be set to a schema that I think should be defined in Prometheus; this is just a short-cut for time crunch
	// results := data["data"].(map[string]interface{})["result"].([]interface{})
	// for _, result := range results {
	// 	resultMap := result.(map[string]interface{})
	// 	metric := resultMap["metric"].(map[string]interface{})
	// 	// extracts out the value part, but not the time
	// 	value := resultMap["value"].([]interface{})[1]

	// 	flatResult := map[string]interface{}{
	// 		"metric_name": metric["__name__"].(string),
	// 		"value": value,
	// 	}
	// 	flatResults = append(flatResults, flatResult)
	// }

	// switch res.StatusCode {
	// case 200:
	// 	// apparently this can fail on JSON unmarshalling...
	// 	_, err := io.ReadAll(res.Body)
	// 	if err != nil {
	// 		return &QueryResult{}, err
	// 	}
	// 	res.Body.Close()

	// 	pq.qCount += 1
	// 	return &QueryResult{}, nil
	// case 400:
	// 	fmt.Errorf("incorrect or missing parameters")
	// case 422:
	// 	fmt.Errorf("unprocessable entity")
	// case 503:
	// 	fmt.Errorf("server response error")
	// default:
	// 	fmt.Errorf("%s received unknown error %s", fullQuery, res.Status)
	// }

	tData, err := data.transformPromResults()
	if err != nil {
		return nil, err
	}

	var isSuccess bool
	switch data.Status {
	case "success":
		isSuccess = true
	default:
		isSuccess = false
	}

	return &QueryResult{success: isSuccess, metrics: tData}, nil
}

type QueryResult struct {
	success bool
	// another terrible thing to do
	metrics []ModifiedPrometheusQueryResult
}

// the following types are more compliant to Prometheus's HTTP API

type PrometheusQueryResponse struct {
	Status    string              `json:"status"`
	Data      PrometheusQueryData `json:"data"`
	ErrorType string              `json:"errorType,omitempty"`
	Error     string              `json:"error,omitempty"`
}

type PrometheusQueryData struct {
	ResultType string                  `json:"resultType"`
	Result     []PrometheusQueryResult `json:"result"`
}

type PrometheusQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// end of prometheus types

// this func is meant to extract a key of interest from PrometheusQueryResult
func (pqr *PrometheusQueryResult) getValue(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("empty key provided")
	}

	value, ok := pqr.Metric[key]
	if !ok {
		return "", fmt.Errorf("key not in the metric")
	}

	return value, nil
}

type ModifiedPrometheusQueryResult struct {
	Instance string
	Pqr      PrometheusQueryResult
}

func (pqr *PrometheusQueryResult) newModifiedPrometheusQueryResult() (*ModifiedPrometheusQueryResult, error) {
	instanceName, err := pqr.getValue("instance")
	if err != nil {
		return nil, err
	}

	return &ModifiedPrometheusQueryResult{
		instanceName,
		*pqr,
	}, nil
}

func (pqr *PrometheusQueryResponse) transformPromResults() ([]ModifiedPrometheusQueryResult, error) {
	modResults := make([]ModifiedPrometheusQueryResult, 0)

	// modify all the PromQueryResults
	for _, queryResponse := range pqr.Data.Result {
		temp, err := queryResponse.newModifiedPrometheusQueryResult()
		if err != nil {
			return nil, fmt.Errorf("error modifying the query response")
		}
		modResults = append(modResults, *temp)
	}

	return modResults, nil
}

// these next types are for our own modified rules

type Score struct {
	Value int32
}

type Info struct {
	// TODO: this name should probably change
	Metric map[string]map[string]string
}

type ModifiedRule struct {
	Name         string                          `json:"name"`
	Condition    string                          `json:"condition"`
	Status       string                          `json:"status"`
	QueryResults []ModifiedPrometheusQueryResult `json:"queryResults"`
	// the key here is the instance name like the original code
	Scores map[string]Score             `json:"scores"`
	Infos  map[string]map[string]string `json:"infos"`
}

type ModifiedRuleBuilder struct {
	Name         string
	Condition    string
	Status       string
	QueryResults []ModifiedPrometheusQueryResult
	// the key here is the instance name like the original code
	Scores map[string]Score
	Infos  map[string]map[string]string
}

// NewModifiedRuleBuilder creates a new ModifiedRuleBuilder instance.
func NewModifiedRuleBuilder() *ModifiedRuleBuilder {
	return &ModifiedRuleBuilder{Scores: make(map[string]Score), Infos: make(map[string]map[string]string)}
}

// WithName sets the Name field of the ModifiedRuleBuilder.
func (b *ModifiedRuleBuilder) WithName(name string) *ModifiedRuleBuilder {
	b.Name = name
	return b
}

// WithStatus sets the Status field of the ModifiedRuleBuilder.
func (b *ModifiedRuleBuilder) WithStatus(status string) *ModifiedRuleBuilder {
	b.Status = status
	return b
}

// WithQueryResults sets the QueryResults field of the ModifiedRuleBuilder.
func (b *ModifiedRuleBuilder) WithQueryResults(results []ModifiedPrometheusQueryResult) *ModifiedRuleBuilder {
	b.QueryResults = results
	return b
}

// WithScores sets the Scores field of the ModifiedRuleBuilder.
func (b *ModifiedRuleBuilder) WithScores(scores map[string]Score) *ModifiedRuleBuilder {
	b.Scores = scores
	return b
}

// WithInfos sets the Infos field of the ModifiedRuleBuilder.
func (b *ModifiedRuleBuilder) WithInfos(infos map[string]map[string]string) *ModifiedRuleBuilder {
	b.Infos = infos
	return b
}

func (b *ModifiedRuleBuilder) WithCondition(condition string) *ModifiedRuleBuilder {
	b.Condition = condition
	return b
}

// Build creates a ModifiedRule instance based on the builder's fields.
func (b *ModifiedRuleBuilder) Build() *ModifiedRule {
	return &ModifiedRule{
		Name:         b.Name,
		Condition:    b.Condition,
		Status:       b.Status,
		QueryResults: b.QueryResults,
		Scores:       b.Scores,
		Infos:        b.Infos,
	}
}

type FinalOutput struct {
	CommunityName map[string]Portfolio `json:"communities"`
}

type Portfolio struct {
	PortfolioName map[string]UseCase `json:"portfolios"`
}

type UseCase struct {
	UseCaseName map[string]AppStatus `json:"usecases"`
}

type AppStatus struct {
	AppStatusName map[string]map[string]string `json:"app_statuses"`
}

// NewFinalOutput initializes a new FinalOutput struct with memory for the nested maps.
func NewFinalOutput() *FinalOutput {
	return &FinalOutput{
		CommunityName: make(map[string]Portfolio),
	}
}

// NewPortfolio initializes a new Portfolio struct with memory for the nested maps.
func NewPortfolio() *Portfolio {
	return &Portfolio{
		PortfolioName: make(map[string]UseCase),
	}
}

// NewUseCase initializes a new UseCase struct with memory for the nested maps.
func NewUseCase() *UseCase {
	return &UseCase{
		UseCaseName: make(map[string]AppStatus),
	}
}

func NewAppStatus() *AppStatus {
	return &AppStatus{
		AppStatusName: make(map[string]map[string]string),
	}
}

func transformToFinalOutput(mr []ModifiedRule) (*FinalOutput, error) {
	// Note that this is not going to be optimal because the struct holding the data
	// for the Metric is in a list, so it is linear to items in list;
	// but this is more so an optimization

	finalOutput := NewFinalOutput()

	for _, rule := range mr {
		// first match the rules condition
		switch rule.Condition {
		// case "any":
		case "any", "all":
			// Just traverse the query results if the score is going to be higher? I'm not sure if this maps one-to-one from the Python script
			for _, queryResult := range rule.QueryResults {
				_, ok := queryResult.Pqr.Metric["publish"]
				if !ok {
					fmt.Println("No Publish Tag")
				} else if strings.ToLower(queryResult.Pqr.Metric["publish"]) != "true" {
					fmt.Println("Publish is not True")
				} else {
				    community := queryResult.Pqr.Metric["community_tenant"]
				    // this is for future reference, if blank, just return because community should not be blank and should not have a default value
				    if community == "" {
					    fmt.Println("Community value is null")
				    } else {
				        // same thing for portfolio as community
				        portfolio := queryResult.Pqr.Metric["portfolio"]
				        if portfolio == "" {
					        fmt.Println("Portfolio value is null")
				        } else {
				            useCase := queryResult.Pqr.Metric["use_case"]
				            if useCase == "" {
					            // 26Feb2024: discussed with team to provide default value for use case being empty, but app/service status should be displayed
					            useCase = "urls"
				            }
				            // all of this is to initialize the memory for final output, I am not certain
				            // how to do this a better way at the moment
				            _, ok = finalOutput.CommunityName[community]
				            if !ok {
					            finalOutput.CommunityName[community] = *NewPortfolio()
				            }
				            _, ok = finalOutput.CommunityName[community].PortfolioName[portfolio]
				            if !ok {
					            finalOutput.CommunityName[community].PortfolioName[portfolio] = *NewUseCase()
				            }
				            _, ok = finalOutput.CommunityName[community].PortfolioName[portfolio].UseCaseName[useCase]
				            if !ok {
					            finalOutput.CommunityName[community].PortfolioName[portfolio].UseCaseName[useCase] = *NewAppStatus()
				            }
				            stateData := make(map[string]string)
				            if strings.ToLower(queryResult.Pqr.Metric["unknown"]) == "true" {
					            stateData["status"] = "unknown"
					            stateData["display_name"] = queryResult.Pqr.Metric["display_name"]
				            } else {
					            stateData["status"] = rule.Status
					            stateData["display_name"] = queryResult.Pqr.Metric["display_name"]
				            }
				            finalOutput.CommunityName[community].PortfolioName[portfolio].UseCaseName[useCase].AppStatusName[queryResult.Instance] = stateData
				        }
				    }
			    }    
			}
		default:
			return nil, fmt.Errorf("unknown condition encountered")
		}
	}

	return finalOutput, nil
}
