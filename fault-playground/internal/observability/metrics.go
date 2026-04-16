package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics captures counters and gauges for the playground services.
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

type defaultMetrics struct {
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
}

// NewMetrics returns a metrics implementation.
func NewMetrics() Metrics {
	return &defaultMetrics{
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
	}
}

func (m *defaultMetrics) IncCounter(name string, labels map[string]string) {
	counter, ok := m.counters[name]
	if !ok {
		counter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: name,
				Help: name,
			},
			getKeys(labels),
		)
		m.counters[name] = counter
	}
	counter.With(labels).Inc()
}

func (m *defaultMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	histogram, ok := m.histograms[name]
	if !ok {
		histogram = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: name,
				Help: name,
			},
			getKeys(labels),
		)
		m.histograms[name] = histogram
	}
	histogram.With(labels).Observe(value)
}

func (m *defaultMetrics) SetGauge(name string, value float64, labels map[string]string) {
	gauge, ok := m.gauges[name]
	if !ok {
		gauge = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
				Help: name,
			},
			getKeys(labels),
		)
		m.gauges[name] = gauge
	}
	gauge.With(labels).Set(value)
}

func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
