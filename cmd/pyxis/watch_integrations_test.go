package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/trackeridentity"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func associationBody(t *testing.T, a proposalAssociation) string {
	t.Helper()
	wire, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return "<!-- pyxis-task-proposal:" + base64.RawURLEncoding.EncodeToString(wire) + " -->"
}

func TestProposalAssociationIsClosedAndExact(t *testing.T) {
	a := proposalAssociation{Schema: "pyxis-task-proposal/v1", Task: "T1", Source: "http://localhost:3000/o/r/issues/1", Number: 1}
	body := associationBody(t, a)
	got, found, err := readProposalAssociation(body)
	if err != nil || !found || got != a {
		t.Fatalf("association=%+v %v %v", got, found, err)
	}
	if _, found, err := readProposalAssociation("ordinary human description"); err != nil || found {
		t.Fatal("invented association")
	}
	for _, bad := range []string{
		strings.TrimSuffix(body, " -->"), body + body,
		"<!-- pyxis-task-proposal:! -->",
		"<!-- pyxis-task-proposal:" + base64.RawURLEncoding.EncodeToString([]byte(`{"schema":"pyxis-task-proposal/v1","task":"T1","source":"x","number":1,"extra":true}`)) + " -->",
		"<!-- pyxis-task-proposal:" + base64.RawURLEncoding.EncodeToString([]byte(`{} {}`)) + " -->",
		associationBody(t, proposalAssociation{Schema: "v0", Task: "T1", Source: "x", Number: 1}),
	} {
		if _, _, err := readProposalAssociation(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestMixedHostCompletionRequiresAssociationAndAncestry(t *testing.T) {
	oldTask, oldProposal, oldClaims, oldAncestry, oldClose := integrationTaskFacts, integrationProposalFacts, integrationClaimFacts, integrationAncestry, integrationCloseTask
	t.Cleanup(func() {
		integrationTaskFacts, integrationProposalFacts, integrationClaimFacts, integrationAncestry, integrationCloseTask = oldTask, oldProposal, oldClaims, oldAncestry, oldClose
	})
	tasks := integrations.ResolvedRepository{Provider: "forgejo", BaseURL: "http://localhost:3000", Repository: "tasks/repo"}
	code := integrations.ResolvedRepository{Provider: "github", BaseURL: "https://github.com", Repository: "code/repo"}
	mapping := planMap{issueByTask: map[string]int{"T1": 1, "T2": 2}, blockedBy: map[string][]string{"T2": {"T1"}}}
	var pulls []forgejocodehost.Pull
	var readErr, ancestryErr error
	closedCount, ancestryCount := 0, 0
	integrationTaskFacts = func(context.Context, string, integrations.ResolvedRepository, planMap) (map[int]bool, error) {
		return map[int]bool{}, readErr
	}
	integrationProposalFacts = func(_ context.Context, _ string, b integrations.ResolvedRepository) ([]forgejocodehost.Pull, error) {
		if b != code {
			t.Fatal("proposal used task binding")
		}
		return pulls, nil
	}
	integrationClaimFacts = func(_ context.Context, _, _, source string) (map[int]bool, error) {
		if source != tasks.Key() {
			t.Fatal("wrong claim scope")
		}
		return map[int]bool{2: true}, nil
	}
	integrationAncestry = func(_ context.Context, _, _, branch, sha string) error {
		ancestryCount++
		if branch != "main" || sha != "merged-sha" {
			t.Fatal("wrong ancestry predicate")
		}
		return ancestryErr
	}
	integrationCloseTask = func(_ context.Context, _ string, b integrations.ResolvedRepository, n int) error {
		if b != tasks || n != 1 {
			t.Fatal("wrong task closed")
		}
		closedCount++
		return nil
	}
	pull := forgejocodehost.Pull{Number: 8, State: "closed", Merged: true, MergeCommitSHA: "merged-sha", Body: associationBody(t, proposalAssociation{Schema: "pyxis-task-proposal/v1", Task: "T1", Source: trackeridentity.SourceRef(tasks, 1), Number: 1})}
	pull.Base.Ref = "main"
	pull.Head.Repo.FullName = code.Repository
	pull.Head.Ref = "feat/" + (port.Task{SourceKey: tasks.Key()}).ScopeToken() + "/t1-title-1"
	pulls = []forgejocodehost.Pull{pull}
	plan, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background())
	if err != nil || len(plan.Tasks) != 2 || !plan.Tasks[0].Closed || !plan.Tasks[1].Claimed || closedCount != 1 || ancestryCount != 1 {
		t.Fatalf("plan=%+v err=%v close=%d ancestor=%d", plan, err, closedCount, ancestryCount)
	}
	closedCount, ancestryCount = 0, 0
	plan, err = integrationPlanSource("repo", "origin", "main", tasks, code, mapping, false)(context.Background())
	if err != nil || plan.Tasks[0].Closed || plan.Tasks[0].Proposal == nil || closedCount != 0 || ancestryCount != 0 {
		t.Fatal("status mutated or concealed unfinished completion")
	}
	ancestryErr = errors.New("not on main")
	if _, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background()); err == nil || closedCount != 0 {
		t.Fatal("unverified ancestry closed task")
	}
	ancestryErr = nil
	for _, mutate := range []func(*forgejocodehost.Pull){
		func(p *forgejocodehost.Pull) { p.Head.Repo.FullName = "other/repo" },
		func(p *forgejocodehost.Pull) { p.Head.Ref = "feat/unscoped-1" },
		func(p *forgejocodehost.Pull) {
			p.Body = associationBody(t, proposalAssociation{Schema: "pyxis-task-proposal/v1", Task: "T1", Source: "http://localhost:3001/tasks/repo/issues/1", Number: 1})
		},
	} {
		p := pull
		mutate(&p)
		pulls = []forgejocodehost.Pull{p}
		plan, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background())
		if err != nil || plan.Tasks[0].Closed || closedCount != 0 {
			t.Fatal("cross-host lookalike affected task")
		}
	}
	pulls = []forgejocodehost.Pull{pull, pull}
	pulls[1].Number = 9
	if _, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background()); err == nil {
		t.Fatal("ambiguous proposals accepted")
	}
	pulls = []forgejocodehost.Pull{pull}
	pulls[0].Base.Ref = "other"
	if _, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background()); err == nil {
		t.Fatal("wrong merge base accepted")
	}
	readErr = errors.New("offline")
	if _, err := integrationPlanSource("repo", "origin", "main", tasks, code, mapping, true)(context.Background()); !errors.Is(err, readErr) {
		t.Fatal("task read failure hidden")
	}
}

