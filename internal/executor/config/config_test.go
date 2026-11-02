package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// T12 acceptance #1: a repo config selects the default backend, and a per-repo
// table routes a matching task to its backend; every miss falls to the default.
func TestResolveHarness_DefaultAndTable(t *testing.T) {
	t.Parallel()
	const src = `executor:
  routing:
    default: codex
    table:
      docs: cursor
      infra: opencode
`
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if got := cfg.DefaultHarness(); got != "codex" {
		t.Fatalf("DefaultHarness() = %q, want %q", got, "codex")
	}

	cases := []struct {
		name string
		task port.Task
		want entity.Harness
	}{
		{"table hit routes to mapped backend", port.Task{ID: "T1", Harness: "docs"}, "cursor"},
		{"second table entry", port.Task{ID: "T2", Harness: "infra"}, "opencode"},
		{"unmatched category falls to default", port.Task{ID: "T3", Harness: "api"}, "codex"},
		{"empty routing slot falls to default", port.Task{ID: "T4"}, "codex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := cfg.ResolveHarness(tc.task, "claude")
			if err != nil {
				t.Fatalf("ResolveHarness(%+v): unexpected error: %v", tc.task, err)
			}
			if got != tc.want {
				t.Fatalf("ResolveHarness(%+v) = %q, want %q", tc.task, got, tc.want)
			}
		})
	}
}

// Defaults inherit the invocation host; they never manufacture a vendor.
func TestResolveHarness_Defaults(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	if got := cfg.DefaultHarness(); got != "" {
		t.Fatalf("Defaults DefaultHarness() = %q, want empty (ambient inheritance)", got)
	}
	got, err := cfg.ResolveHarness(port.Task{ID: "T1", Harness: "anything"}, "codex")
	if err != nil {
		t.Fatalf("ResolveHarness: unexpected error: %v", err)
	}
	if got != "codex" {
		t.Fatalf("ResolveHarness = %q, want ambient codex", got)
	}
}

func TestRunLogDirectory(t *testing.T) {
	t.Parallel()
	if got := config.Defaults().RunLogDirectory(); got != ".pyxis/logs/implmentor" {
		t.Fatalf("default RunLogDirectory = %q", got)
	}
	cfg, err := config.Parse([]byte("executor:\n  logging:\n    directory: var/pyxis-runs\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.RunLogDirectory(); got != "var/pyxis-runs" {
		t.Fatalf("configured RunLogDirectory = %q", got)
	}
	if _, err := config.Parse([]byte("executor:\n  logging:\n    directory:\n")); err == nil {
		t.Fatal("empty logging.directory was accepted")
	}
}

func TestResolveHarness_MissingOrUnknownAmbientFailsClosed(t *testing.T) {
	t.Parallel()
	for _, ambient := range []string{"", "gpt9000"} {
		if _, err := config.Defaults().ResolveHarness(port.Task{ID: "T1"}, ambient); !errors.Is(err, config.ErrNoDefaultHarness) {
			t.Errorf("ambient %q: error = %v, want ErrNoDefaultHarness", ambient, err)
		}
	}
}

// A zero-value Config (never built via Load/Parse/Defaults) fails closed rather
// than routing to "".
func TestResolveHarness_ZeroValueFailsClosed(t *testing.T) {
	t.Parallel()
	var cfg config.Config
	if _, err := cfg.ResolveHarness(port.Task{ID: "T1"}, ""); !errors.Is(err, config.ErrNoDefaultHarness) {
		t.Fatalf("zero-value ResolveHarness error = %v, want ErrNoDefaultHarness", err)
	}
}

// T12 acceptance #2: an unknown config key — and every closed-schema value
// violation — is a parse error carrying the right sentinel.
func TestParse_ClosedSchemaViolations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want error
	}{
		{
			name: "unknown key under executor",
			src:  "executor:\n  bogus: x\n",
			want: config.ErrUnknownKey,
		},
		{
			name: "unknown key under routing",
			src:  "executor:\n  routing:\n    bogus: claude\n",
			want: config.ErrUnknownKey,
		},
		{
			name: "unknown key under caps",
			src:  "executor:\n  caps:\n    bogus: 3\n",
			want: config.ErrUnknownKey,
		},
		{
			name: "unknown default backend",
			src:  "executor:\n  routing:\n    default: gpt\n",
			want: config.ErrInvalidHarness,
		},
		{
			name: "unknown table backend",
			src:  "executor:\n  routing:\n    table:\n      docs: gpt\n",
			want: config.ErrInvalidHarness,
		},
		{
			name: "empty table backend value",
			src:  "executor:\n  routing:\n    table:\n      docs:\n",
			want: config.ErrInvalidHarness,
		},
		{
			name: "non-positive cap",
			src:  "executor:\n  caps:\n    worker_turns: 0\n",
			want: config.ErrInvalidCap,
		},
		{
			name: "non-numeric cap",
			src:  "executor:\n  caps:\n    parallel_width: lots\n",
			want: config.ErrInvalidCap,
		},
		{
			name: "negative cap (leading sign rejected)",
			src:  "executor:\n  caps:\n    ci_self_heal_attempts: -1\n",
			want: config.ErrInvalidCap,
		},
		{
			name: "empty gate command",
			src:  "executor:\n  gate:\n    build:\n",
			want: config.ErrInvalidGate,
		},
		{
			name: "duplicate gate check name",
			src:  "executor:\n  gate:\n    build: go build ./...\n    build: go vet ./...\n",
			want: config.ErrInvalidGate,
		},
		{
			name: "tab in indentation",
			src:  "executor:\n\trouting:\n",
			want: config.ErrMalformed,
		},
		{
			name: "duplicate executor block",
			src:  "executor:\n  routing:\n    default: codex\nexecutor:\n  caps:\n    worker_turns: 5\n",
			want: config.ErrMalformed,
		},
		{
			name: "duplicate routing table key",
			src:  "executor:\n  routing:\n    table:\n      docs: codex\n      docs: cursor\n",
			want: config.ErrMalformed,
		},
		{
			name: "executor as scalar",
			src:  "executor: claude\n",
			want: config.ErrMalformed,
		},
		{
			name: "table entry without a table block",
			src:  "executor:\n  routing:\n      docs: codex\n",
			want: config.ErrMalformed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Parse([]byte(tc.src))
			if !errors.Is(err, tc.want) {
				t.Fatalf("Parse error = %v, want errors.Is(_, %v)", err, tc.want)
			}
		})
	}
}

