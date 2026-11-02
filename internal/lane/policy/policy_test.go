package policy

import (
	"strings"
	"testing"
)

// validPolicy is a well-formed, self-unlock-covering risk policy: its floor
// carries the whole design-bound minimum, its two AUTO classes stay clear of
// the build inputs, and every block is declared. The fixtures below mutate a
// copy of it to exercise each refusal in isolation.
const validPolicy = `schema_version: risk-policy/v1
ratified_by: "@mdarabi"
floor:
  hard_gated_paths:
    - ".github/workflows/**"
    - ".githooks/**"
    - "docs/product-vision.md"
    - "docs/architecture-principles.md"
    - "docs/observability/incidents.md"
    - "docs/policy/**"
    - "AGENTS.md"
    - "CLAUDE.md"
    - ".pyxis/**"
    - ".claude/**"
    - ".codex/**"
    - "cmd/lane/**"
    - "internal/lane/**"
    - "go.mod"
    - "go.sum"
  hard_gated_classes:
    - governing-doc-edit
    - ledger-frozen-entry-edit
auto_classes:
  - name: fleet-task
    paths: ["cmd/**", "internal/**"]
    requires_reviewer_verdict: false
  - name: drive-doc
    paths: ["docs/**"]
    requires_reviewer_verdict: true
thresholds:
  shadow_window_min_proposals: 20
  agreement_bar: 0.95
activation:
  enforce: false
  backstop_change_ref: ""
kill_switch: false
`

// parseValidate runs the pure parse+validate path a reader's Load composes,
// minus the git lineage read.
func parseValidate(src string) []string {
	p, ds := Parse([]byte(src))
	ds = append(ds, Validate(p)...)
	reasons := make([]string, len(ds))
	for i, d := range ds {
		reasons[i] = d.Reason
	}
	return reasons
}

func TestValidPolicyParses(t *testing.T) {
	p, ds := Parse([]byte(validPolicy))
	if len(ds) != 0 {
		t.Fatalf("well-formed policy flagged: %v", ds)
	}
	if vd := Validate(p); len(vd) != 0 {
		t.Fatalf("well-formed policy failed validation: %v", vd)
	}
	// A reader sees the floor, the AUTO class set, activation, and the kill.
	if p.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q", p.SchemaVersion)
	}
	if len(p.Floor.HardGatedPaths) != len(SelfUnlockMinPaths) {
		t.Fatalf("floor paths = %d, want %d", len(p.Floor.HardGatedPaths), len(SelfUnlockMinPaths))
	}
	if len(p.AutoClasses) != 2 || p.AutoClasses[0].Name != "fleet-task" || p.AutoClasses[1].Name != "drive-doc" {
		t.Fatalf("auto classes = %+v", p.AutoClasses)
	}
	if !p.AutoClasses[1].RequiresReviewerVerdict {
		t.Fatalf("drive-doc should require a reviewer verdict")
	}
	if p.Thresholds.ShadowWindowMinProposals != 20 || p.Thresholds.AgreementBar != 0.95 {
		t.Fatalf("thresholds = %+v", p.Thresholds)
	}
	if p.Activation.Enforce || p.Activation.BackstopChangeRef != "" {
		t.Fatalf("activation = %+v", p.Activation)
	}
	if p.HumanOnly() {
		t.Fatalf("kill switch off should not be HUMAN-only")
	}
}

// TestKillSwitchOneDataChange — flipping the single field flips the lane to
// HUMAN-only, and nothing else in the schema changes (A-4).
func TestKillSwitchOneDataChange(t *testing.T) {
	killed := strings.Replace(validPolicy, "kill_switch: false", "kill_switch: true", 1)
	p, ds := Parse([]byte(killed))
	if len(ds) != 0 {
		t.Fatalf("killed policy flagged: %v", ds)
	}
	if !p.HumanOnly() {
		t.Fatalf("kill_switch: true must flip the lane HUMAN-only")
	}
}

