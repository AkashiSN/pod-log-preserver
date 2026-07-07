package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
)

// TestStartMetricsFailsWhenPortInUse verifies that startMetrics binds
// synchronously and returns an error (fails startup fast) when METRICS_PORT is
// already occupied, instead of leaking a background goroutine that serves no
// endpoint.
func TestStartMetricsFailsWhenPortInUse(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port

	cfg := config.Config{MetricsPort: port}
	srv, err := startMetrics(cfg, &metrics.Metrics{})
	if err == nil {
		_ = srv.Close()
		t.Fatal("startMetrics should fail when the metrics port is already in use")
	}
}

// TestStartMetricsServesMetrics verifies startMetrics serves /metrics on the
// configured port after a successful synchronous bind.
func TestStartMetricsServesMetrics(t *testing.T) {
	// Grab a free port, release it, then hand it to startMetrics.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	cfg := config.Config{MetricsPort: port}
	srv, err := startMetrics(cfg, &metrics.Metrics{})
	if err != nil {
		t.Fatalf("startMetrics should succeed on a free port: %v", err)
	}
	defer func() { _ = srv.Close() }()

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", resp.StatusCode)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("read /metrics body: %v", err)
	}
}