type watchRoundTrip func(*http.Request) (*http.Response, error)

func (f watchRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfiguredWatchAndForgejoReads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, enabled, err := configuredWatch(root, "o/r", "gh", root, repoconfig.Defaults(), planMap{}, false); err != nil || enabled {
		t.Fatalf("legacy configured=%v err=%v", enabled, err)
	}
	config := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000/base
  bindings:
    tasks:
      connection: local
      repository: o/r
    code_host:
      connection: local
      repository: o/code
`
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	reader, source, medium, enabled, err := configuredWatch(root, "", "gh", root, repoconfig.Defaults(), planMap{}, false)
	if err != nil || !enabled || source == nil || medium != "forgejo" || reader.Cadence(context.Background()) != 10*time.Second {
		t.Fatalf("configured reader=%+v medium=%s enabled=%v err=%v", reader, medium, enabled, err)
	}
	old := integrationForgejoClient
	t.Cleanup(func() { integrationForgejoClient = old })
	state := "open"
	integrationForgejoClient = func(base, token string) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(base, token, &http.Client{Transport: watchRoundTrip(func(req *http.Request) (*http.Response, error) {
			body := "[]"
			switch req.URL.Path {
			case "/base/api/v1/repos/o/r/issues/1":
				if req.Method == "PATCH" {
					state = "closed"
				}
				body = fmt.Sprintf(`{"number":1,"state":%q}`, state)
			case "/base/api/v1/repos/o/r/pulls":
				if req.URL.Query().Get("page") == "1" {
					body = `[{"number":7,"state":"open"}]`
				}
			default:
				t.Fatalf("unexpected API path %s", req.URL.Path)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})})
	}
	binding := integrations.ResolvedRepository{Provider: "forgejo", BaseURL: "http://127.0.0.1:3000/base", Repository: "o/r"}
	states, err := readIntegrationTasks(context.Background(), root, binding, planMap{issueByTask: map[string]int{"T1": 1}})
	if err != nil || states[1] {
		t.Fatalf("states=%v err=%v", states, err)
	}
	if err := closeIntegrationTask(context.Background(), root, binding, 1); err != nil || state != "closed" {
		t.Fatalf("close=%v state=%s", err, state)
	}
	pulls, err := readIntegrationProposals(context.Background(), root, binding)
	if err != nil || len(pulls) != 1 || pulls[0].Number != 7 {
		t.Fatalf("pulls=%+v err=%v", pulls, err)
	}
	integrationForgejoClient = func(string, string) (*forgejo.Client, error) { return nil, errors.New("bad client") }
	if _, err := readIntegrationTasks(context.Background(), root, binding, planMap{}); err == nil {
		t.Fatal("bad client accepted")
	}
	if err := closeIntegrationTask(context.Background(), root, binding, 1); err == nil {
		t.Fatal("bad close client accepted")
	}
	if _, err := readIntegrationProposals(context.Background(), root, binding); err == nil {
		t.Fatal("bad proposal client accepted")
	}
}

func TestScopedClaimsAndMainlineProofUseRealGit(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	repo := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git(repo, "init", "--bare", remote)
	git(repo, "init", "-b", "main")
	git(repo, "config", "user.name", "test")
	git(repo, "config", "user.email", "test@example.invalid")
	git(repo, "commit", "--allow-empty", "-m", "base")
	sha := git(repo, "rev-parse", "HEAD")
	git(repo, "remote", "add", "origin", remote)
	git(repo, "push", "origin", "main")
	source := "forgejo:http://localhost:3000:o/r"
	scope := (port.Task{SourceKey: source}).ScopeToken()
	git(repo, "push", "origin", "HEAD:refs/claim/"+scope+"/1")
	git(repo, "push", "origin", "HEAD:refs/claim/other/1")
	claims, err := claimedIssuesScoped(context.Background(), repo, "origin", source)
	if err != nil || len(claims) != 1 || !claims[1] {
		t.Fatalf("claims=%v err=%v", claims, err)
	}
	if err := proposalOnMainline(context.Background(), repo, "origin", "main", sha); err != nil {
		t.Fatal(err)
	}
	git(repo, "commit", "--allow-empty", "-m", "not pushed")
	if err := proposalOnMainline(context.Background(), repo, "origin", "main", git(repo, "rev-parse", "HEAD")); err == nil {
		t.Fatal("unlanded commit certified mainline")
	}
	if err := proposalOnMainline(context.Background(), repo, "origin", "main", "-bad"); err == nil {
		t.Fatal("unsafe revision accepted")
	}
	if _, err := claimedIssuesScoped(context.Background(), repo, "missing", source); err == nil {
		t.Fatal("unavailable claims became empty")
	}
}
