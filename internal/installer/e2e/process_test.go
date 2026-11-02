package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstalledProcessBoundaryPrerequisite(t *testing.T) {
	requireHarness(t)
	release := buildFixtureRelease(t)
	repo, env := scratchRepo(t, release)
	profile, err := os.ReadFile(filepath.Join(repo, ".pyxis/security/bwrap.apparmor"))
	if err != nil || !bytes.Equal(profile, readSourceFile(t, ".pyxis/security/bwrap.apparmor")) {
		t.Fatalf("installed AppArmor policy differs from shipped bytes: %v", err)
	}
	if runtime.GOOS != "linux" {
		return
	}
	// Remove only the fixture's bwrap PATH pointer, retaining real git and the
	// installed executable. Doctor must diagnose the prerequisite before a user
	// discovers it by spending an implementation attempt.
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			for _, dir := range filepath.SplitList(value) {
				if err := os.Remove(filepath.Join(dir, "bwrap")); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
		}
	}
	before := snap(t, repo)
	result := pyxis(t, repo, env, "doctor")
	if result.code != 1 || !strings.Contains(result.stdout, "process-boundary") || !strings.Contains(result.stdout, "install Bubblewrap") {
		t.Fatalf("missing Linux prerequisite: %+v", result)
	}
	sameSnap(t, before, snap(t, repo), "doctor prerequisite probe is read-only")
}
