package ecosystem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// goTestJSON runs `go test -json ./...` inside a fresh temp module containing
// files, returning its combined stdout+stderr — the exact shape
// parseGoTestFailures consumes. Real `go test` invocations (not hand-crafted
// JSON literals) so the parser is proven against the real tool's own output,
// mirroring internal/verify/checks' precedent of exercising real go/git
// subprocesses in tests.
func goTestJSON(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture-test\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput() // a failing test suite exits nonzero; that's expected here
	return string(out)
}

func identities(failures []TestFailure) []string {
	ids := make([]string, len(failures))
	for i, f := range failures {
		ids[i] = f.Identity
	}
	sort.Strings(ids)
	return ids
}

func TestParseGoTestFailures_AllPass(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestOk(t *testing.T) {}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("failures = %+v, want none", failures)
	}
}

func TestParseGoTestFailures_SingleFailure(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestFails(t *testing.T) {\n\tt.Fatalf(\"boom: %d != %d\", 1, 2)\n}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want exactly 1", failures)
	}
	f := failures[0]
	if f.Identity != "fixture-test/b::TestFails" {
		t.Errorf("identity = %q, want %q", f.Identity, "fixture-test/b::TestFails")
	}
	if f.Path != "b_test.go" || f.Line != 6 {
		t.Errorf("location = %s:%d, want b_test.go:6", f.Path, f.Line)
	}
}

// TestParseGoTestFailures_MultiplePackages proves package-qualified
// identities: two packages each declaring a same-named failing test must
// produce two DISTINCT identities, never collapse into one.
func TestParseGoTestFailures_MultiplePackages(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"pkg1/pkg1_test.go": "package pkg1\n\nimport \"testing\"\n\nfunc TestShared(t *testing.T) {\n\tt.Fatal(\"pkg1 fails\")\n}\n",
		"pkg2/pkg2_test.go": "package pkg2\n\nimport \"testing\"\n\nfunc TestShared(t *testing.T) {\n\tt.Fatal(\"pkg2 fails\")\n}\n\nfunc TestPkg2Ok(t *testing.T) {}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	got := identities(failures)
	want := []string{"fixture-test/pkg1::TestShared", "fixture-test/pkg2::TestShared"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("identities = %v, want %v", got, want)
	}
}

// TestParseGoTestFailures_SubtestLeafOnly proves a failing subtest's
// ancestor (which go test ALSO reports as "fail") is collapsed away: only
// the leaf identity survives.
func TestParseGoTestFailures_SubtestLeafOnly(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"b/b_test.go": `package b

import "testing"

func TestParent(t *testing.T) {
	t.Run("child_one", func(t *testing.T) {
		t.Fatalf("nested boom")
	})
}
`,
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	got := identities(failures)
	want := []string{"fixture-test/b::TestParent/child_one"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("identities = %v, want %v (the ancestor TestParent must be collapsed away)", got, want)
	}
}

// TestParseGoTestFailures_Panic proves an unrecovered panic still resolves
// to a clean per-test failure identity — go test's own -json stream reports
// a terminal "fail" event for the panicking test before the process exits.
func TestParseGoTestFailures_Panic(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"c/c_test.go": "package c\n\nimport \"testing\"\n\nfunc TestPanics(t *testing.T) {\n\tpanic(\"real panic\")\n}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	got := identities(failures)
	want := []string{"fixture-test/c::TestPanics"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("identities = %v, want %v", got, want)
	}
}

// TestParseGoTestFailures_BuildFailure proves a package whose test binary
// never compiles gets ONE package-level identity naming it, with the
// compiler diagnostic surfaced as Message — never silently dropped, and
// never mistaken for a per-test failure.
func TestParseGoTestFailures_BuildFailure(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"d/d_test.go": "package d\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n\tthisDoesNotExist()\n}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want exactly 1", failures)
	}
	f := failures[0]
	if f.Identity != "fixture-test/d [build failed]" {
		t.Errorf("identity = %q, want %q", f.Identity, "fixture-test/d [build failed]")
	}
	if !strings.Contains(f.Message, "thisDoesNotExist") {
		t.Errorf("message = %q, want it to name the undefined symbol", f.Message)
	}
}

