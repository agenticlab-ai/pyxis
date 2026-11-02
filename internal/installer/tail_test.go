package installer_test

// Hermetic tests for the T8 init tail (installer-v1 §3.5 steps 4–7 + 9; plan
// installer/001 T8): the full pipeline end to end (T7 front + T8 tail) on a
// bare directory against the file:// fixture release whose payload carries the
// REAL skeletons + operand manual + welcome item + installed hook variants.
// Covers seeding (four skeletons + AGENTS.md substituted + registries valid,
// rollups absent), hooks (installed, executable, call pyxis directly, core.hooksPath set, a
// gate-violating commit rejected), wiring (carina store created / github
// transcript == declared table), the GREEN PROOF (exit 0 on the seeded tree —
// carried finding (a)), the welcome path, argus-off vs on, and commit-tree
// inclusion of the seeded set.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// ---- FR-7/FR-15/NG-1: the seeded tree ---------------------------------------

func TestHarnessSkillViewsAreGeneratedFromOneTemplate(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	opts := r.baseOptions(target)
	opts.Answers.CLISurfaces = []string{"claude", "codex", "cursor", "opencode"}
	opts.AnswersProvided = true
	if _, err := installer.Init(context.Background(), opts); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for link, want := range map[string]string{
		".agents/skills":   "../.pyxis/generated/harnesses/neutral/skills",
		".claude/skills":   "../.pyxis/generated/harnesses/claude/skills",
		".codex/skills":    "../.pyxis/generated/harnesses/codex/skills",
		".cursor/skills":   "../.pyxis/generated/harnesses/cursor/skills",
		".opencode/skills": "../.pyxis/generated/harnesses/opencode/skills",
	} {
		got, err := os.Readlink(at(target, link))
		if err != nil || got != want {
			t.Errorf("%s = %q, %v; want symlink %q", link, got, err, want)
		}
	}
	if _, err := os.Lstat(at(target, ".pyxis/generated/harnesses/neutral/skills/implement-now")); !os.IsNotExist(err) {
		t.Fatalf("shared neutral skills expose implement-now: %v", err)
	}
	if got, err := os.Readlink(at(target, ".pyxis/generated/harnesses/neutral/skills/help")); err != nil || got != "../../../../skills/help" {
		t.Fatalf("neutral help pointer = %q, %v", got, err)
	}

	tmpl := readSourceFile(t, payload.HarnessImplementNowSkill)
	for harness, values := range map[string]map[string]string{
		"claude":   {"harness_name": "Claude Code", "backend_id": "claude"},
		"codex":    {"harness_name": "Codex", "backend_id": "codex"},
		"cursor":   {"harness_name": "Cursor CLI", "backend_id": "cursor"},
		"opencode": {"harness_name": "OpenCode", "backend_id": "opencode"},
	} {
		want, err := payload.Substitute(tmpl, values)
		if err != nil {
			t.Fatal(err)
		}
		path := at(target, ".pyxis/generated/harnesses/"+harness+"/skills/implement-now/SKILL.md")
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s generated skill: %v", harness, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s generated skill diverged from the canonical template", harness)
		}
	}
}

