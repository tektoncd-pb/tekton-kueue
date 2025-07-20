package cel

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// celEvaluationsTotal tracks the total number of CEL evaluations
	celEvaluationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tekton_kueue_cel_evaluations_total",
			Help: "Total number of CEL evaluations",
		},
		[]string{"result"}, // result can be "success" or "failure"
	)
)

func init() {
	// Register the metrics with controller-runtime's global registry
	metrics.Registry.MustRegister(celEvaluationsTotal)
}

// RecordEvaluationFailure increments the counter for CEL evaluation failures
func RecordEvaluationFailure() {
	celEvaluationsTotal.WithLabelValues("failure").Inc()
}

// RecordEvaluationSuccess increments the counter for successful CEL evaluations
func RecordEvaluationSuccess() {
	celEvaluationsTotal.WithLabelValues("success").Inc()
}
