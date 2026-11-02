package ecosystem

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestNew(t *testing.T) {
	d := New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
	if d.Version != "1.0.0" {
		t.Errorf("version mismatch: got %s, want 1.0.0", d.Version)
	}
}

func TestForStack(t *testing.T) {
	d := New()
	goStack := d.ForStack("go")
	if goStack == nil {
		t.Fatal("ForStack(go) returned nil")
	}
	if goStack.Name != "go" {
		t.Errorf("stack name mismatch: got %s, want go", goStack.Name)
	}

	unknown := d.ForStack("unknown")
	if unknown != nil {
		t.Error("ForStack(unknown) should return nil")
	}
}

func TestResolveManifestDefaults(t *testing.T) {
	d := New()

	testGlobs, _ := d.ResolveManifestDefaults("go", nil, nil)
	if len(testGlobs) == 0 {
		t.Error("expected test globs from ecosystem defaults")
	}
	if testGlobs[0] != "**/*_test.go" {
		t.Errorf("test glob mismatch: got %s", testGlobs[0])
	}

	customGlobs := []string{"custom/**/*_test.go"}
	testGlobs, _ = d.ResolveManifestDefaults("go", customGlobs, nil)
	if testGlobs[0] != "custom/**/*_test.go" {
		t.Errorf("manifest override not applied: got %s", testGlobs[0])
	}
}

func TestSecretPatterns(t *testing.T) {
	d := New()
	if len(d.SecretPatterns) == 0 {
		t.Error("no secret patterns compiled")
	}

	testStr := "password: secret123"
	hasMatch := false
	for _, pattern := range d.SecretPatterns {
		if pattern.MatchString(testStr) {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		t.Error("password pattern did not match")
	}
}

func TestAssertionPatterns(t *testing.T) {
	d := New()
	goPatterns, ok := d.AssertionPatterns["go"]
	if !ok {
		t.Fatal("no assertion patterns for go stack")
	}
	if len(goPatterns) == 0 {
		t.Error("no go assertion patterns compiled")
	}

	testStr := "\tt.Assert("
	hasMatch := false
	for _, pattern := range goPatterns {
		if pattern.MatchString(testStr) {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		t.Error("assertion pattern did not match t.Assert")
	}

	// Regression (#402 review): the testify assert./require. patterns were
	// malformed regexes silently dropped at compile time, so testify assertions
	// went undetected. They must now compile and match.
	for _, call := range []string{"\tassert.Equal(t, a, b)", "\trequire.NoError(t, err)"} {
		matched := false
		for _, pattern := range goPatterns {
			if pattern.MatchString(call) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("no assertion pattern matched testify call %q", call)
		}
	}

	// Regression (T7 review): only the *f (Fatalf/Errorf) forms were covered,
	// so a hollowed test that kept the bare form (t.Fatal/t.Error — 99 and 35
	// real occurrences in this repo's own tests) would not have its assertion
	// count drop detected.
	for _, call := range []string{"\tt.Fatal(err)", "\tt.Error(err)"} {
		matched := false
		for _, pattern := range goPatterns {
			if pattern.MatchString(call) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("no assertion pattern matched bare call %q", call)
		}
	}
}

func TestSkipMarkers(t *testing.T) {
	d := New()
	goMarkers, ok := d.SkipMarkers["go"]
	if !ok {
		t.Fatal("no skip markers for go stack")
	}
	if len(goMarkers) == 0 {
		t.Error("no go skip markers compiled")
	}

	testStr := "\tt.Skip("
	hasMatch := false
	for _, pattern := range goMarkers {
		if pattern.MatchString(testStr) {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		t.Error("skip marker pattern did not match t.Skip")
	}

	// Regression (T7 review): Skipf (the formatted-message form, 12 real
	// occurrences in this repo's own tests) was not covered by any prior
	// pattern — a silenced test using t.Skipf would have gone undetected.
	if !anyMatch(goMarkers, "\tt.Skipf(\"flaky: %v\", err)") {
		t.Error("no skip-marker pattern matched t.Skipf")
	}
}

func anyMatch(patterns []*regexp.Regexp, s string) bool {
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

func TestCachePins(t *testing.T) {
	d := New()
	goStack := d.ForStack("go")
	if _, ok := goStack.CachePins["GOCACHE"]; !ok {
		t.Error("GOCACHE not in go cache pins")
	}
}

// #402 review F1 / acceptance criterion 1: a minimal manifest resolves the full
// verb set from ecosystem defaults; manifest verbs override, and the resolved
// verbs are copies of the shared defaults.
func TestResolveVerbs(t *testing.T) {
	d := New()

	// No manifest verbs -> the full default go verb set.
	verbs := d.ResolveVerbs("go", nil)
	for _, name := range []string{"format-check", "lint", "build", "test"} {
		if _, ok := verbs[name]; !ok {
			t.Errorf("default go verb %q missing from the resolved set", name)
		}
	}
	if verbs["test"].Command != "go test -json ./..." {
		t.Errorf("default test command = %q, want 'go test -json ./...'", verbs["test"].Command)
	}

	// A manifest verb overrides the default of the same name; a manifest-only verb
	// is added; other defaults remain.
	verbs = d.ResolveVerbs("go", map[string]*schema.Verb{
		"test":  {Command: "gotestsum"},
		"probe": {Command: "./probe.sh"},
	})
	if verbs["test"].Command != "gotestsum" {
		t.Errorf("manifest override not applied: %q", verbs["test"].Command)
	}
	if verbs["probe"] == nil || verbs["probe"].Command != "./probe.sh" {
		t.Error("manifest-only verb not added")
	}
	if verbs["build"].Command != "go build ./..." {
		t.Error("default build verb should remain")
	}

	// Resolved verbs are copies: mutating one must not corrupt the shared defaults
	// a subsequent resolve returns.
	first := d.ResolveVerbs("go", nil)
	first["build"].Command = "mutated"
	if d.ResolveVerbs("go", nil)["build"].Command != "go build ./..." {
		t.Error("mutating a resolved verb corrupted the shared defaults")
	}
}

// TestMarkerFiles pins the per-stack marker-file set adopt's stack detection
// reads (T4, design:existing-repo-onboarding-v2#3.6).
func TestMarkerFiles(t *testing.T) {
	d := New()
	cases := map[string][]string{
		"go":     {"go.mod"},
		"python": {"requirements.txt", "pyproject.toml", "setup.py"},
		"node":   {"package.json"},
	}
	for stack, want := range cases {
		sd := d.ForStack(stack)
		if sd == nil {
			t.Fatalf("ForStack(%s) returned nil", stack)
		}
		if !reflect.DeepEqual(sd.MarkerFiles, want) {
			t.Errorf("%s MarkerFiles = %v, want %v", stack, sd.MarkerFiles, want)
		}
	}
}

// TestStackNames proves the enumeration is sorted — deterministic
// regardless of the underlying map's iteration order.
func TestStackNames(t *testing.T) {
	d := New()
	got := d.StackNames()
	want := []string{"go", "node", "python"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StackNames() = %v, want %v", got, want)
	}
}