// TestT8SeedsGovernanceTreeRollupsAbsent proves the Seed stage instantiated the
// four skeletons byte-verbatim, AGENTS.md knob-substituted, the empty-but-valid
// registries, and left the rollups correctly absent (rule 3). The green proof
// exiting 0 (Init returned no error) already asserts the gate is green over
// exactly this tree.
func TestT8SeedsGovernanceTreeRollupsAbsent(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada Lovelace", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// The two governance skeletons diff byte-clean against the shipped
	// templates (no substitution — their fill points are the human author's).
	skeletons := map[string]string{
		"docs/product-vision.md":          ".pyxis/templates/product-vision-skeleton.md",
		"docs/architecture-principles.md": ".pyxis/templates/architecture-principles-skeleton.md",
	}
	for operand, tmpl := range skeletons {
		got, rerr := os.ReadFile(at(target, operand))
		if rerr != nil {
			t.Fatalf("read seeded %s: %v", operand, rerr)
		}
		want := readSourceFile(t, tmpl)
		if string(got) != string(want) {
			t.Errorf("seeded %s is not byte-verbatim from %s", operand, tmpl)
		}
	}

	// AGENTS.md was substituted: every {{token}} is resolved, and the captured
	// knobs appear (tracker=github, argus=false).
	agents, rerr := os.ReadFile(at(target, "AGENTS.md"))
	if rerr != nil {
		t.Fatalf("read AGENTS.md: %v", rerr)
	}
	if strings.Contains(string(agents), "{{") {
		t.Errorf("AGENTS.md carries an unresolved substitution token:\n%s", firstLines(string(agents), 40))
	}

	// The empty-but-valid registries exist; the design index is present.
	for _, want := range []string{"docs/observability/incidents.md", "docs/tech-design/index.md", "docs/tech-design/index.json"} {
		if _, serr := os.Stat(at(target, want)); serr != nil {
			t.Errorf("registry/index %s absent after seed: %v", want, serr)
		}
	}

	// Rollups stay ABSENT (rule 3 — no capability has shipped).
	for _, rollup := range []string{"docs/prd.md", "docs/techspec.md"} {
		if _, serr := os.Stat(at(target, rollup)); serr == nil {
			t.Errorf("rollup %s exists — it must stay absent pre-first-ship (rule 3)", rollup)
		}
	}

	// The report's green-proof flag is set (the gate ran and passed).
	if !rep.T8.GreenProof.Green {
		t.Error("Report.T8.GreenProof.Green = false, want true (the gate must exit green on the seeded tree)")
	}
	// The seeded set is reported.
	seeded := map[string]bool{}
	for _, d := range rep.T8.Seeded {
		seeded[d.Path] = true
	}
	for _, want := range []string{"docs/product-vision.md", "AGENTS.md", "docs/observability/incidents.md", "docs/tech-design/index.json"} {
		if !seeded[want] {
			t.Errorf("Report.T8.Seeded missing %q (reported: %v)", want, rep.T8.Seeded)
		}
	}
}

// TestT8GreenProofRunsRealGateFindingA is the explicit carried-finding-(a)
// proof: the green proof runs the REAL docgate gate over the seeded tree and it
// is green — meaning lint-rollups vacuous-passed on the rollup-absent operand.
// A red gate would have failed Init; this asserts the run succeeded AND the
// green-proof stage recorded green, so lint-rollups' fresh-operand posture is
// proven end to end, not just in the check's unit test.
func TestT8GreenProofRunsRealGateFindingA(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init (the green proof would have failed on a red gate): %v", err)
	}
	if !rep.T8.GreenProof.Green {
		t.Fatal("green proof did not record green despite Init succeeding")
	}
	// Belt-and-braces: the rollups are absent, so finding (a) is the reason the
	// gate is green (not that they happen to be present-and-valid).
	for _, rollup := range []string{"docs/prd.md", "docs/techspec.md"} {
		if _, serr := os.Stat(at(target, rollup)); serr == nil {
			t.Fatalf("rollup %s present — the green proof did not exercise the finding-(a) absent leg", rollup)
		}
	}
}

// ---- FR-9: hooks installed calling pyxis directly (carried finding (b)) ------

