package faults

import "context"

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
	ShouldInject(ctx context.Context, service string, scenario ScenarioConfig) bool
}

// ApplyDistributedNilPointer modifies the payload to simulate the root cause.
func ApplyDistributedNilPointer(ctx context.Context, payload any) error {
	_ = ctx
	_ = payload
	return nil
}

// ApplyRecoveredPanicHigh simulates a panic that is recovered but still impacts errors.
func ApplyRecoveredPanicHigh(ctx context.Context, payload any) error {
	_ = ctx
	_ = payload
	return nil
}

// ApplyCPUHotloop triggers a CPU hot loop behavior.
func ApplyCPUHotloop(ctx context.Context) error {
	_ = ctx
	return nil
}

// ApplyKafkaConsumerStuck simulates a stuck consumer with growing lag.
func ApplyKafkaConsumerStuck(ctx context.Context) error {
	_ = ctx
	return nil
}