// TestParseGoTestFailures_MixedPassFailBuildFail exercises all three shapes
// at once across separate packages, proving they compose without
// interference.
func TestParseGoTestFailures_MixedPassFailBuildFail(t *testing.T) {
	output := goTestJSON(t, map[string]string{
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestOk(t *testing.T) {}\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestFails(t *testing.T) {\n\tt.Fatal(\"boom\")\n}\n",
		"d/d_test.go": "package d\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n\tthisDoesNotExist()\n}\n",
	})

	failures, err := parseGoTestFailures(output)
	if err != nil {
		t.Fatalf("parseGoTestFailures: %v", err)
	}
	got := identities(failures)
	want := []string{"fixture-test/b::TestFails", "fixture-test/d [build failed]"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("identities = %v, want %v", got, want)
	}
}

func TestParseGoTestFailures_EmptyOutput(t *testing.T) {
	failures, err := parseGoTestFailures("")
	if err != nil {
		t.Fatalf("parseGoTestFailures(\"\") returned an error: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("failures = %+v, want none", failures)
	}
}

// TestParseGoTestFailures_GarbageOutput is UB-9's core case: output that is
// not go test -json at all must fail closed (an error), never silently
// report zero failures.
func TestParseGoTestFailures_GarbageOutput(t *testing.T) {
	for _, garbage := range []string{
		"not valid json at all\nmore garbage\n",
		"{\"Action\":\"pass\"", // truncated mid-object
		"panic: runtime error: some crash dump\ngoroutine 1 [running]:\n",
	} {
		if _, err := parseGoTestFailures(garbage); err == nil {
			t.Errorf("parseGoTestFailures(%q) returned no error, want a parse error", garbage)
		}
	}
}

// TestParseGoTestFailures_OrphanedTest proves a test that started but never
// reached a terminal outcome (e.g. a sibling panic aborted the binary
// first) fails the whole parse closed, rather than silently treating the
// orphaned test as absent (which would misread as baseline shrinkage).
func TestParseGoTestFailures_OrphanedTest(t *testing.T) {
	output := `{"Action":"run","Package":"example.com/x","Test":"TestNeverFinishes"}
{"Action":"fail","Package":"example.com/x"}
`
	_, err := parseGoTestFailures(output)
	if err == nil {
		t.Fatal("parseGoTestFailures returned no error for an orphaned (started, never concluded) test")
	}
	if !strings.Contains(err.Error(), "TestNeverFinishes") {
		t.Errorf("error = %v, want it to name the orphaned test", err)
	}
}

func TestParseFailureIdentities_Dispatch(t *testing.T) {
	d := New()

	if _, err := d.ParseFailureIdentities("go", ""); err != nil {
		t.Errorf("ParseFailureIdentities(go, \"\") = %v, want nil error", err)
	}

	if _, err := d.ParseFailureIdentities("unknown-stack", ""); err == nil {
		t.Error("ParseFailureIdentities(unknown-stack, ...) returned no error, want one (fail closed on an unsupported stack)")
	}
}

func TestLeafNames(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"no nesting", []string{"TestA", "TestB"}, []string{"TestA", "TestB"}},
		{"parent and child", []string{"TestParent", "TestParent/child"}, []string{"TestParent/child"}},
		{"grandparent chain", []string{"TestA", "TestA/b", "TestA/b/c"}, []string{"TestA/b/c"}},
		{"unrelated prefix is not a parent", []string{"TestA", "TestAB"}, []string{"TestA", "TestAB"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leafNames(tt.input)
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("leafNames(%v) = %v, want %v", tt.input, got, want)
			}
		})
	}
}

func TestCleanImportPath(t *testing.T) {
	tests := map[string]string{
		"example.com/scratch/d [example.com/scratch/d.test]": "example.com/scratch/d",
		"example.com/scratch/a":                              "example.com/scratch/a",
	}
	for in, want := range tests {
		if got := cleanImportPath(in); got != want {
			t.Errorf("cleanImportPath(%q) = %q, want %q", in, got, want)
		}
	}
}
