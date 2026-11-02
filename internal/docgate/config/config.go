// Package config parses .pyxis/config.yaml — the pyxis config surface
// (repo-layout-and-system-footprint §3.6, phase 1; schema v2 per
// installer-v1 §3.7). The grammar is a CLOSED YAML subset covering exactly
// the pinned schema: top-level scalars, two-level maps of string scalars,
// string lists, and two single-line flow forms (the knobs.wiring map and
// the knobs.cli_surfaces list, spelled exactly as the installer solution
// shape shows). Unknown keys, structure, or syntax are defects (A-4);
// anything richer belongs to a future phase. An absent file yields the
// defaults (today's locations — rule 7's binding as data). Stdlib only,
// per the repo Go rules.
//
// Two schema generations share the one surface (installer-v1 §3.10):
//
//   - legacy v1 — no schema_version key: today's grammar unchanged — the
//     per-tool pin sections (docgate:, codegate:), paths read into use,
//     and the policy blocks (plan:, channel:, templates:, …).
//   - pyxis-config/v2 — declared by `schema_version: pyxis-config/v2`:
//     adds the system: block (the pin pair + artifact_source + mode), the
//     five captured knobs:, the branch: scalar, and the epoch: block;
//     every non-pin v1 section keeps parsing unchanged. Under v2 the older
//     paths: keys remain CAPTURED defaults (FR-18/UB-8); paths.drafts was
//     introduced later as a configurable feature-draft working-file root.
//
// The generations meet in the dual-read window (installer-v1 §3.10, Q-3):
// while LegacyPinWindowOpen, the legacy per-tool pin sections parse under
// BOTH generations — under v2 they additionally surface a deprecation
// notice via Config.Deprecations() (doctor's feed, never a parse defect).
// The window's closing release flips the switch; a legacy pin section then
// refuses by name.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/entity"
)

// SchemaVersionV2 is the one recognized schema_version value (installer-v1
// §3.7). A config omitting the key is legacy v1; any other value is a
// named defect.
const SchemaVersionV2 = "pyxis-config/v2"

// LegacyPinWindowOpen is THE dual-read window switch (installer-v1 §3.10;
// the window's length is the design's open Q-3, an operator decision at
// the first system release). Default OPEN: the legacy per-tool pin
// sections (docgate:, codegate:) parse in both schema generations,
// surfacing Deprecations() under v2. The closing release flips this to
// false — a legacy pin section then refuses by name, completing the
// migration (design A-4).
var LegacyPinWindowOpen = true

// Config is the parsed surface. String scalars, one int block (plan sizing),
// string lists, and the v2 blocks — still the closed subset, nothing richer.
type Config struct {
	SchemaVersion           string            // "" = legacy v1; SchemaVersionV2 once declared (installer-v1 §3.7)
	System                  System            // the v2 system: block — the pin pair + acquisition knobs
	Knobs                   Knobs             // the five install knobs, captured at init (prd:installer/FR-3)
	Branch                  string            // the repo's default branch — the mainline gates diff, hooks resolve, and the lane/executor/plan-tracker reference (config-driven-portability-v1 §3.2, FR-2); absent → "main"
	Remote                  string            // the git remote the same consumers fetch and resolve refs against (config-driven-portability-v1 §3.2, FR-2); a top-level scalar beside Branch; absent → "origin"
	Epoch                   Epoch             // written once by init (installer-v1 §3.5 step 8); zero until then
	SystemVersion           string            // system.version — kept alongside System.Version so pre-v2 consumers keep reading it (NR-3)
	DocgateVersion          string            // release tag or "source"
	DocgateSHA256           map[string]string // platform → hex; empty iff source mode
	CodegateVersion         string            // release tag or "source" — the additive pin section (codegate-v1 §3.1), same shape as docgate's
	CodegateSHA256          map[string]string // platform → hex; empty iff source mode
	PRDs                    string
	Drafts                  string // working files for unlocked feature contracts
	Draft                   DraftSettings
	Prototype               *PrototypeSettings // absent means the asking session supplies the route
	Approval                ApprovalSettings   // hosted approvers; authorization reads the mainline file without local overlays
	Feedback                string             // review-feedback ledger root (review-feedback-capture §3.1)
	TechDesigns             string
	SolutionShapes          string // solution-shape corpus root — distinct from & never nested under TechDesigns (solution-shape-v1 §3.1, DINV-3)
	Brainstorms             string // brainstorm-record corpus root — stage-1 + stage-3 session records (config-driven-portability-v1 §3.2, FR-1)
	Problems                string // Argus problem-instance root — <slug>/problem.md + delivery.md ledgers (config-driven-portability-v1 §3.2, FR-1)
	Reviews                 string // unmanned-reviewer report corpus root (config-driven-portability-v1 §3.2, FR-1)
	Wiki                    string // descriptive wiki-entry corpus root (config-driven-portability-v1 §3.2, FR-1)
	RollupPRD               string
	RollupTechspec          string
	Incidents               string
	ProductVision           string            // the abstract governing-layer doc — a governance-skeleton target and a governance-policy member (config-driven-portability-v1 §3.2, FR-1/FR-3)
	ArchitecturePrinciples  string            // the citable principle-registry doc — a governance-skeleton target and a governance-policy member (config-driven-portability-v1 §3.2, FR-1/FR-3)
	DesignPrinciples        string            // the design-principles governing pointer — a governance-policy member (config-driven-portability-v1 §3.2, FR-1)
	Plans                   string            // plan documents root (plan-new-v1 §3.1)
	WorkItems               string            // work-item store root — bucket-1 operand content (local-work-tracker §3.8)
	RiskPolicy              string            // the merge-lane risk-policy file (merge-lane-v1 §3.6) — the surface carries only this PATH key, never the policy body (T1; #ALT-5)
	MaintenancePolicy       string            // the maintenance-sweep policy file (maintenance-sweep-v1 §3.4) — one additive PATH key; the closed-parse policy body (waivers + staleness bound) lives in the file, never the surface
	Plan                    PlanSizing        // plangate's split thresholds (plan-new-v1 §3.10)
	Channel                 ChannelBounds     // the wake channel's latency/staleness knobs (wake-channel-v1 §3.2) — closed-parse durations with their relation pinned at load
	TemplatesDir            string            // the instantiable-templates directory (FR-21's scan root)
	SystemMaps              []string          // living map documents whose registry anchors are fenced (FR-22)
	Templates               map[string]string // class → version pin
	CapabilityPaths         []string          // capability trigger set (check-standalone-docs, FR-30)
	Grandfathered           []string          // design slugs exempt from lint-tech-designs (pre-v2, R1)
	PrecedenceGrandfathered []string          // exempt from FR-16's precedence clause ONLY (I1)
	GrandfatheredCitations  []string          // `<file> | <token>` pairs: pre-existing dangling citations frozen into immutable artifacts, exempt from check-citations (citation-resolution A-6)

	deprecations []string // dual-read window notices; surfaced via Deprecations()
}

