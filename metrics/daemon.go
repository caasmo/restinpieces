package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// readHeaderTimeout bounds reading a request header on the metrics listener.
const readHeaderTimeout = 5 * time.Second

// Daemon serves the metrics over an internal listener: loopback or a private
// address. The public application server never serves metrics.
type Daemon struct {
	configProvider *config.Provider
	logger         *slog.Logger
	registry       *prometheus.Registry

	listener     net.Listener
	server       *http.Server
	shutdownDone chan struct{}
}

// NewDaemon creates the metrics daemon serving the given registry.
func NewDaemon(configProvider *config.Provider, logger *slog.Logger, registry *prometheus.Registry) *Daemon {
	return &Daemon{
		configProvider: configProvider,
		logger:         logger,
		registry:       registry,
		shutdownDone:   make(chan struct{}),
	}
}

// Name returns the name of the daemon for logging/identification.
func (d *Daemon) Name() string {
	return "MetricsDaemon"
}

// Start binds the internal listener and serves the metrics in a background
// goroutine. A bind error is a startup failure, so the server aborts startup
// instead of running without metrics.
func (d *Daemon) Start() error {
	addr := d.configProvider.Get().Metrics.ListenAddr

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("metrics daemon: failed to listen on %s: %w", addr, err)
	}

	d.listener = listener
	d.server = &http.Server{
		Handler:           promhttp.HandlerFor(d.registry, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	d.logger.Info("metrics daemon: starting", "listen_addr", listener.Addr().String())

	go func() {
		defer close(d.shutdownDone)
		err := d.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.logger.Error("metrics daemon: server error", "error", err)
		}
	}()

	return nil
}

// Stop shuts the metrics server down and waits for the serving goroutine to
// finish or for the context to expire, whichever comes first.
func (d *Daemon) Stop(ctx context.Context) error {
	if d.server == nil {
		return nil
	}

	d.logger.Info("metrics daemon: stopping")

	err := d.server.Shutdown(ctx)
	if err != nil {
		return err
	}

	select {
	case <-d.shutdownDone:
		d.logger.Info("metrics daemon: stopped gracefully")
		return nil
	case <-ctx.Done():
		d.logger.Info("metrics daemon: shutdown timed out")
		return ctx.Err()
	}
}