func TestParseRefusals(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "unknown top-level key",
			src:  validPolicy + "surprise: 1\n",
			want: `unknown top-level key "surprise"`,
		},
		{
			name: "floor scalar",
			src:  strings.Replace(validPolicy, "floor:\n", "floor: scalar\n", 1),
			want: "floor is a block",
		},
		{
			name: "invalid auto classes scalar",
			src:  strings.Replace(validPolicy, "auto_classes:\n", "auto_classes: scalar\n", 1),
			want: "auto_classes is a block or the empty list",
		},
		{
			name: "auto class item without dash",
			src:  strings.Replace(validPolicy, "  - name: fleet-task", "  name: fleet-task", 1),
			want: "auto_classes items are",
		},
		{
			name: "unknown thresholds key",
			src:  strings.Replace(validPolicy, "  agreement_bar: 0.95", "  agreement_bar: 0.95\n  surprise: 1", 1),
			want: `unknown thresholds key "surprise"`,
		},
		{
			name: "unknown activation key",
			src:  strings.Replace(validPolicy, "  backstop_change_ref: \"\"", "  backstop_change_ref: \"\"\n  surprise: yes", 1),
			want: `unknown activation key "surprise"`,
		},
		{
			name: "unexpected indented line",
			src:  "  dangling: value\n" + validPolicy,
			want: "unexpected indented line",
		},
		{
			name: "unknown floor key",
			src:  strings.Replace(validPolicy, "  hard_gated_classes:", "  soft_gated_paths:\n    - x\n  hard_gated_classes:", 1),
			want: `unknown floor key "soft_gated_paths"`,
		},
		{
			name: "unknown auto_classes field",
			src:  strings.Replace(validPolicy, "    requires_reviewer_verdict: false", "    requires_reviewer_verdict: false\n    color: red", 1),
			want: `unknown auto_classes field "color"`,
		},
		{
			name: "wrong schema version",
			src:  strings.Replace(validPolicy, "risk-policy/v1", "risk-policy/v2", 1),
			want: `unknown schema version "risk-policy/v2"`,
		},
		{
			name: "non-boolean kill switch",
			src:  strings.Replace(validPolicy, "kill_switch: false", "kill_switch: maybe", 1),
			want: `"maybe" is not a boolean`,
		},
		{
			name: "non-integer window",
			src:  strings.Replace(validPolicy, "shadow_window_min_proposals: 20", "shadow_window_min_proposals: lots", 1),
			want: `"lots" is not a positive integer`,
		},
		{
			name: "agreement bar out of range",
			src:  strings.Replace(validPolicy, "agreement_bar: 0.95", "agreement_bar: 1.5", 1),
			want: `"1.5" is not an agreement bar`,
		},
		{
			name: "missing required block",
			src:  strings.Replace(validPolicy, "kill_switch: false\n", "", 1),
			want: `required key "kill_switch" is absent`,
		},
		{
			name: "self-unlock uncovering: dropped governing path",
			src:  strings.Replace(validPolicy, "    - \"docs/product-vision.md\"\n", "", 1),
			want: `floor omits self-unlock-minimum path "docs/product-vision.md"`,
		},
		{
			name: "self-unlock uncovering: dropped build input",
			src:  strings.Replace(validPolicy, "    - \"go.sum\"\n", "", 1),
			want: `floor omits self-unlock-minimum path "go.sum"`,
		},
		{
			name: "self-unlock uncovering: dropped floor class",
			src:  strings.Replace(validPolicy, "    - ledger-frozen-entry-edit\n", "", 1),
			want: `floor omits self-unlock-minimum class "ledger-frozen-entry-edit"`,
		},
		{
			name: "AUTO class claims go.mod exactly",
			src:  strings.Replace(validPolicy, `paths: ["cmd/**", "internal/**"]`, `paths: ["cmd/**", "go.mod"]`, 1),
			want: `AUTO class "fleet-task" claims the lane build input "go.mod"`,
		},
		{
			name: "AUTO class claims build inputs via wildcard",
			src:  strings.Replace(validPolicy, `paths: ["cmd/**", "internal/**"]`, `paths: ["**"]`, 1),
			want: `claims the lane build input`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reasons := parseValidate(tc.src)
			if !anyContains(reasons, tc.want) {
				t.Fatalf("want a defect containing %q, got %v", tc.want, reasons)
			}
		})
	}
}

func anyContains(reasons []string, want string) bool {
	for _, r := range reasons {
		if strings.Contains(r, want) {
			return true
		}
	}
	return false
}
