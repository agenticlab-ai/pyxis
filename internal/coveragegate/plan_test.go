package coveragegate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

const splitMarker = `
func mark(name string) {
	f, err := os.CreateTemp(os.Getenv("SPLIT_MARKERS"), name+"-*")
	if err != nil {
		panic(err)
	}
	f.Close()
}
`

// splitFixture is a module with one package worth splitting, one ordinary
// package, and one without tests whose statements still count.
func splitFixture(t *testing.T, failing bool) string {
	t.Helper()
	dir := t.TempDir()
	check := "1"
	if failing {
		check = "2"
	}
	files := map[string]string{
		"go.mod":       "module example.com/split\n\ngo 1.25\n",
		"slow/slow.go": "package slow\n\nfunc Value(n int) int {\n\tswitch n {\n\tcase 0:\n\t\treturn 0\n\tcase 1:\n\t\treturn 1\n\tcase 2:\n\t\treturn 2\n\t}\n\treturn 3\n}\n",
		"slow/slow_test.go": "package slow\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n" + splitMarker + `
func TestMain(m *testing.M) { os.Exit(m.Run()) }
func Testable() {}
func Test(t *testing.T)  { mark("Test"); Value(0) }
func TestA(t *testing.T) { mark("TestA"); Value(1) }
func TestB(t *testing.T) { mark("TestB"); if Value(1) != ` + check + ` { t.Fatal("TestB failed") } }
func TestC(t *testing.T) { mark("TestC"); t.Run("A", func(t *testing.T) { mark("TestC.A") }) }
func FuzzE(f *testing.F) { f.Add(2); f.Fuzz(func(t *testing.T, n int) { mark("FuzzE"); Value(n) }) }
`,
		"slow/x_test.go": "package slow_test\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"testing\"\n\n\t\"example.com/split/slow\"\n)\n" + splitMarker + `
func TestA(t *testing.T) { mark("TestA.x") }
func TestD(t *testing.T) { mark("TestD"); slow.Value(3) }
func ExampleValue() { mark("ExampleValue"); fmt.Println(slow.Value(1))
	// Output: 1
}
`,
		"fast/fast.go":      "package fast\n\nfunc Ok() bool { return true }\n",
		"fast/fast_test.go": "package fast\n\nimport \"testing\"\n\nfunc TestOk(t *testing.T) { if !Ok() { t.Fatal() } }\n",
		"none/none.go":      "package none\n\nfunc Unused() int { return 7 }\n",
	}
	for name, contents := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func markers(t *testing.T, dir string) map[string]int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ran := make(map[string]int)
	for _, entry := range entries {
		name, _, _ := strings.Cut(entry.Name(), "-")
		ran[name]++
	}
	return ran
}

func TestPlanRunsEachTestOnceAndMatchesTheSingleInvocation(t *testing.T) {
	dir := splitFixture(t, false)
	plan := Plan{
		{Package: "example.com/split/slow", Parts: 3},
		{Package: "example.com/split/slow", Parts: 2},
		{Package: "example.com/split/missing", Parts: 2},
	}
	runs := plan.invocations(t.Context(), dir, "./...")
	var labels []string
	for _, run := range runs {
		labels = append(labels, run.label)
	}
	want := []string{"unsplit packages", "example.com/split/slow part 1/3", "example.com/split/slow part 2/3", "example.com/split/slow part 3/3"}
	if !slices.Equal(labels, want) {
		t.Fatalf("invocations = %q, want %q", labels, want)
	}

	measure := func(measure func(*strings.Builder) (Report, error)) (Report, map[string]int) {
		t.Helper()
		t.Setenv("SPLIT_MARKERS", t.TempDir())
		var output strings.Builder
		report, err := measure(&output)
		if err != nil {
			t.Fatalf("measure: %v\n%s", err, output.String())
		}
		return report, markers(t, os.Getenv("SPLIT_MARKERS"))
	}
	single, singleRan := measure(func(w *strings.Builder) (Report, error) { return Measure(t.Context(), dir, "./...", w) })
	split, splitRan := measure(func(w *strings.Builder) (Report, error) { return plan.Measure(t.Context(), dir, "./...", w) })

	everyOnce := map[string]int{"Test": 1, "TestA": 1, "TestA.x": 1, "TestB": 1, "TestC": 1, "TestC.A": 1, "TestD": 1, "FuzzE": 1, "ExampleValue": 1}
	if !reflect.DeepEqual(singleRan, everyOnce) || !reflect.DeepEqual(splitRan, everyOnce) {
		t.Fatalf("tests ran single=%v split=%v, want each once: %v", singleRan, splitRan, everyOnce)
	}
	if !reflect.DeepEqual(split, single) {
		t.Fatalf("split report = %+v, want single-invocation report %+v", split, single)
	}
	if len(single.Files) != 3 {
		t.Fatalf("report files = %+v, want slow, fast and the untested package", single.Files)
	}
}

func TestPlanKeepsSmallOrUnlistedPackagesWhole(t *testing.T) {
	dir := splitFixture(t, false)
	for name, plan := range map[string]Plan{
		"unlisted":  {{Package: "example.com/split/missing", Parts: 2}},
		"one test":  {{Package: "example.com/split/fast", Parts: 4}},
		"one part":  {{Package: "example.com/split/slow", Parts: 1}},
		"bad scope": {{Package: "example.com/split/slow", Parts: 2}},
	} {
		t.Run(name, func(t *testing.T) {
			pattern := "./..."
			if name == "bad scope" {
				pattern = "./package-that-does-not-exist"
			}
			runs := plan.invocations(t.Context(), dir, pattern)
			if len(runs) != 1 || !slices.Equal(runs[0].args, []string{pattern}) {
				t.Fatalf("invocations = %+v, want the single pattern invocation", runs)
			}
		})
	}
	only := Plan{{Package: "example.com/split/slow", Parts: 2}}.invocations(t.Context(), dir, "./slow")
	if len(only) != 2 || only[0].label != "example.com/split/slow part 1/2" {
		t.Fatalf("fully split pattern invocations = %+v", only)
	}
}

