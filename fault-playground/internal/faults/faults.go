package faults

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// FaultType enumerates available failure modes.
type FaultType string

const (
	FaultNone               FaultType = "none"
	FaultDistributedNil     FaultType = "distributed_nil_pointer"
	FaultRecoveredPanicHigh FaultType = "panic_recovered_but_error_rate_high"
	FaultCPUHotloop         FaultType = "cpu_hotloop"
	FaultKafkaStuck         FaultType = "kafka_consumer_stuck"
)

// ScenarioConfig captures which fault is active and any knobs.
type ScenarioConfig struct {
	Name       string
	Fault      FaultType
	Parameters map[string]string
}

// Injector decides whether to inject a fault for a request.
type Injector interface {
	ShouldInject(ctx context.Context, service string) bool
	SetScenario(config ScenarioConfig)
}

type defaultInjector struct {
	mu       sync.RWMutex
	scenario ScenarioConfig
}

func NewInjector() Injector {
	return &defaultInjector{}
}

func (i *defaultInjector) SetScenario(config ScenarioConfig) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.scenario = config
}

func (i *defaultInjector) ShouldInject(ctx context.Context, service string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.scenario.Fault == FaultNone {
		return false
	}
	// Check if the current scenario applies to this service
	return i.scenario.Fault == FaultDistributedNil || i.scenario.Fault == FaultRecoveredPanicHigh || i.scenario.Fault == FaultCPUHotloop || i.scenario.Fault == FaultKafkaStuck || (i.scenario.Fault != "" && i.scenario.Name == service)
}

// ApplyDistributedNilPointer modifies the payload to simulate the root cause.
func ApplyDistributedNilPointer(ctx context.Context, payload any) error {
	typedPayload, ok := payload.(*chain.Request)
	if !ok {
		return fmt.Errorf("invalid payload type for ApplyDistributedNilPointer: expected *chain.Request, got %T", payload)
	}

	// Simulate the root cause: svc-d strips the profile for "bad-user"
	if typedPayload.UserID == "bad-user" {
		typedPayload.Profile = nil
	}
	return nil
}

// ApplyRecoveredPanicHigh simulates a panic that is recovered but still impacts errors.
func ApplyRecoveredPanicHigh(ctx context.Context, payload any) error {
	// In svc-c, this would be triggered if injector.ShouldInject is true
	// and the fault is FaultRecoveredPanicHigh. The panic itself is handled
	// by the middleware, but this function could be used to inject specific
	// error conditions or metrics.
	return nil
}

// ApplyCPUHotloop triggers a CPU hot loop behavior.
func ApplyCPUHotloop(ctx context.Context) error {
	go func() {
		for {
			// Burn CPU
			_ = fmt.Sprintf("%s", "burning CPU") // Simulate CPU work
		}
	}()
	return nil
}

// ApplyKafkaConsumerStuck simulates a stuck consumer with growing lag.
func ApplyKafkaConsumerStuck(ctx context.Context) error {
	// This fault would ideally involve interacting with Kafka or simulating lag.
	// For now, it's a placeholder that logs the intent.
	fmt.Println("Simulating Kafka consumer stuck...")
	return nil
}
