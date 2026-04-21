package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/api"
	"github.com/Trin9/SynapseFlow/backend/pkg/logger"
)

func main() {
	// Initialize structured logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Init(logLevel)
	defer logger.Sync()

	// Server address
	addr := os.Getenv("SYNAPSE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════╗")
	fmt.Println("  ║     Synapse Workflow Engine v0.1.0    ║")
	fmt.Println("  ╚═══════════════════════════════════════╝")
	fmt.Println()

	server := api.NewServer()
	go func() {
		if err := server.Run(addr); err != nil {
			logger.L().Fatalw("Server failed to start", "error", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		logger.L().Infow("Server shutdown error", "error", err)
	}
}
