package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	serviceName = chain.ServiceGateway
	port        = "8080"
	svcAURL     = "http://svc-a:8080/call"
	svcBURL     = "http://svc-b:8080/call"
	svcCURL     = "http://svc-c:8080/call"
	svcDURL     = "http://svc-d:8080/call"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := observability.NewLogger()
	metrics := observability.NewMetrics()

	logger.Info(ctx, fmt.Sprintf("Starting %s service on :%s", serviceName, port), nil)

	// Initialize the HTTP server
	mux := http.NewServeMux()
	observability.RegisterPProf(mux)

	client := &http.Client{Timeout: 5 * time.Second}
	svcChain := &httpClientChain{client: client, logger: logger, metrics: metrics}
	runner := &chain.Runner{Chain: svcChain}

	// Wrap the handler with the panic recovery middleware
	mux.HandleFunc("/demo/panic", observability.PanicRecoveryMiddleware(func(w http.ResponseWriter, r *http.Request) {
		scenarioName := r.URL.Query().Get("scenario")
		userID := r.URL.Query().Get("id")

		if scenarioName == "" {
			scenarioName = string(faults.FaultDistributedNil)
		}

		// For demonstration, we'll hardcode one of the Phase 1 plans.
		// In a real scenario, this would be dynamic based on the scenarioName.
		plans := chain.BuildPhase1Plans()
		var selectedPlan chain.FlowPlan
		for _, p := range plans {
			if p.Name == "A->C->D->B->C" {
				selectedPlan = p
				break
			}
		}

		if selectedPlan.Name == "" {
			http.Error(w, "Scenario plan not found", http.StatusBadRequest)
			return
		}

		req := chain.Request{
			TraceID: fmt.Sprintf("trace-%d", time.Now().UnixNano()),
			UserID:  userID,
			Profile: &chain.Profile{
				ID:    userID,
				Email: fmt.Sprintf("%s@example.com", userID),
			},
		}

		resp, err := runner.Run(r.Context(), selectedPlan, req)
		if err != nil {
			logger.Error(r.Context(), "Error running flow", map[string]interface{}{"error": err.Error(), "trace_id": req.TraceID}))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Flow completed: %s", resp.Message)))
		logger.Info(r.Context(), "Flow completed", map[string]interface{}{"trace_id": req.TraceID, "message": resp.Message})
	}))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Server failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	<-c
	logger.Info(ctx, "Shutting down server...", nil)

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Server shutdown failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	logger.Info(ctx, "Server gracefully stopped", nil)
}

// httpClientChain implements the Chain interface for HTTP calls.
type httpClientChain struct {
	client  *http.Client
	logger  observability.Logger
	metrics observability.Metrics
}

func (c *httpClientChain) CallA(ctx context.Context, req chain.Request) (chain.Response, error) {
	return c.doRequest(ctx, svcAURL, req, chain.ServiceA)
}

func (c *httpClientChain) CallB(ctx context.Context, req chain.Request) (chain.Response, error) {
	return c.doRequest(ctx, svcBURL, req, chain.ServiceB)
}

func (c *httpClientChain) CallC(ctx context.Context, req chain.Request) (chain.Response, error) {
	return c.doRequest(ctx, svcCURL, req, chain.ServiceC)
}

func (c *httpClientChain) CallD(ctx context.Context, req chain.Request) (chain.Response, error) {
	return c.doRequest(ctx, svcDURL, req, chain.ServiceD)
}

func (c *httpClientChain) doRequest(ctx context.Context, url string, req chain.Request, svcName chain.ServiceName) (chain.Response, error) {
	var resp chain.Response

	fields := map[string]interface{}{"trace_id": req.TraceID, "service": svcName}
	c.logger.Info(ctx, fmt.Sprintf("Calling %s", svcName), fields)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error(ctx, "Failed to marshal request", fields)
		return resp, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error(ctx, "Failed to create HTTP request", fields)
		return resp, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error(ctx, "HTTP request failed", map[string]interface{}{"error": err.Error(), "trace_id": req.TraceID, "service": svcName})
		c.metrics.IncCounter("http_client_errors_total", map[string]string{"service": string(svcName)})
		return resp, fmt.Errorf("http request to %s failed: %w", svcName, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		c.logger.Error(ctx, "HTTP request returned non-OK status", map[string]interface{}{"status": httpResp.Status, "body": string(body), "trace_id": req.TraceID, "service": svcName})
		c.metrics.IncCounter("http_client_errors_total", map[string]string{"service": string(svcName), "status": httpResp.Status})
		return resp, fmt.Errorf("http request to %s returned status %s: %s", svcName, httpResp.Status, string(body))
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		c.logger.Error(ctx, "Failed to read response body", fields)
		return resp, fmt.Errorf("read response body: %w", err)
	}

	err = json.Unmarshal(body, &resp)
	if err != nil {
		c.logger.Error(ctx, "Failed to unmarshal response body", fields)
		return resp, fmt.Errorf("unmarshal response body: %w", err)
	}
	c.logger.Info(ctx, fmt.Sprintf("Received response from %s", svcName), map[string]interface{}{"trace_id": resp.TraceID, "message": resp.Message, "service": svcName})
	return resp, nil
}
