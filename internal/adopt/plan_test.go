package adopt_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
)

// updateGolden regenerates the committed golden fixtures instead of
// comparing against them (`go test ./internal/adopt/... -update`) — the
// standard Go golden-file idiom also used by internal/relmanifest and
// internal/docgate/contractgate.
var updateGolden = flag.Bool("update", false, "regenerate the committed golden fixtures")

// validDraftPlan builds a schema-valid draft plan for mutation in defect
// tests: the fields `apply` has not yet populated all sit at their zero
// value.
func validDraftPlan() adopt.Plan {
	return adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusDraft,
		Entries: []adopt.Entry{
			{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
			{Path: "AGENTS.md", Class: adopt.ClassCollideManual, Detail: "add the pyxis section"},
			{Path: "src/**", Class: adopt.ClassNeverTouch},
		},
	}
}

// validAppliedPlan builds a schema-valid applied plan exercising every
// field the applied lifecycle populates: every Class, a multi-line
// ClassCollideManual detail, a displaced hook path, a detected stack with
// its gate actions, and an added-path baseline for the `add` entry — DINV-7
// (as reconciled 2026-07-26) stamps a baseline only for `add`, since a `chain`
// is git-config-only (reversed via hooks.prior_hooks_path, no wrapper file to
// hash) and its entry is disclosure only.
func validAppliedPlan() adopt.Plan {
	h := strings.Repeat("ab", 32)
	return adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusApplied,
		Epoch:         strings.Repeat("cd", 20),
		Entries: []adopt.Entry{
			{Path: ".pyxis/config.yaml", Class: adopt.ClassAdd},
			{Path: "core.hooksPath", Class: adopt.ClassChain, Detail: "the adopter's core.hooksPath is .husky; the engine chains its checks ahead of it and restores .husky on undo"},
			{Path: "AGENTS.md", Class: adopt.ClassCollideManual, Detail: "- Pyxis: run pyxis converge before every commit.\n- Pyxis: see docs/tech-design/ for design records."},
			{Path: "src/**", Class: adopt.ClassNeverTouch},
			{Path: "docs/tech-design/index.md", Class: adopt.ClassGenerated},
			{Path: ".pyxis/scripts/pyxis-bootstrap.sh", Class: adopt.ClassEngineVerbatim},
		},
		Hooks: adopt.Hooks{PriorHooksPath: ".husky"},
		Stack: adopt.Stack{
			Detected:    []string{"go"},
			GateActions: []string{"executor.gate.build", "executor.gate.test", "executor.gate.lint"},
		},
		AddedPaths: []adopt.AddedPath{
			{Path: ".pyxis/config.yaml", SHA256: h},
		},
	}
}

// TestRoundTrip proves Encode -> Parse reproduces the original value for
// both lifecycle stages (T2 acceptance #1).
func TestRoundTrip(t *testing.T) {
	for name, p := range map[string]adopt.Plan{"draft": validDraftPlan(), "applied": validAppliedPlan()} {
		t.Run(name, func(t *testing.T) {
			b, err := p.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := adopt.Parse(b)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(p, got) {
				t.Fatalf("round trip diverged:\nwant %+v\ngot  %+v", p, got)
			}
		})
	}
}

// TestGoldenPlanRoundTrip pins the applied fixture's exact machine-face
// bytes against a committed golden (mirrors internal/relmanifest's
// TestGoldenManifestRoundTrip): the golden parses, re-encodes to the
// identical bytes, and the re-encoded form parses back to the identical
// value. Regenerate with -update after an intended schema change.
func TestGoldenPlanRoundTrip(t *testing.T) {
	goldenPath := filepath.Join("testdata", "golden-plan.json")

	if *updateGolden {
		b, err := validAppliedPlan().Encode()
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if err := os.WriteFile(goldenPath, b, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("regenerated %s", goldenPath)
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden missing (run with -update to create): %v", err)
	}
	p, err := adopt.Parse(golden)
	if err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	out, err := p.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Equal(out, golden) {
		t.Fatalf("round trip not byte-stable:\n--- golden ---\n%s\n--- re-encoded ---\n%s", golden, out)
	}
	p2, err := adopt.Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Fatalf("value round trip diverged:\n%+v\n%+v", p, p2)
	}
}

