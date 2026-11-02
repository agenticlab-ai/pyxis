package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

func closedGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func closedWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func closedFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "closed", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func closedRepo(t *testing.T, module string) (string, string) {
	t.Helper()
	root := t.TempDir()
	closedGit(t, root, "init", "-q")
	closedGit(t, root, "config", "user.name", "Gate Test")
	closedGit(t, root, "config", "user.email", "gate@example.test")
	closedWrite(t, root, "go.mod", "module "+module+"\n\ngo 1.25\n")
	closedWrite(t, root, "docs/prds/existing/000-base.md", "# Existing PRD\n")
	closedWrite(t, root, "docs/tech-design/existing.md", "# Existing design\n")
	closedGit(t, root, "add", "go.mod", "docs/prds/existing/000-base.md", "docs/tech-design/existing.md")
	closedGit(t, root, "commit", "-qm", "base")
	return root, closedGit(t, root, "rev-parse", "HEAD")
}

func TestLegacyProposalChecksRetiredForPyxis(t *testing.T) {
	cfg := config.Defaults()
	for _, tc := range []struct {
		name, module     string
		standaloneBlocks bool
	}{
		{"Pyxis", "github.com/agenticlab-ai/pyxis", false},
		{"installed document pipeline", "example.test/another-repo", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base := closedRepo(t, tc.module)
			closedWrite(t, root, cfg.PRDs+"/existing/001-change.md", "# Amendment\n")
			closedWrite(t, root, cfg.TechDesigns+"/new.md", "# Design\n")
			closedGit(t, root, "add", cfg.PRDs+"/existing/001-change.md", cfg.TechDesigns+"/new.md")
			closedGit(t, root, "commit", "-qm", "contract change")
			for _, verb := range []string{"check-contract-corpus-closed", "check-standalone-docs"} {
				check, ok := gate.Lookup(verb)
				if !ok {
					t.Fatalf("missing %s", verb)
				}
				got := check.Run(gate.Ctx{Root: root, MergeBase: base, Head: "HEAD", Cfg: cfg})
				want := verb == "check-standalone-docs" && tc.standaloneBlocks
				if (len(got) > 0) != want {
					t.Fatalf("%s defects = %v; want block %t", verb, got, want)
				}
			}
		})
	}
}

func TestDocumentUmbrellaRetiredForPyxis(t *testing.T) {
	cfg := config.Defaults()
	for _, tc := range []struct {
		name, module string
		wantBypass   bool
	}{
		{"Pyxis source", "github.com/agenticlab-ai/pyxis", true},
		{"installed repository", "example.test/another-repo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base := closedRepo(t, tc.module)
			closedWrite(t, root, cfg.TechDesigns+"/new.md", "# New design\n\nUse `.pyxis/bin` directly.\n")
			closedGit(t, root, "add", cfg.TechDesigns+"/new.md")
			closedGit(t, root, "commit", "-qm", "add old-style design")
			for _, ref := range []string{base, ""} {
				got := gate.Gate(root, ref, nil, cfg)
				if (len(got) == 0) != tc.wantBypass {
					t.Fatalf("Gate at %q defects = %v; want bypass %t", ref, got, tc.wantBypass)
				}
			}
			got := gate.GateAt(root, base, "HEAD", nil, cfg)
			if (len(got) == 0) != tc.wantBypass {
				t.Fatalf("pre-push GateAt defects = %v; want bypass %t", got, tc.wantBypass)
			}
		})
	}
}

func TestPyxisGateStillRefusesUnattributedProtectedChanges(t *testing.T) {
	cfg := config.Defaults()
	root, base := closedRepo(t, "github.com/agenticlab-ai/pyxis")
	protected := ".pyxis/skills/argus/authority-contract.md"
	closedWrite(t, root, protected, "# Changed authority\n")
	closedGit(t, root, "add", protected)
	closedGit(t, root, "commit", "-qm", "unattributed authority change")
	for _, defects := range []gate.Defects{
		gate.Gate(root, base, nil, cfg),
		gate.GateAt(root, base, "HEAD", nil, cfg),
	} {
		if !hasGateVerb(defects, attribution.Verb) || !hasGateVerb(defects, "check-authority-contract") {
			t.Fatalf("Pyxis gate defects = %v; want attribution and authority refusals", defects)
		}
		for _, defect := range defects {
			if defect.File != protected {
				t.Fatalf("unexpected defect outside protected path: %v", defect)
			}
		}
	}
	if defects := gate.Gate(root, "", nil, cfg); len(defects) != 1 || defects[0].Verb != "check-authority-contract" {
		t.Fatalf("base-free desk run = %v; want only authority structure without merge base", defects)
	}
}

