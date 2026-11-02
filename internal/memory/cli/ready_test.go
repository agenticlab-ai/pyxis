package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
)

func TestReadyVerbEmitsOneClosedVerdictAndExitCode(t *testing.T) {
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "docs/prds/other/effective.source.json", `{"units":{"FR-1":{"status":"live"}}}`, 0o644)
	writeFixtureFile(t, h.dir, "docs/tech-design/other.md", "# Other design\n", 0o644)
	writeFixtureFile(t, h.dir, "docs/prds/other/notes.txt", "ignored", 0o644)
	h.git("add", "memory/policy.json", "docs/prds/other/effective.source.json", "docs/tech-design/other.md", "docs/prds/other/notes.txt")
	h.git("commit", "-q", "-m", "init memory")
	h.git("checkout", "-q", "-b", "feature/widget")
	prd, err := os.ReadFile("../../draft/compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	prd = []byte(strings.Replace(string(prd), "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1))
	prd = []byte(strings.Replace(string(prd), "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1))
	prd = []byte(strings.Replace(string(prd), "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1))
	prd = []byte(strings.Replace(string(prd), "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1))
	prd = append(prd, []byte("\n## 8. Contract readiness\n")...)
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n")
	result, err := compile.Compile("widget", "main", map[string][]byte{"prd.md": prd, "tech-spec.md": tech})
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, "drafts/widget/prd.md", string(prd), 0o644)
	writeFixtureFile(t, h.dir, "drafts/widget/tech-spec.md", string(tech), 0o644)
	writeFixtureFile(t, h.dir, "drafts/widget/reviews/001.md", "# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+result.Digest+"\n", 0o644)
	writeFixtureFile(t, h.dir, "drafts/widget/reviews/002.md", "# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+result.Digest+"\n", 0o644)
	h.git("add", "drafts/widget/prd.md", "drafts/widget/tech-spec.md", "drafts/widget/reviews/001.md", "drafts/widget/reviews/002.md")
	h.git("commit", "-q", "-m", "draft widget")
	code, out, errout := h.run("ready", "widget")
	if code != 0 || errout != "" {
		t.Fatalf("ready: exit %d, stderr %q, stdout %s", code, errout, out)
	}
	var verdict readiness.Verdict
	if err := json.Unmarshal([]byte(out), &verdict); err != nil || !verdict.Ready || len(verdict.Unmet) != 0 || verdict.SchemaVersion != "draft-readiness/v1" {
		t.Fatalf("ready verdict: %+v, %v", verdict, err)
	}
	readyHead := h.git("rev-parse", "HEAD")
	code, hosted, errout := h.run("ready", "--all", "--base", "main", "--head", readyHead)
	if code != 0 || errout != "" || hosted != out {
		t.Fatalf("hosted verdict differs from local: exit %d stderr %q hosted %s local %s", code, errout, hosted, out)
	}
	h.git("checkout", "-q", "main")
	h.git("checkout", "-q", "-b", "ci-merge")
	h.git("merge", "--no-ff", "-q", "-m", "synthetic pull request merge", "feature/widget")
	h.envs["PYXIS_CI_EVENT"] = "pull_request"
	code, hosted, errout = h.run("ready", "--all", "--base", "main")
	if code != 0 || errout != "" || hosted != out {
		t.Fatalf("merge checkout must judge parent-two head: exit %d stderr %q hosted %s local %s", code, errout, hosted, out)
	}
	delete(h.envs, "PYXIS_CI_EVENT")
	h.git("checkout", "-q", "feature/widget")
	h.git("rm", "drafts/widget/tech-spec.md")
	h.git("commit", "-q", "-m", "remove tech spec")
	code, out, errout = h.run("ready", "widget")
	if code != 1 || errout != "" {
		t.Fatalf("missing part: exit %d, stderr %q, stdout %s", code, errout, out)
	}
	if err := json.Unmarshal([]byte(out), &verdict); err != nil || len(verdict.Unmet) != 1 || verdict.Unmet[0].Condition != "UB-7" || verdict.Unmet[0].Item != "drafts/widget/tech-spec.md" {
		t.Fatalf("missing part verdict: %+v, %v", verdict, err)
	}
	code, hosted, errout = h.run("ready", "--all", "--base", "main")
	if code != exitRefused || errout != "" || hosted != out {
		t.Fatalf("hosted refusal differs from local: exit %d stderr %q hosted %s local %s", code, errout, hosted, out)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"ready", "widget", "extra"}, exitUsage, "one feature"},
		{[]string{"ready", "--unknown", "widget"}, exitUsage, "flag provided but not defined"},
		{[]string{"ready", "widget", "--base", "missing-ref"}, exitRefused, "resolve mainline"},
		{[]string{"ready", "widget", "--head", "missing-ref"}, exitRefused, "resolve draft head"},
	} {
		code, _, stderr := h.run(tc.args...)
		if code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%v: exit %d stderr %q, want %d and %q", tc.args, code, stderr, tc.code, tc.want)
		}
	}
	env, _, stderr := h.env("")
	env.Stdout = gateBrokenWriter{}
	if code := Run(context.Background(), []string{"ready", "widget"}, env); code != exitRefused || !strings.Contains(stderr.String(), "write readiness verdict") {
		t.Fatalf("output failure: exit %d, stderr %q", code, stderr.String())
	}
	h.git("rm", "-r", "drafts/widget")
	h.git("commit", "-q", "-m", "remove completed draft")
	code, out, errout = h.run("ready", "--all", "--base", "main")
	if code != 0 || errout != "" || out != "{\"schema_version\":\"draft-ready-pr/v1\",\"message\":\"no draft\"}\n" {
		t.Fatalf("unrelated proposal should name no draft: exit %d stderr %q stdout %s", code, errout, out)
	}
	env, _, stderr = h.env("")
	env.Stdout = gateBrokenWriter{}
	if code := Run(context.Background(), []string{"ready", "--all", "--base", "main"}, env); code != exitRefused || !strings.Contains(stderr.String(), "write readiness verdict") {
		t.Fatalf("no-draft output failure: exit %d, stderr %q", code, stderr.String())
	}
}

func TestReadyPartDiagnostics(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"draft part drafts/widget/brainstorm/001.md: symlink", "drafts/widget/brainstorm/001.md"},
		{"drafts/widget/unknown.md", "drafts/widget/unknown.md"},
		{"compile draft widget PRD: malformed", "drafts/widget/prd.md"},
		{"compile draft widget tech spec: malformed", "drafts/widget/tech-spec.md"},
		{"compile draft review reviews/001.md: missing digest", "drafts/widget/reviews/001.md"},
		{"compile draft review reviews/002.md", "drafts/widget/reviews/002.md"},
		{"compile draft widget problem P-1: invalid", "drafts/widget/problems.md"},
		{"unclassified draft failure", "drafts/widget"},
	} {
		if got := compilePart("drafts", "widget", errors.New(tc.input)); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestReadyUsesConfiguredDraftRoot(t *testing.T) {
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "schema_version: pyxis-config/v2\npaths:\n  drafts: contracts\n", 0o644)
	h.git("add", "memory/policy.json", ".pyxis/config.yaml")
	h.git("commit", "-q", "-m", "configure draft root")
	h.git("checkout", "-q", "-b", "feature/widget")
	writeFixtureFile(t, h.dir, "contracts/widget/prd.md", "# Widget PRD\n", 0o644)
	h.git("add", "contracts/widget/prd.md")
	h.git("commit", "-q", "-m", "open widget")
	code, out, stderr := h.run("ready", "--all", "--base", "main")
	if code != exitRefused || stderr != "" || !strings.Contains(out, "contracts/widget/tech-spec.md") {
		t.Fatalf("configured draft root: exit %d stderr %q stdout %s", code, stderr, out)
	}
}

func TestReadyFailsClosedOnMalformedMainlineMemory(t *testing.T) {
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "memory/prd/other/Promise-invalid.json", "{broken", 0o644)
	h.git("add", "memory/policy.json", "memory/prd/other/Promise-invalid.json")
	h.git("commit", "-q", "-m", "malformed source fixture")
	h.git("checkout", "-q", "-b", "feature/widget")
	code, _, stderr := h.run("ready", "widget", "--base", "main")
	if code != exitRefused || !strings.Contains(stderr, "mainline memory has unresolved store defects") {
		t.Fatalf("malformed mainline: exit %d stderr %q", code, stderr)
	}
}
