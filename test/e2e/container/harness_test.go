//go:build e2e

// Package container is the e2e container harness for pod-log-preserver. It runs
// the shipped image (and, where a scenario needs it, a real fluent-bit) over a
// shared bind-mounted work dir, so inotify, hardlink inode sharing, and the real
// fluent-bit WAL tail DB read path are all exercised end to end. Build-tagged
// `e2e`: never compiled by `make test`.
package container

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// plpImage is the image under test. CI builds and tags it; locally, build it
// with `docker build -t pod-log-preserver:e2e .` first (see Makefile e2e-container).
func plpImage() string {
	if v := os.Getenv("E2E_IMAGE"); v != "" {
		return v
	}
	return "pod-log-preserver:e2e"
}

const fluentBitImage = "fluent/fluent-bit:3.1.9"

// newWorkDir creates a host temp dir with the pods/pods-preserved/flb subtree,
// all on one filesystem so hardlinks between pods and pods-preserved are valid.
//
// The plp/fluent-bit containers run as root (see Dockerfile), so files and
// directories they create under this bind mount are root-owned on the host,
// with no write permission for anyone else. chmod/unlink of another user's
// files needs ownership (or CAP_FOWNER), which the unprivileged host test
// process lacks, so t.TempDir()'s own removal cleanup cannot recurse into
// them. Reclaim ownership via a throwaway root container instead. Cleanups
// run LIFO, and this is registered right after t.TempDir()'s own cleanup and
// before any container Terminate cleanups a caller adds afterwards, so at
// teardown it runs after the containers under test stop (no process still
// writing) but before t.TempDir() removes the tree.
func newWorkDir(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	t.Cleanup(func() {
		reclaimOwnership(t, work)
	})
	for _, d := range []string{"pods", "pods-preserved", "flb"} {
		if err := os.MkdirAll(filepath.Join(work, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return work
}

// reclaimOwnership chowns everything under dir back to the current (host
// test process) user, undoing root ownership left behind by containers that
// wrote into the bind mount. It runs a tiny busybox container as root since
// the host process itself has no permission to chown files it doesn't own.
func reclaimOwnership(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("docker", "run", "--rm",
		"-v", dir+":/work",
		"busybox:1.37",
		"chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/work",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("reclaim ownership of %s: %v: %s", dir, err, out)
	}
}

type plpContainer struct {
	c          testcontainers.Container
	MetricsURL string
}

// startPLP runs the plp image with /work bind-mounted from the host work dir and
// fast cleanup intervals. extraEnv overrides/augments the defaults (e.g. a
// scenario that disables DB-aware cleanup via an empty-glob PRESERVED_LOG_DB_GLOB).
func startPLP(t *testing.T, work string, extraEnv map[string]string) *plpContainer {
	t.Helper()
	ctx := context.Background()
	env := map[string]string{
		"WATCH_DIR":              "/work/pods",
		"PRESERVE_DIR":           "/work/pods-preserved",
		"PRESERVED_LOG_DB_GLOB":  "/work/flb/flb_kube*.db",
		"CLEANUP_INTERVAL_SEC":   "1",
		"CLEANUP_MAX_AGE_MIN":    "1",
		"CLEANUP_GZ_MAX_AGE_MIN": "1",
		"RESYNC_INTERVAL_SEC":    "2",
		"LOG_LEVEL":              "debug",
		"METRICS_PORT":           "9113",
	}
	for k, v := range extraEnv {
		env[k] = v
	}
	req := testcontainers.ContainerRequest{
		Image:        plpImage(),
		Env:          env,
		ExposedPorts: []string{"9113/tcp"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = []string{work + ":/work"}
		},
		WaitingFor: wait.ForListeningPort("9113/tcp").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Fatalf("start plp: %v", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("plp host: %v", err)
	}
	port, err := c.MappedPort(ctx, "9113/tcp")
	if err != nil {
		t.Fatalf("plp port: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	return &plpContainer{c: c, MetricsURL: fmt.Sprintf("http://%s:%s/metrics", host, port.Port())}
}

// startFluentBit runs fluent-bit tailing /work/pods-preserved into /work/flb.
func startFluentBit(t *testing.T, work string) {
	t.Helper()
	ctx := context.Background()
	conf, err := filepath.Abs("fluent-bit.conf")
	if err != nil {
		t.Fatalf("abs conf: %v", err)
	}
	req := testcontainers.ContainerRequest{
		Image: fluentBitImage,
		Cmd:   []string{"/fluent-bit/bin/fluent-bit", "-c", "/work/fluent-bit.conf"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = []string{work + ":/work", conf + ":/work/fluent-bit.conf:ro"}
		},
		WaitingFor: wait.ForLog("[engine] started").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Fatalf("start fluent-bit: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
}

var metricRe = regexp.MustCompile(`(?m)^([a-z_]+)\s+([0-9.e+-]+)$`)

// readCounter scrapes the plp /metrics endpoint and returns the value of a
// no-label counter/gauge by name, or 0 if absent.
func readCounter(t *testing.T, metricsURL, name string) float64 {
	t.Helper()
	resp, err := http.Get(metricsURL)
	if err != nil {
		t.Fatalf("scrape %s: %v", metricsURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, m := range metricRe.FindAllStringSubmatch(string(body), -1) {
		if m[1] == name {
			v, _ := strconv.ParseFloat(m[2], 64)
			return v
		}
	}
	return 0
}

// sameInode reports whether two paths are hardlinks to the same inode.
func sameInode(t *testing.T, a, b string) bool {
	t.Helper()
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

func nlink(t *testing.T, path string) uint64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return uint64(fi.Sys().(*syscall.Stat_t).Nlink)
}

// preservedDir returns <work>/pods-preserved/<ns>_<pod>_<uid>/<container>.
func preservedDir(work, key, container string) string {
	return filepath.Join(work, "pods-preserved", key, container)
}

func podLogDir(work, key, container string) string {
	return filepath.Join(work, "pods", key, container)
}

// findPreserved returns the single preserved entry under dir, waiting for it.
func findPreserved(t *testing.T, dir string) string {
	t.Helper()
	var found string
	waitFor(t, 15*time.Second, "preserved file appears", func() bool {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) == 0 {
			return false
		}
		found = filepath.Join(dir, entries[0].Name())
		return true
	})
	return found
}

func waitFor(t *testing.T, timeout time.Duration, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", desc)
}