// TestT8HooksInstalledCallPyxisDirectlyAndCoreHooksPathSet proves the Hooks
// stage installed the operand hook variants into .githooks (executable, calling
// the `pyxis` BINARY directly — finding (b): NO `go run`), set core.hooksPath,
// and reported them.
func TestT8HooksInstalledCallPyxisDirectlyAndCoreHooksPathSet(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// core.hooksPath is set to .githooks (the value doctor expects).
	if got := strings.TrimSpace(git(t, target, "config", "--local", "core.hooksPath")); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}
	if rep.T8.Hooks.HooksPath != ".githooks" {
		t.Errorf("Report.T8.Hooks.HooksPath = %q, want .githooks", rep.T8.Hooks.HooksPath)
	}

	for _, name := range []string{"pre-commit", "pre-push", "prepare-commit-msg"} {
		p := at(target, ".githooks/"+name)
		info, serr := os.Stat(p)
		if serr != nil {
			t.Fatalf("operand hook %s not installed: %v", name, serr)
		}
		// Executable.
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("operand hook %s is not executable (mode %v)", name, info.Mode().Perm())
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatal(rerr)
		}
		body := string(b)
		// Finding (b): the operand hook calls the pyxis BINARY, NOT `go run`.
		if strings.Contains(body, "go run") {
			t.Errorf("operand hook %s uses `go run` — an installed operand has no engine source (finding b)", name)
		}
		if !strings.Contains(body, "pyxis exec") {
			t.Errorf("operand hook %s does not delegate through `pyxis exec`:\n%s", name, body)
		}
	}
}

// TestT8HooksRejectGateViolatingCommit proves the activated pre-commit hook
// rejects a commit when its delegate cannot pass — here, because `pyxis` is not
// on PATH, the fail-closed hook aborts the commit (the operand's gate is live
// from install, no manual step). This is the FR-9 "reject a violating commit"
// leg realized without a released binary: the hook fails closed rather than
// letting an ungated commit through.
func TestT8HooksRejectCommitWhenGateCannotRun(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// A subsequent operand commit runs the activated hooks. With no `pyxis` on
	// PATH the fail-closed hook aborts it — the gate is not bypassable.
	if err := os.WriteFile(at(target, "scratch.md"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, target, "add", "--", "scratch.md")
	cmd := gitCmd(target, "commit", "-m", "post-init change")
	// A PATH without pyxis (git must still resolve; point PATH at a dir with a
	// git symlink only — the hook's `command -v pyxis` then misses).
	cmd.Env = append(pathWithGitOnly(t), cmd.Env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a post-init commit succeeded despite the fail-closed gate hook:\n%s", out)
	}
	if !strings.Contains(string(out), "pyxis") {
		t.Errorf("commit rejection is not the named pyxis-hook refusal:\n%s", out)
	}
}

// ---- FR-10: wiring dispatch -------------------------------------------------

// TestT8WiringGitHubExecutesDeclaredTable proves github-mode wiring runs the
// wire operation table through the injected transcript fake, and the executed
// set equals the declared table (the ABUSE-3 bound) for the base knobs.
func TestT8WiringGitHubExecutesDeclaredTable(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	fw := installFakeWire(t)

	opts := r.baseOptions(target) // tracker=github, ci_mode=both, wiring use/required-checks
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.T8.Wiring == nil {
		t.Fatal("Report.T8.Wiring is nil for a github-mode install")
	}
	// The executed set (recorded by the fake) equals the declared table for
	// these knobs: ensure-labels, push-workflow, set-protection, welcome-item.
	got := map[string]bool{}
	for _, op := range fw.ops {
		got[op.Name] = true
	}
	for _, want := range []string{wire.OpEnsureLabels, wire.OpPushWorkflow, wire.OpSetProtection, wire.OpWelcomeItem} {
		if !got[want] {
			t.Errorf("declared table missing %q (ran: %v)", want, fw.opNames())
		}
	}
	// create-repo is NOT declared (wiring.repo=use).
	if got[wire.OpCreateRepo] {
		t.Errorf("create-repo ran despite wiring.repo=use: %v", fw.opNames())
	}
	// The audit is recorded on the report; the welcome-item op created it, so
	// the Welcome stage did NOT also create a carina item.
	if rep.T8.Welcome.ItemID != 0 {
		t.Errorf("github mode created a carina welcome item (id %d) — the wire welcome-item op owns it", rep.T8.Welcome.ItemID)
	}
}

// TestT8WiringCarinaInitializesLocalStore proves carina-mode wiring + welcome
// creates the local work-item store and it answers a query post-init (the
// store file exists and carries the welcome item).
func TestT8WiringCarinaInitializesLocalStore(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := r.carinaOptions(target)
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.Knobs.Tracker != "carina" {
		t.Fatalf("tracker = %q, want carina", rep.Knobs.Tracker)
	}
	// No github wiring ran.
	if rep.T8.Wiring != nil {
		t.Errorf("carina mode ran github wiring: %+v", rep.T8.Wiring)
	}
	// The welcome item was created in the local store (id >= 1) and its file
	// exists under the configured work-items root — the store answers a query.
	if rep.T8.Welcome.ItemID < 1 {
		t.Fatalf("carina welcome item id = %d, want >= 1", rep.T8.Welcome.ItemID)
	}
	storeRoot := at(target, ".workitems")
	if _, serr := os.Stat(filepath.Join(storeRoot, "items")); serr != nil {
		t.Fatalf("carina store items dir absent post-init: %v", serr)
	}
	// The item file carries the welcome title.
	found := false
	entries, _ := os.ReadDir(filepath.Join(storeRoot, "items"))
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(storeRoot, "items", e.Name()))
		if strings.Contains(string(b), "Welcome") {
			found = true
		}
	}
	if !found {
		t.Error("the carina store carries no welcome item after init")
	}
}

