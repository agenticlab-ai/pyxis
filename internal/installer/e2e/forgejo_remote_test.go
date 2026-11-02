package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// forgejoRemote is the opt-in, real-server leg of the installed feature test.
// It never creates a repository, advances main, or changes branch protection.
// The caller must finish workers before test cleanup; cleanup has its own
// context and credential-configured Git directory, surviving caller TempDirs.
type forgejoRemote struct {
	Client     *forgejo.Client
	Binding    integrations.ResolvedRepository
	RunID      string
	PlanRef    string
	BaseBranch string
	Label      string
	MainSHA    string
	GitURL     string

	gitDir        string
	before        forgejoRemoteSnapshot
	refs          map[string]bool
	issues        map[int]bool
	proposals     map[int]bool
	labelExpected bool
}

type forgejoRemoteLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// Include comment counts and last-edit timestamps so accidental activity on an
// existing item is detected as well as title/body/state edits. Labels are
// ordered before comparison; concurrent external edits fail the audit without
// attempting to restore or overwrite another actor's work.
type forgejoRemoteIssue struct {
	Number    int                  `json:"number"`
	Title     string               `json:"title"`
	Body      string               `json:"body"`
	State     string               `json:"state"`
	Comments  int                  `json:"comments"`
	UpdatedAt string               `json:"updated_at"`
	Labels    []forgejoRemoteLabel `json:"labels"`
}

type forgejoRemoteSnapshot struct {
	Issues      map[int]forgejoRemoteIssue
	Labels      map[int64]forgejoRemoteLabel
	Refs        map[string]string
	Protections json.RawMessage
}

func newForgejoRemote(t *testing.T) *forgejoRemote {
	t.Helper()
	if os.Getenv("PYXIS_E2E_FORGEJO") != "1" {
		t.Skip("live Forgejo feature test: set PYXIS_E2E_FORGEJO=1 to use the selected existing repository")
	}
	requireHarness(t)
	base := os.Getenv("PYXIS_E2E_FORGEJO_URL")
	if base == "" {
		base = "http://127.0.0.1:3000"
	}
	repo := os.Getenv("PYXIS_E2E_FORGEJO_REPO")
	if repo == "" {
		repo = "operator/test-repo"
	}
	if os.Getenv("PYXIS_FORGEJO_TOKEN") == "" {
		t.Fatal("live Forgejo feature test requires PYXIS_FORGEJO_TOKEN")
	}
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion,
		Connections: map[string]integrations.Connection{"e2e": {Provider: integrations.Forgejo, BaseURL: base, TokenEnv: "PYXIS_FORGEJO_TOKEN"}},
		Bindings:    integrations.Bindings{CodeHost: integrations.Binding{Connection: "e2e", Repository: repo}}}
	binding, err := profile.CodeHost("")
	if err != nil {
		t.Fatal(err)
	}
	client, err := forgejo.New(binding.BaseURL, binding.Token())
	if err != nil {
		t.Fatal(err)
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	runID := "pyxis-e2e-forgejo-" + hex.EncodeToString(random[:])
	r := &forgejoRemote{Client: client, Binding: binding, RunID: runID, PlanRef: runID + "/001",
		BaseBranch: runID + "-base", Label: runID, GitURL: binding.BaseURL + "/" + binding.Repository + ".git",
		gitDir: t.TempDir(), refs: map[string]bool{}, issues: map[int]bool{}, proposals: map[int]bool{}}
	// This directory's removal was registered first, so it outlives this
	// cleanup even if the caller's checkout is removed after a failed init.
	if _, err := r.command(t.Context(), r.gitDir, "git", "init", "--quiet"); err != nil {
		t.Fatal(err)
	}
	config, err := profile.Render()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.gitDir, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.gitDir, ".pyxis", "config.yaml"), config, 0600); err != nil {
		t.Fatal(err)
	}
	r.configureGit(t, r.gitDir, builtBins["pyxis"], nil)
	var repository struct {
		Empty       bool `json:"empty"`
		Permissions struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}
	if err := client.Request(t.Context(), http.MethodGet, r.path(""), nil, &repository); err != nil {
		t.Fatal(err)
	}
	if repository.Empty || !repository.Permissions.Push {
		t.Fatal("live Forgejo fixture requires an existing nonempty repository with push permission")
	}
	r.before, err = r.snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r.MainSHA = r.before.Refs["refs/heads/main"]
	if r.MainSHA == "" {
		t.Fatal("live Forgejo fixture requires existing main; it never seeds or modifies main")
	}
	for _, label := range r.before.Labels {
		if strings.EqualFold(label.Name, r.Label) {
			t.Fatal("unique fixture label already exists")
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := r.cleanup(ctx); err != nil {
			t.Errorf("Forgejo cleanup %s: %v", r.RunID, err)
		} else {
			t.Logf("Forgejo cleanup %s verified: owned tasks/PRs closed, owned refs/label absent, %d existing items and %d existing refs unchanged", r.RunID, len(r.before.Issues), len(r.before.Refs))
		}
	})
	r.ownRef(t, "refs/heads/"+r.BaseBranch)
	t.Logf("live Forgejo fixture %s on %s; main=%s", r.RunID, binding.Key(), r.MainSHA)
	return r
}

