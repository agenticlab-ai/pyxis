package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func TestDefaultsWhenAbsent(t *testing.T) {
	c, d := config.Load(t.TempDir())
	if len(d) != 0 || c.PRDs != "docs/prds" || c.DocgateVersion != "source" {
		t.Fatalf("defaults wrong: %+v %v", c, d)
	}
	// The codegate: pin section is additive (codegate-v1 §3.1): a config
	// predating codegate's first release omits it entirely and still parses
	// clean, defaulting to source mode exactly like docgate's own default.
	if c.CodegateVersion != "source" || len(c.CodegateSHA256) != 0 {
		t.Fatalf("codegate defaults wrong: version=%q sha256=%v", c.CodegateVersion, c.CodegateSHA256)
	}
	// The default shapes root resolves OUTSIDE the technical-design corpus,
	// with no defect (solution-shape-v1 §3.1, DINV-3).
	if c.SolutionShapes != "docs/solution-shapes" {
		t.Fatalf("default solution-shapes root wrong: %q", c.SolutionShapes)
	}
}

func TestDraftConfigurationIsClosedAndConfigurable(t *testing.T) {
	c, defects := config.Parse([]byte("schema_version: pyxis-config/v2\npaths:\n  drafts: contracts/work\ndraft:\n  branch_prefix: feature/draft/\n  agent_login: pyxis-agent\n  agent_token_env: PYXIS_DRAFT_AGENT_TOKEN\n"))
	if len(defects) != 0 || c.Drafts != "contracts/work" || c.Draft.BranchPrefix != "feature/draft/" || c.Draft.AgentLogin != "pyxis-agent" || c.Draft.AgentTokenEnv != "PYXIS_DRAFT_AGENT_TOKEN" {
		t.Fatalf("draft config: %+v, %v", c, defects)
	}
	for _, tc := range []struct{ source, want string }{
		{"draft:\n  branch_prefix: ../draft/\n", "draft.branch_prefix"},
		{"draft:\n  agent_token_env: bad-name\n", "draft.agent_token_env"},
		{"draft:\n  branch_perfix: draft/\n", "unknown key"},
		{"paths:\n  drafts: ../escape\n", "paths.drafts"},
	} {
		_, defects := config.Parse([]byte(tc.source))
		if len(defects) == 0 || !strings.Contains(defects[0].Reason, tc.want) {
			t.Errorf("%q: want %q, got %v", tc.source, tc.want, defects)
		}
	}
}

// The shapes root must resolve outside the technical-design corpus: a config
// nesting it under (or equal to) docs/tech-design is a named defect; a
// distinct root parses clean.
func TestSolutionShapesOutsideDesignCorpus(t *testing.T) {
	if _, d := config.Parse([]byte("paths:\n  solution_shapes: docs/shapes\n")); len(d) != 0 {
		t.Fatalf("distinct shapes root flagged: %v", d)
	}
	c, d := config.Parse([]byte("paths:\n  solution_shapes: docs/tech-design/shapes\n"))
	if c.SolutionShapes != "docs/tech-design/shapes" {
		t.Fatalf("shapes root not parsed: %q", c.SolutionShapes)
	}
	found := false
	for _, x := range d {
		if strings.Contains(x.Reason, "not outside the technical-design corpus") {
			found = true
		}
	}
	if !found {
		t.Fatalf("nested shapes root not flagged: %v", d)
	}
}

// The work-item store root defaults to the operand-root .workitems/ bucket
// and is a parseable paths key (local-work-tracker §3.8).
func TestWorkItemsRoot(t *testing.T) {
	if config.Defaults().WorkItems != ".workitems" {
		t.Fatalf("default work-items root wrong: %q", config.Defaults().WorkItems)
	}
	c, d := config.Parse([]byte("paths:\n  work_items: backlog\n"))
	if len(d) != 0 || c.WorkItems != "backlog" {
		t.Fatalf("work_items key not parsed: %q %v", c.WorkItems, d)
	}
}

// The maintenance-sweep policy home is one additive paths key defaulting to the
// repository policy directory (maintenance-sweep-v1 §3.4); overriding it parses
// clean, and it never disturbs the other paths defaults.
func TestMaintenancePolicyKey(t *testing.T) {
	if config.Defaults().MaintenancePolicy != "docs/policy/maintenance-sweep.yaml" {
		t.Fatalf("default maintenance policy home wrong: %q", config.Defaults().MaintenancePolicy)
	}
	c, d := config.Parse([]byte("paths:\n  maintenance_policy: docs/policy/sweep.yaml\n"))
	if len(d) != 0 || c.MaintenancePolicy != "docs/policy/sweep.yaml" {
		t.Fatalf("maintenance_policy key not parsed: %q %v", c.MaintenancePolicy, d)
	}
	// additive: the risk policy home is untouched by the new key.
	if c.RiskPolicy != "docs/policy/risk-policy.yaml" {
		t.Fatalf("risk policy home disturbed: %q", c.RiskPolicy)
	}
}

// The wake channel's knobs are closed-parse durations with their relation
// pinned at load: the staleness bound must strictly exceed the tick interval
// (wake-channel-v1 §3.2). Defaults satisfy the relation.
func TestChannelBounds(t *testing.T) {
	d := config.Defaults()
	if d.Channel.TickInterval != time.Hour || d.Channel.StalenessBound != 24*time.Hour {
		t.Fatalf("default channel bounds wrong: %+v", d.Channel)
	}
	if _, defects := config.Parse(nil); len(defects) != 0 {
		t.Fatalf("defaults violate the relation guard: %v", defects)
	}

	// A well-formed pair parses and the relation holds.
	c, defects := config.Parse([]byte("channel:\n  tick_interval: 30m\n  staleness_bound: 6h\n"))
	if len(defects) != 0 {
		t.Fatalf("valid channel bounds flagged: %v", defects)
	}
	if c.Channel.TickInterval != 30*time.Minute || c.Channel.StalenessBound != 6*time.Hour {
		t.Fatalf("channel bounds not parsed: %+v", c.Channel)
	}
}

