package metrics

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func TestDaemon(t *testing.T) {
	provider := config.NewProvider(&config.Config{
		Metrics: config.Metrics{
			Enabled:    true,
			Activated:  true,
			ListenAddr: "127.0.0.1:0",
		},
	})

	metric := NewMetric()
	registry := prometheus.NewRegistry()
	registry.MustRegister(metric.RequestsTotal)
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	daemon := NewDaemon(provider, slog.New(slog.NewTextHandler(io.Discard, nil)), registry)
	if err := daemon.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	response, err := http.Get("http://" + daemon.listener.Addr().String() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the response failed: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics returned status %d", response.StatusCode)
	}
	if !strings.Contains(string(body), "go_goroutines") {
		t.Errorf("the metrics body does not contain go_goroutines")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := daemon.Stop(ctx); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
}
