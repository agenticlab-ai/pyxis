package sweep_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/sweep"
)

// The unwired-packages detector runs the real go toolchain against a hermetic
// temp module (design §3.9: the production runner integration-covered by the
// fixture repos). The module path is the real one so the reported subjects are
// the historical package names verbatim.

const fixtureModule = "github.com/agenticlab-ai/pyxis"

// writeModule materializes a temp module from a path→content map and returns its
// root. All imports in the fixtures are in-module, so `go list` resolves offline.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func pkgFile(pkg, body string) string {
	return "package " + pkg + "\n" + body + "\n"
}

// mainFile builds a cmd main importing the given in-module relative packages and
// referencing each, so `go list -deps ./cmd/...` walks into every one.
func mainFile(imports ...string) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n")
	for i, rel := range imports {
		fmt.Fprintf(&b, "\ta%d %q\n", i, fixtureModule+"/"+rel)
	}
	b.WriteString(")\n\nfunc main() {\n")
	for i := range imports {
		fmt.Fprintf(&b, "\t_ = a%d.V\n", i)
	}
	b.WriteString("}\n")
	return b.String()
}

// replayFixture reproduces the pre-wiring mainline of the historical dead-feature
// instances (#514 the merge lane, #515 the wake channel): the lane and wake
// package trees present but reachable from no cmd binary, alongside a wired
// package that a binary does reach (design §3.2; prd FR-2).
func replayFixture(t *testing.T) string {
	return writeModule(t, map[string]string{
		"go.mod": "module " + fixtureModule + "\n\ngo 1.26\n",
		// the shipped binary reaches only internal/wired — never lane or wake.
		"cmd/docgate/main.go":     mainFile("internal/wired"),
		"internal/wired/wired.go": pkgFile("wired", "var V int"),
		// #514 — the merge lane's organs, landed reachable from no binary.
		"internal/lane/lane.go":          pkgFile("lane", "var V int"),
		"internal/lane/policy/policy.go": pkgFile("policy", "var V int"),
		"internal/lane/state/state.go":   pkgFile("state", "var V int"),
		// #515 — the wake channel, likewise unwired.
		"internal/wake/wake.go":      pkgFile("wake", "var V int"),
		"internal/wake/host/host.go": pkgFile("host", "var V int"),
	})
}

// subjects returns the sorted finding subjects for concise assertions.
func subjects(res sweep.DetectorResult) []string {
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.Subject)
	}
	sort.Strings(out)
	return out
}

func run(t *testing.T, root string, policy sweep.Policy) sweep.DetectorResult {
	t.Helper()
	res := sweep.DetectUnwired(context.Background(), sweep.State{Root: root, Runner: sweep.ExecRunner{}}, policy)
	if res.Fault != "" {
		t.Fatalf("unexpected detector fault: %s", res.Fault)
	}
	return res
}

// On the pinned replay fixture the historical package trees are reported; the
// wired package is not (acceptance 1, first clause; prd FR-2).
func TestReplayReportsHistoricalTrees(t *testing.T) {
	res := run(t, replayFixture(t), sweep.Empty())
	got := subjects(res)
	want := []string{
		"internal/lane", "internal/lane/policy", "internal/lane/state",
		"internal/wake", "internal/wake/host",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("replay subjects\n got: %v\nwant: %v", got, want)
	}
	for _, f := range res.Findings {
		if f.Subject == "internal/wired" {
			t.Fatal("the wired package was reported unwired")
		}
	}
}

// On the fully-wired fixture (every package reachable from the binary) zero
// unwired findings emit (acceptance 1, second clause; prd FR-2).
func TestFullyWiredReportsZero(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod": "module " + fixtureModule + "\n\ngo 1.26\n",
		"cmd/docgate/main.go": mainFile(
			"internal/wired", "internal/lane", "internal/lane/policy",
			"internal/lane/state", "internal/wake", "internal/wake/host",
		),
		"internal/wired/wired.go":        pkgFile("wired", "var V int"),
		"internal/lane/lane.go":          pkgFile("lane", "var V int"),
		"internal/lane/policy/policy.go": pkgFile("policy", "var V int"),
		"internal/lane/state/state.go":   pkgFile("state", "var V int"),
		"internal/wake/wake.go":          pkgFile("wake", "var V int"),
		"internal/wake/host/host.go":     pkgFile("host", "var V int"),
	})
	res := run(t, root, sweep.Empty())
	if len(res.Findings) != 0 {
		t.Fatalf("fully-wired fixture reported findings: %v", subjects(res))
	}
}

