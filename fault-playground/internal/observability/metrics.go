package observability

// Metrics captures counters and gauges for the playground services.
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

// NewMetrics returns a metrics implementation.
func NewMetrics() Metrics {
	return nil
}
