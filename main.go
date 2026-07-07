package main

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed VERSION
var versionRaw string

// version is the release version, embedded at build time from the VERSION
// file. It must match the git tag a release is cut from (see the release CI).
var version = strings.TrimSpace(versionRaw)

// main is a bootstrap stub: it prints the embedded version and exits. The
// DaemonSet runtime (config, preservation, cleanup, metrics) is wired up in
// later issues of the v0.5 milestone.
func main() {
	fmt.Printf("pod-log-preserver %s\n", version)
}
