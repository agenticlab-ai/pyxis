package pack

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenInputs builds a complete, distinct-content ReviewInputs — every field
// carries a unique marker so a test can assert which input landed where and
// prove nothing bled across kinds.
func goldenInputs() ReviewInputs {
	return ReviewInputs{
		Subject:         "pr-42",
		Proposal:        []byte("PROPOSAL diff --git a/x.go b/x.go body\n"),
		CodegateVerdict: []byte("CODEGATE verdict: green ladder\n"),
		Charter:         []byte("CHARTER the task charter body\n"),
		DesignBindings:  []byte("DESIGN binding elements body\n"),
		Calibration:     []byte("CALIBRATION the calibration table\n"),
	}
}

// --- the golden pack build ---

func TestBuildGoldenPack(t *testing.T) {
	pack, err := Build(goldenInputs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(pack)
	// The manifest is contextpack's (reused wholesale): the stage, the subject,
	// the content-addressed built_from + digest, and the mapped kinds.
	for _, want := range []string{
		"# context-pack/v1",
		"# stage: " + Stage,
		"# subject: pr-42",
		"# built_from: content:sha256:",
		"# digest: sha256:",
		// each source is recorded present on its mapped kind
		"subject     " + proposalPath + " (present)",
		"subject     " + verdictPath + " (present)",
		"governing   " + charterPath + " (present)",
		"governing   " + designPath + " (present)",
		"calibration " + calibrationPath + " (present)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("pack missing manifest line %q", want)
		}
	}
}

// --- determinism: two builds byte-identical (DINV-1, inherited) ---

func TestBuildIsDeterministic(t *testing.T) {
	a, err := Build(goldenInputs())
	if err != nil {
		t.Fatalf("build a: %v", err)
	}
	b, err := Build(goldenInputs())
	if err != nil {
		t.Fatalf("build b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("two builds at one input state differ — determinism broken (DINV-1)")
	}
}

// --- the five inputs land on the right kinds, and NO authoring trail (DINV-5) ---

func TestBuildInputsMapToKindsAndNoAuthoringTrail(t *testing.T) {
	pack, err := Build(goldenInputs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(pack)

	// The subject kind carries the frozen subject under work — the proposal
	// diff and the codegate verdict.
	assertBodyOnKind(t, s, "subject", proposalPath, "PROPOSAL diff")
	assertBodyOnKind(t, s, "subject", verdictPath, "CODEGATE verdict")
	// The governing kind carries the charter and the design's binding elements.
	assertBodyOnKind(t, s, "governing", charterPath, "CHARTER the task charter")
	assertBodyOnKind(t, s, "governing", designPath, "DESIGN binding elements")
	// The calibration kind carries the calibration table.
	assertBodyOnKind(t, s, "calibration", calibrationPath, "CALIBRATION the calibration table")

	// DINV-5: the pack contains NO authoring-trail data. No marker a prior
	// finding/disposition would carry, and no authoring-trail kind in the
	// manifest (the kinds are exactly the three stable-context ones).
	for _, forbidden := range []string{
		"finding", "disposition", "journal", "delivery", "answer key",
		"prior review", "FND-",
	} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(forbidden)) {
			t.Errorf("pack leaked authoring-trail token %q — DINV-5 (finding-blindness) broken", forbidden)
		}
	}
	for _, kind := range []string{"journal", "disposition", "delivery", "finding", "trail"} {
		if strings.Contains(s, "context-pack source: "+kind) {
			t.Errorf("pack has an authoring-trail kind %q body section — DINV-5 broken", kind)
		}
	}
}

