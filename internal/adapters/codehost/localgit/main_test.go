package localgit

import (
	"fmt"
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
)

// The operator disabled this package's tests on 2026-10-16 until localgit's
// scope is reviewed: they started about 170,000 git processes per run, the
// largest share of the coverage gate. The tests still compile, and the
// coverage gate leaves this package out of its verdict. Set
// PYXIS_LOCALGIT_TESTS=1 to run them.
func TestMain(m *testing.M) {
	if !localgittest.Enabled() {
		fmt.Fprintln(os.Stderr, "localgit tests are disabled; set PYXIS_LOCALGIT_TESTS=1 to run them")
		return
	}
	os.Exit(m.Run())
}