func (r *forgejoRemote) path(suffix string) string {
	return forgejo.RepoPath(r.Binding.Repository, suffix)
}

func (r *forgejoRemote) marker() string {
	return "<!-- pyxis:e2e-forgejo/v1 " + r.RunID + " -->"
}

// configureGit uses the installed production helper, never embeds the token in
// Git config, URLs, or process arguments, and refuses a different code binding.
func (r *forgejoRemote) configureGit(t *testing.T, dir, pyxisBin string, env []string) {
	t.Helper()
	profile, err := integrations.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := profile.CodeHost("")
	if err != nil || bound.Key() != r.Binding.Key() || bound.TokenEnv != r.Binding.TokenEnv {
		t.Fatalf("fixture checkout code-host binding differs from selected server: %v", err)
	}
	run := func(program string, args ...string) string {
		if env != nil {
			return forgejoRun(t, dir, env, program, args...)
		}
		out, err := r.command(t.Context(), dir, program, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	run(pyxisBin, "credential", "--repo", dir, "configure")
	hasOrigin := false
	for name := range strings.FieldsSeq(run("git", "remote")) {
		hasOrigin = hasOrigin || name == "origin"
	}
	if !hasOrigin {
		run("git", "remote", "add", "origin", r.GitURL)
	} else if remote := run("git", "remote", "get-url", "origin"); strings.TrimSuffix(strings.TrimSpace(remote), ".git") != strings.TrimSuffix(r.GitURL, ".git") {
		t.Fatal("fixture origin differs from selected Forgejo repository")
	}
}

// pushBase admits only a new, uniquely named ref, with a create-only Git CAS.
// The fixture's first base cannot pass an operand pre-push gate that compares
// against that not-yet-existing remote base. Copy its exact commit through real
// local Git into the independent credential-configured setup directory, which
// has no operand hooks, and seed only this owned base from there. Subsequent
// feature pushes use the installed operand and its real hooks unchanged.
func (r *forgejoRemote) pushBase(t *testing.T, dir, installedHead, sha string, env []string) {
	t.Helper()
	if env == nil {
		t.Fatal("fixture push requires its explicit installed environment")
	}
	ref := "refs/heads/" + r.BaseBranch
	r.ownRef(t, ref)
	forgejoRun(t, r.gitDir, env, "git", "fetch", "--no-tags", "--", dir, sha)
	// Init already published this owned branch. Advance only its verified
	// installation head while seeding the feature fixture; never expect absence.
	forgejoRun(t, r.gitDir, env, "git", "push", "--force-with-lease="+ref+":"+installedHead, "origin", sha+":"+ref)
}

// createTask projects the same native type metadata and comment capsule read
// by the real executor. The unique label and plan marker isolate its frontier.
func (r *forgejoRemote) createTask(t *testing.T, title, body, capsule string) port.Task {
	t.Helper()
	if _, err := taskspec.Parse([]string{capsule}); err != nil {
		t.Fatalf("fixture fs-spec capsule: %v", err)
	}
	projection := forgejotracker.NewForgejo(r.Client, r.Binding.Repository)
	r.labelExpected = true // set before the write, covering ambiguous transport failure
	if err := projection.EnsureLabel(r.Label, "537FAD", r.marker()); err != nil {
		t.Fatal(err)
	}
	body += "\n\n" + r.marker() + "\n<!-- pyxis:plan-task " + r.PlanRef + "/T1 -->\n"
	if !strings.HasPrefix(title, "[T1]") {
		title = "[T1] " + title
	}
	number, err := projection.CreateIssue(title, body, []string{r.Label}, "task")
	if err != nil {
		t.Fatal(err)
	}
	r.issues[number] = true
	if err := projection.CommentContext(t.Context(), number, capsule); err != nil {
		t.Fatal(err)
	}
	executorTracker, err := forgejotracker.New(r.Client, r.Binding.Repository, r.Label, r.PlanRef)
	if err != nil {
		t.Fatal(err)
	}
	frontier, err := executorTracker.Frontier(t.Context())
	if err != nil || len(frontier) != 1 || frontier[0].Number != number || frontier[0].SourceKey != r.Binding.Key() {
		t.Fatalf("projected Forgejo frontier is not the one owned task: %+v: %v", frontier, err)
	}
	r.ownRef(t, "refs/claim/"+frontier[0].ScopeToken()+"/"+strconv.Itoa(number))
	return frontier[0]
}

func (r *forgejoRemote) registerProposal(t *testing.T, proposal port.Proposal) {
	t.Helper()
	host := forgejocodehost.New(r.Client, r.Binding.BaseURL, r.Binding.Repository, r.BaseBranch)
	pull, err := host.Pull(t.Context(), proposal.Number)
	if err != nil {
		t.Fatal(err)
	}
	if _, existed := r.before.Issues[pull.Number]; existed || pull.Base.Ref != r.BaseBranch || !r.allowedRef("refs/heads/"+pull.Head.Ref) || pull.Head.Repo.FullName != r.Binding.Repository {
		t.Fatal("proposal is not owned by this Forgejo fixture")
	}
	r.proposals[pull.Number] = true
	r.ownRef(t, "refs/heads/"+pull.Head.Ref)
}

func (r *forgejoRemote) allowedRef(ref string) bool {
	if strings.HasPrefix(ref, "refs/heads/"+r.RunID+"-") || strings.HasPrefix(ref, "refs/pyxis/"+r.RunID+"/") {
		return true
	}
	for number := range r.issues {
		task := port.Task{Number: number, SourceKey: r.Binding.Key()}
		if ref == "refs/claim/"+task.ScopeToken()+"/"+strconv.Itoa(number) ||
			(strings.HasPrefix(ref, "refs/heads/feat/"+task.ScopeToken()+"/") && strings.HasSuffix(ref, "-"+strconv.Itoa(number))) {
			return true
		}
	}
	return false
}

// ownRef registers exact runtime claims, task branches, and per-run evidence
// refs before dispatch. Shared/default evidence namespaces are never admitted.
func (r *forgejoRemote) ownRef(t *testing.T, ref string) {
	t.Helper()
	if !r.allowedRef(ref) || r.before.Refs[ref] != "" {
		t.Fatalf("ref %q is not new and owned by fixture %s", ref, r.RunID)
	}
	if _, err := r.command(t.Context(), r.gitDir, "git", "check-ref-format", ref); err != nil {
		t.Fatal(err)
	}
	r.refs[ref] = true
}

func (r *forgejoRemote) command(ctx context.Context, dir, program string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "GH_") || strings.HasPrefix(name, "GITHUB_") {
			continue
		}
		cmd.Env = append(cmd.Env, pair)
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0")
	b, err := cmd.CombinedOutput()
	text := string(b)
	if token := r.Binding.Token(); token != "" {
		text = strings.ReplaceAll(text, token, "[redacted]")
	}
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", filepath.Base(program), strings.Join(args, " "), err, text)
	}
	return text, nil
}