// ---- FR-15: welcome emission ------------------------------------------------

// TestT8WelcomeComputesNextSteps proves the Welcome stage produced the
// epilogue steps on the Report in both tracker modes — the cmd report tail
// prints them as the run's LAST block (#937), so the report field is the
// contract surface here, and stdout must NOT carry them mid-run anymore.
// wantGatesActiveByMode is the resolved-ci_mode gates row each fixture mode
// must produce: baseOptions (github) answers ci_mode=both, carinaOptions
// answers ci_mode=local — the two enum values the promptless fixtures
// exercise (installer_test.go's baseOptions/carinaOptions).
var wantGatesActiveByMode = map[string]string{
	"github": "gates: docs gate live at your desk (git hooks) + hosted CI (ci_mode=both)",
	"carina": "gates: docs gate live at your desk (git hooks) — hosted CI off (ci_mode=local)",
}

// TestT8WelcomeComputesNextSteps also proves the plan/init-hardening/001
// T8 completion-boundary acceptance criterion (prd:init-hardening/FR-14,
// design init-hardening-v1 §3.9): the epilogue states which gates are
// active — worded to the RESOLVED ci_mode knob, not a fixed literal — and
// that the code gate is pending with its enabling seed command named, in
// both tracker modes.
func TestT8WelcomeComputesNextSteps(t *testing.T) {
	for _, mode := range []string{"github", "carina"} {
		t.Run(mode, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			r := newRelease(t)
			target := t.TempDir()

			var stdout strings.Builder
			var opts installer.Options
			if mode == "github" {
				installFakeWire(t)
				opts = r.baseOptions(target)
			} else {
				opts = r.carinaOptions(target)
			}
			opts.Stdout = &stdout

			rep, err := installer.Init(context.Background(), opts)
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			if !rep.T8.Welcome.Emitted {
				t.Error("Report.T8.Welcome.Emitted = false")
			}
			steps := strings.Join(rep.T8.Welcome.NextSteps, "\n")
			for _, want := range []string{
				"/feature-brainstorm", "/plan-new", "pyxis doctor",
				wantGatesActiveByMode[mode],
				"pending: the code gate", "pyxis exec codegate -- seed",
			} {
				if !strings.Contains(steps, want) {
					t.Errorf("welcome next-steps missing %q:\n%s", want, steps)
				}
			}
			// #948: the guide is LINKED (github: the wire op's URL; carina:
			// the item id + store path), the filed todo list renders, and
			// the epilogue ends on the send-off.
			if mode == "github" {
				if !strings.HasPrefix(rep.T8.Welcome.Ref, "https://github.com/") {
					t.Errorf("github welcome ref = %q, want the issue URL", rep.T8.Welcome.Ref)
				}
				if !strings.Contains(steps, "full guide: https://github.com/") {
					t.Errorf("github epilogue does not link the guide:\n%s", steps)
				}
			} else {
				if rep.T8.Welcome.Ref == "" || !strings.Contains(steps, "full guide: item #") {
					t.Errorf("carina welcome ref %q / steps do not name the item:\n%s", rep.T8.Welcome.Ref, steps)
				}
				if !strings.Contains(steps, "your todo list: 3 onboarding item(s)") {
					t.Errorf("carina epilogue does not list the filed todos:\n%s", steps)
				}
			}
			if last := rep.T8.Welcome.NextSteps[len(rep.T8.Welcome.NextSteps)-1]; !strings.Contains(last, "one /feature-brainstorm away") {
				t.Errorf("epilogue does not end on the send-off: %q", last)
			}
			// The epilogue is the cmd report tail's closing block now — the
			// installer's own narration must not print it mid-run, or it
			// would scroll away behind the summary (#937).
			if strings.Contains(stdout.String(), "/feature-brainstorm") {
				t.Errorf("installer stdout still carries the epilogue mid-run:\n%s", stdout.String())
			}
		})
	}
}

