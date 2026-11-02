// Package localgittest is the switch for tests of the local git code host:
// localgit's own tests and the tests elsewhere that drive local code-host mode.
//
// The operator disabled them until localgit's scope is reviewed. GitHub is the
// code host in use, and these tests started the largest share of the coverage
// gate's git processes. They still compile, and the coverage gate leaves out
// the code that only they reach. Set PYXIS_LOCALGIT_TESTS=1 to run them.
package localgittest

import (
	"os"
	"testing"
)

// Enabled reports whether tests of the local git code host run.
func Enabled() bool {
	return os.Getenv("PYXIS_LOCALGIT_TESTS") == "1"
}

// Skip skips t unless tests of the local git code host run.
func Skip(t testing.TB) {
	t.Helper()
	if !Enabled() {
		t.Skip("localgit tests are disabled; set PYXIS_LOCALGIT_TESTS=1 to run them")
	}
}