// T12 acceptance #3: the caps read from config are surfaced with sensible
// defaults, overridden per key, with unset caps keeping their default (partial
// override).
func TestCaps_DefaultsAndOverride(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                                string
		src                                                 string
		wantTurns, wantSelfHeal, wantWidth, wantReviewWidth int
	}{
		{
			// 0 turns = no turn cap; the Worker's wall-clock budget is the bound.
			name:            "defaults with no executor block",
			src:             "",
			wantTurns:       0,
			wantSelfHeal:    3,
			wantWidth:       4,
			wantReviewWidth: 2,
		},
		{
			name:            "defaults with empty executor block",
			src:             "executor:\n  caps:\n",
			wantTurns:       0,
			wantSelfHeal:    3,
			wantWidth:       4,
			wantReviewWidth: 2,
		},
		{
			name:            "full override of all four caps",
			src:             "executor:\n  caps:\n    worker_turns: 25\n    ci_self_heal_attempts: 5\n    parallel_width: 8\n    review_width: 2\n",
			wantTurns:       25,
			wantSelfHeal:    5,
			wantWidth:       8,
			wantReviewWidth: 2,
		},
		{
			name:            "partial override keeps unset caps at default",
			src:             "executor:\n  caps:\n    worker_turns: 10\n",
			wantTurns:       10,
			wantSelfHeal:    3,
			wantWidth:       4,
			wantReviewWidth: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Parse([]byte(tc.src))
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}
			if got := cfg.WorkerTurns(); got != tc.wantTurns {
				t.Errorf("WorkerTurns() = %d, want %d", got, tc.wantTurns)
			}
			if got := cfg.CISelfHealAttempts(); got != tc.wantSelfHeal {
				t.Errorf("CISelfHealAttempts() = %d, want %d", got, tc.wantSelfHeal)
			}
			if got := cfg.ParallelWidth(); got != tc.wantWidth {
				t.Errorf("ParallelWidth() = %d, want %d", got, tc.wantWidth)
			}
			if got := cfg.ReviewWidth(); got != tc.wantReviewWidth {
				t.Errorf("ReviewWidth() = %d, want %d", got, tc.wantReviewWidth)
			}
		})
	}
}