// ---- FR-16: argus off vs on at seed time ------------------------------------

// TestT8ArgusOffVsOnSeedDifference proves the argus knob's seed-time effect:
// off creates no authority surface; on starts at floor with a native-valid
// birth row, never Source's operator grants. AGENTS.md records either knob.
func TestT8ArgusOffVsOnSeedDifference(t *testing.T) {
	cases := []struct {
		name  string
		argus bool
	}{{"argus off", false}, {"argus on", true}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			r := newRelease(t)
			target := t.TempDir()

			opts := r.carinaOptions(target)
			opts.Answers.Argus = &c.argus
			rep, err := installer.Init(context.Background(), opts)
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			if rep.Knobs.Argus != c.argus {
				t.Errorf("knobs.argus = %t, want %t", rep.Knobs.Argus, c.argus)
			}
			// Preview exposes the once-only policy seed and performs no write.
			previewTarget := t.TempDir()
			previewOpts := r.carinaOptions(previewTarget)
			previewOpts.Answers.Argus = &c.argus
			previewOpts.Preview = true
			preview, err := installer.Init(context.Background(), previewOpts)
			if err != nil {
				t.Fatal(err)
			}
			if c.argus {
				if len(preview.WouldSeed) != 1 || preview.WouldSeed[0].Path != payload.AuthorityContractTarget {
					t.Fatalf("preview omitted authority creation: %+v", preview.WouldSeed)
				}
			} else if len(preview.WouldSeed) != 0 {
				t.Fatalf("argus off proposed authority creation: %+v", preview.WouldSeed)
			}
			if _, err := os.Lstat(at(previewTarget, payload.AuthorityContractTarget)); !os.IsNotExist(err) {
				t.Fatalf("preview created authority: %v", err)
			}
			body, serr := os.ReadFile(at(target, payload.AuthorityContractTarget))
			if !c.argus {
				if !os.IsNotExist(serr) {
					t.Fatalf("argus off created authority: %v", serr)
				}
			} else {
				if serr != nil {
					t.Fatal(serr)
				}
				contract, defects := authoritycontract.Parse("test", payload.AuthorityContractTarget, body)
				if len(defects) != 0 {
					t.Fatalf("parse: %v", defects)
				}
				if defects := authoritycontract.Evaluate("test", payload.AuthorityContractTarget, contract); len(defects) != 0 {
					t.Fatalf("evaluate: %v", defects)
				}
				if contract.CurrentTier != "floor" || len(contract.History) != 1 || contract.History[0].Move != "none" || contract.History[0].Tier != "floor" {
					t.Fatalf("not a floor birth: %+v", contract)
				}
				if strings.Contains(string(body), "agenticlab-ai/pyxis") || strings.Contains(string(body), "operator-directed early promotion") {
					t.Fatal("Source policy leaked into operand")
				}
				for _, row := range contract.Ladder {
					if row.Tier == "floor" && (len(row.Tokens) != 1 || row.Tokens[0] != "dispose-advisory-finding") {
						t.Fatalf("unexpected floor grant: %+v", row)
					}
				}
				if _, err := os.Stat(at(target, "docs/authority-contract.md")); !os.IsNotExist(err) {
					t.Fatalf("unexpected second authority location: %v", err)
				}
			}
			// AGENTS.md records the knob value (argus_enabled substituted).
			agents, _ := os.ReadFile(at(target, "AGENTS.md"))
			wantTok := boolToken(c.argus)
			if !strings.Contains(string(agents), wantTok) {
				t.Errorf("AGENTS.md does not record argus=%t (looked for %q)", c.argus, wantTok)
			}
		})
	}
}