// System is the v2 system: block (installer-v1 §3.7; the solution shape's
// operand config). The pin surface is EXACTLY the pair Version +
// ManifestSHA256 (DINV-2) — any further pin key under system: refuses by
// name. Mode is admitted at parse only ("release" | "source"); the rule
// that source mode is legal only where the source tree lives (DINV-8) and
// pin-completeness refusals both belong to the resolver (T3), never here.
type System struct {
	Version        string // the one release pin (DINV-2)
	ManifestSHA256 string // the trust root: everything else verifies through it
	ArtifactSource string // optional FR-22 mirror/fork override; "" = the release host
	Mode           string // "release" | "source"; defaults to "source" — an absent config is the source-repo era, matching the per-tool "source" defaults
}

// Knobs is the v2 knobs: block — the five install decisions captured at
// init (prd:installer/FR-3). Zero values mean "never captured":
// recommended defaults belong to init's wizard (T7), never to this parser.
type Knobs struct {
	Tracker     string   // "github" | "carina"
	CIMode      string   // "local" | "actions" | "both"
	Wiring      Wiring   // GitHub wiring choices
	CLISurfaces []string // agent-CLI surfaces to wire — an open value set (adapters grow); only the flow-list structure is closed
	Argus       bool     // the delegated-authority surface, opt-in (prd:installer/FR-16)
}

// Wiring is the knobs.wiring flow map, exactly the shape's spelling:
// `wiring: { repo: use|create, protection: none|required-checks|checks+review }`
// — single line, closed keys, closed enums.
type Wiring struct {
	Repo       string // "use" | "create"
	Protection string // "none" | "required-checks" | "checks+review"
}

// DraftSettings names the branch namespace and the credential reference used
// to author draft pull requests. The token value never enters this config.
type DraftSettings struct {
	BranchPrefix  string
	AgentLogin    string
	AgentTokenEnv string
}

// PrototypeSettings names a harness and one of its implementation profiles.
// Profile resolution belongs to the prototype command, using the effective catalog.
type PrototypeSettings struct {
	Harness entity.Harness
	Profile string
}

// ApprovalSettings names authenticated GitHub accounts, never credentials.
// OperatorLogin has no default: Pyxis ships no one's identity, so an adopter
// configures it, and until then no approval can pass. An Argus account also needs
// approve-merge in the mainline authority contract.
type ApprovalSettings struct {
	OperatorLogin string
	ArgusLogin    string
}

// Epoch is the v2 epoch: block — written once by init (installer-v1 §3.5
// step 8) and optional until then; doctor reports it, and history-scoped
// consumers arrive with the adoption follow-up.
type Epoch struct {
	SystemVersion  string
	Commit         string
	ManifestSHA256 string
}

// PlanSizing is the closed plan: block — the split thresholds plangate's
// fold enforces structurally (plan-new-v1 §3.10; prd:plan-new/FR-5/FR-6).
// Values are data: retuning is a config PR, never a code change. Seeds are
// the predecessor's measured baselines recorded in the plan-new PRD.
type PlanSizing struct {
	MaxAcceptanceCriteria int
	MaxScopeBullets       int
	MaxEstLogicLines      int
	MaxTaskSharePct       int
}

// ChannelBounds carries the wake channel's two pinned durations
// (wake-channel-v1 §3.2). TickInterval realizes the configured latency
// bound — interval = the bound, the backstop's worst-case latency.
// StalenessBound is the age past which an unresumed await is a named
// wake-staleness defect (DINV-5); it MUST strictly exceed TickInterval so a
// staleness alarm can never fire on an await the very next tick would legally
// serve. The relation is enforced at load (a violation is a named defect).
type ChannelBounds struct {
	TickInterval   time.Duration
	StalenessBound time.Duration
}

// Deprecations returns the dual-read window's notices — one per legacy
// per-tool pin section present in a pyxis-config/v2 config (installer-v1
// §3.10). They are advisories for doctor's deprecation surface, never
// parse defects, while LegacyPinWindowOpen. A legacy v1 config yields
// none: the notice marks a MIXED v2 config that has not finished migrating
// its pins to the system pair.
func (c Config) Deprecations() []string { return c.deprecations }

// MainlineRef composes the remote-tracking ref for the repo's default branch
// — "<remote>/<branch>" (e.g. "origin/main") — from the two config scalars.
// It is the ONE place the composition lives (AP-11): every branch/remote
// consumer that needs the mainline ref (the git-hooks' runtime merge-base
// resolution, the merge lane, the plan-tracker's frozen read) derives it
// here rather than baking a literal, so the generated workflow and the
// runtime-resolving surfaces never carry a second, independently-maintained
// copy (config-driven-portability-v1 §3.1, DINV-1/DINV-4). On a default
// config it is byte-identical to the old "origin/main" literal (DINV-2).
func (c Config) MainlineRef() string { return c.Remote + "/" + c.Branch }

