package resolve

import (
	"os"
	"testing"
)

// The two artifact-source variables are cleared to keep the binary hermetic:
// nothing reads them any more, and clearing them keeps that true rather than
// merely believed.
func TestMain(m *testing.M) {
	for _, k := range []string{"PYXIS_ARTIFACT_SOURCE", "PYXIS_CHANNEL"} {
		if err := os.Unsetenv(k); err != nil {
			panic("resolve tests: clearing " + k + ": " + err.Error())
		}
	}
	// XDG_CONFIG_HOME decides where the installer's release.env is read from;
	// point it at a directory with none, so a real record on the developer's
	// machine cannot supply a channel to a test that did not ask for one.
	dir, err := os.MkdirTemp("", "resolve-hermetic-*")
	if err != nil {
		panic("resolve tests: scratch config home: " + err.Error())
	}
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		panic("resolve tests: setting XDG_CONFIG_HOME: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
