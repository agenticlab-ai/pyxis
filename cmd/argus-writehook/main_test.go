package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// dcfg is the default configuration the drive-resolution tests run under
// (today's problems root); the config-driven resolution is proven at the
// boundary/attribution layers.
var dcfg = config.Defaults()

func TestMainDelegatesUsageExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"argus-writehook", "--help"}
	got := -1
	processExit = func(code int) { got = code }

	main()

	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

func claudeReq(cwd, absPath string) string {
	b, _ := json.Marshal(map[string]any{
		"tool_name":  "Write",
		"cwd":        cwd,
		"tool_input": map[string]any{"file_path": absPath},
	})
	return string(b)
}

// runShim drives run() with a stdin string, returning exit code, stdout, stderr.
func runShim(args []string, stdin string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestShim_ClaudeDenyAndAllowEndToEnd(t *testing.T) {
	const cwd = "/repo"
	// A protected-path write is denied (exit 0 + Claude deny JSON).
	code, out, _ := runShim(
		[]string{"--cli=claude", "--first-pr", "--slug=widget"},
		claudeReq(cwd, cwd+"/docs/product-vision.md"))
	if code != 0 {
		t.Errorf("deny exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Fatalf("stdout is not a Claude deny: %q", out)
	}
	for _, part := range []string{"docs/product-vision.md", "governance", "remedy:"} {
		if !strings.Contains(out, part) {
			t.Errorf("deny reason omits %q: %q", part, out)
		}
	}

	// An in-scope write is allowed.
	_, out, _ = runShim(
		[]string{"--cli=claude", "--first-pr", "--slug=widget"},
		claudeReq(cwd, cwd+"/docs/prds/widget/000-base.md"))
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Errorf("in-scope write not allowed: %q", out)
	}
}

func TestShim_UnknownCLIFailsClosed(t *testing.T) {
	// An unknown/absent CLI denies fail-closed (exit 2), never a silent allow.
	code, _, errb := runShim([]string{"--cli=bogus"}, "{}")
	if code != 2 {
		t.Errorf("unknown CLI exit = %d, want 2", code)
	}
	if !strings.Contains(errb, "denied") {
		t.Errorf("unknown CLI stderr = %q, want a deny", errb)
	}
}

func TestShim_NoSlugUnresolvableDenies(t *testing.T) {
	const cwd = "/repo"
	// No drive slug: a scope-dependent write is unresolvable ⇒ deny.
	_, out, _ := runShim([]string{"--cli=claude"}, claudeReq(cwd, cwd+"/internal/engine/core.go"))
	if !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Fatalf("no-slug scope-dependent write not denied: %q", out)
	}
	if !strings.Contains(out, "unresolvable") {
		t.Errorf("no-slug deny should cite the unresolvable scope: %q", out)
	}
	// A path-based protected class still fires with no slug (more specific).
	_, out, _ = runShim([]string{"--cli=claude"}, claudeReq(cwd, cwd+"/.pyxis/skills/argus/authority-contract.md"))
	if !strings.Contains(out, "authority contract") {
		t.Errorf("authority-contract write not denied with no slug: %q", out)
	}
}

func TestShim_GuaranteesLists(t *testing.T) {
	// A-3: the guarantee listing names each supported CLI and its guarantee.
	code, out, _ := runShim([]string{"guarantees"}, "")
	if code != 0 {
		t.Errorf("guarantees exit = %d, want 0", code)
	}
	for _, want := range []string{"claude", "fail-closed", "codex", "merge-boundary"} {
		if !strings.Contains(out, want) {
			t.Errorf("guarantees output omits %q:\n%s", want, out)
		}
	}
}

func TestShim_Version(t *testing.T) {
	code, out, _ := runShim([]string{"version"}, "")
	if code != 0 || strings.TrimSpace(out) != boundary.DataVersion {
		t.Errorf("version = %q (exit %d), want %q", out, code, boundary.DataVersion)
	}
}