func (r *forgejoRemote) remoteRefs(ctx context.Context) (map[string]string, error) {
	out, err := r.command(ctx, r.gitDir, "git", "ls-remote", "--refs", "origin")
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 || refs[parts[1]] != "" {
			return nil, errors.New("invalid or duplicate remote Git ref")
		}
		refs[parts[1]] = parts[0]
	}
	return refs, nil
}

func (r *forgejoRemote) snapshot(ctx context.Context) (forgejoRemoteSnapshot, error) {
	s := forgejoRemoteSnapshot{Issues: map[int]forgejoRemoteIssue{}, Labels: map[int64]forgejoRemoteLabel{}}
	var issues []forgejoRemoteIssue
	if err := r.Client.List(ctx, r.path("issues")+"?state=all", &issues); err != nil {
		return s, err
	}
	for _, issue := range issues {
		if _, duplicate := s.Issues[issue.Number]; issue.Number <= 0 || duplicate {
			return s, errors.New("invalid or duplicate issue identity in fixture snapshot")
		}
		sort.Slice(issue.Labels, func(i, j int) bool { return issue.Labels[i].ID < issue.Labels[j].ID })
		s.Issues[issue.Number] = issue
	}
	var labels []forgejoRemoteLabel
	if err := r.Client.List(ctx, r.path("labels"), &labels); err != nil {
		return s, err
	}
	for _, label := range labels {
		if _, duplicate := s.Labels[label.ID]; label.ID <= 0 || duplicate {
			return s, errors.New("invalid or duplicate label identity in fixture snapshot")
		}
		s.Labels[label.ID] = label
	}
	if err := r.Client.Request(ctx, http.MethodGet, r.path("branch_protections"), nil, &s.Protections); err != nil {
		return s, err
	}
	var err error
	s.Refs, err = r.remoteRefs(ctx)
	return s, err
}

