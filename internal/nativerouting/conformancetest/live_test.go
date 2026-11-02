package conformancetest

import (
	"errors"
	"testing"
	"time"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	_ "github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type liveAuthorization struct{}

func (liveAuthorization) Authorize(proof nativerouting.AuthorizationProof) error {
	if proof.Kind != nativerouting.AuthorizationRepositoryOperator || proof.Repository != "agenticlab-ai/pyxis" || proof.OperatorID == "" {
		return errors.New("qualification is not repository-operator authorized")
	}
	return nil
}

func TestCompleteLiveRoutingMatrix(t *testing.T) {
	registry := runtime.LiveRegistry()
	domain, err := GenerateDomain(executiondemand.DefaultPolicy, registry, Roles{nativerouting.RoleReview, nativerouting.RoleImplementation})
	if err != nil {
		t.Fatal(err)
	}
	if len(domain.Harnesses) != 4 {
		t.Fatalf("live registry has %d harnesses (%v), want all four", len(domain.Harnesses), domain.Harnesses)
	}
	matrix, report, err := Check(domain, registry, fakeClock{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}, liveAuthorization{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix.Results) != len(domain.Harnesses)*len(domain.Roles)*len(domain.Profiles) {
		t.Fatalf("matrix has %d results for %d cells", len(matrix.Results), len(domain.Cells))
	}
	resolved := 0
	for _, result := range matrix.Results {
		if result.Result != ResultPass {
			t.Fatalf("live cell did not pass: %#v", result)
		}
		if result.Expected == ExpectedResolved {
			resolved++
		}
	}
	if len(report.Rows) != resolved || report.Signoff.MatrixIdentity != matrix.MatrixIdentity {
		t.Fatalf("qualification report rows=%d resolved=%d signoff=%#v", len(report.Rows), resolved, report.Signoff)
	}
}