func TestPyxisGateRetainsArgusAuthorityAndLedgerChecks(t *testing.T) {
	cfg := config.Defaults()
	t.Run("authority structure without merge base", func(t *testing.T) {
		root, _ := closedRepo(t, "github.com/agenticlab-ai/pyxis")
		closedWrite(t, root, ".pyxis/skills/argus/authority-contract.md", "# malformed authority contract\n")
		closedGit(t, root, "add", ".pyxis/skills/argus/authority-contract.md")
		closedGit(t, root, "commit", "-qm", "malformed authority")
		if !hasGateVerb(gate.Gate(root, "", nil, cfg), "check-authority-contract") {
			t.Fatal("Pyxis desk gate skipped authority-contract structure")
		}
	})
	t.Run("merged ledger entry changed", func(t *testing.T) {
		root, _ := closedRepo(t, "github.com/agenticlab-ai/pyxis")
		path := cfg.Problems + "/foo/delivery.md"
		closedWrite(t, root, path, histDelivery)
		closedGit(t, root, "add", path)
		closedGit(t, root, "commit", "-qm", "seed delivery")
		base := closedGit(t, root, "rev-parse", "HEAD")
		closedWrite(t, root, path, strings.Replace(histDelivery, "the first entry", "rewritten", 1))
		closedGit(t, root, "add", path)
		closedGit(t, root, "commit", "-qm", "rewrite merged entry")
		if !hasGateVerb(gate.Gate(root, base, nil, cfg), "check-ledger-history") {
			t.Fatal("Pyxis merge gate skipped add-only ledger history")
		}
	})
	t.Run("citation into derived state", func(t *testing.T) {
		root, _ := closedRepo(t, "github.com/agenticlab-ai/pyxis")
		ledger := cfg.Problems + "/seeded-demo/delivery.md"
		closedWrite(t, root, ledger, seededDelivery)
		closedWrite(t, root, cfg.Wiki+"/summary.md", "See delivery:seeded-demo#Status for current status.\n")
		closedGit(t, root, "add", ledger, cfg.Wiki+"/summary.md")
		closedGit(t, root, "commit", "-qm", "cite derived state")
		if !hasGateVerb(gate.Gate(root, "", nil, cfg), "check-ledger-citations") {
			t.Fatal("Pyxis desk gate skipped delivery-ledger citation check")
		}
	})
}

func hasGateVerb(defects gate.Defects, verb string) bool {
	for _, defect := range defects {
		if defect.Verb == verb {
			return true
		}
	}
	return false
}