// TestParseUnknownKeysRefusedByName proves the closed parse at the top
// level and at every nested shape — an unknown key anywhere is a named
// defect, never silently ignored (T2 acceptance #1).
func TestParseUnknownKeysRefusedByName(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			"top level",
			`{"schema_version":"adoption-plan/v1","status":"draft","epoch":"","entries":[],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[]},"added_paths":[],"unexpected":true}`,
			`unknown field "unexpected"`,
		},
		{
			"entry",
			`{"schema_version":"adoption-plan/v1","status":"draft","epoch":"","entries":[{"path":"a","class":"add","detail":"","priority":1}],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[]},"added_paths":[]}`,
			`unknown field "priority"`,
		},
		{
			"hooks",
			`{"schema_version":"adoption-plan/v1","status":"draft","epoch":"","entries":[],"hooks":{"prior_hooks_path":"","extra":"x"},"stack":{"detected":[],"gate_actions":[]},"added_paths":[]}`,
			`unknown field "extra"`,
		},
		{
			"stack",
			`{"schema_version":"adoption-plan/v1","status":"draft","epoch":"","entries":[],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[],"dominant":"go"},"added_paths":[]}`,
			`unknown field "dominant"`,
		},
		{
			"added_paths",
			`{"schema_version":"adoption-plan/v1","status":"applied","epoch":"e","entries":[],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[]},"added_paths":[{"path":"a","sha256":"` + strings.Repeat("0", 64) + `","mtime":1}]}`,
			`unknown field "mtime"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := adopt.Parse([]byte(c.json))
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

// TestParseValidatesAfterDecode proves Parse's own validate() call is
// wired, not just Encode's: well-typed JSON that decodes cleanly but
// violates a schema invariant is still refused by name.
func TestParseValidatesAfterDecode(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			"bad schema_version",
			`{"schema_version":"adoption-plan/v2","status":"draft","epoch":"","entries":[],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[]},"added_paths":[]}`,
			`schema_version "adoption-plan/v2"`,
		},
		{
			"bad status",
			`{"schema_version":"adoption-plan/v1","status":"pending","epoch":"","entries":[],"hooks":{"prior_hooks_path":""},"stack":{"detected":[],"gate_actions":[]},"added_paths":[]}`,
			`status "pending"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := adopt.Parse([]byte(c.json))
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

// TestParseTrailingContent proves one object, closed — content after the
// plan object is a named defect, not silently dropped.
func TestParseTrailingContent(t *testing.T) {
	b, err := validDraftPlan().Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = adopt.Parse(append(b, []byte("{}")...))
	if err == nil || !strings.Contains(err.Error(), "trailing content") {
		t.Fatalf("trailing content not refused: %v", err)
	}
}