// A staleness bound that does not STRICTLY exceed the tick interval is a named
// load defect — both the equal and the smaller case.
func TestChannelRelationGuard(t *testing.T) {
	for _, src := range []string{
		"channel:\n  tick_interval: 24h\n  staleness_bound: 24h\n", // equal
		"channel:\n  tick_interval: 48h\n  staleness_bound: 24h\n", // smaller
		"channel:\n  tick_interval: 48h\n",                         // raised interval past the default bound
	} {
		_, defects := config.Parse([]byte(src))
		found := false
		for _, x := range defects {
			if strings.Contains(x.Reason, "does not strictly exceed") {
				found = true
			}
		}
		if !found {
			t.Fatalf("relation violation not flagged for %q: %v", src, defects)
		}
	}
}

// A malformed or non-positive channel duration is a named defect (closed parse).
func TestChannelMalformedDuration(t *testing.T) {
	for _, src := range []string{
		"channel:\n  tick_interval: soon\n",
		"channel:\n  staleness_bound: -1h\n",
		"channel:\n  tick_interval: 5\n", // bare number, no unit
	} {
		_, defects := config.Parse([]byte(src))
		found := false
		for _, x := range defects {
			if strings.Contains(x.Reason, "not a positive duration") {
				found = true
			}
		}
		if !found {
			t.Fatalf("malformed duration not flagged for %q: %v", src, defects)
		}
	}
}

// An unknown key under channel is rejected — the schema is closed.
func TestChannelClosedSchema(t *testing.T) {
	_, defects := config.Parse([]byte("channel:\n  jitter: 5s\n"))
	found := false
	for _, x := range defects {
		if strings.Contains(x.Reason, "unknown key") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unknown channel key not flagged: %v", defects)
	}
}

// A recognized foreign consumer's block (executor:) coexists on the shared
// pyxis-config surface: docgate recognizes it and skips its whole subtree
// without defects, while still parsing its own blocks around it — and a
// genuinely unknown top-level key is still flagged, so typo-catching for
// docgate's own schema is preserved.
func TestForeignBlockCoexists(t *testing.T) {
	src := `
executor:
  routing:
    default: claude
  gate:
    build: go build ./...
    vet: go vet ./...
paths:
  prds: docs/requirements
plan:
  max_acceptance_criteria: 3
`
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("foreign executor block flagged defects: %v", d)
	}
	if c.PRDs != "docs/requirements" {
		t.Errorf("paths.prds = %q, want docs/requirements (a block after the foreign one)", c.PRDs)
	}
	if c.Plan.MaxAcceptanceCriteria != 3 {
		t.Errorf("plan.max_acceptance_criteria = %d, want 3", c.Plan.MaxAcceptanceCriteria)
	}

	// A non-foreign unknown top-level block is still rejected — the schema stays
	// closed for docgate's own surface.
	_, d = config.Parse([]byte("bogus:\n  x: y\n"))
	found := false
	for _, x := range d {
		if strings.Contains(x.Reason, "unknown top-level") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a non-foreign unknown top-level key was not flagged: %v", d)
	}
}

// harness-config is a foreign consumer block; docgate skips it the same way
// it skips executor: / code-gate:.
func TestHarnessConfigForeignBlock(t *testing.T) {
	src := `
harness-config:
  opencode:
    implementation:
      implementation-economy:
        model: opencode-go/gpt-5.6-luna
paths:
  prds: docs/prds
`
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("foreign harness-config flagged defects: %v", d)
	}
	if c.PRDs != "docs/prds" {
		t.Errorf("paths.prds = %q, want docs/prds", c.PRDs)
	}
}

