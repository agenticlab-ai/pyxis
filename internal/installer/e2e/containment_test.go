package e2e

// Extends the credential-containment discipline (the gh token reaches only
// github.com/api.github.com, internal/resolve/fetch_internal_test.go) to the
// pre-write PROBES the checked front door adds (design init-hardening-v1 §3.5's
// security posture, NFR-2; plan init-hardening/001 T11): the GitHub-mode
// presence/auth/existence/scope probes (T3). They reuse the SAME production
// machinery (wire.Runner for gh) rather than a parallel implementation —
// DINV-2's one-evaluator-per-predicate, held for credential attachment too — so
// these tests exercise the probes' own call sites, not a re-derivation of
// wire's already-proven internals. Hermetic and fast: no built binaries needed
// (a direct preflight.Run, exactly as internal/preflight's own fixture matrices
// call it).

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

// TestGitHubProbesStayWithinDeclaredVocabularyNoSecretLeak proves the gh
// half of NFR-2: every gh call the checks phase makes — presence, auth,
// coordinate existence, best-effort scope prediction — stays within the
// fixed, named argv vocabulary (github.com is the only host gh is ever
// pointed at, via --hostname), and none of it ever carries a credential
// value in argv (gh manages its own ambient auth; pyxis never attaches
// one) — proven by planting canary secrets under env names real code has no
// reason to read and asserting they never surface in any recorded call.
func TestGitHubProbesStayWithinDeclaredVocabularyNoSecretLeak(t *testing.T) {
	t.Setenv("GH_TOKEN", "canary-token-must-never-appear-in-argv")

	runner := &recordingGH{}
	knobs := config.Knobs{
		Tracker: "github", CIMode: "both",
		Wiring: config.Wiring{Repo: "use", Protection: "required-checks"},
	}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: t.TempDir(), Knobs: knobs, Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out := findOutcome(t, rep, preflight.GitHubModeID); out.Status != preflight.StatusPass {
		t.Fatalf("github-mode outcome = %+v, want pass (a clean happy-path transcript)", out)
	}

	allowed := [][]string{
		{"--version"},
		{"auth", "status", "--hostname", "github.com"},
		{"api", "-i", "user"},
		{"repo", "view", "octo/widget", "--json", "name"},
	}
	for _, argv := range runner.calls {
		ok := false
		for _, want := range allowed {
			if slices.Equal(argv, want) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("checks phase issued a gh call outside the declared vocabulary: %v", argv)
		}
		for _, tok := range argv {
			if strings.Contains(tok, "canary") {
				t.Errorf("a gh call leaked the canary credential in argv: %v", argv)
			}
		}
	}
	if len(runner.calls) == 0 {
		t.Fatal("no gh calls recorded — the fixture did not exercise the probes at all")
	}
}

// findOutcome finds id's outcome in rep, fataling when absent.
func findOutcome(t *testing.T, rep preflight.Report, id string) preflight.Outcome {
	t.Helper()
	for _, o := range rep {
		if o.Check == id {
			return o
		}
	}
	t.Fatalf("Report carries no %s outcome: %+v", id, rep)
	return preflight.Outcome{}
}

// recordingGH is a hermetic happy-path wire.Runner recording every argv it
// is asked to run — the containment matrix's evidence, distinct from
// ghTranscript (github_test.go) only in that it exists to be inspected
// directly rather than driven through a full wiring walk.
type recordingGH struct {
	calls [][]string
}

func (g *recordingGH) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	g.calls = append(g.calls, append([]string(nil), args...))
	switch {
	case len(args) == 1 && args[0] == "--version":
		return "gh version 2.40.0 (recording)\n", "", 0, nil
	case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
		return "", "Logged in to github.com as e2e-fake\n", 0, nil
	case len(args) >= 3 && args[0] == "api" && args[1] == "-i" && args[2] == "user":
		return "HTTP/2.0 200 OK\nX-Oauth-Scopes: repo\n\n{}", "", 0, nil
	case len(args) >= 2 && args[0] == "repo" && args[1] == "view":
		return `{"name":"widget"}`, "", 0, nil
	}
	return "", "", 0, nil
}