// The gate block parses into ordered CI-mirroring checks: each entry's name is
// the check it mirrors and its command is whitespace-split into argv, in
// declaration order. An absent gate yields no checks (a vacuous gate — CI is
// authoritative, DINV-3).
func TestGate_DefaultsAndParsing(t *testing.T) {
	t.Parallel()

	// No gate block: no checks.
	base, err := config.Parse([]byte("executor:\n  caps:\n    worker_turns: 10\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := base.GateChecks(); len(got) != 0 {
		t.Fatalf("GateChecks() = %v, want none for a repo with no gate block", got)
	}

	// A gate block with three checks, order preserved, commands split to argv.
	src := "executor:\n  gate:\n    build: go build ./...\n    vet: go vet ./...\n    test: go test -race ./...\n"
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := cfg.GateChecks()
	want := []config.GateCheck{
		{Name: "build", Argv: []string{"go", "build", "./..."}},
		{Name: "vet", Argv: []string{"go", "vet", "./..."}},
		{Name: "test", Argv: []string{"go", "test", "-race", "./..."}},
	}
	if len(got) != len(want) {
		t.Fatalf("GateChecks() len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || !slices.Equal(got[i].Argv, want[i].Argv) {
			t.Errorf("GateChecks()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// The returned slice is a copy — mutating it does not affect the Config.
	got[0].Argv[0] = "MUTATED"
	if again := cfg.GateChecks(); again[0].Argv[0] != "go" {
		t.Errorf("GateChecks() returned a shared slice; mutation leaked: %q", again[0].Argv[0])
	}
}

// The executor block coexists with other consumers' blocks on the shared
// pyxis-config surface: sibling top-level blocks (here docgate-style paths,
// lists, and nested maps) are skipped, not rejected, while the executor block
// still parses closed.
func TestParse_CoexistsWithForeignBlocks(t *testing.T) {
	t.Parallel()
	const src = `system:
  version: 0.1.0
paths:
  prds: docs/prds
  incidents: docs/observability/incidents.md
capability_paths:
  - cmd/
  - internal/
docgate:
  version: source
  sha256:
    darwin_arm64: deadbeef
executor:
  routing:
    default: cursor
    table:
      infra: opencode
  caps:
    parallel_width: 6
plan:
  max_scope_bullets: 4
`
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if got := cfg.DefaultHarness(); got != "cursor" {
		t.Fatalf("DefaultHarness() = %q, want %q", got, "cursor")
	}
	got, err := cfg.ResolveHarness(port.Task{ID: "T1", Harness: "infra"}, "claude")
	if err != nil {
		t.Fatalf("ResolveHarness: unexpected error: %v", err)
	}
	if got != "opencode" {
		t.Fatalf("ResolveHarness = %q, want %q", got, "opencode")
	}
	if got := cfg.ParallelWidth(); got != 6 {
		t.Fatalf("ParallelWidth() = %d, want %d", got, 6)
	}
	// Unset caps still default despite the surrounding foreign blocks (0 turns =
	// no turn cap).
	if got := cfg.WorkerTurns(); got != 0 {
		t.Fatalf("WorkerTurns() = %d, want %d", got, 0)
	}
}

// A closed-schema violation inside the executor block is still caught when the
// block sits amid foreign blocks.
func TestParse_ClosedWithinExecutorAmidForeign(t *testing.T) {
	t.Parallel()
	const src = `paths:
  prds: docs/prds
executor:
  caps:
    bogus: 3
`
	if _, err := config.Parse([]byte(src)); !errors.Is(err, config.ErrUnknownKey) {
		t.Fatalf("Parse error = %v, want ErrUnknownKey", err)
	}
}

// Load reads .pyxis/config.yaml under the operand root. An absent file yields
// Defaults with no error; a present file parses the executor block.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("absent file yields defaults", func(t *testing.T) {
		t.Parallel()
		cfg, err := config.Load(t.TempDir())
		if err != nil {
			t.Fatalf("Load: unexpected error: %v", err)
		}
		if got := cfg.DefaultHarness(); got != "" {
			t.Fatalf("DefaultHarness() = %q, want ambient inheritance", got)
		}
		if got := cfg.ParallelWidth(); got != 4 {
			t.Fatalf("ParallelWidth() = %d, want %d", got, 4)
		}
	})

	t.Run("present file parses the executor block", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		const src = "executor:\n  routing:\n    default: codex\n  caps:\n    worker_turns: 30\n"
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(src), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		cfg, err := config.Load(root)
		if err != nil {
			t.Fatalf("Load: unexpected error: %v", err)
		}
		if got := cfg.DefaultHarness(); got != "codex" {
			t.Fatalf("DefaultHarness() = %q, want %q", got, "codex")
		}
		if got := cfg.WorkerTurns(); got != 30 {
			t.Fatalf("WorkerTurns() = %d, want %d", got, 30)
		}
	})

	t.Run("malformed file surfaces the parse error", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("executor:\n  routing:\n    default: gpt\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := config.Load(root); !errors.Is(err, config.ErrInvalidHarness) {
			t.Fatalf("Load error = %v, want ErrInvalidHarness", err)
		}
	})
}

