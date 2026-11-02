package gate_test

// The behavioural face of install-epoch scoping through the umbrella
// (existing-repo-onboarding/001 T9): GateAt computes the scan scope once and
// surfaces an unreachable epoch's cause. The scan-set mechanics themselves are
// covered internally in epoch_scope_test.go.

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

func TestGateAtSurfacesUnreachableEpoch(t *testing.T) {
	root, run, write := fixtureRepo(t)
	cfg := config.Defaults()
	write("docs/x.md", "# x\n")
	run("add", "-A")
	run("commit", "-q", "-m", "corpus")
	cfg.Epoch = config.Epoch{Commit: strings.Repeat("0", 40)} // a commit this repo lacks

	defects := gate.GateAt(root, "", "", nil, cfg)
	for _, d := range defects {
		if strings.Contains(d.Reason, "unreachable") {
			return // the umbrella named the cause — the wiring is proven
		}
	}
	t.Fatalf("GateAt did not surface the unreachable-epoch cause through the umbrella: %v", defects)
}
