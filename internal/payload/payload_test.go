package payload_test

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// TestSkeletonTargets proves the governance-skeleton target map is a pure
// function of the governing-doc path config (config-driven-portability-v1 §3.2,
// FR-3; DINV-1): a default config yields today's targets byte-identical; a
// non-default config resolves each target to its configured location; and the
// derivation holds no cache — successive calls under changed config yield
// changed results.
func TestSkeletonTargets(t *testing.T) {
	// (c) default config → today's targets, byte-identical.
	def := payload.SkeletonTargets(config.Defaults())
	wantDefault := map[string]string{
		payload.ProductVisionSkeleton:          "docs/product-vision.md",
		payload.ArchitecturePrinciplesSkeleton: "docs/architecture-principles.md",
	}
	if !maps.Equal(def, wantDefault) {
		t.Fatalf("SkeletonTargets(defaults) = %v, want %v", def, wantDefault)
	}

	// (a) non-default governing-doc paths → targets resolve to the configured
	// locations, and the OLD default locations are no longer targeted.
	cfg := config.Defaults()
	cfg.ProductVision = "governance/vision.md"
	cfg.ArchitecturePrinciples = "governance/principles.md"
	reloc := payload.SkeletonTargets(cfg)
	wantReloc := map[string]string{
		payload.ProductVisionSkeleton:          "governance/vision.md",
		payload.ArchitecturePrinciplesSkeleton: "governance/principles.md",
	}
	if !maps.Equal(reloc, wantReloc) {
		t.Fatalf("SkeletonTargets(relocated) = %v, want %v", reloc, wantReloc)
	}
	for _, tgt := range reloc {
		if tgt == "docs/product-vision.md" || tgt == "docs/architecture-principles.md" {
			t.Errorf("relocated target %q is still a stale default location", tgt)
		}
	}

	// (b) no cache: the very next call under a THIRD config reflects it — the
	// function recomputes from config, never returns a memoized earlier map.
	cfg.ProductVision = "third/vision.md"
	third := payload.SkeletonTargets(cfg)
	if got := third[payload.ProductVisionSkeleton]; got != "third/vision.md" {
		t.Errorf("no-cache: after changing config, product-vision target = %q, want third/vision.md", got)
	}
	// The earlier map was not mutated in place either — it is a fresh value.
	if reloc[payload.ProductVisionSkeleton] != "governance/vision.md" {
		t.Errorf("no-cache: the prior derived map was mutated; got %q", reloc[payload.ProductVisionSkeleton])
	}
}

// repoRoot resolves the source repo root from the package directory; the
// templates under test are this package's deliverable, so their absence is a
// loud failure, never a skip.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, payload.TemplatesDir)); err != nil {
		t.Fatalf("template set not found at repo root: %v", err)
	}
	return root
}

func read(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

// The two knob value sets the acceptance names: one per tracker mode
// (prd:installer/FR-3's enums, spelled exactly as the config schema admits).
var (
	manualModes = []map[string]string{
		{"tracker": "github", "ci_mode": "both", "argus_enabled": "false", "branch": "main"},
		{"tracker": "carina", "ci_mode": "local", "argus_enabled": "true", "branch": "main"},
	}
	welcomeModes = []map[string]string{
		{"tracker": "github", "prds_path": "docs/prds", "tech_designs_path": "docs/tech-design", "gates_active": "Active gates: the docs gate at your desk (git hooks) and in hosted CI (ci_mode=both)."},
		{"tracker": "carina", "prds_path": "docs/prds", "tech_designs_path": "docs/tech-design", "gates_active": "Active gate: the docs gate at your desk (git hooks) — hosted CI is off (ci_mode=local)."},
	}
)

// T9 acceptance 2: the installed-repo workflow variant carries exec-grammar
// gate steps and zero source-toolchain steps — the greppable assertions the
// plan names, plus the honest-bootstrap contract (pyxis reaches PATH only
// through the documented shim path). The variant is now a GENERATED template
// carrying {{branch}}/{{remote}} tokens (config-driven-portability-v1 §3.1,
// FR-4): the template holds the tokenized merge-base, and the DEFAULT render
// (branch main, remote origin) produces the original functional literals
// byte-for-byte (DINV-2, A-5) — so the template still speaks exec grammar
// against the mainline once rendered.
func TestInstalledWorkflowSpeaksExecGrammarWithoutSourceToolchain(t *testing.T) {
	b := read(t, payload.InstalledWorkflow)
	low := strings.ToLower(string(b))
	for _, banned := range []string{"setup-go", "go test", "go vet", "gofmt"} {
		if strings.Contains(low, banned) {
			t.Errorf("installed workflow variant carries source-toolchain step %q — those belong to the source repo's own CI (installer-v1 §3.10)", banned)
		}
	}
	// The template holds the tokenized merge-base + the toolchain-free steps.
	for _, want := range []string{
		"pyxis exec docgate -- gate --merge-base {{remote}}/{{branch}}",
		"git fetch {{remote}} {{branch}}",
		"branches: [{{branch}}]",
		"pyxis exec docgate -- coverage",
		".pyxis/scripts/pyxis-bootstrap.sh",
		"fetch-depth: 0",
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("installed workflow template missing %q", want)
		}
	}
	// The default render reproduces the pre-config-driven functional literals
	// exactly — the exec-grammar gate against origin/main (DINV-2).
	d := config.Defaults()
	rendered, err := payload.RenderStagedInstalledWorkflow(b, d.Branch, d.Remote)
	if err != nil {
		t.Fatalf("default render of the installed workflow failed: %v", err)
	}
	for _, want := range []string{
		"pyxis exec docgate -- gate --merge-base origin/main --defects-json -",
		"git fetch origin main",
		"branches: [main]",
	} {
		if !strings.Contains(string(rendered), want) {
			t.Errorf("default-rendered workflow missing %q", want)
		}
	}
	if strings.Contains(string(rendered), "{{") {
		t.Errorf("default-rendered workflow retains a substitution token")
	}
}

