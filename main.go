package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var (
	appConfig Config
)

func main() {
	if err := run(); err != nil {
		log.Println(err)
	}
}

func run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// // Set up OpenTelemetry.
	// serviceName := "dice"
	// serviceVersion := "0.1.0"
	// otelShutdown, err := setupOTelSDK(ctx, serviceName, serviceVersion)
	// if err != nil {
	// 	return
	// }
	// // Handle shutdown properly so nothing leaks.
	// defer func() {
	// 	err = errors.Join(err, otelShutdown(context.Background()))
	// }()

	// TODO: Parse configuation file
	config, err := LoadConfig("./config.json")
	if err != nil {
		log.Printf("error loading config: %v", err)
	}
	// this is a god awful hack on the config
	appConfig = *config

	fmt.Println("targeting endpoint:", appConfig.Defaults.Host)

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":8080",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 0,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	// Ticker setup
	ticker := time.NewTicker(time.Duration(appConfig.Schedule) * time.Minute)
	go func() {
		for t := range ticker.C {
			_, err := http.Get("http://localhost:8080/querier")
			if err != nil {
				log.Printf("%v error calling internal http %v", t, err)
			}

		}
	}()

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// TODO: This needs to be declared as a type, but that will follow in a more comprehensive
	// refactor.
	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		mux.Handle(pattern, handler)
	}

	// Register handlers.
	handleFunc("/build", build)
	handleFunc("/querier", querier)
	handleFunc("/deploy", deploy)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}