// TestPlanInvariantsRefusedByName table-drives every schema invariant
// Encode/Parse enforce through the shared validation: Encode is the
// exported face exercised here (it validates before rendering, fail
// closed) — mirrors internal/relmanifest's TestManifestInvariantsRefuseByName.
func TestPlanInvariantsRefusedByName(t *testing.T) {
	h := strings.Repeat("ab", 32)
	cases := []struct {
		name   string
		base   func() adopt.Plan
		mutate func(*adopt.Plan)
		want   string
	}{
		{"wrong schema_version", validDraftPlan, func(p *adopt.Plan) { p.SchemaVersion = "adoption-plan/v2" },
			`schema_version "adoption-plan/v2"`},
		{"unknown status", validDraftPlan, func(p *adopt.Plan) { p.Status = "pending" },
			`status "pending"`},
		{"entry unknown class", validDraftPlan, func(p *adopt.Plan) { p.Entries[0].Class = "delete" },
			`unknown class "delete"`},
		{"entry empty path", validDraftPlan, func(p *adopt.Plan) { p.Entries[0].Path = "" },
			"entry with an empty path"},
		{"entry duplicate path", validDraftPlan, func(p *adopt.Plan) {
			p.Entries = append(p.Entries, p.Entries[0])
		}, `duplicate entry path ".pyxis/config.yaml"`},
		{"entry absolute path", validDraftPlan, func(p *adopt.Plan) { p.Entries[0].Path = "/etc/passwd" },
			`entry path "/etc/passwd"`},
		{"entry dot-dot path", validDraftPlan, func(p *adopt.Plan) { p.Entries[0].Path = "../escape.md" },
			`entry path "../escape.md"`},
		{"entry unclean path", validDraftPlan, func(p *adopt.Plan) { p.Entries[0].Path = ".pyxis/./config.yaml" },
			"not a clean, relative, slash-separated path"},
		{"collide-manual with empty detail", validDraftPlan, func(p *adopt.Plan) { p.Entries[1].Detail = "" },
			`entry "AGENTS.md": class collide-manual requires a non-empty detail`},
		{"draft with non-empty epoch", validDraftPlan, func(p *adopt.Plan) { p.Epoch = strings.Repeat("cd", 20) },
			"status draft carries a non-empty epoch"},
		{"draft with non-empty prior hooks path", validDraftPlan, func(p *adopt.Plan) { p.Hooks.PriorHooksPath = ".husky" },
			"status draft carries a non-empty hooks.prior_hooks_path"},
		{"draft with added_paths", validDraftPlan, func(p *adopt.Plan) {
			p.AddedPaths = []adopt.AddedPath{{Path: "a", SHA256: h}}
		}, "status draft carries added_paths entries"},
		{"applied with empty epoch", validAppliedPlan, func(p *adopt.Plan) { p.Epoch = "" },
			"status applied carries no epoch"},
		{"added_paths empty path", validAppliedPlan, func(p *adopt.Plan) { p.AddedPaths[0].Path = "" },
			"added_paths entry with an empty path"},
		{"added_paths duplicate path", validAppliedPlan, func(p *adopt.Plan) {
			p.AddedPaths = append(p.AddedPaths, p.AddedPaths[0])
		}, `duplicate added_paths path ".pyxis/config.yaml"`},
		{"added_paths unclean path", validAppliedPlan, func(p *adopt.Plan) { p.AddedPaths[0].Path = "a/../b" },
			`added_paths path "a/../b"`},
		{"added_paths short sha256", validAppliedPlan, func(p *adopt.Plan) { p.AddedPaths[0].SHA256 = "short" },
			`is not a 64-char lowercase sha256 hex`},
		{"added_paths uppercase sha256", validAppliedPlan, func(p *adopt.Plan) { p.AddedPaths[0].SHA256 = strings.ToUpper(h) },
			`is not a 64-char lowercase sha256 hex`},
		{"add entry applied with no stamped baseline", validAppliedPlan, func(p *adopt.Plan) {
			p.AddedPaths = nil // drops the .pyxis/config.yaml baseline the add entry requires
		}, `entry ".pyxis/config.yaml": class add requires an added_paths baseline`},
		{"added_paths entry no add entry claims", validAppliedPlan, func(p *adopt.Plan) {
			p.AddedPaths = append(p.AddedPaths, adopt.AddedPath{Path: "nonexistent-file.txt", SHA256: h})
		}, `added_paths "nonexistent-file.txt": no add entry claims this path`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.base()
			c.mutate(&p)
			_, err := p.Encode()
			if err == nil {
				t.Fatalf("no error; want one containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}

	// The unmutated fixtures encode clean — the table's baseline.
	if _, err := validDraftPlan().Encode(); err != nil {
		t.Fatalf("valid draft plan refused: %v", err)
	}
	if _, err := validAppliedPlan().Encode(); err != nil {
		t.Fatalf("valid applied plan refused: %v", err)
	}
}

// TestAppliedPlanWithNoAddOrChainEntriesEncodesClean proves the DINV-7
// baseline check is conditional, not a blanket "added_paths non-empty"
// demand: a re-adopt against an already-adopted repository with no drift
// (UB-4) carries no add/chain entries at all, so an empty added_paths is
// exactly correct, not a defect.
func TestAppliedPlanWithNoAddOrChainEntriesEncodesClean(t *testing.T) {
	p := adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusApplied,
		Epoch:         strings.Repeat("cd", 20),
		Entries: []adopt.Entry{
			{Path: "src/**", Class: adopt.ClassNeverTouch},
			{Path: ".pyxis/scripts/pyxis-bootstrap.sh", Class: adopt.ClassEngineVerbatim},
		},
	}
	if _, err := p.Encode(); err != nil {
		t.Fatalf("valid no-drift applied plan refused: %v", err)
	}
}

// TestEncodeDeterministic proves repeated Encode calls over the same value
// are byte-identical — no clock, no map-order nondeterminism (Plan carries
// no map field, but the guarantee is worth pinning directly).
func TestEncodeDeterministic(t *testing.T) {
	p := validAppliedPlan()
	a, err := p.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b, err := p.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("Encode not byte-stable across calls")
	}
}