func (r *forgejoRemote) cleanup(ctx context.Context) error {
	var failures []error
	record := func(err error) {
		if err != nil {
			failures = append(failures, err)
		}
	}
	// Rediscover only exact marker-owned issues and this run's branch namespace
	// so a failed response after a successful POST/push still gets cleaned up.
	var issues []forgejoRemoteIssue
	if err := r.Client.List(ctx, r.path("issues")+"?state=all&type=issues", &issues); err != nil {
		record(err)
	} else {
		for _, issue := range issues {
			if _, existed := r.before.Issues[issue.Number]; !existed && strings.Contains(issue.Body, r.marker()) {
				r.issues[issue.Number] = true
			}
		}
	}
	host := forgejocodehost.New(r.Client, r.Binding.BaseURL, r.Binding.Repository, r.BaseBranch)
	pulls, err := host.Pulls(ctx, "all")
	if err != nil {
		record(err)
	} else {
		for _, pull := range pulls {
			_, existed := r.before.Issues[pull.Number]
			if existed || pull.Base.Ref != r.BaseBranch || !r.allowedRef("refs/heads/"+pull.Head.Ref) || pull.Head.Repo.FullName != r.Binding.Repository {
				continue
			}
			r.proposals[pull.Number] = true
			if pull.Merged {
				record(fmt.Errorf("fixture proposal #%d was unexpectedly merged", pull.Number))
			}
			if pull.State == "open" {
				record(r.Client.Request(ctx, http.MethodPatch, r.path(fmt.Sprintf("pulls/%d", pull.Number)), struct {
					State string `json:"state"`
				}{"closed"}, nil))
			}
		}
	}
	projection := forgejotracker.NewForgejo(r.Client, r.Binding.Repository)
	for number := range r.issues {
		record(projection.CloseContext(ctx, number))
	}
	var labels []forgejoRemoteLabel
	if r.labelExpected {
		if err := r.Client.List(ctx, r.path("labels"), &labels); err != nil {
			record(err)
		} else {
			for _, label := range labels {
				if label.Name == r.Label && label.Description == r.marker() {
					if _, existed := r.before.Labels[label.ID]; !existed {
						record(r.Client.Request(ctx, http.MethodDelete, r.path(fmt.Sprintf("labels/%d", label.ID)), nil, nil))
					}
				}
			}
		}
	}
	refs, err := r.remoteRefs(ctx)
	if err != nil {
		record(err)
	} else {
		for ref, sha := range refs {
			if !r.allowedRef(ref) || r.before.Refs[ref] != "" {
				continue
			}
			r.refs[ref] = true
			_, err := r.command(ctx, r.gitDir, "git", "push", "--force-with-lease="+ref+":"+sha, "origin", ":"+ref)
			record(err)
		}
	}
	after, err := r.snapshot(ctx)
	if err != nil {
		record(err)
		return errors.Join(failures...)
	}
	for number, issue := range r.before.Issues {
		if !reflect.DeepEqual(issue, after.Issues[number]) {
			record(fmt.Errorf("pre-existing issue/PR #%d changed", number))
		}
	}
	for id, label := range r.before.Labels {
		if after.Labels[id] != label {
			record(fmt.Errorf("pre-existing label %d changed", id))
		}
	}
	record(r.verifyRefs(after.Refs))
	if !reflect.DeepEqual(r.before.Protections, after.Protections) {
		record(errors.New("pre-existing branch protections changed"))
	}
	for number := range r.issues {
		if after.Issues[number].State != "closed" {
			record(fmt.Errorf("owned task #%d remains open or missing", number))
		}
	}
	for number := range r.proposals {
		if after.Issues[number].State != "closed" {
			record(fmt.Errorf("owned proposal #%d remains open or missing", number))
		}
		pull, err := host.Pull(ctx, number)
		if err != nil {
			record(err)
		} else if pull.State != "closed" || pull.Merged {
			record(fmt.Errorf("owned proposal #%d is not closed and unmerged", number))
		}
	}
	for _, label := range after.Labels {
		if label.Name == r.Label {
			record(errors.New("owned routing label remains"))
		}
	}
	return errors.Join(failures...)
}

