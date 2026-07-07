package validate

import (
	"fmt"
	"os"
	"path/filepath"
)

// probeName is the temporary file used by the startup hardlink gate. It is
// created under the watch directory, linked into the preserve directory, and
// removed again — both copies are cleaned up before ValidateHardlink returns.
const probeName = ".pod-log-preserver-hardlink-probe"

// ValidateHardlink is the fail-fast startup gate for the preservation invariant
// (spec §4.1 / §5.2): the watch and preserve directories must share a
// filesystem, or every os.Link would fail at runtime with logs already at risk.
// It creates the preserve directory, then proves a hardlink from the watch
// filesystem into the preserve directory succeeds using a throwaway probe.
//
// This is the minimal same-filesystem check; issue #6 replaces it with a test
// against the pod's own container log.
func ValidateHardlink(watchDir, preserveDir string) error {
	if err := os.MkdirAll(preserveDir, 0o755); err != nil {
		return fmt.Errorf("create preserve dir %s: %w", preserveDir, err)
	}

	src := filepath.Join(watchDir, probeName)
	dst := filepath.Join(preserveDir, probeName)
	// Clear any probe left by a crashed prior run before re-probing.
	_ = os.Remove(src)
	_ = os.Remove(dst)

	if err := os.WriteFile(src, []byte("probe"), 0o644); err != nil {
		return fmt.Errorf("write hardlink probe in watch dir %s: %w", watchDir, err)
	}
	defer func() { _ = os.Remove(src) }()

	if err := os.Link(src, dst); err != nil {
		return fmt.Errorf("hardlink probe %s -> %s failed (are the watch and preserve dirs on the same filesystem?): %w", src, dst, err)
	}
	defer func() { _ = os.Remove(dst) }()

	return nil
}