func validDraftBranchPrefix(prefix string) bool {
	if !strings.HasSuffix(prefix, "/") || strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "//") {
		return false
	}
	for part := range strings.SplitSeq(strings.TrimSuffix(prefix, "/"), "/") {
		if part == "" || part[0] < 'a' || part[0] > 'z' {
			return false
		}
		for _, r := range part[1:] {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func validEnvName(name string) bool {
	if name == "" || name[0] != '_' && (name[0] < 'A' || name[0] > 'Z') {
		return false
	}
	for _, r := range name[1:] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// noteDeprecation records one dual-read notice per legacy pin section in a
// v2 config, deduplicated (a section appearing twice yields one notice).
func (c *Config) noteDeprecation(section string) {
	msg := fmt.Sprintf("legacy per-tool pin section %q is deprecated under %s: the system pin pair (system.version + system.manifest_sha256) replaces per-tool pins; the section stops parsing when the dual-read window closes (installer-v1 §3.10)", section, SchemaVersionV2)
	if slices.Contains(c.deprecations, msg) {
		return
	}
	c.deprecations = append(c.deprecations, msg)
}

// Defaults are today's locations and pins — a repo with no config file
// behaves exactly as before the surface existed.
func Defaults() Config {
	return Config{
		Branch:                 "main",
		Remote:                 "origin",
		System:                 System{Version: "0.1.0", Mode: "source"}, // Version mirrors SystemVersion below; Mode "source" = the pre-installer posture (see System)
		SystemVersion:          "0.1.0",
		DocgateVersion:         "source",
		DocgateSHA256:          map[string]string{},
		CodegateVersion:        "source",
		CodegateSHA256:         map[string]string{},
		PRDs:                   "docs/prds",
		Drafts:                 "drafts",
		Draft:                  DraftSettings{BranchPrefix: "draft/"},
		Feedback:               "docs/feedback",
		TechDesigns:            "docs/tech-design",
		SolutionShapes:         "docs/solution-shapes",
		Brainstorms:            "docs/brainstorms",
		Problems:               "docs/problems",
		Reviews:                "docs/reviews",
		Wiki:                   "docs/wiki",
		RollupPRD:              "docs/prd.md",
		RollupTechspec:         "docs/techspec.md",
		Incidents:              "docs/observability/incidents.md",
		ProductVision:          "docs/product-vision.md",
		ArchitecturePrinciples: "docs/architecture-principles.md",
		DesignPrinciples:       "docs/design-principles.md",
		Plans:                  "docs/plans",
		WorkItems:              ".workitems",
		RiskPolicy:             "docs/policy/risk-policy.yaml",
		MaintenancePolicy:      "docs/policy/maintenance-sweep.yaml",
		Plan:                   PlanSizing{MaxAcceptanceCriteria: 4, MaxScopeBullets: 4, MaxEstLogicLines: 300, MaxTaskSharePct: 50},
		Channel:                ChannelBounds{TickInterval: time.Hour, StalenessBound: 24 * time.Hour},
		TemplatesDir:           ".pyxis/templates",
		SystemMaps:             []string{"docs/domain-model.md", "docs/system-design.md"},
		Templates: map[string]string{
			"prd":         schema.PRDTemplateV2,
			"amendment":   schema.AmendmentTemplateV2,
			"tech_design": "tech-design-template/v1",
			"plan":        "plan-template/v1",
		},
		CapabilityPaths:         []string{"cmd/", "internal/", ".pyxis/", ".github/workflows/"},
		Grandfathered:           []string{"prd-amendment-model", "tech-design-conventions"},
		PrecedenceGrandfathered: []string{"docgate-v2"},
	}
}

// pathSchema is the paths: block's closed key set — THE authoritative
// enumeration of every per-repo-configurable single path the engine reads
// (installer-v1 §3.7: the schema is the authoritative list, closed by
// construction). Each key maps to its Config field; the compiled defaults
// live in Defaults(). Under schema v2 the older keys enforce the
// refuse-non-default posture (FR-18/UB-8); drafts is configurable. The path-closure conformance
// test (TestPathClosureConformance) is the audit surface the design names:
// it asserts this set matches the audited engine path-read set exactly, so
// no configurable-path read can hide outside the schema and no schema key
// can silently ignore the posture. List-valued path surfaces (system_maps,
// capability_paths, the grandfather lists) are their own top-level
// sections, deliberately outside the paths: block.
var pathSchema = map[string]func(*Config) *string{
	"prds":                    func(c *Config) *string { return &c.PRDs },
	"drafts":                  func(c *Config) *string { return &c.Drafts },
	"feedback":                func(c *Config) *string { return &c.Feedback },
	"tech_designs":            func(c *Config) *string { return &c.TechDesigns },
	"solution_shapes":         func(c *Config) *string { return &c.SolutionShapes },
	"brainstorms":             func(c *Config) *string { return &c.Brainstorms },
	"problems":                func(c *Config) *string { return &c.Problems },
	"reviews":                 func(c *Config) *string { return &c.Reviews },
	"wiki":                    func(c *Config) *string { return &c.Wiki },
	"rollup_prd":              func(c *Config) *string { return &c.RollupPRD },
	"rollup_techspec":         func(c *Config) *string { return &c.RollupTechspec },
	"incidents":               func(c *Config) *string { return &c.Incidents },
	"product_vision":          func(c *Config) *string { return &c.ProductVision },
	"architecture_principles": func(c *Config) *string { return &c.ArchitecturePrinciples },
	"design_principles":       func(c *Config) *string { return &c.DesignPrinciples },
	"plans":                   func(c *Config) *string { return &c.Plans },
	"work_items":              func(c *Config) *string { return &c.WorkItems },
	"risk_policy":             func(c *Config) *string { return &c.RiskPolicy },
	"maintenance_policy":      func(c *Config) *string { return &c.MaintenancePolicy },
	"templates_dir":           func(c *Config) *string { return &c.TemplatesDir },
}

// PathDefaults returns the closed paths: schema — key → compiled default —
// the enumeration surface the path-closure conformance test audits
// (installer-v1 §3.7). Derived from the one pathSchema table so the parser
// and the audit can never disagree.
func PathDefaults() map[string]string {
	d := Defaults()
	out := make(map[string]string, len(pathSchema))
	for k, f := range pathSchema {
		out[k] = *f(&d)
	}
	return out
}

const relPath = configlayer.GlobalPath

func def(reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "config", File: relPath, Op: "parse", Reason: reason, Remedy: remedy}
}

// Load merges root/.pyxis/config.local.yaml over root/.pyxis/config.yaml;
// absent layers yield Defaults with no defects.
func Load(root string) (Config, []schema.Defect) {
	b, err := configlayer.Load(root)
	if err != nil {
		return Defaults(), []schema.Defect{def(fmt.Sprintf("unreadable: %v", err), "fix permissions or remove the global or local file to use defaults")}
	}
	return Parse(b)
}

// foreignSections are top-level blocks owned by OTHER consumers of the shared
// pyxis-config surface (docs/tech-design/repo-layout-and-system-footprint.md
// §3.6) — e.g. the executor engine's `executor:` block and the code-gate's
// `code-gate:` block (docs/tech-design/code-gate-mirror.md §3.1). docgate
// recognizes them so multiple consumers coexist on one file, but does NOT
// validate their subtrees: each consumer's own closed parser owns its block
// (the executor and code-gate parsers likewise skip docgate's blocks). This
// keeps docgate's schema closed for its OWN keys — a typo in `paths:` is still
// caught — while a sibling consumer's block is neither rejected nor policed.
//
// `wake:` is the wake-watch-v2 status verb's staleness-threshold block
// (design:wake-watch-v2#3.8; plan self-drive-wake/001 T9) — the
// `wake.stale.{review,ci,worker,checker}` keys, owned and closed-parsed by
// internal/watch (LoadThresholds), never by this package. It is distinct
// from the existing `channel:` block above, which belongs to the argus
// backstop tick (wake-channel-v1) and stays docgate's own.
var foreignSections = map[string]bool{
	"executor":       true,
	"code-gate":      true,
	"wake":           true,
	"harness-config": true,
	"integrations":   true,
}

// Parse validates the closed grammar over normalized bytes.
func Parse(b []byte) (Config, []schema.Defect) {
	c := Defaults()
	defs := Defaults() // pristine copy: the compiled defaults v2 path keys compare against
	var defects []schema.Defect
	bad := func(n int, reason, remedy string) {
		defects = append(defects, def(fmt.Sprintf("line %d: %s", n, reason), remedy))
	}
	lines := strings.Split(string(schema.Normalize(b)), "\n")

	// Pre-scan for the schema generation: the v2 grammar is engaged by a
	// top-level schema_version scalar wherever it appears (YAML maps are
	// unordered — a single pass would mis-grade keys seen before the
	// marker). Any declared value other than SchemaVersionV2 is a named
	// defect; the file still parses under the v2 grammar so one bad marker
	// does not cascade into per-key noise.
	isV2 := false
	for i, raw := range lines {
		line := strings.TrimRight(raw, " ")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || len(line) != len(t) {
			continue // blank, comment, or indented — not a top-level scalar
		}
		if k, v, ok := strings.Cut(t, ":"); ok && strings.TrimSpace(k) == "schema_version" {
			isV2 = true
			if u := unquote(v); u != SchemaVersionV2 {
				bad(i+1, fmt.Sprintf("unsupported schema_version %q", u),
					fmt.Sprintf("this parser speaks exactly %q; a legacy v1 config omits the key entirely (installer-v1 §3.7)", SchemaVersionV2))
			}
		}
	}
	// v2Key gates a v2-only key found in a legacy config — refuse by name.
	v2Key := func(n int, key string) bool {
		if isV2 {
			return true
		}
		bad(n, fmt.Sprintf("%s is a pyxis-config/v2 key in a legacy config", key),
			fmt.Sprintf("declare `schema_version: %s` to use the v2 schema (installer-v1 §3.7)", SchemaVersionV2))
		return false
	}

	section, sub := "", ""
	skip := false // a refused or gated section's subtree is skipped wholesale
	seenSHA := false
	seenCodegateSHA := false
	prototypeKeys := map[string]bool{}
	for n, raw := range lines {
		line := strings.TrimRight(raw, " ")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimSpace(line)
		// A recognized foreign consumer's block is skipped wholesale: its subtree
		// belongs to another parser on the shared surface, so docgate does not
		// validate it. The block's own top-level key line (indent 0) still runs the
		// switch below, where it is recognized rather than flagged unknown. A
		// section refused by name (a closed-window legacy pin section, a v2-only
		// section in a legacy config) is likewise skipped: one named defect
		// instead of per-line noise.
		if indent > 0 && (foreignSections[section] || skip) {
			continue
		}
		if indent > 0 && section == "prototype" {
			k, v, ok := strings.Cut(t, ":")
			k, v = strings.TrimSpace(k), unquote(v)
			if indent != 2 || !ok || strings.HasPrefix(t, "-") {
				bad(n+1, "unrecognized structure in prototype", "prototype is a two-key block: harness and profile")
				continue
			}
			if k != "harness" && k != "profile" {
				bad(n+1, fmt.Sprintf("unknown key %q in prototype", k), "prototype accepts only harness and profile")
				continue
			}
			if prototypeKeys[k] {
				bad(n+1, "duplicate prototype."+k, "declare each prototype key once")
				continue
			}
			prototypeKeys[k] = true
			if v == "" || strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
				bad(n+1, "prototype."+k+" requires a non-empty scalar", "name a harness and an implementation profile")
				continue
			}
			if k == "profile" {
				c.Prototype.Profile = v
			} else if h, err := entity.ParseHarness(v); err != nil {
				bad(n+1, fmt.Sprintf("unknown prototype.harness %q", v), "name a supported harness: claude, codex, cursor, or opencode")
			} else {
				c.Prototype.Harness = h
			}
			continue
		}
		switch {
		case indent == 0 && strings.HasSuffix(t, ":"):
			section, sub, skip = strings.TrimSuffix(t, ":"), "", false
			switch section {
			case "prototype":
				if c.Prototype != nil {
					bad(n+1, "duplicate prototype block", "declare prototype once")
					skip = true
				} else {
					c.Prototype = &PrototypeSettings{}
				}
			case "system", "paths", "draft", "approval", "templates", "capability_paths", "grandfathered_designs", "grandfathered_citations", "precedence_grandfathered", "system_maps", "plan", "channel":
			case "docgate", "codegate":
				// The dual-read migration window (installer-v1 §3.10): while
				// open, the legacy per-tool pin sections parse in both schema
				// generations — under v2 with a deprecation notice for doctor.
				// Once the window closes they refuse by name (design A-4).
				if !LegacyPinWindowOpen {
					bad(n+1, fmt.Sprintf("legacy per-tool pin section %q refused: the dual-read window is closed", section),
						"pin the system release instead — system.version + system.manifest_sha256 under pyxis-config/v2 (installer-v1 §3.10, DINV-2)")
					skip = true
				} else if isV2 {
					c.noteDeprecation(section)
				}
			case "knobs", "epoch":
				if !isV2 {
					bad(n+1, fmt.Sprintf("%q is a pyxis-config/v2 section in a legacy config", section),
						fmt.Sprintf("declare `schema_version: %s` to use the v2 schema (installer-v1 §3.7)", SchemaVersionV2))
					skip = true
				}
			default:
				if !foreignSections[section] {
					bad(n+1, fmt.Sprintf("unknown top-level key %q", section), "the schema is closed: see the layout design §3.6")
				}
			}
		case indent == 0 && strings.Contains(t, ":"):
			// Top-level scalars — the v2 additions (schema_version, branch,
			// remote); any other top-level scalar keeps today's refusal.
			section, sub, skip = "", "", false
			k, v, _ := strings.Cut(t, ":")
			k, v = strings.TrimSpace(k), unquote(v)
			switch k {
			case "prototype":
				bad(n+1, "prototype must be a block", "declare prototype with both harness and profile")
			case "schema_version":
				c.SchemaVersion = v // value validity is the pre-scan's defect
			case "branch":
				if v2Key(n+1, "branch") {
					c.Branch = v // the default branch every branch-referencing consumer resolves (FR-2)
				}
			case "remote":
				if v2Key(n+1, "remote") {
					c.Remote = v // the git remote the same consumers resolve refs against (FR-2)
				}
			default:
				bad(n+1, "unrecognized structure", "flat two-level maps and one list only (closed subset)")
			}
		case indent == 2 && strings.HasPrefix(t, "- "):
			if section == "grandfathered_designs" {
				if sub == "" {
					c.Grandfathered = nil
					sub = "list"
				}
				c.Grandfathered = append(c.Grandfathered, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
				continue
			}
			if section == "precedence_grandfathered" {
				if sub == "" {
					c.PrecedenceGrandfathered = nil
					sub = "list"
				}
				c.PrecedenceGrandfathered = append(c.PrecedenceGrandfathered, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
				continue
			}
			if section == "grandfathered_citations" {
				if sub == "" {
					c.GrandfatheredCitations = nil
					sub = "list"
				}
				c.GrandfatheredCitations = append(c.GrandfatheredCitations, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
				continue
			}
			if section == "system_maps" {
				if sub == "" {
					c.SystemMaps = nil
					sub = "list"
				}
				c.SystemMaps = append(c.SystemMaps, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
				continue
			}
			if section != "capability_paths" {
				bad(n+1, "list item outside capability_paths", "only capability_paths is a list")
				continue
			}
			if sub == "" {
				c.CapabilityPaths = nil
				sub = "list"
			}
			c.CapabilityPaths = append(c.CapabilityPaths, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
		case indent == 2 && strings.Contains(t, ":"):
			k, v, _ := strings.Cut(t, ":")
			k, v = strings.TrimSpace(k), unquote(v)
			// The paths: block parses off the one closed pathSchema table.
			// Under v2 the older keys are CAPTURED defaults: a non-default value
			// is the named not-yet-configurable refusal (FR-18/UB-8,
			// installer-v1 §3.7) until the adoption follow-up honors
			// overrides; the compiled default is accepted unchanged. Drafts is
			// configurable from its introduction. Legacy
			// v1 configs keep reading paths into use exactly as before (NR-3).
			if section == "paths" {
				setter, ok := pathSchema[k]
				if !ok {
					bad(n+1, fmt.Sprintf("unknown key %q in %q", k, section), "the schema is closed: see the layout design §3.6")
					continue
				}
				if isV2 && k != "drafts" && v != *setter(&defs) {
					bad(n+1, fmt.Sprintf("paths.%s %q is not yet configurable (captured default %q)", k, v, *setter(&defs)),
						"captured path keys hold their compiled defaults until the adoption follow-up honors overrides (FR-18/UB-8, installer-v1 §3.7)")
					continue
				}
				*setter(&c) = v
				continue
			}
			switch section + "." + k {
			case "draft.branch_prefix":
				c.Draft.BranchPrefix = v
			case "draft.agent_login":
				c.Draft.AgentLogin = v
			case "draft.agent_token_env":
				c.Draft.AgentTokenEnv = v
			case "approval.operator_login":
				if v == "" {
					bad(n+1, "approval.operator_login is empty", "name the GitHub account that approves, or omit the key")
					continue
				}
				c.Approval.OperatorLogin = v
			case "approval.argus_login":
				c.Approval.ArgusLogin = v
			case "system.version":
				c.SystemVersion = v
				c.System.Version = v // one key, both surfaces: pre-v2 consumers keep reading SystemVersion (NR-3)
			case "system.manifest_sha256":
				if v2Key(n+1, "system.manifest_sha256") {
					c.System.ManifestSHA256 = v
				}
			case "system.artifact_source":
				if v2Key(n+1, "system.artifact_source") {
					c.System.ArtifactSource = v
				}
			case "system.mode":
				// Parse-level admission only: source-only-where-source-lives
				// (DINV-8) and pin completeness are the resolver's refusals (T3).
				if v2Key(n+1, "system.mode") {
					setEnum(&c.System.Mode, v, n+1, "system.mode", "release|source", bad)
				}
			case "knobs.tracker":
				setEnum(&c.Knobs.Tracker, v, n+1, "knobs.tracker", "github|carina|forgejo", bad)
			case "knobs.ci_mode":
				setEnum(&c.Knobs.CIMode, v, n+1, "knobs.ci_mode", "local|actions|both", bad)
			case "knobs.wiring":
				setWiring(&c.Knobs.Wiring, v, n+1, bad)
			case "knobs.cli_surfaces":
				setFlowList(&c.Knobs.CLISurfaces, v, n+1, "knobs.cli_surfaces", bad)
			case "knobs.argus":
				setBool(&c.Knobs.Argus, v, n+1, "knobs.argus", bad)
			case "epoch.system_version":
				c.Epoch.SystemVersion = v
			case "epoch.commit":
				c.Epoch.Commit = v
			case "epoch.manifest_sha256":
				c.Epoch.ManifestSHA256 = v
			case "docgate.version":
				c.DocgateVersion = v
			case "docgate.sha256":
				seenSHA = true
				sub = "sha256"
			case "codegate.version":
				c.CodegateVersion = v
			case "codegate.sha256":
				seenCodegateSHA = true
				sub = "sha256"
			case "plan.max_acceptance_criteria":
				setInt(&c.Plan.MaxAcceptanceCriteria, v, n+1, bad)
			case "plan.max_scope_bullets":
				setInt(&c.Plan.MaxScopeBullets, v, n+1, bad)
			case "plan.max_est_logic_lines":
				setInt(&c.Plan.MaxEstLogicLines, v, n+1, bad)
			case "plan.max_task_share_pct":
				setInt(&c.Plan.MaxTaskSharePct, v, n+1, bad)
			case "channel.tick_interval":
				setDuration(&c.Channel.TickInterval, v, n+1, bad)
			case "channel.staleness_bound":
				setDuration(&c.Channel.StalenessBound, v, n+1, bad)
			case "templates.prd", "templates.amendment", "templates.tech_design", "templates.plan":
				c.Templates[k] = v
			default:
				// Under v2, a pin-shaped stray inside system: is the DINV-2
				// refusal by name — the pin surface is exactly the pair.
				if isV2 && section == "system" && (strings.Contains(k, "sha256") || strings.Contains(k, "version")) {
					bad(n+1, fmt.Sprintf("pin key %q beyond the system pair in %q", k, section),
						"the v2 pin surface is exactly system.version + system.manifest_sha256 (installer-v1 DINV-2)")
					continue
				}
				bad(n+1, fmt.Sprintf("unknown key %q in %q", k, section), "the schema is closed: see the layout design §3.6")
			}
		case indent == 4 && sub == "sha256" && strings.Contains(t, ":"):
			k, v, _ := strings.Cut(t, ":")
			switch section {
			case "docgate":
				c.DocgateSHA256[strings.TrimSpace(k)] = unquote(v)
			case "codegate":
				c.CodegateSHA256[strings.TrimSpace(k)] = unquote(v)
			}
		default:
			bad(n+1, "unrecognized structure", "flat two-level maps and one list only (closed subset)")
		}
	}
	if c.Prototype != nil && (!prototypeKeys["harness"] || !prototypeKeys["profile"]) {
		defects = append(defects, def("prototype requires both harness and profile", "declare both keys or omit the prototype block"))
	}
	if c.DocgateVersion == "source" && len(c.DocgateSHA256) > 0 {
		defects = append(defects, def(`docgate.version "source" with sha256 pins`, "source mode and release pins are mutually exclusive (DINV-2)"))
	}
	if c.DocgateVersion != "source" && (!seenSHA || len(c.DocgateSHA256) == 0) {
		defects = append(defects, def(fmt.Sprintf("docgate release %q with no sha256 pins", c.DocgateVersion), "a release pin requires per-platform checksums (DINV-2)"))
	}
	if c.CodegateVersion == "source" && len(c.CodegateSHA256) > 0 {
		defects = append(defects, def(`codegate.version "source" with sha256 pins`, "source mode and release pins are mutually exclusive (DINV-2)"))
	}
	if c.CodegateVersion != "source" && (!seenCodegateSHA || len(c.CodegateSHA256) == 0) {
		defects = append(defects, def(fmt.Sprintf("codegate release %q with no sha256 pins", c.CodegateVersion), "a release pin requires per-platform checksums (DINV-2)"))
	}
	// The solution-shape corpus must resolve OUTSIDE the technical-design
	// corpus — never equal, never nested either way — so a shape is never
	// admitted to the design index nor reached by the immutability gate
	// (solution-shape-v1 §3.1, DINV-3).
	if nestedOrEqual(c.SolutionShapes, c.TechDesigns) || nestedOrEqual(c.TechDesigns, c.SolutionShapes) {
		defects = append(defects, def(
			fmt.Sprintf("solution-shapes root %q is not outside the technical-design corpus %q", c.SolutionShapes, c.TechDesigns),
			"root the shape corpus at a directory distinct from and never nested under docs/tech-design (DINV-3)"))
	}
	// The wake channel's relation is pinned here: the staleness bound must
	// STRICTLY exceed the tick interval, so a staleness alarm can never fire on
	// an await the very next tick would legally serve (wake-channel-v1 §3.2).
	if c.Channel.StalenessBound <= c.Channel.TickInterval {
		defects = append(defects, def(
			fmt.Sprintf("channel staleness_bound %s does not strictly exceed tick_interval %s", c.Channel.StalenessBound, c.Channel.TickInterval),
			"raise staleness_bound above tick_interval so a staleness alarm never fires on an await the next tick would serve (wake-channel-v1 §3.2)"))
	}
	if !validDraftBranchPrefix(c.Draft.BranchPrefix) {
		defects = append(defects, def(fmt.Sprintf("draft.branch_prefix %q is not a safe git branch prefix", c.Draft.BranchPrefix), "use a lowercase slash-terminated branch prefix such as draft/"))
	}
	if c.Draft.AgentTokenEnv != "" && !validEnvName(c.Draft.AgentTokenEnv) {
		defects = append(defects, def(fmt.Sprintf("draft.agent_token_env %q is not an environment variable name", c.Draft.AgentTokenEnv), "name an environment variable holding the agent's code-host token"))
	}
	for name, login := range map[string]string{"operator_login": c.Approval.OperatorLogin, "argus_login": c.Approval.ArgusLogin} {
		// Neither is required here: a repository with no operator login still
		// loads, requests no reviewer, and fails closed when an approval is read.
		if login == "" {
			continue
		}
		if len(login) > 100 || strings.Trim(login, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-[]") != "" {
			defects = append(defects, def("approval."+name+" must name a GitHub account", "use a login without whitespace, paths, or an @ prefix"))
		}
	}
	if c.Approval.ArgusLogin != "" && strings.EqualFold(c.Approval.OperatorLogin, c.Approval.ArgusLogin) {
		defects = append(defects, def("approval.operator_login and approval.argus_login must be distinct", "keep operator and delegated driver identities separate"))
	}
	// Cross-field lexical containment (config-driven-portability-v1 §3.3,
	// #DINV-3, #A-4). Every path-typed config value must be a repository-
	// relative path the engine can join UNDER the repository root; a value
	// that is absolute, escapes the root (`..`), or is otherwise not repo-
	// relative is refused HERE, by name, before any consumer resolves it —
	// so a config value can never aim a gate outside the tree
	// (prd:config-driven-portability/FR-5 · UB-1 · UB-2 · ABUSE-1). The check
	// is deliberately lexical and hermetic: no os.Stat, no EvalSymlinks — a
	// symlink pointing out of the repo is the same principal's own content,
	// not a config escape (§3.3, same-principal threat model). Realizing
	// UB-2's "reference to an absent surface" THROUGH containment: a value
	// that cannot live in the repo-relative space IS an out-of-tree/absent
	// reference. This pass runs over the FINAL parsed values (any key left at
	// its compiled default validates too; the defaults are all clean repo-
	// relative paths, so a default config is byte-identical — DINV-2, A-3).
	// A refusal here is a named defect on the returned slice: load has refused
	// the configuration by name before any consumer resolves a path, so there
	// is no partial resolution at the load boundary (a defect-honoring caller
	// stops; DINV-3 binds load, not each downstream reader). NR-2's unknown-key
	// refusal is untouched — that fires earlier, per line, and this pass only
	// ADDS containment defects.
	//
	// The single-path keys are THE authoritative pathSchema enumeration; the
	// path-typed list surfaces are validated per entry, named by section +
	// value. system_maps (living map documents) and capability_paths (trigger
	// tree paths) hold tree paths and ARE validated. The grandfather lists are
	// NOT tree paths and so are excluded: grandfathered_designs and
	// precedence_grandfathered hold design SLUGS (e.g. "docgate-v2"), and
	// grandfathered_citations holds `<file> | <token>` citation pairs (a
	// compound token, not a path) — none is a location the engine joins under
	// the root, so containment does not apply to them.
	//
	// The single-path keys are walked in SORTED key order: pathSchema is a map,
	// Go randomizes map iteration, and defects render in slice order — so an
	// unsorted walk would report a multi-key violation in a different sequence
	// on every run. The list surfaces keep their configured order, which is
	// already deterministic.
	pathKeys := make([]string, 0, len(pathSchema))
	for k := range pathSchema {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)
	for _, k := range pathKeys {
		v := *pathSchema[k](&c)
		if reason, ok := contained(v); !ok {
			defects = append(defects, def(
				fmt.Sprintf("paths.%s %q %s", k, v, reason),
				fmt.Sprintf("configure paths.%s as a repository-relative path under the repo root — never absolute, never escaping with `..` (config-driven-portability-v1 §3.3, DINV-3)", k)))
		}
	}
	for _, m := range c.SystemMaps {
		if reason, ok := contained(m); !ok {
			defects = append(defects, def(
				fmt.Sprintf("system_maps entry %q %s", m, reason),
				"list every system map as a repository-relative path under the repo root — never absolute, never escaping with `..` (config-driven-portability-v1 §3.3, DINV-3)"))
		}
	}
	for _, p := range c.CapabilityPaths {
		if reason, ok := contained(p); !ok {
			defects = append(defects, def(
				fmt.Sprintf("capability_paths entry %q %s", p, reason),
				"list every capability path as a repository-relative path under the repo root — never absolute, never escaping with `..` (config-driven-portability-v1 §3.3, DINV-3)"))
		}
	}
	return c, defects
}

// contained is the lexical containment predicate (config-driven-portability-v1
// §3.3, #DINV-3): ok reports whether value is a repository-relative path the
// engine may join under the repository root, and reason names why it is not
// when it is refused. The check is the standard idiom and deliberately
// hermetic — no filesystem access, no symlink resolution — so it is
// deterministic on bytes alone: a value is refused when it is absolute, when
// its cleaned form is a bare parent reference (`..`) or begins with a parent
// escape (`../…`), or when it is empty (naming no repository-relative
// location). It is distinct from nestedOrEqual (which asks "is this nested
// under a given parent?"); this asks "is this a clean, non-escaping,
// repository-relative path at all?".
func contained(value string) (reason string, ok bool) {
	if value == "" {
		return "is empty, not a repository-relative path", false
	}
	if filepath.IsAbs(value) {
		return "is an absolute path (escapes the repository root)", false
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "escapes the repository root with a parent-directory reference", false
	}
	return "", true
}

// nestedOrEqual reports whether child is the same directory as parent or
// lives beneath it, over cleaned paths.
func nestedOrEqual(child, parent string) bool {
	child, parent = filepath.Clean(child), filepath.Clean(parent)
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}

// setInt parses a positive-integer scalar; anything else is a named defect.
func setInt(dst *int, v string, line int, bad func(int, string, string)) {
	n := 0
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 || fmt.Sprintf("%d", n) != v {
		bad(line, fmt.Sprintf("plan threshold %q is not a positive integer", v), "plan: values are positive integers (plan-new-v1 §3.10)")
		return
	}
	*dst = n
}

// setDuration parses a positive Go duration scalar (e.g. "1h", "72h",
// "30m"); a malformed or non-positive value is a named defect and the prior
// (default) value stands (wake-channel-v1 §3.2 — the knobs are closed-parse).
func setDuration(dst *time.Duration, v string, line int, bad func(int, string, string)) {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		bad(line, fmt.Sprintf("channel bound %q is not a positive duration", v), "channel: values are positive Go durations like \"1h\" or \"72h\" (wake-channel-v1 §3.2)")
		return
	}
	*dst = d
}

// setEnum admits exactly the |-separated closed value set; anything else is
// a named defect and the prior (default) value stands.
func setEnum(dst *string, v string, line int, key, allowed string, bad func(int, string, string)) {
	if slices.Contains(strings.Split(allowed, "|"), v) {
		*dst = v
		return
	}
	bad(line, fmt.Sprintf("%s %q is not one of %s", key, v, allowed), "the v2 schema's enums are closed (installer-v1 §3.7)")
}

// setBool parses a bare true/false scalar; anything else is a named defect.
func setBool(dst *bool, v string, line int, key string, bad func(int, string, string)) {
	switch v {
	case "true":
		*dst = true
	case "false":
		*dst = false
	default:
		bad(line, fmt.Sprintf("%s %q is not a boolean", key, v), "write true or false (installer-v1 §3.7)")
	}
}

// setWiring parses the knobs.wiring flow map exactly as the solution shape
// spells it — `{ repo: use|create, protection: none|required-checks|checks+review }`
// — single line, closed keys, closed enums; any other structure is a named
// defect (block style is deliberately outside the closed grammar).
func setWiring(dst *Wiring, v string, line int, bad func(int, string, string)) {
	if !strings.HasPrefix(v, "{") || !strings.HasSuffix(v, "}") {
		bad(line, fmt.Sprintf("knobs.wiring %q is not a `{ repo: …, protection: … }` flow map", v),
			"write wiring: { repo: use, protection: required-checks } (the installer-v1 shape's spelling)")
		return
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return
	}
	for part := range strings.SplitSeq(inner, ",") {
		k, val, ok := strings.Cut(part, ":")
		if !ok {
			bad(line, fmt.Sprintf("knobs.wiring entry %q is not `key: value`", strings.TrimSpace(part)),
				"write wiring: { repo: use, protection: required-checks } (the installer-v1 shape's spelling)")
			continue
		}
		k, val = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(val), `"`)
		switch k {
		case "repo":
			setEnum(&dst.Repo, val, line, "knobs.wiring.repo", "use|create", bad)
		case "protection":
			setEnum(&dst.Protection, val, line, "knobs.wiring.protection", "none|required-checks|checks+review", bad)
		default:
			bad(line, fmt.Sprintf("unknown key %q in \"knobs.wiring\"", k), "the schema is closed: repo and protection only (installer-v1 §3.7)")
		}
	}
}

// setFlowList parses a single-line `[a, b]` flow list (the shape's
// cli_surfaces spelling). The value set is open — agent-CLI adapters grow —
// so items are free strings; only the structure is closed.
func setFlowList(dst *[]string, v string, line int, key string, bad func(int, string, string)) {
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		bad(line, fmt.Sprintf("%s %q is not a `[a, b]` flow list", key, v),
			fmt.Sprintf("write %s: [claude, codex] (the installer-v1 shape's spelling)", key))
		return
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		*dst = nil
		return
	}
	var items []string
	for part := range strings.SplitSeq(inner, ",") {
		it := strings.Trim(strings.TrimSpace(part), `"`)
		if it == "" {
			bad(line, fmt.Sprintf("%s carries an empty item", key), "list entries are non-empty surface names")
			continue
		}
		items = append(items, it)
	}
	*dst = items
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"`)
}

// SetEpochCommit records the install-epoch commit into the repo config surface
// (existing-repo-onboarding-v2 §3.4, DINV-4; plan existing-repo-onboarding/001
// T8): adoption's `apply` writes the repository's current commit as the epoch
// the tree-local gates scope their scan-set against. It is a targeted edit —
// an existing `epoch.commit` is replaced in place and the epoch block is added
// when absent, touching nothing else in the closed-schema config — and the
// result is re-parsed closed before it lands, so a write that would corrupt
// the surface is refused rather than persisted (a defence against a bug here,
// mirroring the closed-schema refusal INV-32 gives the read path). The commit
// value is quoted exactly as renderConfig emits the epoch block.
func SetEpochCommit(root, commit string) error {
	path := filepath.Join(root, relPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read %s: %w", relPath, err)
	}
	updated := withEpochCommit(b, commit)
	if _, defects := Parse(updated); len(defects) > 0 {
		return fmt.Errorf("config: writing epoch.commit would break the closed schema (%s: %s) — refusing", relPath, defects[0].Reason)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", relPath, err)
	}
	return nil
}

// ClearEpochCommit removes the epoch's `commit:` line from the repo config
// surface — adoption's `undo` reversing the epoch `apply` recorded (existing-
// repo-onboarding-v2 §3.5): with the commit gone the tree-local gates stop
// grandfathering (scanScope sees no epoch), while the operator's other config
// stays untouched. An absent config, an absent epoch block, or a
// commit-less block is a no-op. The result is re-parsed closed before it lands.
func ClearEpochCommit(root string) error {
	path := filepath.Join(root, relPath)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no config surface → nothing to clear
		}
		return fmt.Errorf("config: read %s: %w", relPath, err)
	}
	updated := withoutEpochCommit(b)
	if _, defects := Parse(updated); len(defects) > 0 {
		return fmt.Errorf("config: clearing epoch.commit would break the closed schema (%s: %s) — refusing", relPath, defects[0].Reason)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", relPath, err)
	}
	return nil
}

// withoutEpochCommit returns b with the epoch block's `commit:` line removed,
// preserving every other byte; b unchanged when there is no epoch block or no
// commit line within it (the block ends at the first non-indented, non-blank
// line).
func withoutEpochCommit(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	epochIdx := -1
	for i, l := range lines {
		if strings.TrimRight(l, " \t") == "epoch:" {
			epochIdx = i
			break
		}
	}
	if epochIdx < 0 {
		return b
	}
	for i := epochIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			break // left the epoch block
		}
		if strings.HasPrefix(strings.TrimSpace(l), "commit:") {
			out := append(lines[:i:i], lines[i+1:]...)
			return []byte(strings.Join(out, "\n"))
		}
	}
	return b
}

