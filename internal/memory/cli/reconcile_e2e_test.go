package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestE2EOfflineReconcileCloseoutRetirementAndDrift(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	seedReconciliationFixture(t, h)
	h.ok("init")

	before := memoryRecordCount(t, h.dir)
	code, out, errout := h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("initial reconciliation must find gaps: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	initial := decodeReconciliationRun(t, out)
	if !initial.Report.Blocking() || initial.Report.Counts.Actions == 0 {
		t.Fatalf("initial report did not expose writable gaps: %+v", initial.Report.Counts)
	}
	if after := memoryRecordCount(t, h.dir); after != before {
		t.Fatalf("--check wrote memory records: before %d, after %d", before, after)
	}

	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitRefused {
		t.Fatalf("lossless import should remain blocked on semantic gaps: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	written := decodeReconciliationRun(t, out)
	if written.Written == 0 || !hasFinding(written.Report.Findings, "missing-design-memory") || hasFinding(written.Report.Findings, "missing-code-memory") {
		t.Fatalf("write pass did not separate safe imports from semantic gaps: written=%d findings=%+v", written.Written, findingCodes(written.Report.Findings))
	}

	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK {
		t.Fatalf("design distillation pass failed: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	distilled := decodeReconciliationRun(t, out)
	if distilled.Written != 2 || distilled.Report.Blocking() || distilled.Report.Counts.Actions != 0 {
		t.Fatalf("design distillation did not reach a fixed point: written=%d counts=%+v", distilled.Written, distilled.Report.Counts)
	}

	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitOK {
		t.Fatalf("aligned feature did not reconcile: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	aligned := decodeReconciliationRun(t, out)
	if aligned.Report.Blocking() || aligned.Report.Counts.Actions != 0 {
		t.Fatalf("aligned report is not a fixed point: %+v", aligned.Report.Counts)
	}
	code, out, errout = h.run("reconcile", "--write", "--json")
	if code != exitOK {
		t.Fatalf("idempotent reconcile --write failed: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	if got := decodeReconciliationRun(t, out).Written; got != 0 {
		t.Fatalf("idempotent reconcile appended %d records", got)
	}

	code, out, errout = h.run("closeout", "--feature", "widget", "--check", "--json")
	if code != exitOK {
		t.Fatalf("complete feature was not closeout-ready: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	var ready closeoutOutput
	decodeJSON(t, out, &ready)
	if ready.Status != "ready" || ready.Receipt.Feature != "widget" || len(ready.Receipt.CodeUnits) != 2 || len(ready.Receipt.SourceDocuments) != 4 {
		t.Fatalf("unexpected readiness receipt: %+v", ready)
	}
	zsh := ready.Receipt.CodeUnits[0]
	if zsh.Kind != codeinventory.ExecutableScript || zsh.Path != ".pyxis/scripts/startup/codex-worktree.zsh" || len(zsh.Files) != 1 || zsh.Files[0] != zsh.Path {
		t.Fatalf("closeout omitted sourced zsh binding: %#v", ready.Receipt.CodeUnits)
	}

	code, out, errout = h.run("closeout", "--feature", "widget", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK {
		t.Fatalf("closeout write failed: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	var sealed closeoutOutput
	decodeJSON(t, out, &sealed)
	if sealed.Status != "sealed" || sealed.RecordID == "" {
		t.Fatalf("closeout did not return its seal: %+v", sealed)
	}
	sealedCount := memoryRecordCount(t, h.dir)
	code, out, errout = h.run("closeout", "--feature", "widget", "--write", "--json")
	if code != exitOK {
		t.Fatalf("idempotent closeout failed: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	var sealedAgain closeoutOutput
	decodeJSON(t, out, &sealedAgain)
	if sealedAgain.RecordID != sealed.RecordID || memoryRecordCount(t, h.dir) != sealedCount {
		t.Fatalf("idempotent closeout appended a second seal: first=%s second=%s", sealed.RecordID, sealedAgain.RecordID)
	}
	h.commit("capture memory and closeout")

	retireReconciliationSources(t, h.dir)
	h.commit("retire completed feature context")
	code, out, errout = h.run("verify", "--offline", "--json")
	if code != exitOK {
		t.Fatalf("valid closeout did not authorize source retirement: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	var verified verificationOutput
	decodeJSON(t, out, &verified)
	if verified.Report.Blocking() || strings.Join(verified.Closeouts, ",") != "widget" {
		t.Fatalf("retired-source verification is not green: closeouts=%v findings=%v", verified.Closeouts, findingCodes(verified.Report.Findings))
	}

	code, _, errout = h.run("add", "--kind", "Promise", "--subject", "prd/widget", "--family", "orphan-code-scope",
		"--statement", "A nonexistent package remains associated.", "--tag", "code:internal/ghost")
	if code != exitRefused || !strings.Contains(errout, "evidence-membership-mismatch") {
		t.Fatalf("writer accepted unresolved evidence: %d %s", code, errout)
	}
	// The reconciler must still describe inherited legacy associations, even
	// though the upgraded writer refuses to create them.
	dangling := h.add("--kind", "Promise", "--subject", "prd/widget", "--family", "orphan-code-scope", "--statement", "A nonexistent package remains associated.")
	legacy, _ := readEvidenceRecord(t, h, dangling)
	legacy.Tags = []string{"code:internal/ghost"}
	writeEvidenceRecord(t, h, legacy)
	h.commit("retain a legacy association fixture")
	code, out, errout = h.run("verify", "--offline", "--json")
	if code != exitRefused {
		t.Fatalf("memory with no associated code must fail verification: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	decodeJSON(t, out, &verified)
	if !hasFinding(verified.Report.Findings, "dangling-code-scope") {
		t.Fatalf("dangling code memory was not classified: %v", findingCodes(verified.Report.Findings))
	}
	h.ok("revoke", dangling, "--reason", "the package never existed")
	code, out, errout = h.run("verify", "--offline", "--json")
	if code != exitOK {
		t.Fatalf("revoking dangling memory did not restore integrity: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}

	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\n\nfunc Enabled() bool { return false }\n", 0o644)
	h.commit("drift widget implementation")
	code, out, errout = h.run("verify", "--offline", "--json")
	if code != exitRefused {
		t.Fatalf("code drift must fail verification: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	decodeJSON(t, out, &verified)
	if !hasFinding(verified.Report.Findings, "closeout-code-drift") {
		t.Fatalf("code drift was not tied to the closeout: %v", findingCodes(verified.Report.Findings))
	}
	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitRefused {
		t.Fatalf("the scheduled reconciliation path must see closeout drift: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	if report := decodeReconciliationRun(t, out).Report; !hasFinding(report.Findings, "closeout-code-drift") {
		t.Fatalf("reconcile omitted closeout drift seen by verify: %v", findingCodes(report.Findings))
	}
}

func TestE2EBaselineAllowsCodeChangesWithoutInventedKnowledge(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\n\nfunc Enabled() bool { return true }\n", 0o644)
	h.commit("add widget implementation")

	code, out, errout := h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK || hasFinding(decodeReconciliationRun(t, out).Report.Findings, "missing-code-memory") {
		t.Fatalf("code inventory required semantic capture: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	h.add("--kind", "Decision", "--subject", "design/widget", "--family", "implementation",
		"--statement", "Keep the widget enabled by default.",
		"--rationale", "Existing clients depend on opt-out activation.", "--tag", "code:internal/widget")
	h.commit("align widget memory")

	code, baseline, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("trusted baseline generation = exit %d\nstdout: %s\nstderr: %s", code, baseline, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("record aligned reconciliation baseline")
	trusted := h.git("rev-parse", "HEAD")

	before := memoryRecordCount(t, h.dir)
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\n\nconst enabled = true\nfunc Enabled() bool { return enabled }\n", 0o644)
	h.commit("refactor widget under its existing decision")
	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK || errout != "" || decodeReconciliationRun(t, out).Written != 0 || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("code refactor manufactured memory: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	code, baseline, errout = h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("candidate baseline generation = exit %d\nstdout: %s\nstderr: %s", code, baseline, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("record current inventory")

	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted, "--json")
	if code != exitOK || errout != "" || strings.Contains(out, "code-memory-transition") {
		t.Fatalf("existing governing decision could not be reused: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestE2EBaselineDoesNotInventRetirementLearningForUnidentifiedCode(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	codePath := "internal/legacy/legacy.go"
	codeBody := []byte("package legacy\n\nfunc Enabled() bool { return true }\n")
	writeFixtureFile(t, h.dir, codePath, string(codeBody), 0o644)
	h.commit("add uncovered legacy code")

	before := memoryRecordCount(t, h.dir)
	code, baseline, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("trusted legacy baseline generation = exit %d\nstdout: %s\nstderr: %s", code, baseline, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("record uncovered legacy debt")
	trusted := h.git("rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(h.dir, filepath.FromSlash(codePath))); err != nil {
		t.Fatal(err)
	}
	h.commit("delete uncovered legacy code")
	code, baseline, errout = h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("unacknowledged deletion baseline generation = exit %d\nstdout: %s\nstderr: %s", code, baseline, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
	h.commit("attempt to accept unacknowledged deletion")
	code, out, errout := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted, "--json")
	if code != exitOK || errout != "" || strings.Contains(out, "code-memory-transition") || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("unidentified code deletion required new knowledge: exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestE2EBaselineRejectsRetirementWhenTrustedSourceBytesPostdateCloseout(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	seedReconciliationFixture(t, h)
	brainstormPath := "docs/brainstorms/widget.md"
	brainstorm, err := os.ReadFile(filepath.Join(h.dir, filepath.FromSlash(brainstormPath)))
	if err != nil {
		t.Fatal(err)
	}
	h.ok("init")

	code, out, errout := h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitRefused {
		t.Fatalf("initial source capture = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK {
		t.Fatalf("design distillation pass = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	if written := decodeReconciliationRun(t, out); written.Written != 2 || written.Report.Blocking() || written.Report.Counts.Actions != 0 {
		t.Fatalf("design distillation did not reach a fixed point: written=%d counts=%+v", written.Written, written.Report.Counts)
	}
	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitOK {
		t.Fatalf("aligned feature = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	code, out, errout = h.run("closeout", "--feature", "widget", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK {
		t.Fatalf("closeout = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	h.commit("seal widget before context correction")

	writeFixtureFile(t, h.dir, brainstormPath, string(brainstorm)+"\nPost-closeout correction.\n", 0o644)
	h.commit("correct source after closeout")
	code, generated, errout := h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("trusted baseline generation = exit %d\nstdout: %s\nstderr: %s", code, generated, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, generated, 0o644)
	h.commit("record corrected source debt")
	trusted := h.git("rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(h.dir, filepath.FromSlash(brainstormPath))); err != nil {
		t.Fatal(err)
	}
	h.commit("attempt to retire unsealed corrected bytes")
	code, generated, errout = h.run("reconcile", "--baseline-generate")
	if code != exitOK || errout != "" {
		t.Fatalf("candidate baseline generation = exit %d\nstdout: %s\nstderr: %s", code, generated, errout)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, generated, 0o644)
	h.commit("attempt deletion baseline")

	code, out, errout = h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
	if code != exitRefused || !strings.Contains(out, "authored-source-retirement "+brainstormPath) || !strings.Contains(errout, "checked-in reconciliation baseline widens") {
		t.Fatalf("post-closeout source edit/delete check = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func TestE2ECloseoutRefusesTopicOnlySourceCommit(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	seedReconciliationFixture(t, h)
	alignReconciliationFixture(t, h)
	h.commit("land completed feature and memory")

	code, out, errout := h.run("closeout", "--feature", "widget", "--check", "--json")
	if code != exitOK {
		t.Fatalf("mainline closeout readiness = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	h.git("checkout", "-q", "-b", "feature/topic-only-closeout")
	h.git("commit", "-q", "--allow-empty", "-m", "topic-only source commit")

	before := memoryRecordCount(t, h.dir)
	for _, args := range [][]string{
		{"closeout", "--feature", "widget", "--check", "--json"},
		{"closeout", "--feature", "widget", "--write", "--actor", "agent:e2e", "--json"},
	} {
		code, out, errout = h.run(args...)
		if code != exitRefused || !strings.Contains(errout, "closeout source must be the trusted mainline main") || !strings.Contains(errout, "land the completed feature and its memory") {
			t.Fatalf("topic closeout %v = exit %d\nstdout: %s\nstderr: %s", args, code, out, errout)
		}
	}
	if after := memoryRecordCount(t, h.dir); after != before {
		t.Fatalf("refused topic closeout wrote memory records: before %d, after %d", before, after)
	}
}

func alignReconciliationFixture(t *testing.T, h *harness) {
	t.Helper()
	h.ok("init")
	code, out, errout := h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitRefused {
		t.Fatalf("initial source capture = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	code, out, errout = h.run("reconcile", "--write", "--actor", "agent:e2e", "--json")
	if code != exitOK {
		t.Fatalf("design distillation pass = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	if written := decodeReconciliationRun(t, out); written.Written != 2 || written.Report.Blocking() || written.Report.Counts.Actions != 0 {
		t.Fatalf("design distillation did not reach a fixed point: written=%d counts=%+v", written.Written, written.Report.Counts)
	}
	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitOK {
		t.Fatalf("aligned feature = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
}

func seedReconciliationFixture(t *testing.T, h *harness) {
	t.Helper()
	base := []byte(`# Widget — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v1 |

## 5. Requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When enabled, the widget shall report ready. | callers need a stable readiness signal | Must | med | Enabled returns true | Test | G1 |
`)
	result := fold.Fold([]ledger.Entry{{Name: "000-base.md", Bytes: base}})
	if len(result.Defects) != 0 {
		t.Fatalf("fold fixture PRD: %v", result.Defects)
	}
	design := []byte(`# Widget v1

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @owner |
| **Approver(s)** | @owner |
| **Date** | 2026-09-08 |
| **Relevant-when** | changing widget readiness |
| **Scope** | widget package |
| **Tracking** | — |
| **Builds-on** | — |
| **Supersedes** | — |
| **Serves** | prd:widget/FR-1 |
| **Security/privacy impact** | none — local deterministic package |
| **Template version** | tech-design-template/v1 |

## 3. The design

The widget exposes one deterministic readiness function.

**Decision:** Implement the readiness behavior in ` + "`internal/widget`" + ` and isolate interactive launches in ` + "`.pyxis/scripts/startup/codex-worktree.zsh`" + `.

### 3.4 Observability

none — pure local function.

### 3.5 Security & privacy

none — no data or external boundary.
`)
	parsed, defects := techdesigns.Parse("widget-v1", design)
	if parsed == nil || len(defects) != 0 {
		t.Fatalf("parse fixture design: %v", defects)
	}
	env := techdesigns.Env{
		Grandfathered:           map[string]bool{},
		PrecedenceGrandfathered: map[string]bool{},
		PRDUnits:                map[string]map[string]string{"widget": {"FR-1": "live"}},
		CorpusIDs:               map[string]bool{}, NewDesigns: map[string]bool{},
		DocExists: func(string) bool { return true },
	}
	indexMD, indexJSON, defects := techdesigns.GenIndex([]*techdesigns.Design{parsed}, env)
	if len(defects) != 0 {
		t.Fatalf("generate fixture design index: %v", defects)
	}
	writeFixtureFile(t, h.dir, "docs/prds/widget/000-base.md", string(base), 0o644)
	writeFixtureFile(t, h.dir, "docs/prds/widget/effective.md", string(result.EffectiveMD), 0o644)
	writeFixtureFile(t, h.dir, "docs/prds/widget/effective.source.json", string(result.SourceJSON), 0o644)
	writeFixtureFile(t, h.dir, "docs/tech-design/widget-v1.md", string(design), 0o644)
	writeFixtureFile(t, h.dir, "docs/tech-design/index.md", string(indexMD), 0o644)
	writeFixtureFile(t, h.dir, "docs/tech-design/index.json", string(indexJSON), 0o644)
	writeFixtureFile(t, h.dir, "docs/brainstorms/widget.md", `# Widget — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Shape widget readiness. |
| **Relevant-when** | Changing widget behavior. |
| **Idea** | A ready widget. |
| **Operator** | @owner |
| **Date(s)** | 2026-09-08 |
| **Status** | gated: proceed |
| **Tracking** | — |
| **Feeds** | docs/prds/widget/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Idea

A ready widget.
`, 0o644)
	writeFixtureFile(t, h.dir, "docs/reviews/widget-000.md", `# Review: widget base — review report

| Field | Value |
|---|---|
| **In one line** | Widget contract approved. |
| **Relevant-when** | Auditing widget requirements. |
| **Subject** | `+"`prd:widget/000`"+` |
| **Moment** | base |
| **Reviewer** | prd-review |
| **Date(s)** | 2026-09-08 |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

No findings.
`, 0o644)
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\n\nfunc Enabled() bool { return true }\n", 0o644)
	writeFixtureFile(t, h.dir, ".pyxis/scripts/startup/codex-worktree.zsh", "codex() { command codex \"$@\"; }\n", 0o644)
	h.commit("add completed widget feature")
}

func retireReconciliationSources(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{
		"docs/prds/widget/000-base.md",
		"docs/prds/widget/effective.md",
		"docs/prds/widget/effective.source.json",
		"docs/tech-design/widget-v1.md",
		"docs/tech-design/index.md",
		"docs/tech-design/index.json",
		"docs/brainstorms/widget.md",
		"docs/reviews/widget-000.md",
	} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("retire %s: %v", name, err)
		}
	}
}

func writeFixtureFile(t *testing.T, root, name, body string, mode fs.FileMode) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func memoryRecordCount(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(filepath.Join(root, "memory"), func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(name, ".json") && filepath.Base(name) != "policy.json" {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func decodeReconciliationRun(t *testing.T, body string) reconciliationOutput {
	t.Helper()
	var out reconciliationOutput
	decodeJSON(t, body, &out)
	return out
}

func decodeJSON(t *testing.T, body string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), out); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, body)
	}
}

func hasFinding(findings []reconcile.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func findingCodes(findings []reconcile.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Code)
	}
	return out
}
