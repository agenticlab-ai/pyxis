package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	memorycli "github.com/agenticlab-ai/pyxis/internal/memory/cli"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/precheck"
)

func localLockCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	root, origin := localGitCLIFixture(t, true)
	for _, name := range []string{"PYXIS_CI_EVENT", "PYXIS_CI_BASE", "GITHUB_ACTIONS"} {
		t.Setenv(name, "")
	}
	var output, diagnostic bytes.Buffer
	env := memorycli.OSEnv()
	env.Dir, env.Stdout, env.Stderr = root, &output, &diagnostic
	env.Getenv = func(string) string { return "" }
	if code := memorycli.Run(t.Context(), []string{"init"}, env); code != 0 {
		t.Fatalf("memory initialization: %d %s", code, &diagnostic)
	}
	localGitCLI(t, root, "add", "--", "memory/policy.json")
	localGitCLI(t, root, "commit", "-m", "initialize memory")
	output.Reset()
	if code := memorycli.Run(t.Context(), []string{"reconcile", "--baseline-generate"}, env); code != 0 {
		t.Fatalf("initial inventory: %d %s", code, &diagnostic)
	}
	writeLocalLockFile(t, root, ".github/memory-reconciliation-baseline.json", output.Bytes())
	localGitCLI(t, root, "add", "--", ".github/memory-reconciliation-baseline.json")
	localGitCLI(t, root, "commit", "-m", "initial memory inventory")
	localGitCLI(t, root, "push", "upstream", "trunk")
	base := localGitCLI(t, root, "rev-parse", "HEAD")
	if code := memorycli.Run(t.Context(), []string{"reconcile", "--baseline-sync", "--migrate-v2", "--baseline-ref", base}, env); code != 0 {
		t.Fatalf("data-only inventory migration: %d %s", code, &diagnostic)
	}
	localGitCLI(t, root, "add", "--", ".github/memory-reconciliation-baseline.json", ".github/memory-reconciliation-baseline")
	localGitCLI(t, root, "commit", "-m", "migrate unchanged memory inventory")
	localGitCLI(t, root, "push", "upstream", "trunk")
	base = localGitCLI(t, root, "rev-parse", "HEAD")
	localGitCLI(t, root, "switch", "-c", "draft/widget")
	prd, err := os.ReadFile("../../internal/draft/compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(prd), "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1)
	body = strings.Replace(body, "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1)
	body = strings.Replace(body, "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1)
	body = strings.Replace(body, "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1) + "\n## 8. Contract readiness\n"
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	digest := compile.ContractDigest([]byte(body), tech)
	for name, content := range map[string][]byte{
		"prd.md": []byte(body), "tech-spec.md": tech,
		"events.md":      []byte("# Draft events\n"),
		"reviews/001.md": []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** " + digest + "\n"),
		"reviews/002.md": []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** " + digest + "\n"),
	} {
		writeLocalLockFile(t, root, "drafts/widget/"+name, content)
	}
	localGitCLI(t, root, "add", "--", "drafts/widget")
	localGitCLI(t, root, "commit", "-m", "ready reviewed draft")
	options := localgit.Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}
	if _, err := localgit.NewProposals(localgit.ProposalOptions{Git: options, Kind: "draft"}).Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget-v1", Feature: "widget", Branch: "refs/heads/draft/widget", Head: localGitCLI(t, root, "rev-parse", "HEAD"), State: "open", ExpectedTargetOID: base}); err != nil {
		t.Fatal(err)
	}
	return root, origin
}