func TestPlanFallsBackWhenTestsCannotBeScanned(t *testing.T) {
	dir := splitFixture(t, false)
	if err := os.WriteFile(filepath.Join(dir, "slow", "broken_test.go"), []byte("package slow\n\nfunc TestBroken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := Plan{{Package: "example.com/split/slow", Parts: 2}}
	if runs := plan.invocations(t.Context(), dir, "./..."); len(runs) != 1 {
		t.Fatalf("invocations = %+v, want the single pattern invocation", runs)
	}
	if _, err := plan.Measure(t.Context(), dir, "./...", &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "go test with coverage") {
		t.Fatalf("broken package error = %v", err)
	}
}

func TestPlanNamesTheFailingPart(t *testing.T) {
	dir := splitFixture(t, true)
	t.Setenv("SPLIT_MARKERS", t.TempDir())
	var output strings.Builder
	_, err := Plan{{Package: "example.com/split/slow", Parts: 3}}.Measure(t.Context(), dir, "./...", &output)
	if err == nil || !strings.Contains(err.Error(), "go test with coverage: example.com/split/slow part ") {
		t.Fatalf("failing part error = %v", err)
	}
	if !strings.Contains(output.String(), "--- FAIL: TestB") || !strings.Contains(output.String(), "ok  \texample.com/split/fast") {
		t.Fatalf("output lacks the failing and passing invocations:\n%s", output.String())
	}
}

func TestPartitionDealsNamesAndSkipsTheRest(t *testing.T) {
	got := partition([]string{"TestA", "TestB", "TestC", "TestD", "TestE"}, 3)
	want := [][]string{
		{"-run", "^(TestA|TestD)$"},
		{"-run", "^(TestB|TestE)$"},
		{"-skip", "^(TestA|TestD|TestB|TestE)$"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partition = %q, want %q", got, want)
	}
	if got := partition([]string{"TestA"}, 3); got != nil {
		t.Fatalf("single test partition = %q", got)
	}
}

func TestInterleaveStartsEveryPackageEarly(t *testing.T) {
	parts := func(pkg string, n int) []invocation {
		out := make([]invocation, n)
		for i := range out {
			out[i] = invocation{label: pkg + strings.Repeat("'", i)}
		}
		return out
	}
	var labels []string
	for _, part := range interleave([][]invocation{parts("a", 3), parts("b", 1), parts("c", 2)}) {
		labels = append(labels, part.label)
	}
	if want := []string{"a", "b", "c", "a'", "c'", "a''"}; !slices.Equal(labels, want) {
		t.Fatalf("interleave = %q, want %q", labels, want)
	}
}

func TestIsTestFollowsGoTestNaming(t *testing.T) {
	for name, want := range map[string]bool{
		"Test": true, "TestX": true, "Test_x": true, "Test1": true, "TestÜber": true,
		"TestMain": false, "Testable": false, "testX": false, "BenchmarkX": false, "ExampleX": false,
	} {
		if got := isTest(name); got != want {
			t.Errorf("isTest(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestReadProfilesReportsMissingProfile(t *testing.T) {
	if _, err := readProfiles([]string{filepath.Join(t.TempDir(), "missing.out")}); err == nil || !strings.Contains(err.Error(), "open coverage profile") {
		t.Fatalf("missing profile error = %v", err)
	}
}

func TestReadProfilesRejectsConflictingModes(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"a": "mode: atomic\nx.go:1.1,2.1 1 1\n", "b": "mode: set\nx.go:1.1,2.1 1 0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readProfiles([]string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}); err == nil || !strings.Contains(err.Error(), "duplicate mode") {
		t.Fatalf("conflicting modes error = %v", err)
	}
}

type failingWriter struct{}

var errWrite = errors.New("output closed")

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestLineWriterForwardsWholeLines(t *testing.T) {
	var out strings.Builder
	lines := &lineWriter{mu: new(sync.Mutex), w: &out}
	for _, chunk := range []string{"ok  \tpkg", "/a 1s\nFAIL\tp", "kg/b\n--- partial"} {
		if n, err := lines.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if out.String() != "ok  \tpkg/a 1s\nFAIL\tpkg/b\n" {
		t.Fatalf("forwarded %q before flush", out.String())
	}
	if err := lines.flush(); err != nil || out.String() != "ok  \tpkg/a 1s\nFAIL\tpkg/b\n--- partial\n" {
		t.Fatalf("flush = %v, output %q", err, out.String())
	}
	if err := lines.flush(); err != nil {
		t.Fatalf("empty flush = %v", err)
	}

	broken := &lineWriter{mu: new(sync.Mutex), w: failingWriter{}}
	if _, err := broken.Write([]byte("line\n")); !errors.Is(err, errWrite) {
		t.Fatalf("Write error = %v", err)
	}
	tail := &lineWriter{mu: new(sync.Mutex), w: failingWriter{}}
	if _, err := tail.Write([]byte("tail")); err != nil {
		t.Fatalf("partial Write error = %v", err)
	}
	if err := tail.flush(); !errors.Is(err, errWrite) {
		t.Fatalf("flush error = %v", err)
	}
}
