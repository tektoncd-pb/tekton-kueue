package cel

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// celEvaluationFailuresTotal tracks the total number of CEL evaluation failures
	celEvaluationFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_cel_evaluation_failures_total",
			Help: "Total number of CEL evaluation failures",
		},
		[]string{},
	)
)

func init() {
	// Register the metric with controller-runtime's global registry
	metrics.Registry.MustRegister(celEvaluationFailuresTotal)
}

// RecordEvaluationFailure increments the counter for CEL evaluation failures
func RecordEvaluationFailure() {
	celEvaluationFailuresTotal.WithLabelValues().Inc()
}