func writeLocalLockFile(t *testing.T, root, name string, body []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalDraftCLIReviewsLocksAndVerifiesFromAnotherClone(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localLockCLIFixture(t)
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	swap(t, &draftCommand, func(context.Context, string, []string, string, ...string) ([]byte, error) {
		t.Fatal("local lock entered hosted workflow")
		return nil, nil
	})
	swap(t, &localDraftCommand, draftshell.RunLocalGit)
	call := func(args ...string) []byte {
		t.Helper()
		var out, diagnostic bytes.Buffer
		if code := draftVerb(args, &out, &diagnostic); code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, &out, &diagnostic)
		}
		return out.Bytes()
	}
	if !strings.Contains(string(call("digest", "widget")), "sha256:") {
		t.Fatal("local review digest missing")
	}
	packet := string(call("prepare-lock", "widget"))
	generation := localGitCLI(t, origin, "rev-parse", "refs/pyxis/proposals/draft-widget-v1")
	if !strings.Contains(packet, "review-request draft-widget-v1 --generation "+generation+" > request.json") {
		t.Fatal("packet's save command does not pin its displayed generation")
	}
	if !strings.Contains(packet, "Before record-review, display the complete saved binding") || !strings.Contains(packet, "\ncat request.json\n") {
		t.Fatal("operator flow does not display the full saved request before approval")
	}
	if !strings.Contains(packet, "## PRD §0") || !strings.Contains(packet, "draft-widget-v1") || strings.Contains(packet, "https://github.com") {
		t.Fatalf("wrong local preparation packet: %s", packet)
	}
	if resumed := string(call("prepare-lock", "widget")); resumed != packet {
		t.Fatal("prepared CLI retry changed or omitted the complete review packet")
	}
	request := filepath.Join(t.TempDir(), "request.json")
	requestBody := call("review-request", "draft-widget-v1", "--generation", generation)
	var presented codehost.ReviewBinding
	if err := json.Unmarshal(requestBody, &presented); err != nil || presented.Validate() != nil || presented.ProposalOID != generation || presented.TargetRef != "refs/heads/trunk" || presented.TargetOID != localGitCLI(t, origin, "rev-parse", "refs/heads/trunk") {
		t.Fatalf("displayed request omits the exact approval binding: %+v %v", presented, err)
	}
	if err := os.WriteFile(request, requestBody, 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := draftVerb([]string{"lock", "--request", request}, &out, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "receipt") {
		t.Fatalf("unreviewed lock accepted: %d %s", code, &diagnostic)
	}
	call("record-review", "--request", request, "--reviewer", "operator")
	var locked codehost.DraftLandingResult
	if err := json.Unmarshal(call("lock", "--request", request), &locked); err != nil || !locked.Created {
		t.Fatalf("lock result=%+v err=%v", locked, err)
	}
	if got := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk"); got != locked.Lock.MergeCommit {
		t.Fatal("configured mainline did not receive the lock")
	}
	var retry codehost.DraftLandingResult
	if err := json.Unmarshal(call("lock", "--request", request), &retry); err != nil || retry.Created || retry.Lock != locked.Lock {
		t.Fatalf("retry minted another lock: %+v %v", retry, err)
	}
	// A normal local landing does not install its tag in the author checkout.
	// Planning must make its advertised Basis usable by the actual plan fold.
	if tags := localGitCLI(t, root, "tag", "--list", "lock/widget/v1"); tags != "" {
		t.Fatal("fixture unexpectedly has the local lock tag before planning")
	}
	basis, defects := precheck.RunWithLocalLock(t.Context(), root, "widget", origin, draftshell.RunLocalGit,
		localgit.NewDraftLandings(localgit.Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}))
	if basis != "lock/widget/v1" || len(defects) != 0 {
		t.Fatalf("locked draft could not enter planning: %q %v", basis, defects)
	}
	writeLocalLockFile(t, root, "docs/plans/widget/001-plan.md", []byte(localLockedPlan))
	folded := plan.FoldFile(root, config.Defaults(), "docs/plans/widget/001-plan.md")
	if len(folded.Defects) != 0 || len(folded.JSON) == 0 {
		t.Fatalf("advertised local Basis could not fold its plan: %v", folded.Defects)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	localGitCLI(t, filepath.Dir(clone), "clone", "--origin", "upstream", "--branch", "trunk", origin, clone)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	root = clone
	var verified codehost.LocalDraftLock
	if err := json.Unmarshal(call("verify-lock", "widget", "--version", "1"), &verified); err != nil || verified != locked.Lock || verified.Review.Trust != (codehost.Trust{SharedAccount: true}) {
		t.Fatalf("clone verification=%+v err=%v", verified, err)
	}
	if exit := draftVerb([]string{"verify-lock", "widget", "--version", "1"}, draftReviewFailWriter{}, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "closed output") {
		t.Fatal("lost result output was not reported")
	}
	call("status", "widget")
}

const localLockedPlan = `# Widget capability — plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | prd/widget |
| **Design** | design/widget |
| **Basis** | lock/widget/v1 |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Open widgets locally |
| **Status** | Draft |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Open widgets and retain offline requests

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=2 |
| **Pattern** | — |
| **Touches** | internal/widget |

**Goal:** Opening a widget retains its request when offline.
**Current behavior:** No widget open operation exists.
**Desired behavior:** Opening returns the widget or its queued request identity.

**Scope:**
- widget opening and offline request retention
- compatibility with existing widgets

**Out of scope:**
- exporting widgets

**Acceptance criteria:**
- [ ] an offline open returns a retained request identity
- [ ] existing widgets open with the same identity
- [ ] a fixture open completes within one millisecond

**Key interfaces:**
- OpenWidget — returns a widget or queued request identity

**Traces:** prd:widget/FR-1 · prd:widget/NR-1 · prd:widget/NFR-1 · prd:widget/UB-1 · design:widget#D-1

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-1 | T1 |
| prd:widget/NR-1 | T1 |
| prd:widget/NFR-1 | T1 |
| prd:widget/UB-1 | T1 |
`