// The overlay README beside the variant documents the packer mapping and the
// T11 bootstrap-shim dependency the workflow is written against.
func TestOverlayReadmeDocumentsMappingAndBootstrapDependency(t *testing.T) {
	b := string(read(t, ".pyxis/dist/github-workflows/README.md"))
	for _, want := range []string{
		payload.InstalledWorkflow,
		payload.InstalledWorkflowTarget,
		".pyxis/scripts/pyxis-bootstrap.sh",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("overlay README does not document %q", want)
		}
	}
}

// T9 acceptance 3 (declaration half): each substituted template's `{{…}}`
// set equals its declared closed set exactly — no undeclared variable in a
// template, no declared variable a template dropped — and the two skeletons
// carry none (their fill points are the human's, prd:installer/NG-1).
func TestSubstitutedTemplatesDeclareExactVariableSets(t *testing.T) {
	for tmpl, want := range payload.Vars {
		got := payload.VarsIn(read(t, tmpl))
		if !slices.Equal(got, want) {
			t.Errorf("%s substitution variables = %v, declared %v", tmpl, got, want)
		}
	}
	for _, tmpl := range []string{
		payload.ProductVisionSkeleton,
		payload.ArchitecturePrinciplesSkeleton,
	} {
		if got := payload.VarsIn(read(t, tmpl)); len(got) != 0 {
			t.Errorf("skeleton %s carries substitution variables %v — skeleton fill points are the human author's, never knob-substituted", tmpl, got)
		}
	}
}

// T9 acceptance 3 (render half): both tracker-mode value sets render the
// manual and the welcome item with every substitution resolved — no `{{`
// remnant — and an unresolved variable refuses by name.
func TestBothTrackerModesRenderFullyResolved(t *testing.T) {
	cases := []struct {
		tmpl  string
		modes []map[string]string
	}{
		{payload.OperandAgentsManual, manualModes},
		{payload.WelcomeItem, welcomeModes},
	}
	for _, c := range cases {
		doc := read(t, c.tmpl)
		for _, vals := range c.modes {
			out, err := payload.Substitute(doc, vals)
			if err != nil {
				t.Fatalf("%s (tracker=%s): %v", c.tmpl, vals["tracker"], err)
			}
			if strings.Contains(string(out), "{{") || strings.Contains(string(out), "}}") {
				t.Errorf("%s (tracker=%s): substitution remnant left in rendered output", c.tmpl, vals["tracker"])
			}
			if !strings.Contains(string(out), vals["tracker"]) {
				t.Errorf("%s (tracker=%s): rendered output never states the tracker", c.tmpl, vals["tracker"])
			}
		}
		short := maps.Clone(c.modes[0])
		delete(short, "tracker")
		if _, err := payload.Substitute(doc, short); err == nil || !strings.Contains(err.Error(), "tracker") {
			t.Errorf("%s: missing variable did not refuse by name (err=%v)", c.tmpl, err)
		}
	}
}

// emptyIncidents is the empty-but-valid incident registry the fixture seeds —
// the registry shape init's code creates (the registries are init-created,
// never shipped templates: installer-tech-design record D-9). The format
// example sits in a fence, so lint-incidents sees zero entries.
const emptyIncidents = `# Incidents registry

The repository's paid-for lessons, one bounded entry each — the staging
ground principles graduate from (see ` + "`docs/architecture-principles.md`" + `).

## Format / legend

` + "```" + `
**I<n> (YYYY-MM-DD) [tag, tag, …]**

- **Decision:** what was decided / built.
- **Broke:** how it broke (the observed failure).
- **Lesson:** the distilled takeaway — what to do instead.
- **Links:** provenance; once graduated, Graduated: AP-<n>.
` + "```" + `

Entries append through the coded writer only; IDs run dense from 1 and are
never renumbered; a merged entry is immutable (its Links line may grow at
graduation).

## Entries
`