// withEpochCommit returns b with the epoch block's `commit:` set to commit,
// preserving every other byte: it replaces an existing commit line inside the
// epoch block, inserts one right after a blockless `epoch:`, or appends a fresh
// two-line epoch block when the config has none. The value is quoted with %q to
// match renderConfig's spelling (a bare commit hash needs no quoting, but the
// closed parser accepts the quoted form and it keeps one canonical shape).
func withEpochCommit(b []byte, commit string) []byte {
	commitLine := fmt.Sprintf("  commit: %q", commit)
	lines := strings.Split(string(b), "\n")
	epochIdx := -1
	for i, l := range lines {
		if strings.TrimRight(l, " \t") == "epoch:" {
			epochIdx = i
			break
		}
	}
	if epochIdx < 0 {
		// No epoch block — append one, after exactly one trailing newline.
		return []byte(strings.TrimRight(string(b), "\n") + "\nepoch:\n" + commitLine + "\n")
	}
	// Within the epoch block — its indented (or blank) lines following `epoch:`
	// — replace an existing commit line; a non-indented, non-blank line ends it.
	for i := epochIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			break // left the epoch block without finding a commit line
		}
		if strings.HasPrefix(strings.TrimSpace(l), "commit:") {
			lines[i] = commitLine
			return []byte(strings.Join(lines, "\n"))
		}
	}
	// The epoch block has no commit line — insert one right after `epoch:`.
	out := append([]string{}, lines[:epochIdx+1]...)
	out = append(out, commitLine)
	out = append(out, lines[epochIdx+1:]...)
	return []byte(strings.Join(out, "\n"))
}