func TestParse(t *testing.T) {
	valid := `
system:
  version: "0.1.0"
docgate:
  version: "v1.2.0"
  sha256:
    darwin-arm64: "aa"
codegate:
  version: "v2.3.0"
  sha256:
    darwin-arm64: "cc"
    linux-amd64: "dd"
paths:
  prds: docs/requirements
  solution_shapes: docs/shapes
  plans: docs/roadmaps
plan:
  max_acceptance_criteria: 6
capability_paths:
  - src/
`
	c, d := config.Parse([]byte(valid))
	if len(d) != 0 {
		t.Fatalf("valid config flagged: %v", d)
	}
	if c.PRDs != "docs/requirements" || len(c.CapabilityPaths) != 1 || c.CapabilityPaths[0] != "src/" ||
		c.DocgateSHA256["darwin-arm64"] != "aa" || c.RollupPRD != "docs/prd.md" {
		t.Fatalf("parse wrong: %+v", c)
	}
	// The codegate: section parses closed alongside docgate's, same shape
	// (codegate-v1 §3.1): version plus a per-platform sha256 map.
	if c.CodegateVersion != "v2.3.0" || c.CodegateSHA256["darwin-arm64"] != "cc" || c.CodegateSHA256["linux-amd64"] != "dd" {
		t.Fatalf("codegate pin section not parsed: version=%q sha256=%v", c.CodegateVersion, c.CodegateSHA256)
	}
	if c.Plans != "docs/roadmaps" || c.Plan.MaxAcceptanceCriteria != 6 || c.Plan.MaxScopeBullets != 4 {
		t.Fatalf("plan surface wrong: Plans=%q Plan=%+v", c.Plans, c.Plan)
	}
	if c.SolutionShapes != "docs/shapes" {
		t.Fatalf("solution-shapes root not parsed: %q", c.SolutionShapes)
	}
	cases := map[string]string{
		"mood:":                "unknown top-level",
		"mood: fine":           "unrecognized structure",
		"paths:\n  mood: fine": "unknown key",
		"docgate:\n  version: \"source\"\n  sha256:\n    a: \"b\"":  "mutually exclusive",
		"docgate:\n  version: \"v1.0.0\"":                           "requires per-platform checksums",
		"codegate:\n  version: \"source\"\n  sha256:\n    a: \"b\"": "mutually exclusive",
		"codegate:\n  version: \"v1.0.0\"":                          "requires per-platform checksums",
		"codegate:\n  mood: fine":                                   "unknown key",
		"paths:\n  - src/":                                          "list item outside capability_paths",
		"plan:\n  mood: fine":                                       "unknown key",
		"plan:\n  max_scope_bullets: many":                          "not a positive integer",
		"plan:\n  max_est_logic_lines: -3":                          "not a positive integer",
	}
	for in, want := range cases {
		_, d := config.Parse([]byte(in))
		found := false
		for _, x := range d {
			if strings.Contains(x.Reason+x.Remedy, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("input %q: no defect containing %q (got %v)", in, want, d)
		}
	}
}

// ——— schema v2 (installer-v1 §3.7/§3.10; plan installer/T2) ———

// containsDefect scans reasons+remedies for a substring, the house pattern.
func containsDefect(t *testing.T, src, want string) bool {
	t.Helper()
	_, d := config.Parse([]byte(src))
	for _, x := range d {
		if strings.Contains(x.Reason+x.Remedy, want) {
			return true
		}
	}
	t.Logf("input %q: defects %v", src, d)
	return false
}

// TestV2ShapeExample parses the solution shape's operand-config example
// (docs/solution-shapes/installer-v1.md) structure-verbatim — flow forms,
// trailing comments, quoted scalars — and asserts every v2 field lands.
func TestV2ShapeExample(t *testing.T) {
	src := `
schema_version: pyxis-config/v2
system:
  version: "v1.2.3"                  # the one release pin (design DINV-2)
  manifest_sha256: "ab12cd"          # the trust root
  artifact_source: ""                # optional FR-22 override
  mode: release                      # release | source
knobs:
  tracker: github                    # github | carina
  ci_mode: both                      # local | actions | both
  wiring: { repo: use, protection: required-checks }   # use|create · none|required-checks|checks+review
  cli_surfaces: [claude, codex]      # auto-detect result or explicit set
  argus: false                       # opt-in, off by default (prd FR-16)
branch: main                         # captured; only main honored in v1
paths:
  prds: docs/prds
  tech_designs: docs/tech-design
  solution_shapes: docs/solution-shapes
epoch:
  system_version: "v1.2.3"
  commit: "deadbeef"
  manifest_sha256: "ab12cd"
`
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("the shape's example config flagged: %v", d)
	}
	if c.SchemaVersion != config.SchemaVersionV2 {
		t.Errorf("SchemaVersion = %q, want %q", c.SchemaVersion, config.SchemaVersionV2)
	}
	if c.System.Version != "v1.2.3" || c.System.ManifestSHA256 != "ab12cd" || c.System.ArtifactSource != "" || c.System.Mode != "release" {
		t.Errorf("system block wrong: %+v", c.System)
	}
	// One key, both surfaces: pre-v2 consumers keep reading SystemVersion (NR-3).
	if c.SystemVersion != "v1.2.3" {
		t.Errorf("SystemVersion = %q, want the system.version value on both surfaces", c.SystemVersion)
	}
	if c.Knobs.Tracker != "github" || c.Knobs.CIMode != "both" || c.Knobs.Argus {
		t.Errorf("knobs wrong: %+v", c.Knobs)
	}
	if c.Knobs.Wiring.Repo != "use" || c.Knobs.Wiring.Protection != "required-checks" {
		t.Errorf("wiring flow map wrong: %+v", c.Knobs.Wiring)
	}
	if len(c.Knobs.CLISurfaces) != 2 || c.Knobs.CLISurfaces[0] != "claude" || c.Knobs.CLISurfaces[1] != "codex" {
		t.Errorf("cli_surfaces flow list wrong: %v", c.Knobs.CLISurfaces)
	}
	if c.Branch != "main" {
		t.Errorf("branch = %q, want main", c.Branch)
	}
	if c.Epoch.SystemVersion != "v1.2.3" || c.Epoch.Commit != "deadbeef" || c.Epoch.ManifestSHA256 != "ab12cd" {
		t.Errorf("epoch block wrong: %+v", c.Epoch)
	}
	// Default paths spelled explicitly parse clean under v2 (captured, not refused).
	if c.PRDs != "docs/prds" || c.TechDesigns != "docs/tech-design" || c.SolutionShapes != "docs/solution-shapes" {
		t.Errorf("default paths wrong under v2: %q %q %q", c.PRDs, c.TechDesigns, c.SolutionShapes)
	}
	if len(c.Deprecations()) != 0 {
		t.Errorf("a pin-clean v2 config carries deprecations: %v", c.Deprecations())
	}
}