// A waived package produces an applied-waiver line with its reason and no
// finding; removing the waiver restores the finding (acceptance 2; prd FR-10).
func TestPackageWaiverSuppressesAndSurfaces(t *testing.T) {
	root := replayFixture(t)
	const reason = "test-harness library: consumed by adapter suites, deliberately no binary"
	policy := sweep.Empty()
	policy.PackageWaivers = []sweep.PackageWaiver{{Package: "internal/lane", Reason: reason}}

	res := run(t, root, policy)
	for _, f := range res.Findings {
		if f.Subject == "internal/lane" {
			t.Fatal("waived package internal/lane still produced a finding")
		}
	}
	var found *sweep.AppliedWaiver
	for i := range res.WaiversApplied {
		if res.WaiversApplied[i].Subject == "internal/lane" {
			found = &res.WaiversApplied[i]
		}
	}
	if found == nil {
		t.Fatalf("no applied-waiver line for internal/lane: %+v", res.WaiversApplied)
	}
	if found.Reason != reason {
		t.Fatalf("applied waiver reason not surfaced: %q", found.Reason)
	}
	if found.Kind != sweep.WaiverKindPackage {
		t.Fatalf("applied waiver kind wrong: %q", found.Kind)
	}
	// The waiver is exact, not a prefix: internal/lane/policy is still reported.
	subs := subjects(res)
	if !contains(subs, "internal/lane/policy") {
		t.Fatalf("waiver over-suppressed subpackages: %v", subs)
	}

	// Removing the waiver restores the finding.
	restored := run(t, root, sweep.Empty())
	if !contains(subjects(restored), "internal/lane") {
		t.Fatalf("removing the waiver did not restore internal/lane: %v", subjects(restored))
	}
}

// Every emitted finding carries all four contracted fields (acceptance 3;
// DINV-5). The report's own marshal re-checks this; here we assert at source.
func TestEveryFindingCarriesFourFields(t *testing.T) {
	res := run(t, replayFixture(t), sweep.Empty())
	if len(res.Findings) == 0 {
		t.Fatal("no findings to check")
	}
	for _, f := range res.Findings {
		if f.Detector != sweep.DetectorUnwiredPackages {
			t.Fatalf("finding detector wrong: %q", f.Detector)
		}
		if f.Subject == "" || f.Evidence == "" || f.Remedy == "" {
			t.Fatalf("finding missing a contracted field: %+v", f)
		}
		if f.SchemaVersion != sweep.FindingSchemaVersion {
			t.Fatalf("finding schema version wrong: %q", f.SchemaVersion)
		}
	}
}

// The detector performs no write: the fixture tree is byte-identical before and
// after a pass (acceptance 4, first clause; DINV-1).
func TestDetectorPerformsNoWrite(t *testing.T) {
	root := replayFixture(t)
	before := treeDigest(t, root)
	_ = run(t, root, sweep.Empty())
	after := treeDigest(t, root)
	if before != after {
		t.Fatal("the detector mutated the evaluated tree")
	}
}

// The detector consults nothing but the evaluated state and the policy: a runner
// that records its commands shows only read-only `go list` reads against the
// evaluated root, and no other input surface is touched (acceptance 4).
func TestDetectorConsultsOnlyStateAndPolicy(t *testing.T) {
	root := replayFixture(t)
	rec := &recordingRunner{inner: sweep.ExecRunner{}}
	res := sweep.DetectUnwired(context.Background(), sweep.State{Root: root, Runner: rec}, sweep.Empty())
	if res.Fault != "" {
		t.Fatalf("unexpected fault: %s", res.Fault)
	}
	for _, c := range rec.calls {
		if c.dir != root {
			t.Fatalf("a read ran outside the evaluated root: %q", c.dir)
		}
		if c.name != "go" || len(c.args) == 0 || c.args[0] != "list" {
			t.Fatalf("a non-read-only command was issued: %s %v", c.name, c.args)
		}
	}
	if len(rec.calls) == 0 {
		t.Fatal("the detector issued no reads")
	}
}

// A repo the toolchain cannot evaluate faults in-band with zero findings, never
// a silent clean sweep (DINV-3/DINV-4; prd UB-1).
func TestUnreadableRepoFaults(t *testing.T) {
	root := t.TempDir() // no go.mod: `go list -m` fails
	res := sweep.DetectUnwired(context.Background(), sweep.State{Root: root, Runner: sweep.ExecRunner{}}, sweep.Empty())
	if res.Fault == "" {
		t.Fatal("an unreadable repo produced no fault")
	}
	if len(res.Findings) != 0 {
		t.Fatalf("a faulted detector emitted findings: %v", subjects(res))
	}
}

// --- helpers ---

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// treeDigest hashes every regular file's path and content under root so a write
// anywhere in the tree changes the digest.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%x\x00", rel, sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// recordingRunner wraps a runner and records every command it is asked to run,
// so a test can assert the detector's read surface.
type recordingRunner struct {
	inner sweep.Runner
	calls []recordedCall
}

type recordedCall struct {
	dir  string
	name string
	args []string
}

func (r *recordingRunner) Run(ctx context.Context, dir, name string, args ...string) ([]string, error) {
	r.calls = append(r.calls, recordedCall{dir: dir, name: name, args: args})
	return r.inner.Run(ctx, dir, name, args...)
}