// verifyRefs audits without repairing: an unexpected shared ref is a test
// failure even on its first creation, and remains available for inspection.
// Forgejo retains refs/pull/* after an owned PR is closed; those are server
// records, outside the fixture's deletion authority.
func (r *forgejoRemote) verifyRefs(after map[string]string) error {
	var failures []error
	for ref, sha := range r.before.Refs {
		if after[ref] != sha {
			failures = append(failures, fmt.Errorf("pre-existing ref %s changed", ref))
		}
	}
	for ref := range after {
		switch {
		case r.refs[ref]:
			failures = append(failures, fmt.Errorf("owned ref %s remains", ref))
		case r.before.Refs[ref] == "" && !strings.HasPrefix(ref, "refs/pull/"):
			failures = append(failures, fmt.Errorf("unexpected remote ref %s was created; preserved for inspection", ref))
		}
	}
	return errors.Join(failures...)
}

func TestForgejoRemoteCleanupRejectsUnexpectedRefs(t *testing.T) {
	const owned = "refs/heads/pyxis-e2e-forgejo-owned-base"
	r := &forgejoRemote{
		before: forgejoRemoteSnapshot{Refs: map[string]string{"refs/heads/main": "main-sha"}},
		refs:   map[string]bool{owned: true},
	}
	for _, tc := range []struct {
		name string
		ref  string
		sha  string
		want string
	}{
		{"unchanged baseline", "refs/heads/main", "main-sha", ""},
		{"retained server PR", "refs/pull/42/head", "pr-sha", ""},
		{"first shared process ref", "refs/pyxis/impl-review-process-state", "new-sha", "unexpected remote ref"},
		{"first shared publication ref", "refs/pyxis/impl-review-publication-state", "new-sha", "unexpected remote ref"},
		{"unregistered branch", "refs/heads/unexpected", "new-sha", "unexpected remote ref"},
		{"unregistered claim", "refs/claim/42", "new-sha", "unexpected remote ref"},
		{"main moved", "refs/heads/main", "different-sha", "pre-existing ref"},
		{"owned ref left behind", owned, "new-sha", "owned ref"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := maps.Clone(r.before.Refs)
			after[tc.ref] = tc.sha
			untouched := maps.Clone(after)
			err := r.verifyRefs(after)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("verify refs: %v, want %q", err, tc.want)
			}
			if !maps.Equal(after, untouched) {
				t.Fatal("ref audit modified unexpected remote state")
			}
		})
	}
}

func TestForgejoRemoteCleanupRefOwnership(t *testing.T) {
	r := &forgejoRemote{RunID: "pyxis-e2e-forgejo-owned", Binding: integrations.ResolvedRepository{
		Provider: integrations.Forgejo, BaseURL: "http://127.0.0.1:3000", Repository: "operator/test-repo"}, issues: map[int]bool{42: true}}
	scope := (port.Task{SourceKey: r.Binding.Key()}).ScopeToken()
	for _, tc := range []struct {
		ref  string
		want bool
	}{
		{"refs/heads/" + r.RunID + "-base", true},
		{"refs/pyxis/" + r.RunID + "/review", true},
		{"refs/claim/" + scope + "/42", true},
		{"refs/heads/feat/" + scope + "/t1-add-sum-42", true},
		{"refs/heads/main", false},
		{"refs/pyxis/impl-review-publication-state", false},
		{"refs/heads/another-run-base", false},
		{"refs/claim/42", false},
		{"refs/claim/" + scope + "/43", false},
		{"refs/heads/feat/" + scope + "/t1-add-sum-142", false},
		{"refs/pull/42/head", false},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			if got := r.allowedRef(tc.ref); got != tc.want {
				t.Fatalf("cleanup ownership of %s = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}
