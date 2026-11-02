package reviewinvoke

import (
	"context"
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
)

// The live invoker test runs inside this trusted host executable. Provide the
// same private assembler entrypoint as implement-now so its reviewer can use
// the compiled canonical builder without compiling proposal code.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--internal-review-pack" {
		os.Exit(pack.RunCommand(context.Background(), os.Args[2:], os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "--internal-review-context" {
		os.Exit(pack.RunContextIDCommand(context.Background(), os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}