// TestShim_BaseDefaultResolvesConfigMainline proves the committed-ledger --base
// default resolves the CONFIG's mainline ref (<remote>/<branch>), not a baked
// "origin/main" (config-driven-portability lift, FR-2). The hook runs at the repo
// root and loads config from the cwd, so the test chdirs into a real repo whose
// v2 config declares a NON-default remote/branch and whose committed delivery
// ledger is reachable ONLY at that ref (there is no origin/main): a resolver still
// defaulting to "origin/main" would fail to read the ledger and deny an in-scope
// write. It confirms byte-identity too — the default config resolves origin/main.
func TestShim_BaseDefaultResolvesConfigMainline(t *testing.T) {
	testShimConfiguredMainlineAndPlansRoot(t, "docs/plans")
}

func TestShim_RepositoryRootPlans(t *testing.T) {
	testShimConfiguredMainlineAndPlansRoot(t, ".")
}

func testShimConfiguredMainlineAndPlansRoot(t *testing.T, plansRoot string) {
	t.Helper()
	if config.Defaults().MainlineRef() != "origin/main" {
		t.Fatalf("default MainlineRef is no longer byte-identical to the old literal (%q)", config.Defaults().MainlineRef())
	}

	// The exact delivery-ledger fixture the author-independent-scope test uses —
	// proven to parse defect-free through the boundary resolver.
	const committedLedger = `# Widget — delivery ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | prd-new |
| **Open items** | — |
| **Park state** | — |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — stage-entered — prd-new

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: docs/prds/widget/000-base.md
- **Body**: prose.
`
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@pyxis.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@pyxis.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	// V2 admits the compiled plan root and a non-default mainline. Legacy
	// configuration admits custom plan roots; exercise a valid config for each.
	configBody := "schema_version: pyxis-config/v2\nremote: upstream\nbranch: trunk\n"
	if plansRoot != config.Defaults().Plans {
		configBody = "paths:\n  plans: " + plansRoot + "\n"
	}
	write(".pyxis/config.yaml", configBody)
	loaded, defects := config.Load(repo)
	if len(defects) != 0 || loaded.Plans != plansRoot {
		t.Fatalf("fixture config: plans=%q, want %q, defects=%v", loaded.Plans, plansRoot, defects)
	}
	write("docs/problems/widget/delivery.md", committedLedger)
	git("add", ".")
	git("commit", "-q", "-m", "seed")
	head := strings.TrimSpace(mustOut(t, repo, "rev-parse", "HEAD"))
	// The committed ledger is reachable only at the configured mainline ref.
	// In the v2 case there is no origin/main; a baked default would fail.
	git("update-ref", "refs/remotes/"+loaded.MainlineRef(), head)

	t.Chdir(repo)
	t.Setenv("ARGUS_DRIVE_BASE", "") // force the config-resolved default, not the env override

	// An in-scope write under the drive's owned subtree: allowed ONLY if the
	// committed ledger resolved, which requires the base to be the configured ref.
	cwd := repo
	code, out, errb := runShim(
		[]string{"--cli=claude", "--slug=widget"},
		claudeReq(cwd, filepath.Join(cwd, "docs/prds/widget/000-base.md")))
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, errb)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("in-scope write was not allowed — the committed ledger did not resolve at the configured mainline ref (base default not config-driven):\n%s", out)
	}
	// The same committed-input resolver must allow the bounded downstream
	// documentation transition without admitting unrelated or protected paths.
	for _, tc := range []struct {
		target, decision string
	}{
		{filepath.Join(plansRoot, "widget/001-plan.md"), "allow"},
		{filepath.Join(plansRoot, "widget/001-plan.json"), "allow"},
		{"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json", "allow"},
		{"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md", "allow"},
		{filepath.Join(plansRoot, "other/001-plan.md"), "deny"},
		{filepath.Join(plansRoot, "widget-other/001-plan.md"), "deny"},
		{filepath.Join(plansRoot, "widget/code.py"), "deny"},
		{filepath.Join(plansRoot, "widget/nested/001-plan.md"), "deny"},
		{"memory/problem/other/Document-01M4767N12EEK6YZWRM29DF0RP.json", "deny"},
		{"memory/source/pyxis-config/Document-01M4767N12EEK6YZWRM29DF0RP.json", "deny"},
		{"aec_memory/store.py", "deny"},
		{".pyxis/config.yaml", "deny"},
		{".pyxis/skills/argus/authority-contract.md", "deny"},
		{"docs/product-vision.md", "deny"},
		{"memory/problem/widget/notes.md", "deny"},
		{"memory/problem/widget/code.py", "deny"},
		{"memory/problem/widget/nested/Document-01M4767N12EEK6YZWRM29DF0RP.json", "deny"},
		{"memory/problem/widget/Decision-01M4767N12EEK6YZWRM29DF0RP.json", "deny"},
		{"memory/problem/widget/Document-short.json", "deny"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			code, out, errb := runShim([]string{"--cli=claude", "--slug=widget"},
				claudeReq(cwd, filepath.Join(cwd, filepath.FromSlash(tc.target))))
			if code != 0 || !strings.Contains(out, `"permissionDecision":"`+tc.decision+`"`) {
				t.Fatalf("%s: want %s, exit=%d stdout=%q stderr=%q", tc.target, tc.decision, code, out, errb)
			}
		})
	}
	const problem = `# Widget — problem ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | problem-ledger/v1 |

## Idea

Build the widget.

## Authority entries

### A-1 — fixture ruling

- **Date**: 2026-10-05
- **Ruling**: Build the widget.
`
	if _, defects := argusledger.ParseAs("test", "docs/problems/widget/problem.md", schema.LedgerClasses["problem-ledger"], []byte(problem)); len(defects) != 0 {
		t.Fatalf("invalid problem fixture: %v", defects)
	}
	write("docs/problems/widget/problem.md", problem)
	git("rm", "docs/problems/widget/delivery.md")
	git("add", "--", "docs/problems/widget/problem.md")
	git("commit", "-q", "-m", "problem without committed delivery")
	derivedAt := strings.Index(committedLedger, "## Derived state")
	journalAt := strings.Index(committedLedger, "## Journal")
	reordered := committedLedger[:derivedAt] + committedLedger[journalAt:] + "\n" + committedLedger[derivedAt:journalAt]
	for _, state := range []struct{ name, delivery string }{
		{"absent", ""},
		{"malformed", "not a delivery ledger\n"},
		{"missing slug", strings.Replace(committedLedger, "| **Slug** | widget |\n", "", 1)},
		{"mismatched slug", strings.Replace(committedLedger, "| **Slug** | widget |", "| **Slug** | other |", 1)},
		{"incomplete derived state", strings.Replace(committedLedger, "| **Open items** | — |\n", "", 1)},
		{"invalid derived status", strings.Replace(committedLedger, "| **Status** | open |", "| **Status** | approved |", 1)},
		{"reordered regions", reordered},
		{"unframed tables", strings.ReplaceAll(strings.ReplaceAll(committedLedger, "| Field | Value |\n", ""), "|---|---|\n", "")},
		{"empty journal", committedLedger[:strings.Index(committedLedger, "### E-1")]},
		{"untyped journal", strings.Replace(committedLedger, "### E-1 — stage-entered — prd-new", "### E-1 nonsense", 1)},
		{"invalid journal date", strings.Replace(committedLedger, "- **Date**: 2026-10-06", "- **Date**: 2026-02-30", 1)},
		{"external placeholder", strings.Replace(strings.Replace(committedLedger, "stage-entered", "item-materialized", 1), "- **External effect**: docs/prds/widget/000-base.md", "- **External effect**: —", 1)},
		{"principal placeholder", strings.Replace(committedLedger, "- **Principal**: Argus", "- **Principal**: —", 1)},
		{"annotated principal placeholder", strings.Replace(committedLedger, "- **Principal**: Argus", "- **Principal**: — (unknown)", 1)},
		{"annotated external placeholder", strings.Replace(strings.Replace(committedLedger, "stage-entered", "item-materialized", 1), "- **External effect**: docs/prds/widget/000-base.md", "- **External effect**: none (not recorded)", 1)},
		{"internal journal", strings.Replace(committedLedger, "- **External effect**: docs/prds/widget/000-base.md\n", "", 1)},
		{"internal disposition", strings.Replace(strings.Replace(committedLedger, "stage-entered", "proposal-disposed", 1), "- **External effect**: docs/prds/widget/000-base.md\n", "", 1)},
	} {
		if state.delivery != "" {
			write("docs/problems/widget/delivery.md", state.delivery)
			git("add", "--", "docs/problems/widget/delivery.md")
			git("commit", "-q", "-m", state.name+" committed delivery")
		}
		git("update-ref", "refs/remotes/"+loaded.MainlineRef(), strings.TrimSpace(mustOut(t, repo, "rev-parse", "HEAD")))
		// Valid candidate bytes must not replace missing/malformed committed input.
		write("docs/problems/widget/delivery.md", committedLedger)
		decision := "deny"
		if state.name == "internal journal" || state.name == "internal disposition" {
			decision = "allow"
		}
		for _, target := range []string{filepath.Join(plansRoot, "widget/001-plan.md"), filepath.Join(plansRoot, "widget/001-plan.json"), "memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md"} {
			code, out, errb := runShim([]string{"--cli=claude", "--slug=widget"}, claudeReq(cwd, filepath.Join(cwd, filepath.FromSlash(target))))
			if code != 0 || !strings.Contains(out, `"permissionDecision":"`+decision+`"`) {
				t.Fatalf("%s committed delivery: want %s for %s, exit=%d stdout=%q stderr=%q", state.name, decision, target, code, out, errb)
			}
		}
		if state.name == "invalid derived status" || state.name == "reordered regions" {
			for _, target := range []string{"docs/prds/widget/000-base.md", "docs/problems/widget/delivery.md"} {
				code, out, errb := runShim([]string{"--cli=claude", "--slug=widget"}, claudeReq(cwd, filepath.Join(cwd, filepath.FromSlash(target))))
				if code != 0 || !strings.Contains(out, `"permissionDecision":"allow"`) {
					t.Fatalf("legacy ledger historical path %s: want allow, exit=%d stdout=%q stderr=%q", target, code, out, errb)
				}
			}
		}
	}
}

