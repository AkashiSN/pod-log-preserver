package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// validateProbeName is the throwaway source file the startup hardlink probe
// creates in the preserve directory; validateProbeLink is the hardlink made to
// it. Both live entirely under the preserve directory we own — nothing is ever
// written into the kubelet-owned watch tree.
const (
	validateProbeName = ".pod-log-preserver-validate"
	validateProbeLink = ".pod-log-preserver-validate.link"
)

// linkFile is os.Link behind a seam so a hardlink-unsupported filesystem can be
// exercised deterministically in tests.
var linkFile = os.Link

// statDev returns the device number (st_dev) of the filesystem holding path,
// behind a seam so a cross-filesystem layout can be exercised in tests without a
// second real filesystem.
var statDev = func(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return st.Dev, nil
}

// ValidateFilesystem is the fail-fast startup gate for the preservation
// invariant (spec §4.1 / §5.2): the watch and preserve directories must share a
// hardlink-capable filesystem, or every runtime os.Link would fail with logs
// already at risk. It creates the preserve directory, then proves the invariant
// deterministically — with no dependency on pod identity or any pre-existing log:
//
//  1. Same filesystem: stat both directories and compare st_dev. A mismatch is
//     the cross-filesystem misconfiguration and is fatal.
//  2. Hardlink support: create a throwaway file in the preserve directory and
//     hardlink it there. Success proves the filesystem supports hardlinks;
//     failure (e.g. EOPNOTSUPP) is fatal. Because (1) already proved the watch
//     directory is on the same filesystem, a preserve-local hardlink working
//     implies a watch->preserve hardlink works.
//
// It returns an error only when the preserve dir cannot be created, a directory
// cannot be stat'd, the two directories are on different filesystems, or the
// hardlink probe fails. The gate never skips.
func ValidateFilesystem(watchDir, preserveDir string) error {
	if err := os.MkdirAll(preserveDir, 0o755); err != nil {
		return fmt.Errorf("create preserve dir %s: %w", preserveDir, err)
	}

	watchDev, err := statDev(watchDir)
	if err != nil {
		return fmt.Errorf("stat watch dir %s: %w", watchDir, err)
	}
	preserveDev, err := statDev(preserveDir)
	if err != nil {
		return fmt.Errorf("stat preserve dir %s: %w", preserveDir, err)
	}
	if watchDev != preserveDev {
		return fmt.Errorf("watch dir %s and preserve dir %s are on different filesystems (st_dev %d != %d); hardlink preservation requires them to share one filesystem", watchDir, preserveDir, watchDev, preserveDev)
	}

	if err := hardlinkProbe(preserveDir); err != nil {
		return fmt.Errorf("hardlink probe in preserve dir %s failed (does the filesystem support hardlinks?): %w", preserveDir, err)
	}
	return nil
}

// hardlinkProbe creates a throwaway file in preserveDir and hardlinks it there,
// cleaning up both regardless of outcome. It returns an error if the filesystem
// rejects the hardlink.
func hardlinkProbe(preserveDir string) error {
	src := filepath.Join(preserveDir, validateProbeName)
	dst := filepath.Join(preserveDir, validateProbeLink)
	// Clear any files left by a crashed prior run before probing.
	_ = os.Remove(src)
	_ = os.Remove(dst)
	defer func() {
		_ = os.Remove(src)
		_ = os.Remove(dst)
	}()

	if err := os.WriteFile(src, nil, 0o600); err != nil {
		return err
	}
	return linkFile(src, dst)
}