// All four known backends are accepted as the default and as table targets.
func TestParse_AllKnownHarnesses(t *testing.T) {
	t.Parallel()
	for _, b := range []string{"claude", "codex", "cursor", "opencode"} {
		t.Run(b, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Parse([]byte("executor:\n  routing:\n    default: " + b + "\n"))
			if err != nil {
				t.Fatalf("Parse(default=%s): unexpected error: %v", b, err)
			}
			if got := cfg.DefaultHarness(); got != entity.Harness(b) {
				t.Fatalf("DefaultHarness() = %q, want %q", got, b)
			}
		})
	}
}

// Inline comments and blank lines are ignored; quoted scalars are unquoted.
func TestParse_CommentsBlanksAndQuotes(t *testing.T) {
	t.Parallel()
	const src = `# executor policy
executor:

  routing:
    default: "codex"   # the repo default
  caps:
    worker_turns: 42 # bumped for this repo
`
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if got := cfg.DefaultHarness(); got != "codex" {
		t.Fatalf("DefaultHarness() = %q, want %q", got, "codex")
	}
	if got := cfg.WorkerTurns(); got != 42 {
		t.Fatalf("WorkerTurns() = %d, want %d", got, 42)
	}
}

// TestReviewSection pins the impl-review seam's config half (#556): the review
// section parses its two keys, defaults its budget, stays closed, and its
// absence leaves the review seam uninstalled.
func TestReviewSection(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  routing:\n    default: claude\n  review:\n    backend: claude\n    budget_minutes: 25\n    evidence_ref: refs/pyxis/custom-review\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewHarness() != "claude" {
		t.Errorf("ReviewHarness = %q", cfg.ReviewHarness())
	}
	if cfg.ReviewBudget() != 25*time.Minute {
		t.Errorf("ReviewBudget = %v", cfg.ReviewBudget())
	}
	if cfg.ReviewEvidenceRef() != "refs/pyxis/custom-review" {
		t.Errorf("ReviewEvidenceRef = %q", cfg.ReviewEvidenceRef())
	}

	cfg, err = config.Parse([]byte("executor:\n  routing:\n    default: claude\n  review:\n    backend: codex\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewBudget() != 15*time.Minute {
		t.Errorf("default ReviewBudget = %v, want 15m", cfg.ReviewBudget())
	}

	if _, err := config.Parse([]byte("executor:\n  routing:\n    default: claude\n  review:\n    backend: gpt9000\n")); !errors.Is(err, config.ErrInvalidHarness) {
		t.Errorf("invalid review backend: got %v", err)
	}
	if _, err := config.Parse([]byte("executor:\n  routing:\n    default: claude\n  review:\n    frobnicate: yes\n")); !errors.Is(err, config.ErrUnknownKey) {
		t.Errorf("unknown review key must refuse (closed parse): got %v", err)
	}
	if _, err := config.Parse([]byte("executor:\n  review:\n    evidence_ref: refs/heads/main\n")); !errors.Is(err, config.ErrMalformed) {
		t.Errorf("unsafe review evidence ref: got %v", err)
	}

	cfg, err = config.Parse([]byte("executor:\n  routing:\n    default: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewHarness() != "" {
		t.Errorf("absent review section must leave the seam uninstalled, got %q", cfg.ReviewHarness())
	}
	if cfg.ReviewConfigured() {
		t.Fatal("absent review section reported configured")
	}

	cfg, err = config.Parse([]byte("executor:\n  routing:\n    default: claude\n  review:\n    budget_minutes: 9\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReviewConfigured() || cfg.ReviewHarness() != "" {
		t.Fatalf("inherited review block = configured %v backend %q", cfg.ReviewConfigured(), cfg.ReviewHarness())
	}
}

func TestRoutingEvidenceActivationIsAbsentOrComplete(t *testing.T) {
	checkpoint := strings.Repeat("a", 64)
	cfg, err := config.Parse([]byte("executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/routing-evidence/producer-policy.json\n    activation_checkpoint: " + checkpoint + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := cfg.RoutingEvidenceActivation()
	if !ok || got.EvidenceRef != "refs/pyxis/routing-evidence" || got.ProducerPolicyRef != ".pyxis/routing-evidence/producer-policy.json" || got.CheckpointEvent != checkpoint {
		t.Fatalf("RoutingEvidenceActivation = %+v, %v", got, ok)
	}

	absent, err := config.Parse([]byte("executor:\n  routing:\n    default: codex\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := absent.RoutingEvidenceActivation(); ok {
		t.Fatal("absent routing_evidence block activated production routing")
	}

	for name, src := range map[string]string{
		"partial":           "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n",
		"unsafe policy":     "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: ../policy.json\n    activation_checkpoint: " + checkpoint + "\n",
		"non-json policy":   "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/policy.yaml\n    activation_checkpoint: " + checkpoint + "\n",
		"uppercase event":   "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/policy.json\n    activation_checkpoint: " + strings.Repeat("A", 64) + "\n",
		"unknown key":       "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/policy.json\n    activation_checkpoint: " + checkpoint + "\n    enabled: true\n",
		"duplicate section": "executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/policy.json\n    activation_checkpoint: " + checkpoint + "\n  routing_evidence:\n    evidence_ref: refs/pyxis/other\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Parse([]byte(src))
			if name == "unknown key" {
				if !errors.Is(err, config.ErrUnknownKey) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if name == "duplicate section" {
				if !errors.Is(err, config.ErrMalformed) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if !errors.Is(err, config.ErrInvalidRoutingEvidence) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestRoutingEvidenceActivationCannotOmitConfiguredReview(t *testing.T) {
	_, err := config.Parse([]byte("executor:\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/routing/producer-policy.json\n    activation_checkpoint: " + strings.Repeat("a", 64) + "\n"))
	if !errors.Is(err, config.ErrInvalidRoutingEvidence) {
		t.Fatalf("Parse = %v, want ErrInvalidRoutingEvidence", err)
	}
}

func TestNativeRoutingCompleteReplacementReference(t *testing.T) {
	const src = `executor:
  routing:
    default: codex
  native_routing:
    schema_version: native-routing-catalog-override/v1
    resolver_compatibility: native-routing-resolver/v1
    harness: codex
    role: implementation
    catalog_ref: .pyxis/native-routing/codex-implementation.json
    qualifications_ref: .pyxis/native-routing/codex-implementation-qualifications.json
    authorization:
      kind: repository_operator
      repository: agenticlab-ai/pyxis
      operator_id: operator-1
      evidence_digest: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      authorized_at: 2026-08-03T09:00:00Z
  caps:
    worker_turns: 23
  review:
    backend: claude
    budget_minutes: 19
  gate:
    test: go test ./...
`
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	override := cfg.NativeRoutingOverride()
	if override == nil || override.Harness != "codex" || override.Role != "implementation" ||
		override.CatalogReference != ".pyxis/native-routing/codex-implementation.json" ||
		override.QualificationReference != ".pyxis/native-routing/codex-implementation-qualifications.json" ||
		override.Authorization.Repository != "agenticlab-ai/pyxis" || override.Authorization.OperatorID != "operator-1" {
		t.Fatalf("NativeRoutingOverride = %#v", override)
	}
	if cfg.DefaultHarness() != "codex" || cfg.WorkerTurns() != 23 || cfg.ReviewHarness() != "claude" || cfg.ReviewBudget() != 19*time.Minute || len(cfg.GateChecks()) != 1 {
		t.Fatalf("native policy changed unrelated executor settings: backend=%q turns=%d review=%q/%v gate=%v", cfg.DefaultHarness(), cfg.WorkerTurns(), cfg.ReviewHarness(), cfg.ReviewBudget(), cfg.GateChecks())
	}
	copy := cfg.NativeRoutingOverride()
	copy.Harness = "cursor"
	if cfg.NativeRoutingOverride().Harness != "codex" {
		t.Fatal("NativeRoutingOverride returned shared state")
	}
	if got, err := config.Parse([]byte("executor:\n  routing:\n    default: codex\n")); err != nil || got.NativeRoutingOverride() != nil {
		t.Fatalf("absent native_routing must preserve prior config: override=%#v err=%v", got.NativeRoutingOverride(), err)
	}
}

func TestNativeRoutingReplacementParsesClosedAndComplete(t *testing.T) {
	const valid = `executor:
  native_routing:
    schema_version: native-routing-catalog-override/v1
    resolver_compatibility: native-routing-resolver/v1
    harness: codex
    role: implementation
    catalog_ref: .pyxis/native-routing/catalog.json
    qualifications_ref: .pyxis/native-routing/qualifications.json
    authorization:
      kind: repository_operator
      repository: agenticlab-ai/pyxis
      operator_id: operator-1
      evidence_digest: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      authorized_at: 2026-08-03T09:00:00Z
`
	tests := []struct {
		name string
		src  string
		want error
	}{
		{"partial block", "executor:\n  native_routing:\n    schema_version: native-routing-catalog-override/v1\n", config.ErrInvalidNativeRouting},
		{"unknown key", strings.Replace(valid, "    role: implementation\n", "    role: implementation\n    surprise: true\n", 1), config.ErrUnknownKey},
		{"raw argument key", strings.Replace(valid, "    role: implementation\n", "    role: implementation\n    raw_arguments: --model hacked\n", 1), config.ErrUnknownKey},
		{"raw argument reference", strings.Replace(valid, ".pyxis/native-routing/catalog.json", "--model=hacked", 1), config.ErrInvalidNativeRouting},
		{"embedded raw argument reference", strings.Replace(valid, ".pyxis/native-routing/catalog.json", ".pyxis/native-routing/catalog.json --model=hacked", 1), config.ErrInvalidNativeRouting},
		{"path escape", strings.Replace(valid, ".pyxis/native-routing/catalog.json", "../../catalog.json", 1), config.ErrInvalidNativeRouting},
		{"incompatible schema", strings.Replace(valid, "native-routing-catalog-override/v1", "native-routing-catalog-override/v2", 1), config.ErrInvalidNativeRouting},
		{"incompatible resolver", strings.Replace(valid, "native-routing-resolver/v1", "native-routing-resolver/v2", 1), config.ErrInvalidNativeRouting},
		{"contributor authorization", strings.Replace(valid, "kind: repository_operator", "kind: contributor", 1), config.ErrInvalidNativeRouting},
		{"worker authorization", strings.Replace(valid, "kind: repository_operator", "kind: worker", 1), config.ErrInvalidNativeRouting},
		{"unknown authorization key", strings.Replace(valid, "      repository: agenticlab-ai/pyxis\n", "      repository: agenticlab-ai/pyxis\n      worker_claim: yes\n", 1), config.ErrUnknownKey},
		{"malformed authorization time", strings.Replace(valid, "2026-08-03T09:00:00Z", "today", 1), config.ErrInvalidNativeRouting},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := config.Parse([]byte(test.src))
			if !errors.Is(err, test.want) {
				t.Fatalf("Parse error = %v, want errors.Is(_, %v)", err, test.want)
			}
		})
	}
}

func TestLoadAppliesMachineLocalOverrides(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	global := "executor:\n  routing:\n    default: claude\n  caps:\n    worker_turns: 23\n    parallel_width: 4\n"
	local := "executor:\n  routing:\n    default: codex\n  caps:\n    parallel_width: 2\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultHarness() != "codex" || cfg.WorkerTurns() != 23 || cfg.ParallelWidth() != 2 {
		t.Fatalf("effective config = backend %q, turns %d, width %d", cfg.DefaultHarness(), cfg.WorkerTurns(), cfg.ParallelWidth())
	}
}