// mustOut runs git in dir and returns stdout, failing on error.
func mustOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func TestResolveDrive_AuthorIndependentScope(t *testing.T) {
	// committedLedger mirrors boundary's fixture: problem "widget" owns its own
	// subtree plus the paths its committed journal attributes to it.
	const committedLedger = `# Widget — delivery ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | prd-new |
| **Open items** | — |
| **Park state** | — |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — stage-entered — prd-new

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: docs/prds/widget/000-base.md
- **Body**: prose.
`
	reader := func(path string) ([]byte, bool) {
		if path == "docs/problems/widget/delivery.md" {
			return []byte(committedLedger), true
		}
		return nil, false
	}

	// Committed-ledger scope owns what the journal attributes; a broader-named
	// slug buys nothing it does not own.
	d := resolveDrive(dcfg, "widget", false, "origin/main", reader)
	if d.Scope == nil {
		t.Fatal("committed ledger did not resolve a scope")
	}
	if got := boundary.Classify(dcfg, "docs/prds/widget/000-base.md", d); got != boundary.InScope {
		t.Errorf("owned path = %v, want InScope", got)
	}
	if got := boundary.Classify(dcfg, "internal/engine/core.go", d); got != boundary.OutOfScope {
		t.Errorf("un-owned code path = %v, want OutOfScope", got)
	}

	// No slug ⇒ nil scope ⇒ unresolvable (fail closed).
	if resolveDrive(dcfg, "", false, "origin/main", reader).Scope != nil {
		t.Error("empty slug resolved a non-nil scope")
	}
	// A slug with no committed ledger and not a first PR ⇒ unresolvable.
	if resolveDrive(dcfg, "ghost", false, "origin/main", reader).Scope != nil {
		t.Error("a slug with no committed ledger resolved a scope outside first-PR mode")
	}
	// An unparseable committed ledger ⇒ unresolvable.
	garbage := func(string) ([]byte, bool) { return []byte("not a ledger"), true }
	if resolveDrive(dcfg, "widget", false, "origin/main", garbage).Scope != nil {
		t.Error("an unparseable committed ledger resolved a scope")
	}
	// First-PR mode ⇒ slug-derived scope resolves without any committed read.
	if resolveDrive(dcfg, "widget", true, "origin/main", func(string) ([]byte, bool) { return nil, false }).Scope == nil {
		t.Error("first-PR mode did not resolve a slug-derived scope")
	}
}
