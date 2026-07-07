package validate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// validateLinkName is the throwaway link created in the preserve directory by
// the startup hardlink test. Unlike the earlier probe, no file is written into
// the kubelet-owned watch tree: the test hardlinks the pod's own existing
// container log, then removes the link.
const validateLinkName = ".pod-log-preserver-validate"

// ValidationResult reports what the startup filesystem validation did so the
// caller can log it. A zero-value result (Skipped=false, empty fields) with a
// nil error means the hardlink test ran and passed.
type ValidationResult struct {
	Skipped   bool   // true when the pod's own container log could not be located
	Reason    string // why it was skipped (empty unless Skipped)
	TestedLog string // the own log that was hardlink-tested (empty when skipped)
}

// ValidateFilesystem is the fail-fast startup gate for the preservation
// invariant (spec §4.1 / §5.2): the watch and preserve directories must share a
// filesystem, or every os.Link would fail at runtime with logs already at risk.
// It creates the preserve directory, locates the pod's own container log under
// the watch tree (via the downward-API identity), and proves a hardlink of that
// real log into the preserve directory succeeds.
//
// When the pod's own log cannot be located (identity not injected, or no log
// yet), it warns and skips rather than failing — there is nothing safe to test
// against. It only returns an error when the preserve dir cannot be created or
// the hardlink itself fails (the same-filesystem invariant is broken).
func ValidateFilesystem(watchDir, preserveDir, podNamespace, podName, podUID string) (ValidationResult, error) {
	if err := os.MkdirAll(preserveDir, 0o755); err != nil {
		return ValidationResult{}, fmt.Errorf("create preserve dir %s: %w", preserveDir, err)
	}

	ownLog, reason := findOwnContainerLog(watchDir, podNamespace, podName, podUID)
	if ownLog == "" {
		return ValidationResult{Skipped: true, Reason: reason}, nil
	}

	dst := filepath.Join(preserveDir, validateLinkName)
	// Clear any link left by a crashed prior run before re-testing.
	_ = os.Remove(dst)
	if err := os.Link(ownLog, dst); err != nil {
		return ValidationResult{}, fmt.Errorf("hardlink test %s -> %s failed (are the watch and preserve dirs on the same filesystem?): %w", ownLog, dst, err)
	}
	_ = os.Remove(dst)

	return ValidationResult{TestedLog: ownLog}, nil
}

// findOwnContainerLog returns a regular `*.log` file belonging to this pod,
// found under <watchDir>/<ns>_<name>_<uid>/. It returns an empty path and a
// human-readable reason when the pod identity is not fully injected or no own
// container log exists yet.
func findOwnContainerLog(watchDir, podNamespace, podName, podUID string) (path, reason string) {
	if podNamespace == "" || podName == "" || podUID == "" {
		return "", "pod identity not injected via the downward API (POD_NAMESPACE/POD_NAME/POD_UID)"
	}
	podDir := filepath.Join(watchDir, podNamespace+"_"+podName+"_"+podUID)

	var found string
	err := filepath.WalkDir(podDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && filepath.Ext(p) == ".log" {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if err != nil || found == "" {
		return "", fmt.Sprintf("no own container log found under %s", podDir)
	}
	return found, ""
}
