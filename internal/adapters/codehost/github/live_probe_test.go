package ghcodehost

import (
	"context"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"os"
	"testing"
)

// A manual live probe (never runs in CI): surfaces the raw graphqlShell error
// the state reader's classifier would otherwise collapse into "offline".
func TestLiveGraphQLProbe(t *testing.T) {
	if os.Getenv("PYXIS_LIVE_PROBE") == "" {
		t.Skip("set PYXIS_LIVE_PROBE=1 to run the live probe")
	}
	body, err := graphqlShell{}.Query(context.Background(), GitHubStateQuery,
		map[string]any{"owner": "agenticlab-ai", "repo": "pyxis", "first": githubStatePageSize})
	if err != nil {
		t.Fatalf("raw query error: %v", err)
	}
	t.Logf("ok, %d bytes", len(body))
	r := NewStateReader("agenticlab-ai/pyxis")
	states, serr := r.States(context.Background(), []port.ProposalRef{{Repo: "agenticlab-ai/pyxis", Number: 1151}}, port.Cursor{})
	if serr != nil {
		t.Fatalf("States: %v", serr)
	}
	if states[0].Unknown != nil {
		t.Fatalf("state unknown: %s", states[0].Unknown)
	}
	t.Logf("decision=%s comments=%d", states[0].ReviewDecision, len(states[0].Comments))
}