// TestV2SystemBlock covers the system: block's parse-level contract: both
// modes are admitted (source-legality is the resolver's DINV-8 refusal, T3),
// a bad mode is a named enum defect, the epoch block is optional and closed,
// and a minimal v2 config parses clean.
func TestV2SystemBlock(t *testing.T) {
	// mode: source is ACCEPTED at parse — enforcement lives in the resolver.
	c, d := config.Parse([]byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  mode: source\n"))
	if len(d) != 0 || c.System.Mode != "source" {
		t.Fatalf("mode source not admitted: mode=%q defects=%v", c.System.Mode, d)
	}
	if !containsDefect(t, "schema_version: pyxis-config/v2\nsystem:\n  mode: hybrid\n", "not one of release|source") {
		t.Error("bad mode not refused by name")
	}
	// artifact_source override parses.
	c, d = config.Parse([]byte("schema_version: pyxis-config/v2\nsystem:\n  artifact_source: \"https://mirror.example/rel\"\n"))
	if len(d) != 0 || c.System.ArtifactSource != "https://mirror.example/rel" {
		t.Fatalf("artifact_source not parsed: %q %v", c.System.ArtifactSource, d)
	}
	// Minimal v2: the marker alone is a valid config; epoch stays zero until
	// init writes it (optional block).
	c, d = config.Parse([]byte("schema_version: pyxis-config/v2\n"))
	if len(d) != 0 {
		t.Fatalf("minimal v2 flagged: %v", d)
	}
	if c.Epoch != (config.Epoch{}) {
		t.Errorf("epoch not zero on a minimal v2 config: %+v", c.Epoch)
	}
	// A partial epoch block parses closed.
	c, d = config.Parse([]byte("schema_version: pyxis-config/v2\nepoch:\n  commit: \"abc\"\n"))
	if len(d) != 0 || c.Epoch.Commit != "abc" || c.Epoch.SystemVersion != "" {
		t.Fatalf("partial epoch wrong: %+v %v", c.Epoch, d)
	}
	if !containsDefect(t, "schema_version: pyxis-config/v2\nepoch:\n  mood: x\n", "unknown key") {
		t.Error("unknown epoch key not refused")
	}
	// An unrecognized schema_version value refuses by name.
	if !containsDefect(t, "schema_version: pyxis-config/v3\n", "unsupported schema_version") {
		t.Error("unsupported schema_version not refused by name")
	}
	// An unknown top-level section stays refused under v2 — the schema is closed.
	if !containsDefect(t, "schema_version: pyxis-config/v2\nmood:\n  x: y\n", "unknown top-level") {
		t.Error("unknown top-level section not refused under v2")
	}
}

// TestV2ExtraPinKeys is the design's A-4 leg: under v2 the pin surface is
// exactly the system pair — any further pin-shaped key inside system:
// refuses by name (DINV-2).
func TestV2ExtraPinKeys(t *testing.T) {
	header := "schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  manifest_sha256: \"aa\"\n"
	for _, extra := range []string{
		"  docgate_sha256: \"bb\"\n",
		"  codegate_version: \"v1.0.0\"\n",
		"  sha256:\n",
	} {
		if !containsDefect(t, header+extra, "beyond the system pair") {
			t.Errorf("extra pin key %q not refused by name", strings.TrimSpace(extra))
		}
	}
	// A non-pin-shaped unknown key in system: keeps the generic closed-schema refusal.
	if !containsDefect(t, header+"  mood: fine\n", "unknown key") {
		t.Error("non-pin unknown key in system not refused")
	}
}

// TestV2KnobsClosedParse covers the knobs: block — the five captured
// decisions with closed enums, the wiring flow map, and the cli_surfaces
// flow list, all refusing by name on any deviation.
func TestV2KnobsClosedParse(t *testing.T) {
	valid := "schema_version: pyxis-config/v2\nknobs:\n" +
		"  tracker: carina\n" +
		"  ci_mode: local\n" +
		"  wiring: { repo: create, protection: checks+review }\n" +
		"  cli_surfaces: [claude]\n" +
		"  argus: true\n"
	c, d := config.Parse([]byte(valid))
	if len(d) != 0 {
		t.Fatalf("valid knobs flagged: %v", d)
	}
	if c.Knobs.Tracker != "carina" || c.Knobs.CIMode != "local" || !c.Knobs.Argus ||
		c.Knobs.Wiring.Repo != "create" || c.Knobs.Wiring.Protection != "checks+review" ||
		len(c.Knobs.CLISurfaces) != 1 || c.Knobs.CLISurfaces[0] != "claude" {
		t.Fatalf("knobs not parsed: %+v", c.Knobs)
	}
	// An empty flow list is admitted (nothing wired).
	c, d = config.Parse([]byte("schema_version: pyxis-config/v2\nknobs:\n  cli_surfaces: []\n"))
	if len(d) != 0 || len(c.Knobs.CLISurfaces) != 0 {
		t.Fatalf("empty cli_surfaces wrong: %v %v", c.Knobs.CLISurfaces, d)
	}
	cases := map[string]string{
		"  tracker: gitlab\n":                          "not one of github|carina",
		"  ci_mode: never\n":                           "not one of local|actions|both",
		"  wiring: { repo: fork, protection: none }\n": "not one of use|create",
		"  wiring: { repo: use, protection: full }\n":  "not one of none|required-checks|checks+review",
		"  wiring: use\n":                              "flow map",
		"  wiring: { repo: use, mood: x }\n":           `unknown key "mood" in "knobs.wiring"`,
		"  cli_surfaces: claude\n":                     "flow list",
		"  argus: maybe\n":                             "not a boolean",
		"  mood: x\n":                                  `unknown key "mood" in "knobs"`,
	}
	for in, want := range cases {
		if !containsDefect(t, "schema_version: pyxis-config/v2\nknobs:\n"+in, want) {
			t.Errorf("knobs input %q: no defect containing %q", strings.TrimSpace(in), want)
		}
	}
}

// TestV2DualReadDeprecation is the window's open half (installer-v1 §3.10):
// legacy per-tool pin sections parse in BOTH generations; under v2 they
// surface deprecation notices through Deprecations() — doctor's feed, never
// parse defects. Legacy validations (DINV-2's pairings) still apply to the
// dual-read sections.
func TestV2DualReadDeprecation(t *testing.T) {
	src := "schema_version: pyxis-config/v2\n" +
		"system:\n  version: \"v1.0.0\"\n  manifest_sha256: \"aa\"\n" +
		"docgate:\n  version: \"source\"\n" +
		"codegate:\n  version: \"source\"\n"
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("dual-read v2 config flagged: %v", d)
	}
	if c.DocgateVersion != "source" || c.CodegateVersion != "source" {
		t.Fatalf("legacy pin sections not dual-read: %q %q", c.DocgateVersion, c.CodegateVersion)
	}
	dep := c.Deprecations()
	if len(dep) != 2 {
		t.Fatalf("want 2 deprecation notices, got %v", dep)
	}
	joined := strings.Join(dep, "\n")
	for _, want := range []string{`"docgate"`, `"codegate"`, "dual-read window"} {
		if !strings.Contains(joined, want) {
			t.Errorf("deprecation notices missing %q: %v", want, dep)
		}
	}
	// Under legacy v1 the same sections yield NO notices — the whole config
	// is legacy; the notice marks a mixed, part-migrated v2 config.
	c, d = config.Parse([]byte("docgate:\n  version: \"source\"\n"))
	if len(d) != 0 || len(c.Deprecations()) != 0 {
		t.Fatalf("v1 config grew deprecations: %v %v", c.Deprecations(), d)
	}
	// Dual-read means fully validated: a release pin with no checksums in a
	// v2 config still trips the legacy DINV-2 pairing.
	if !containsDefect(t, "schema_version: pyxis-config/v2\ndocgate:\n  version: \"v1.0.0\"\n", "requires per-platform checksums") {
		t.Error("legacy DINV-2 validation lost under dual-read")
	}
}

