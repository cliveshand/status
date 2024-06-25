package main

import (
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	buildTracer = otel.Tracer("build")
	buildMeter  = otel.Meter("build")
	buildTime   metric.Int64Counter
)

func init() {
	var err error
	buildTime, err = buildMeter.Int64Counter("build.time",
		metric.WithDescription("The seconds for performing hugo build"),
		metric.WithUnit("{ti}"))
	if err != nil {
		panic(err)
	}
}

func build(w http.ResponseWriter, r *http.Request) {

	ctx, span := buildTracer.Start(r.Context(), "build")
	defer span.End()

	startTime := time.Now()

	cmd := exec.Command("hugo", "-v")
	result := "success"
	err := cmd.Run()
	if err != nil {
		result = "failed to build"
	}
	duration := time.Since(startTime)

	// Add the custom attribute to the span and counter.
	buildValueAttr := attribute.Int("build.time", int(duration.Seconds()))
	span.SetAttributes(buildValueAttr)
	buildTime.Add(ctx, 1, metric.WithAttributes(buildValueAttr))

	// check the result
	if result != "success" {
		log.Println("hugo build failed")
		return
	}

	// include the http GET to the Deploy section
	_, err = http.Get("http://localhost:8080/deploy")
	if err != nil {
		log.Printf("failed to call deploy with error: %v", err)
	}

	if _, err := io.WriteString(w, result); err != nil {
		log.Printf("write failed: %v\n", err)
	}
}
