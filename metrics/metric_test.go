package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetric(t *testing.T) {
	metric := NewMetric()
	if metric.RequestsTotal == nil {
		t.Fatalf("RequestsTotal is nil")
	}

	metric.RequestsTotal.WithLabelValues("200").Inc()

	if got := testutil.ToFloat64(metric.RequestsTotal.WithLabelValues("200")); got != 1 {
		t.Errorf("counter for code 200 = %v, want 1", got)
	}
}