// instantiateOperand builds the seeded end-state fixture at root: the two
// governance skeletons at their operand paths, the knob-substituted manual at
// AGENTS.md, the empty-but-valid registries, and the taxonomy directories —
// rollups correctly absent (installer-v1 §3.5 step 4).
func instantiateOperand(t *testing.T, root string) config.Config {
	t.Helper()
	src := repoRoot(t)
	cfg := config.Defaults()
	write := func(rel string, b []byte) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for tmpl, target := range payload.SkeletonTargets(cfg) {
		b, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(tmpl)))
		if err != nil {
			t.Fatalf("read %s: %v", tmpl, err)
		}
		write(target, b)
	}
	manual, err := payload.Substitute(read(t, payload.OperandAgentsManual), manualModes[0])
	if err != nil {
		t.Fatalf("substitute manual: %v", err)
	}
	write(payload.OperandAgentsManualTarget, manual)
	write(cfg.Incidents, []byte(emptyIncidents))
	for _, dir := range []string{cfg.PRDs, cfg.TechDesigns, cfg.SolutionShapes, "docs/brainstorms"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	md, js, defects := gate.GenIndexBytes(root, cfg)
	if len(defects) != 0 {
		t.Fatalf("generate empty design index: %v", defects)
	}
	write(cfg.TechDesigns+"/index.md", md)
	write(cfg.TechDesigns+"/index.json", js)
	return cfg
}

// T9 acceptance 1: a fixture instantiation of the two governance skeletons plus
// the registries passes the gate's structure checks — run against the real
// docgate check functions (every registered base-free check; the
// base-requiring ones run in the git-fixture tests below). The fixture is a
// committed git repo because init's end state is one and the corpus-scoped
// checks enumerate tracked files. lint-rollups is the one deliberate skip:
// rule 3 keeps the rollups correctly ABSENT until the operand's first
// capability ships, and reconciling that check's fresh-operand posture is
// init's green-proof concern (plan T8), not a template property.
func TestSkeletonInstantiationPassesGateStructureChecks(t *testing.T) {
	root, run := gitFixture(t)
	cfg := instantiateOperand(t, root)
	run("add", ".")
	run("commit", "-qm", "seed: pyxis init")
	for _, chk := range gate.Checks() {
		if chk.NeedsMergeBase || chk.Verb == "lint-rollups" {
			continue
		}
		if d := chk.Run(gate.Ctx{Root: root, Cfg: cfg}); len(d) != 0 {
			t.Errorf("%s over the seeded operand fixture: %v", chk.Verb, d)
		}
	}
	dirs, err := gate.LedgerDirs(root, cfg)
	if err != nil {
		t.Fatalf("ledger dirs over the seeded fixture: %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("a fresh operand carries no PRD ledgers, found %v", dirs)
	}
}

// gitFixture is the house temp-repo pattern (gate_v3_test.go): a real git
// repo for the history-scoped checks.
func gitFixture(t *testing.T) (string, func(...string)) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	return root, run
}

func appendFile(t *testing.T, root, rel, add string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, []byte(add)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func editFile(t *testing.T, root, rel, old, new string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Replace(string(b), old, new, 1)
	if s == string(b) {
		t.Fatalf("edit %s: %q not found", rel, old)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The governing skeletons are append-only-shaped (check-governing's
// contract): authoring appends sections below the authoring line and passes
// with the operator trailer; editing a committed skeleton line is refused
// even with the trailer. This is the shape that lets an operand's vision be
// authored at all after the seed commit lands.
func TestGoverningSkeletonsAuthorByAppendOnly(t *testing.T) {
	root, run := gitFixture(t)
	instantiateOperand(t, root)
	run("add", ".")
	run("commit", "-qm", "seed: pyxis init")
	appendFile(t, root, "docs/product-vision.md",
		"\n## 1. North star\n\nOne governing job: ship the outcome the user came for.\n")
	appendFile(t, root, "docs/architecture-principles.md",
		"\n**AP-1 — Fail closed on ambiguity.** Destructive actions require positive evidence; ambiguity never deletes.\n")
	run("add", ".")
	run("commit", "-qm", "author the governing documents\n\nGoverning-Change: operator")
	if d := gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()); len(d) != 0 {
		t.Fatalf("authoring by append with the trailer was refused: %v", d)
	}
	if d := gate.LintGoverning(root, config.Defaults()); len(d) != 0 {
		t.Fatalf("authored governing docs fail the purity lint: %v", d)
	}
	// The negative: a line edit to the committed skeleton head is refused
	// even with the trailer — the trust root is append-only from birth.
	editFile(t, root, "docs/product-vision.md", "# Product Vision", "# Product Vision (revised)")
	run("add", ".")
	run("commit", "-qm", "rewrite the head\n\nGoverning-Change: operator")
	d := gate.CheckGoverning(root, "HEAD~1", "", config.Defaults())
	if len(d) == 0 {
		t.Fatal("editing a committed governing line passed — append-only not enforced on the seeded shape")
	}
}

// check-maps' fill-then-protect behavior over the living maps (a seeded map
// protects no anchors; committed anchors become permanent; removing one is
// caught by name) is covered directly in docgate's gate_v3_test.go:TestCheckMaps
// — the maps are no longer seeded here, so the integration proof moved there.