// TestLegacyPinWindowClosed flips the window switch — the LATER release's
// posture — and asserts the legacy pin sections refuse by name with their
// subtrees skipped (one named defect, no per-line noise), while everything
// else keeps parsing.
func TestLegacyPinWindowClosed(t *testing.T) {
	config.LegacyPinWindowOpen = false
	defer func() { config.LegacyPinWindowOpen = true }()

	// v2 config: the section refuses by name; no deprecation notice (it is
	// refused, not tolerated).
	c, d := config.Parse([]byte("schema_version: pyxis-config/v2\ndocgate:\n  version: \"source\"\n"))
	if len(d) != 1 || !strings.Contains(d[0].Reason, "dual-read window is closed") || !strings.Contains(d[0].Reason, `"docgate"`) {
		t.Fatalf("closed window: want exactly the one named refusal, got %v", d)
	}
	if len(c.Deprecations()) != 0 {
		t.Errorf("a refused section still produced deprecations: %v", c.Deprecations())
	}
	// The window governs the sections themselves, whichever generation
	// carries them: a legacy v1 config's pin section refuses too.
	_, d = config.Parse([]byte("codegate:\n  version: \"v1.0.0\"\n  sha256:\n    darwin-arm64: \"aa\"\n"))
	if len(d) != 1 || !strings.Contains(d[0].Reason, `"codegate"`) {
		t.Fatalf("closed window on v1: want exactly the one named refusal, got %v", d)
	}
	// Pin-free configs are untouched by the closing.
	if _, d = config.Parse([]byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v1.0.0\"\n  manifest_sha256: \"aa\"\n")); len(d) != 0 {
		t.Errorf("pin-free v2 config flagged under the closed window: %v", d)
	}
	if _, d = config.Parse([]byte("paths:\n  prds: docs/requirements\n")); len(d) != 0 {
		t.Errorf("pin-free v1 config flagged under the closed window: %v", d)
	}
}

// TestV2OnlyKeysRefusedUnderV1: the v1 grammar is frozen — every v2-only
// section, scalar, and system-block key in a config without the v2 marker
// refuses by name, pointing at the marker.
func TestV2OnlyKeysRefusedUnderV1(t *testing.T) {
	cases := map[string]string{
		"knobs:\n  tracker: github\n":         `"knobs" is a pyxis-config/v2 section`,
		"epoch:\n  commit: \"a\"\n":           `"epoch" is a pyxis-config/v2 section`,
		"branch: main\n":                      "branch is a pyxis-config/v2 key",
		"remote: origin\n":                    "remote is a pyxis-config/v2 key",
		"system:\n  manifest_sha256: \"a\"\n": "system.manifest_sha256 is a pyxis-config/v2 key",
		"system:\n  artifact_source: \"x\"\n": "system.artifact_source is a pyxis-config/v2 key",
		"system:\n  mode: source\n":           "system.mode is a pyxis-config/v2 key",
	}
	for in, want := range cases {
		if !containsDefect(t, in, want) {
			t.Errorf("v1 input %q: no defect containing %q", in, want)
		}
	}
	// A gated section is skipped wholesale: one named defect, no noise.
	if _, d := config.Parse([]byte("knobs:\n  tracker: github\n  argus: true\n")); len(d) != 1 {
		t.Errorf("gated v2 section not skipped wholesale under v1: %v", d)
	}
}

// TestV2Defaults pins the new compiled defaults: an absent config file (the
// pre-installer posture) mirrors the per-tool "source" stance and declares
// no schema generation.
func TestV2Defaults(t *testing.T) {
	d := config.Defaults()
	if d.SchemaVersion != "" {
		t.Errorf("Defaults SchemaVersion = %q, want legacy \"\"", d.SchemaVersion)
	}
	if d.Branch != "main" {
		t.Errorf("Defaults Branch = %q, want main", d.Branch)
	}
	if d.Remote != "origin" {
		t.Errorf("Defaults Remote = %q, want origin", d.Remote)
	}
	if d.MainlineRef() != "origin/main" {
		t.Errorf("Defaults MainlineRef = %q, want origin/main (byte-identical, DINV-2)", d.MainlineRef())
	}
	if d.System.Version != d.SystemVersion || d.System.Mode != "source" || d.System.ManifestSHA256 != "" || d.System.ArtifactSource != "" {
		t.Errorf("Defaults System wrong: %+v (SystemVersion %q)", d.System, d.SystemVersion)
	}
	if d.Knobs.Tracker != "" || d.Knobs.CIMode != "" || d.Knobs.Argus || len(d.Knobs.CLISurfaces) != 0 {
		t.Errorf("Defaults Knobs not zero (knobs are captured at init, never compiled): %+v", d.Knobs)
	}
	if d.Epoch != (config.Epoch{}) {
		t.Errorf("Defaults Epoch not zero: %+v", d.Epoch)
	}
	if len(d.Deprecations()) != 0 {
		t.Errorf("Defaults carry deprecations: %v", d.Deprecations())
	}
	c, defects := config.Load(t.TempDir())
	if len(defects) != 0 || c.Branch != "main" || c.Remote != "origin" || c.System.Mode != "source" {
		t.Errorf("absent-file Load defaults wrong: branch=%q remote=%q %+v %v", c.Branch, c.Remote, c.System, defects)
	}
}