func TestLocalLockCLIRefusesInvalidInvocationAndConfiguration(t *testing.T) {
	localgittest.Skip(t)
	for _, args := range [][]string{{"lock"}, {"lock", "--unknown"}, {"lock", "--request", "x", "extra"}, {"verify-lock"}, {"verify-lock", "widget"}, {"verify-lock", "widget", "--version", "0"}, {"verify-lock", "widget", "--version", "1", "extra"}} {
		var out, diagnostic bytes.Buffer
		if code := draftVerb(args, &out, &diagnostic); code != 2 || diagnostic.Len() == 0 {
			t.Fatalf("bad invocation accepted: %v: %d %s", args, code, &diagnostic)
		}
	}
	for _, state := range []string{"outside repository", "invalid configuration", "hosted selection", "missing mainline", "missing request", "missing lock"} {
		t.Run(state, func(t *testing.T) {
			root, origin := localGitCLIFixture(t, state != "missing mainline")
			swap(t, &draftRepoRoot, func() (string, error) {
				if state == "outside repository" {
					return "", errors.New("not a repository")
				}
				return root, nil
			})
			if state == "invalid configuration" {
				writeLocalLockFile(t, root, ".pyxis/config.yaml", []byte("[bad YAML"))
			}
			if state == "hosted selection" {
				writeLocalLockFile(t, root, ".pyxis/config.yaml", []byte("integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n"))
			}
			before := localGitCLI(t, origin, "for-each-ref", "--format=%(refname) %(objectname)")
			args := []string{"lock", "--request", filepath.Join(t.TempDir(), "absent.json")}
			if state == "missing lock" {
				args = []string{"verify-lock", "widget", "--version", "1"}
			}
			var out, diagnostic bytes.Buffer
			if code := draftVerb(args, &out, &diagnostic); code != 1 || out.Len() != 0 || diagnostic.Len() == 0 {
				t.Fatalf("invalid lock context accepted: %d %s", code, &diagnostic)
			}
			if localGitCLI(t, origin, "for-each-ref", "--format=%(refname) %(objectname)") != before {
				t.Fatal("invalid invocation changed origin refs")
			}
		})
	}
	t.Setenv("ARGUS_DRIVE_SLUG", "widget")
	var diagnostic bytes.Buffer
	if code := draftVerb([]string{"lock", "--request", "request.json"}, &bytes.Buffer{}, &diagnostic); code != 1 || !strings.Contains(diagnostic.String(), "operator procedure") {
		t.Fatal("drive invoked operator lock")
	}
}

func TestInstalledLocalDraftMemoryReportsIntegrityRefusal(t *testing.T) {
	localgittest.Skip(t)
	root, _ := localGitCLIFixture(t, true)
	base := localGitCLI(t, root, "rev-parse", "HEAD")
	if err := checkLocalDraftMemory(t.Context(), root, base); err == nil || !strings.Contains(err.Error(), "memory gate refused") {
		t.Fatal("uninitialized integrity policy accepted")
	}
}

func TestLocalDraftCLIRefusesDirtyDigestAndFailedPreparationMemory(t *testing.T) {
	localgittest.Skip(t)
	root, origin := localLockCLIFixture(t)
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	before := localGitCLI(t, origin, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/", "refs/pyxis/proposals/", "refs/pyxis/proposal-archives/", "refs/tags/lock/")
	head := localGitCLI(t, root, "rev-parse", "HEAD")
	draftPath := filepath.Join(root, "drafts/widget/prd.md")
	original, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draftPath, append(append([]byte(nil), original...), []byte("\nUncommitted contract edit.\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := draftVerb([]string{"digest", "widget"}, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "commit") {
		t.Fatalf("dirty digest accepted: %d %s", code, &diagnostic)
	}
	if err := os.WriteFile(draftPath, original, 0644); err != nil {
		t.Fatal(err)
	}
	swap(t, &localDraftMemory, func(context.Context, string, string) error {
		return errors.New("simulated installed memory refusal")
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"prepare-lock", "widget"}, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "simulated installed memory refusal") {
		t.Fatalf("failed memory preparation accepted: %d %s", code, &diagnostic)
	}
	if localGitCLI(t, root, "rev-parse", "HEAD") != head || localGitCLI(t, origin, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/", "refs/pyxis/proposals/", "refs/pyxis/proposal-archives/", "refs/tags/lock/") != before {
		t.Fatal("refused preparation partially published or changed the working draft")
	}
}