// ---- commit-tree includes the seeded set ------------------------------------

// TestT8CommitTreeIncludesSeededPaths proves the genesis commit staged the T8
// seeded set (skeletons, AGENTS.md, registries, hooks) — `git ls-files` after
// init shows every one, and nothing gitignored leaked in.
func TestT8CommitTreeIncludesSeededPaths(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	tracked := map[string]bool{}
	for ln := range strings.SplitSeq(strings.TrimSpace(git(t, target, "ls-files")), "\n") {
		tracked[ln] = true
	}
	for _, want := range []string{
		"docs/product-vision.md", "docs/architecture-principles.md", "AGENTS.md",
		"docs/observability/incidents.md", "docs/tech-design/index.md", "docs/tech-design/index.json",
		".githooks/pre-commit", ".githooks/pre-push", ".githooks/prepare-commit-msg",
	} {
		if !tracked[want] {
			t.Errorf("seeded path %q not tracked in the genesis commit", want)
		}
	}
	// The rollups and the opt-in living maps are NOT tracked — only the two
	// governing docs are seeded; everything else is added by the repo if wanted.
	for _, absent := range []string{"docs/prd.md", "docs/techspec.md", "docs/domain-model.md", "docs/system-design.md"} {
		if tracked[absent] {
			t.Errorf("non-seeded path %q was committed — it must stay absent", absent)
		}
	}
	// Exactly two commits.
	if n := gitRevListCount(t, target); n != 2 {
		t.Errorf("genesis commit count = %d, want 2", n)
	}
}

// The carina lane files the standing onboarding set beside the welcome item
// (#947): three items on a clean install (no wiring hand-off), refs and the
// human detail on the report, the item files present in the store.
func TestT8CarinaFilesOnboardingItems(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.carinaOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	onb := rep.T8.Onboarding
	if onb == nil {
		t.Fatal("Report.T8.Onboarding = nil, want the filed set")
	}
	if len(onb.Refs) != 3 || !strings.Contains(onb.Detail, "filed 3 onboarding item(s)") {
		t.Fatalf("onboarding = %+v, want 3 refs and the count detail", onb)
	}
	var joined strings.Builder
	entries, _ := os.ReadDir(filepath.Join(at(target, ".workitems"), "items"))
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(at(target, ".workitems"), "items", e.Name()))
		joined.Write(b)
	}
	for _, want := range []string{"Author your product vision", "Author your architecture principles", "Seed the code gate"} {
		if !strings.Contains(joined.String(), want) {
			t.Errorf("store lacks onboarding item %q", want)
		}
	}
}

// The github lane declares the standing set on the wire table (the op rides
// the audited walk) and the report carries the op's outcome.
func TestT8GitHubDeclaresOnboardingItems(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	fw := installFakeWire(t)
	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	var declared *wire.Op
	for i := range fw.ops {
		if fw.ops[i].Name == wire.OpOnboardingItems {
			declared = &fw.ops[i]
		}
	}
	if declared == nil {
		t.Fatal("the wire table declares no onboarding-items op")
	}
	if len(declared.Items) != 3 {
		t.Fatalf("declared items = %d, want the standing 3", len(declared.Items))
	}
	if rep.T8.Onboarding == nil {
		t.Fatal("Report.T8.Onboarding = nil, want the audit row's outcome")
	}
}
