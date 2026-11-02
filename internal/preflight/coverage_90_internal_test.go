package preflight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

type coverage90GHResponse struct {
	stdout string
	stderr string
	exit   int
	err    error
}

type coverage90GHRunner struct {
	responses []coverage90GHResponse
	calls     int
}

func (r *coverage90GHRunner) RunGH(context.Context, ...string) (string, string, int, error) {
	response := r.responses[r.calls]
	r.calls++
	return response.stdout, response.stderr, response.exit, response.err
}

func TestCoverage90AdmissionBoundaries(t *testing.T) {
	root := t.TempDir()
	gitLoop := filepath.Join(root, ".git")
	if err := os.Symlink(gitLoop, gitLoop); err != nil {
		t.Fatal(err)
	}
	if _, err := probeAdmission(t.Context(), Input{Target: root}); err == nil {
		t.Fatal("admission ignored an unreadable git marker")
	}
	if got := conflictPaths(nil, []string{"x", "?? longer"}); !strings.Contains(strings.Join(got, ","), "x") {
		t.Fatalf("short porcelain path missing: %v", got)
	}
	if got := currentBranch(t.Context(), filepath.Join(root, "missing")); got != defaultBranch {
		t.Fatalf("missing repository branch = %q", got)
	}
	if _, err := gitLines(t.Context(), root, "not-a-command"); err == nil {
		t.Fatal("invalid git command returned lines")
	}
}

func TestCoverage90ToolchainParsingAndBuildDefects(t *testing.T) {
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid embedded floor JSON did not panic")
			}
		}()
		_ = mustParseFloors([]byte("{"))
	}()
	if !versionAtLeast("1.2.3", "1.2.3") {
		t.Fatal("equal versions did not satisfy the floor")
	}
	if got := parseVersionTriple("nightly.2x.3"); got != [3]int{0, 2, 3} {
		t.Fatalf("suffix parse = %v", got)
	}

	priorTools, priorFloors := toolchainTools, toolchainFloors
	toolchainTools = []toolchainTool{{name: "missing-floor", extract: extractGitVersion}}
	toolchainFloors = map[string]string{}
	t.Cleanup(func() { toolchainTools, toolchainFloors = priorTools, priorFloors })
	if _, err := probeToolchain(t.Context(), Input{}); err == nil {
		t.Fatal("missing toolchain floor was accepted")
	}
}

func TestCoverage90GitHubProbeBoundaries(t *testing.T) {
	if runnerFor(Input{}) == nil {
		t.Fatal("default GitHub runner is nil")
	}
	if out, ok := probeGHPresence(t.Context(), &coverage90GHRunner{responses: []coverage90GHResponse{{stderr: "failed\nmore", exit: 1}}}); ok || out.Status != StatusRefuse {
		t.Fatalf("non-zero presence probe = %+v, %v", out, ok)
	}
	want := errors.New("auth transport failed")
	if out, ok := probeGHAuth(t.Context(), &coverage90GHRunner{responses: []coverage90GHResponse{{err: want}}}); ok || out.Status != StatusRefuse {
		t.Fatalf("auth transport failure = %+v, %v", out, ok)
	}
	if got := RemoteSlug(t.Context(), " ", ""); got != "" {
		t.Fatalf("empty target slug = %q", got)
	}
	if samePath(filepath.Join(t.TempDir(), "missing"), t.TempDir()) {
		t.Fatal("missing path compared equal")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeSleep(canceled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled sleeper = %v", err)
	}
	if out, ok := probeRepoExists(t.Context(), &coverage90GHRunner{responses: []coverage90GHResponse{{err: want}}}, "owner/repo"); ok || out.Status != StatusRefuse {
		t.Fatalf("repository transport failure = %+v, %v", out, ok)
	}
	priorSleep := probeSleep
	probeSleep = func(context.Context, time.Duration) error { return want }
	t.Cleanup(func() { probeSleep = priorSleep })
	transient := &coverage90GHRunner{responses: []coverage90GHResponse{{stderr: "HTTP 502", exit: 1}}}
	if out, ok := probeRepoExists(t.Context(), transient, "owner/repo"); ok || out.Status != StatusRefuse {
		t.Fatalf("interrupted repository retry = %+v, %v", out, ok)
	}

	if out, err := probeScopePrediction(t.Context(), Input{Knobs: config.Knobs{}}); err != nil || out.Status != StatusPass {
		t.Fatalf("operation-free scope prediction = %+v, %v", out, err)
	}
	if scopes, ok := parseOAuthScopes("X-Oauth-Scopes:\r\n"); !ok || len(scopes) != 0 {
		t.Fatalf("empty OAuth header = %v, %v", scopes, ok)
	}
	if got := firstStderrLine(" \n\t"); got != "" {
		t.Fatalf("empty stderr line = %q", got)
	}
}

func TestCoverage90ArtifactSourceGitHubRemedy(t *testing.T) {
	out := refuseArtifactSource("manifest", "https://github.com/owner/repo/releases/file", errors.New("offline"))
	if out.Status != StatusRefuse || !strings.Contains(out.Remedy, "GH_TOKEN") {
		t.Fatalf("GitHub source refusal = %+v", out)
	}
}