// TestV2RemoteScalar covers the remote top-level scalar (config-driven-
// portability-v1 §3.2, FR-2), mirroring branch: a v2 config may set a non-
// default remote; an absent remote takes the "origin" default (byte-identical);
// and MainlineRef composes the configured pair.
func TestV2RemoteScalar(t *testing.T) {
	// A v2 config setting BOTH a non-default branch and an explicit remote.
	nonDefault := `schema_version: pyxis-config/v2
branch: trunk
remote: upstream
`
	c, d := config.Parse([]byte(nonDefault))
	if len(d) != 0 {
		t.Fatalf("v2 branch+remote config flagged: %v", d)
	}
	if c.Branch != "trunk" || c.Remote != "upstream" {
		t.Fatalf("parsed branch/remote = %q/%q, want trunk/upstream", c.Branch, c.Remote)
	}
	if c.MainlineRef() != "upstream/trunk" {
		t.Fatalf("MainlineRef = %q, want upstream/trunk", c.MainlineRef())
	}

	// A v2 config that sets branch but NOT remote takes the origin default —
	// byte-identical to the pre-change engine for the remote (DINV-2, FR-6).
	branchOnly := `schema_version: pyxis-config/v2
branch: trunk
`
	c2, d2 := config.Parse([]byte(branchOnly))
	if len(d2) != 0 {
		t.Fatalf("v2 branch-only config flagged: %v", d2)
	}
	if c2.Remote != "origin" {
		t.Fatalf("absent remote = %q, want the origin default", c2.Remote)
	}
	if c2.MainlineRef() != "origin/trunk" {
		t.Fatalf("MainlineRef = %q, want origin/trunk", c2.MainlineRef())
	}
}

// TestPathClosureConformance is THE audit surface installer-v1 §3.7 names:
// the paths: schema is the authoritative, closed list of every
// per-repo-configurable single path the engine reads. The test (a) asserts
// the schema key set equals the audited engine path-read set declared here
// — a new engine path read must land in pathSchema AND in this list, a
// schema key without an engine read must leave both — and (b) sweeps every
// older key for the FR-18/UB-8 posture: under v2 a non-default value refuses with
// the named not-yet-configurable defect (zero silent-ignore), the compiled
// default parses clean, and legacy v1 behavior is untouched.
//
// The audited set below is the 2026-07-19 read audit of Config's consumers
// (docgate gate/fold/contractgate, plangate, carina, lane policy,
// implement-now). List-valued path surfaces (system_maps,
// capability_paths, the grandfather lists) have their own top-level schema
// sections and are deliberately outside the paths: block.
func TestPathClosureConformance(t *testing.T) {
	audited := map[string]string{
		"prds":                    "docs/prds",
		"drafts":                  "drafts",
		"feedback":                "docs/feedback",
		"tech_designs":            "docs/tech-design",
		"solution_shapes":         "docs/solution-shapes",
		"brainstorms":             "docs/brainstorms",
		"problems":                "docs/problems",
		"reviews":                 "docs/reviews",
		"wiki":                    "docs/wiki",
		"rollup_prd":              "docs/prd.md",
		"rollup_techspec":         "docs/techspec.md",
		"incidents":               "docs/observability/incidents.md",
		"product_vision":          "docs/product-vision.md",
		"architecture_principles": "docs/architecture-principles.md",
		"design_principles":       "docs/design-principles.md",
		"plans":                   "docs/plans",
		"work_items":              ".workitems",
		"risk_policy":             "docs/policy/risk-policy.yaml",
		"maintenance_policy":      "docs/policy/maintenance-sweep.yaml",
		"templates_dir":           ".pyxis/templates",
	}
	got := config.PathDefaults()
	for k := range audited {
		if _, ok := got[k]; !ok {
			t.Errorf("audited engine path read %q has no paths: schema key — the closure is broken (installer-v1 §3.7)", k)
		}
	}
	for k, def := range got {
		want, ok := audited[k]
		if !ok {
			t.Errorf("schema key %q is not in the audited engine path-read set — audit the new read or drop the key", k)
			continue
		}
		if def != want {
			t.Errorf("compiled default for paths.%s = %q, want %q", k, def, want)
		}
	}

	// The refuse-non-default sweep applies to every captured installer path;
	// drafts arrived later as a configurable working-file root.
	for k, def := range got {
		nonDefault := "elsewhere/" + k
		v2 := "schema_version: pyxis-config/v2\npaths:\n  " + k + ": " + nonDefault + "\n"
		_, d := config.Parse([]byte(v2))
		if k == "drafts" {
			if len(d) != 0 {
				t.Errorf("paths.drafts: new draft working-file root refused: %v", d)
			}
			continue // introduced as a configurable path after the installer captured its older paths
		}
		found := false
		for _, x := range d {
			if strings.Contains(x.Reason, "paths."+k) && strings.Contains(x.Reason, "not yet configurable") {
				found = true
			}
		}
		if !found {
			t.Errorf("paths.%s: non-default value silently ignored under v2 — want the named not-yet-configurable refusal (got %v)", k, d)
		}
		// The compiled default, spelled explicitly, parses clean under v2.
		if _, d := config.Parse([]byte("schema_version: pyxis-config/v2\npaths:\n  " + k + ": " + def + "\n")); len(d) != 0 {
			t.Errorf("paths.%s: compiled default %q refused under v2: %v", k, def, d)
		}
		// Legacy v1 behavior is preserved: the same non-default value still
		// parses into use with no defect (NR-3 — existing consumers unchanged).
		if _, d := config.Parse([]byte("paths:\n  " + k + ": " + nonDefault + "\n")); len(d) != 0 {
			t.Errorf("paths.%s: v1 non-default behavior changed: %v", k, d)
		}
	}
}

// ——— A-4: load-time lexical containment (config-driven-portability-v1 §3.3,
// #DINV-3, #A-4) ———

