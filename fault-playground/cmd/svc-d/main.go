package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fault-playground/internal/chain"
	"fault-playground/internal/faults"
	"fault-playground/internal/observability"
)

const (
	serviceName = chain.ServiceD
	port        = "8080"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := observability.NewLogger()
	metrics := observability.NewMetrics()
	injector := faults.NewInjector()

	mux := http.NewServeMux()
	observability.RegisterPProf(mux)

	mux.HandleFunc("/call", observability.PanicRecoveryMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req chain.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		fields := map[string]interface{}{
			"trace_id": req.TraceID,
			"user_id":  req.UserID,
		}
		logger.Info(r.Context(), fmt.Sprintf("%s received request", serviceName), fields)

		// PHASE 1: distributed_nil_pointer logic
		if injector.ShouldInject(r.Context(), string(serviceName)) {
			if req.UserID == "bad-user" {
				logger.Info(r.Context(), "DEBUG: Stripping profile for bad-user", fields)
				// This is where the fault injection happens: Profile is set to nil
				req.Profile = nil
			}
		}

		// PHASE 2: cpu_hotloop and kafka_consumer_stuck logic
		if injector.ShouldInject(r.Context(), string(serviceName)) {
			sswitch injector.scenario.Fault {
			case faults.FaultCPUHotloop:
				logger.Info(r.Context(), "Applying CPU Hotloop fault", fields)
				faults.ApplyCPUHotloop(r.Context())
			case faults.FaultKafkaStuck:
				logger.Info(r.Context(), "Applying Kafka Stuck fault", fields)
				faults.ApplyKafkaConsumerStuck(r.Context())
			}
		}

		// Simulate some work
		time.Sleep(20 * time.Millisecond)

		resp := chain.Response{
			TraceID: req.TraceID,
			Message: fmt.Sprintf("Response from %s", serviceName),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		
		metrics.IncCounter("service_requests_total", map[string]string{"service": string(serviceName)})
	}))

	mux.HandleFunc("/admin/scenario", func(w http.ResponseWriter, r *http.Request) {
		var config faults.ScenarioConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		injector.SetScenario(config)
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		logger.Info(ctx, fmt.Sprintf("Starting %s on :%s", serviceName, port), nil)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Server failed", map[string]interface{}{"error": err.Error()})
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info(ctx, "Shutting down...", nil)
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