func TestContractCorpusClosedAdditions(t *testing.T) {
	check, ok := gate.Lookup("check-contract-corpus-closed")
	if !ok || !check.NeedsMergeBase {
		t.Fatal("closure check must self-register as a merge-base check")
	}
	cfg := config.Defaults()
	contract := closedFixture(t, "contract.md")
	plan := closedFixture(t, "plan.md")
	for _, tc := range []struct {
		name, file, content, kind string
		refuse                    bool
	}{
		{"PRD base", cfg.PRDs + "/new/000-base.md", contract, "PRD ledger entry", true},
		{"PRD amendment", cfg.PRDs + "/existing/001-change.md", contract, "PRD ledger entry", true},
		{"technical design", cfg.TechDesigns + "/new.md", contract, "technical design", true},
		{"brainstorm record", cfg.Brainstorms + "/new.md", contract, "brainstorm record", true},
		{"review report", cfg.Reviews + "/new.md", contract, "review report", true},
		{"plan", cfg.Plans + "/new/002-plan.md", plan, "", false},
		{"PRD projection", cfg.PRDs + "/existing/effective.md", contract, "", false},
		{"design index", cfg.TechDesigns + "/index.md", contract, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base := closedRepo(t, "github.com/agenticlab-ai/pyxis")
			closedWrite(t, root, tc.file, tc.content)
			closedGit(t, root, "add", tc.file)
			closedGit(t, root, "commit", "-qm", "addition")
			defects := gate.CheckContractCorpusClosed(root, base, "HEAD", cfg)
			if !tc.refuse {
				if len(defects) != 0 {
					t.Fatalf("accepted path refused: %v", defects)
				}
				return
			}
			if len(defects) != 1 || defects[0].File != tc.file || !strings.Contains(defects[0].Reason, tc.kind) || !strings.Contains(defects[0].Remedy, "feature draft") {
				t.Fatalf("expected named closure and draft remedy: %v", defects)
			}
		})
	}
}

func TestContractCorpusClosedKeepsImmutableCheck(t *testing.T) {
	cfg := config.Defaults()
	root, base := closedRepo(t, "github.com/agenticlab-ai/pyxis")
	closedWrite(t, root, "docs/prds/existing/000-base.md", "# Edited PRD\n")
	closedWrite(t, root, "docs/tech-design/existing.md", "# Edited design\n")
	closedGit(t, root, "commit", "-qam", "edit")
	if d := gate.CheckContractCorpusClosed(root, base, "HEAD", cfg); len(d) != 0 {
		t.Fatalf("closure check must leave edits to existing checks: %v", d)
	}
	if d := gate.CheckImmutable(root, base, "HEAD", cfg); len(d) != 2 {
		t.Fatalf("existing immutable checks must still refuse both edits: %v", d)
	}
}

func TestContractCorpusClosedIdentityAndRename(t *testing.T) {
	cfg := config.Defaults()
	for _, tc := range []struct {
		name, module string
		refuse       bool
	}{
		{"Pyxis", "github.com/agenticlab-ai/pyxis", true},
		{"installed docgate", "example.test/another-repo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, base := closedRepo(t, tc.module)
			closedGit(t, root, "mv", "docs/tech-design/existing.md", "docs/tech-design/renamed.md")
			// The head's module cannot change the base's identity.
			closedWrite(t, root, "go.mod", "module example.test/changed\n")
			closedGit(t, root, "add", "go.mod")
			closedGit(t, root, "commit", "-qm", "rename")
			got := gate.CheckContractCorpusClosed(root, base, "HEAD", cfg)
			if tc.refuse && (len(got) != 1 || got[0].File != "docs/tech-design/renamed.md") {
				t.Fatalf("renamed contract must be refused: %v", got)
			}
			if !tc.refuse && len(got) != 0 {
				t.Fatalf("another repository must remain open: %v", got)
			}
		})
	}
}

func TestContractCorpusClosedReportsUnusableDiffRefs(t *testing.T) {
	cfg := config.Defaults()
	root, base := closedRepo(t, "github.com/agenticlab-ai/pyxis")
	for _, tc := range []struct {
		name, mergeBase, head, reason string
	}{
		{"invalid merge base", "missing-merge-base", "HEAD", "cannot identify the repository"},
		{"invalid head", base, "missing-head", "git diff against"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := gate.CheckContractCorpusClosed(root, tc.mergeBase, tc.head, cfg)
			if len(got) != 1 || !strings.Contains(got[0].Reason, tc.reason) {
				t.Fatalf("unusable diff ref needs a named refusal: %v", got)
			}
		})
	}
}

func TestContractCorpusClosedReportsMalformedBaseModule(t *testing.T) {
	root, base := closedRepo(t, "")
	got := gate.CheckContractCorpusClosed(root, base, "HEAD", config.Defaults())
	if len(got) != 1 || !strings.Contains(got[0].Reason, "go.mod has no module directive") {
		t.Fatalf("malformed base module needs a named refusal: %v", got)
	}
}