// hasDefectContaining reports whether ANY defect's reason contains want — the
// house scan, but returning the defect slice too so a caller can also assert
// "no partial resolution" (a non-empty slice = the configuration is refused).
func hasDefectContaining(src, want string) (bool, []schema.Defect) {
	_, d := config.Parse([]byte(src))
	for _, x := range d {
		if strings.Contains(x.Reason, want) {
			return true, d
		}
	}
	return false, d
}

// TestConfigContainmentRefusals is the A-4 leg for the FOUR named cases —
// absolute, escaping (`..`), non-relative, and unknown-key — each refused BY
// NAME at load, with NO partial resolution (config-driven-portability-v1 §3.3,
// #DINV-3; A-4; prd:config-driven-portability/FR-5 · UB-1 · UB-2 · ABUSE-1 ·
// NR-2). Containment is validated OVER SINGLE-PATH keys (pathSchema) and the
// path-typed list surfaces (system_maps, capability_paths); it is a lexical,
// hermetic check (no filesystem, no symlink resolution).
func TestConfigContainmentRefusals(t *testing.T) {
	cases := []struct {
		name string // the A-4 leg
		src  string
		key  string // the offending config key/entry the defect must name
		want string // the containment reason substring
	}{
		// A-4 leg 1 — ABSOLUTE: a single-path key whose value escapes the root.
		{"absolute", "paths:\n  prds: /etc/x\n", "paths.prds", "absolute path"},
		// A-4 leg 2 — ESCAPING: a `..` parent-directory escape.
		{"escaping", "paths:\n  reviews: ../outside\n", "paths.reviews", "escapes the repository root"},
		// A-4 leg 2 — ESCAPING even when the escape hides mid-path (Clean resolves it).
		{"escaping-midpath", "paths:\n  wiki: docs/../../secret\n", "paths.wiki", "escapes the repository root"},
		// A-4 leg 3 — NON-RELATIVE: an empty value names no repo-relative location.
		{"non-relative-empty", "paths:\n  incidents:\n", "paths.incidents", "not a repository-relative path"},
		// The path-typed list surfaces are validated per entry, named by section.
		{"system_maps-absolute", "system_maps:\n  - /var/maps.md\n", "system_maps entry", "absolute path"},
		{"system_maps-escaping", "system_maps:\n  - ../../maps.md\n", "system_maps entry", "escapes the repository root"},
		{"capability_paths-absolute", "capability_paths:\n  - /usr/bin\n", "capability_paths entry", "absolute path"},
		{"capability_paths-escaping", "capability_paths:\n  - ../elsewhere\n", "capability_paths entry", "escapes the repository root"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, d := hasDefectContaining(tc.src, tc.want)
			if !ok {
				t.Fatalf("case %q: no containment defect containing %q (got %v)", tc.name, tc.want, d)
			}
			// Refused BY NAME: some defect names the offending key/entry.
			named := false
			for _, x := range d {
				if strings.Contains(x.Reason, tc.key) {
					named = true
				}
			}
			if !named {
				t.Errorf("case %q: refusal does not name the offending key %q: %v", tc.name, tc.key, d)
			}
			// NO PARTIAL RESOLUTION: every defect rides the ONE named-defect
			// envelope def() stamps — verb "config", file ".pyxis/config.yaml",
			// op "parse". That is the pre-existing closed-schema fail-closed
			// refusal path (§3.3), so the refusal is raised BY THE PARSE, on the
			// same slice every existing defect-honoring caller already stops on,
			// before any consumer resolves a path.
			for _, x := range d {
				if x.Verb != "config" || x.File != ".pyxis/config.yaml" || x.Op != "parse" {
					t.Errorf("case %q: defect is off the config-load refusal path: %+v", tc.name, x)
				}
				if x.Remedy == "" {
					t.Errorf("case %q: refusal names no remedy: %+v", tc.name, x)
				}
			}
		})
	}
}

