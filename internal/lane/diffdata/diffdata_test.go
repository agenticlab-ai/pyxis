package diffdata

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// fakeRun returns a Runner serving canned name-status output and recording the
// argv it was invoked with.
func fakeRun(out string, err error, got *[]string) Runner {
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if got != nil {
			*got = args
		}
		if err != nil {
			return nil, err
		}
		return []byte(out), nil
	}
}

func TestDeriveWith(t *testing.T) {
	cfg := config.Defaults()
	cases := []struct {
		name    string
		listing string
		paths   []string
		classes []string
	}{
		{
			name:    "plain code diff carries no class",
			listing: "M\tinternal/lane/policy/policy.go\nA\tcmd/lane/main.go",
			paths:   []string{"internal/lane/policy/policy.go", "cmd/lane/main.go"},
		},
		{
			name:    "governing doc edit carries the class",
			listing: "M\tdocs/product-vision.md",
			paths:   []string{"docs/product-vision.md"},
			classes: []string{ClassGoverningDocEdit},
		},
		{
			name:    "incident registry append still reads governing (conservative)",
			listing: "M\tdocs/observability/incidents.md",
			paths:   []string{"docs/observability/incidents.md"},
			classes: []string{ClassGoverningDocEdit},
		},
		{
			name:    "modified merged PRD entry carries the frozen class",
			listing: "M\tdocs/prds/argus/000-base.md",
			paths:   []string{"docs/prds/argus/000-base.md"},
			classes: []string{ClassLedgerFrozenEntryEdit},
		},
		{
			name:    "ADDED PRD entry is new content, not frozen",
			listing: "A\tdocs/prds/argus/003-new.md",
			paths:   []string{"docs/prds/argus/003-new.md"},
		},
		{
			name:    "deleted tech design carries the frozen class",
			listing: "D\tdocs/tech-design/merge-lane-v1.md",
			paths:   []string{"docs/tech-design/merge-lane-v1.md"},
			classes: []string{ClassLedgerFrozenEntryEdit},
		},
		{
			name:    "regenerated design index is not frozen",
			listing: "M\tdocs/tech-design/index.md",
			paths:   []string{"docs/tech-design/index.md"},
		},
		{
			name:    "projection files are not dense entries",
			listing: "M\tdocs/prds/argus/effective.md",
			paths:   []string{"docs/prds/argus/effective.md"},
		},
		{
			name:    "both classes accumulate",
			listing: "M\tdocs/architecture-principles.md\nM\tdocs/prds/argus/001-a.md",
			paths:   []string{"docs/architecture-principles.md", "docs/prds/argus/001-a.md"},
			classes: []string{ClassGoverningDocEdit, ClassLedgerFrozenEntryEdit},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := DeriveWith(context.Background(), fakeRun(tc.listing, nil, nil), ".", "base", "head", cfg)
			if err != nil {
				t.Fatalf("DeriveWith: %v", err)
			}
			if !reflect.DeepEqual(d.Paths, tc.paths) {
				t.Errorf("paths = %v, want %v", d.Paths, tc.paths)
			}
			if !reflect.DeepEqual(d.Classes, tc.classes) && !(len(d.Classes) == 0 && len(tc.classes) == 0) {
				t.Errorf("classes = %v, want %v", d.Classes, tc.classes)
			}
		})
	}
}

func TestDeriveWithRefusals(t *testing.T) {
	cfg := config.Defaults()
	t.Run("missing base or head", func(t *testing.T) {
		if _, err := DeriveWith(context.Background(), fakeRun("", nil, nil), ".", "", "head", cfg); err == nil {
			t.Fatal("want error for empty base")
		}
		if _, err := DeriveWith(context.Background(), fakeRun("", nil, nil), ".", "base", " ", cfg); err == nil {
			t.Fatal("want error for empty head")
		}
	})
	t.Run("git failure surfaces", func(t *testing.T) {
		if _, err := DeriveWith(context.Background(), fakeRun("", errors.New("boom"), nil), ".", "b", "h", cfg); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("want wrapped git error, got %v", err)
		}
	})
	t.Run("unparsable listing line is an error, never a skip", func(t *testing.T) {
		if _, err := DeriveWith(context.Background(), fakeRun("garbage-without-tab", nil, nil), ".", "b", "h", cfg); err == nil {
			t.Fatal("want error for unparsable line")
		}
	})
	t.Run("empty diff yields empty inputs (classifier fails closed downstream)", func(t *testing.T) {
		d, err := DeriveWith(context.Background(), fakeRun("", nil, nil), ".", "b", "h", cfg)
		if err != nil {
			t.Fatalf("DeriveWith: %v", err)
		}
		if len(d.Paths) != 0 || len(d.Classes) != 0 {
			t.Errorf("want empty diff, got %+v", d)
		}
	})
}

// TestGoverningDocClassIsConfigDriven proves the governing-doc-edit class
// resolves the vision / principles / incidents locations from config, not from
// baked-in literals (config-driven-portability-v1 §3.2, FR-1/FR-3; DINV-1): a
// diff touching the RELOCATED governing docs carries the class, and a diff
// touching the STALE default locations does not. The default-config arm is the
// byte-identity proof (DINV-2) — the former literals still classify.
func TestGoverningDocClassIsConfigDriven(t *testing.T) {
	relocated := config.Defaults()
	relocated.ProductVision = "governance/vision.md"
	relocated.ArchitecturePrinciples = "governance/principles.md"
	relocated.Incidents = "governance/incidents.md"

	cases := []struct {
		name     string
		cfg      config.Config
		listing  string
		wantClas bool // whether ClassGoverningDocEdit is present
	}{
		{"default: former vision literal still classifies", config.Defaults(), "M\tdocs/product-vision.md", true},
		{"default: former principles literal still classifies", config.Defaults(), "M\tdocs/architecture-principles.md", true},
		{"default: former incidents literal still classifies", config.Defaults(), "M\tdocs/observability/incidents.md", true},
		{"relocated: configured vision carries the class", relocated, "M\tgovernance/vision.md", true},
		{"relocated: configured principles carries the class", relocated, "M\tgovernance/principles.md", true},
		{"relocated: configured incidents carries the class", relocated, "M\tgovernance/incidents.md", true},
		{"relocated: STALE default vision no longer classifies", relocated, "M\tdocs/product-vision.md", false},
		{"relocated: STALE default principles no longer classifies", relocated, "M\tdocs/architecture-principles.md", false},
		{"relocated: STALE default incidents no longer classifies", relocated, "M\tdocs/observability/incidents.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := DeriveWith(context.Background(), fakeRun(tc.listing, nil, nil), ".", "base", "head", tc.cfg)
			if err != nil {
				t.Fatalf("DeriveWith: %v", err)
			}
			got := false
			for _, c := range d.Classes {
				if c == ClassGoverningDocEdit {
					got = true
				}
			}
			if got != tc.wantClas {
				t.Errorf("ClassGoverningDocEdit present = %v, want %v (classes=%v)", got, tc.wantClas, d.Classes)
			}
		})
	}
}

// TestDeriveArgv pins the exact git invocation: --name-status with rename
// detection disabled, so a rename reads as its D+A pair and no perimeter is
// escaped by moving a file.
func TestDeriveArgv(t *testing.T) {
	var got []string
	_, err := DeriveWith(context.Background(), fakeRun("", nil, &got), ".", "b", "h", config.Defaults())
	if err != nil {
		t.Fatalf("DeriveWith: %v", err)
	}
	want := []string{"diff", "--name-status", "--no-renames", "b", "h"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %v, want %v", got, want)
	}
}