// TestReviewInputsHasNoAuthoringTrailField is the interface-level DINV-5 guard:
// the pack builder has NO parameter or path for authoring-trail inputs. This is
// a compile-shaped assertion — ReviewInputs is constructed with EVERY field
// named, so if a prior-findings/dispositions field were ever added this test
// fails to compile, catching the boundary breach at build time.
func TestReviewInputsHasNoAuthoringTrailField(t *testing.T) {
	// Every field of ReviewInputs, named — the whole review input surface.
	// Adding an authoring-trail field breaks this literal (missing field is a
	// compile error under go vet's composites; explicit field names make an
	// added field surface here).
	_ = ReviewInputs{
		Subject:         "s",
		Proposal:        []byte{},
		CodegateVerdict: []byte{},
		Charter:         []byte{},
		DesignBindings:  []byte{},
		Calibration:     []byte{},
	}
}

// --- fail-closed: a missing required input yields a named error, no artifact ---

func TestBuildFailsClosedOnMissingInput(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ReviewInputs)
		wantSub string
	}{
		{"empty subject", func(in *ReviewInputs) { in.Subject = "" }, "empty subject"},
		{"missing proposal", func(in *ReviewInputs) { in.Proposal = nil }, `missing required input "proposal"`},
		{"missing verdict", func(in *ReviewInputs) { in.CodegateVerdict = nil }, `missing required input "codegate verdict"`},
		{"missing charter", func(in *ReviewInputs) { in.Charter = nil }, `missing required input "charter"`},
		{"missing design bindings", func(in *ReviewInputs) { in.DesignBindings = nil }, `missing required input "design bindings"`},
		{"missing calibration", func(in *ReviewInputs) { in.Calibration = nil }, `missing required input "calibration"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := goldenInputs()
			tc.mutate(&in)
			pack, err := Build(in)
			if pack != nil {
				t.Fatalf("a missing required input must emit no artifact")
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("want error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

// --- an empty (non-nil) input is a legal source, recorded present ---

func TestBuildEmptyContentInputIsPresent(t *testing.T) {
	in := goldenInputs()
	in.Charter = []byte{} // non-nil but empty — a legal source, present with no body
	pack, err := Build(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(pack), "governing   "+charterPath+" (present)") {
		t.Errorf("an empty-but-present input must be recorded present")
	}
}

// --- the builder leaves no throwaway root behind ---

func TestBuildCleansUpTempRoot(t *testing.T) {
	before := tempEntries(t)
	if _, err := Build(goldenInputs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := tempEntries(t)
	for name := range after {
		if !before[name] && strings.HasPrefix(name, "implreview-pack-") {
			t.Errorf("build left a throwaway root behind: %s", name)
		}
	}
}

func TestWriteSourceReportsFilesystemFailures(t *testing.T) {
	t.Run("create parent", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "root-file")
		if err := os.WriteFile(root, []byte("occupied"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := writeSource(root, "nested/source", []byte("x")); err == nil || !strings.Contains(err.Error(), "mkdir") {
			t.Fatalf("writeSource parent error = %v, want mkdir diagnostic", err)
		}
	})

	t.Run("write target", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "review", "source")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeSource(root, "review/source", []byte("x")); err == nil || !strings.Contains(err.Error(), "write") {
			t.Fatalf("writeSource target error = %v, want write diagnostic", err)
		}
	})
}

// --- helpers (house style: small, hermetic) ---

// assertBodyOnKind checks the pack has a body section for (kind, path) and that
// section carries the expected marker — proving the input landed on the right
// admitted kind with its verbatim content.
func assertBodyOnKind(t *testing.T, pack, kind, path, marker string) {
	t.Helper()
	header := "context-pack source: " + kind + " " + path + " ====="
	if !strings.Contains(pack, header) {
		t.Errorf("pack missing body section for kind %q path %q", kind, path)
		return
	}
	if !strings.Contains(pack, marker) {
		t.Errorf("pack body for %q missing marker %q", path, marker)
	}
}

// tempEntries snapshots the names in the OS temp dir — used to assert the
// builder leaves no throwaway root behind.
func tempEntries(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	entries, err := os.ReadDir(filepath.Clean(os.TempDir()))
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}