// TestConfigContainmentRefusedAtLoadBoundary proves the refusal happens at
// LOAD — the entry point every consumer actually calls — and not somewhere
// downstream: Load reads the file and returns the named containment defect
// itself, so a defect-honoring caller stops before resolving anything
// (config-driven-portability-v1 §3.3, #DINV-3; A-4's "at load, with no partial
// resolution").
func TestConfigContainmentRefusedAtLoadBoundary(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("paths:\n  prds: /etc/prds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, d := config.Load(root)
	found := false
	for _, x := range d {
		if strings.Contains(x.Reason, "paths.prds") && strings.Contains(x.Reason, "absolute path") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Load did not refuse the absolute configured path by name: %v", d)
	}
}

// TestConfigContainmentUnderSchemaV2 pins the interaction with the v2 captured-
// default posture (FR-18/UB-8). Under pyxis-config/v2 a non-default paths value
// never lands — it is refused earlier as not-yet-configurable — so A-4's
// by-name refusal holds under BOTH schema generations, by two different named
// defects. The list surfaces are NOT captured, so they reach the containment
// pass under v2 exactly as under v1. When the adoption follow-up lifts the
// captured-default posture, the paths keys start landing and this test's first
// leg is what keeps the escape refused rather than silently accepted.
func TestConfigContainmentUnderSchemaV2(t *testing.T) {
	const v2 = "schema_version: pyxis-config/v2\n"
	// A captured paths key: refused by name, and the escaping value never lands.
	c, d := config.Parse([]byte(v2 + "paths:\n  prds: /etc/x\n"))
	if len(d) == 0 {
		t.Fatalf("v2: an absolute configured path was accepted")
	}
	named := false
	for _, x := range d {
		if strings.Contains(x.Reason, "paths.prds") {
			named = true
		}
	}
	if !named {
		t.Errorf("v2: refusal does not name paths.prds: %v", d)
	}
	if c.PRDs != "docs/prds" {
		t.Errorf("v2: the refused value landed anyway: %q", c.PRDs)
	}
	// A list surface under v2 reaches the containment pass.
	if ok, d := hasDefectContaining(v2+"capability_paths:\n  - ../elsewhere\n", "escapes the repository root"); !ok {
		t.Fatalf("v2: an escaping capability_paths entry was accepted: %v", d)
	}
}

// TestConfigContainmentDefectOrderIsDeterministic guards the SORTED walk over
// pathSchema. The table is a map and Go randomizes map iteration, so an
// unsorted walk renders a multi-key violation in a different order run to run —
// non-reproducible gate output. The assertion is the exact sorted key sequence
// (not merely "two runs agree", which a skewed iteration order can satisfy by
// luck), repeated so any residual ordering drift also fails.
func TestConfigContainmentDefectOrderIsDeterministic(t *testing.T) {
	// Four offending keys, deliberately written in NON-sorted order.
	src := "paths:\n  wiki: /a\n  prds: /b\n  plans: /c\n  reviews: /d\n"
	want := []string{"paths.plans", "paths.prds", "paths.reviews", "paths.wiki"}
	for i := range 32 {
		_, d := config.Parse([]byte(src))
		var got []string
		for _, x := range d {
			if k, _, ok := strings.Cut(x.Reason, " "); ok {
				got = append(got, k)
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d: containment defects out of sorted key order:\n got %v\nwant %v", i, got, want)
		}
	}
}

// TestConfigContainmentUnknownKeyPreserved is A-4's unknown-key leg AND NR-2:
// the pre-existing closed-schema unknown-key refusal still fires after the
// containment extension — the new pass only ADDS containment defects, it never
// relaxes the closed grammar.
func TestConfigContainmentUnknownKeyPreserved(t *testing.T) {
	// Unknown key under paths: (the classic NR-2 case) still refused by name.
	if ok, d := hasDefectContaining("paths:\n  mood: docs/x\n", "unknown key"); !ok {
		t.Fatalf("unknown paths key no longer refused after the containment extension: %v", d)
	}
	// Unknown TOP-LEVEL key still refused — the closed grammar is intact.
	if ok, d := hasDefectContaining("bogus:\n  x: y\n", "unknown top-level"); !ok {
		t.Fatalf("unknown top-level key no longer refused: %v", d)
	}
	// An unknown key AND an escaping value together: BOTH refusals surface —
	// containment does not swallow the unknown-key refusal, and vice versa.
	_, d := config.Parse([]byte("paths:\n  mood: ../x\n  prds: /abs\n"))
	var sawUnknown, sawAbsolute bool
	for _, x := range d {
		if strings.Contains(x.Reason, "unknown key") {
			sawUnknown = true
		}
		if strings.Contains(x.Reason, "paths.prds") && strings.Contains(x.Reason, "absolute path") {
			sawAbsolute = true
		}
	}
	if !sawUnknown || !sawAbsolute {
		t.Fatalf("want BOTH the unknown-key and the containment refusal: unknown=%v absolute=%v (%v)", sawUnknown, sawAbsolute, d)
	}
}

// TestConfigContainmentValidLoadsUnchanged is A-4's valid leg and DINV-2/FR-6:
// a configuration with only valid repository-relative paths loads with ZERO
// defects and its values land unchanged — the containment pass must NOT over-
// refuse a legitimate configured path (R3). Covers ordinary corpora, the
// dot-prefixed system trees (.pyxis/, .workitems), trailing-slash trigger
// paths, and both path-typed list surfaces.
func TestConfigContainmentValidLoadsUnchanged(t *testing.T) {
	src := `paths:
  prds: docs/requirements
  wiki: docs/knowledge/wiki
  work_items: .backlog
  templates_dir: .pyxis/templates
system_maps:
  - docs/domain-model.md
  - docs/system-design.md
capability_paths:
  - cmd/
  - internal/
  - .pyxis/
`
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("a valid repository-relative config was refused (over-refusal, R3): %v", d)
	}
	if c.PRDs != "docs/requirements" || c.Wiki != "docs/knowledge/wiki" || c.WorkItems != ".backlog" || c.TemplatesDir != ".pyxis/templates" {
		t.Fatalf("valid single-path values not landed unchanged: %+v", c)
	}
	if len(c.SystemMaps) != 2 || len(c.CapabilityPaths) != 3 || c.CapabilityPaths[2] != ".pyxis/" {
		t.Fatalf("valid list-surface values not landed unchanged: maps=%v caps=%v", c.SystemMaps, c.CapabilityPaths)
	}
	// The compiled defaults (an absent/empty config) are all clean repo-
	// relative paths and MUST load clean — byte-identical to the pre-change
	// engine (DINV-2, A-3). This is the strongest over-refusal guard.
	if _, d := config.Parse(nil); len(d) != 0 {
		t.Fatalf("the compiled defaults were refused by the containment pass (DINV-2 broken): %v", d)
	}
}

// TestConfigContainmentGrandfatherListsUnaffected proves the deliberate
// exclusion (config-driven-portability-v1 §3.3): the grandfather lists are NOT
// tree paths — grandfathered_designs / precedence_grandfathered hold design
// SLUGS and grandfathered_citations holds `<file> | <token>` citation pairs —
// so containment does not apply. A leading-dot or slash-bearing SLUG must not
// be mistaken for an escaping path.
func TestConfigContainmentGrandfatherListsUnaffected(t *testing.T) {
	// Slugs and citation tokens that would trip a naive path check MUST parse
	// clean — they are not locations the engine joins under the repo root.
	src := `grandfathered_designs:
  - some-design-slug
precedence_grandfathered:
  - another-slug
grandfathered_citations:
  - docs/wiki/x.md | SOME-TOKEN
`
	c, d := config.Parse([]byte(src))
	if len(d) != 0 {
		t.Fatalf("a grandfather list was wrongly subjected to containment: %v", d)
	}
	if len(c.Grandfathered) != 1 || len(c.PrecedenceGrandfathered) != 1 || len(c.GrandfatheredCitations) != 1 {
		t.Fatalf("grandfather lists not parsed: %+v", c)
	}
}
