package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureReleaseCopiesRemainIndependent(t *testing.T) {
	first := buildFixtureRelease(t)
	second := buildFixtureRelease(t)
	if first.src == second.src || first.pin != second.pin {
		t.Fatal("identical releases must have distinct writable directories and equal pins")
	}
	for _, name := range []string{
		"release-manifest-" + first.version + ".json",
		"payload-" + first.version + ".tar.gz",
		"pyxis-" + first.version + "-" + first.plat,
		"docgate-" + first.version + "-" + first.plat,
	} {
		t.Run(name, func(t *testing.T) {
			want := hashFile(t, filepath.Join(second.src, name))
			if err := os.WriteFile(filepath.Join(first.src, name), []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
			third := buildFixtureRelease(t)
			for _, release := range []fixtureRelease{second, third} {
				if got := hashFile(t, filepath.Join(release.src, name)); got != want {
					t.Fatalf("tampering with one release changed another copy of %s", name)
				}
			}
		})
	}
}
