package release_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
)

// ghStep is one expected call in a transcript-fake gh runner (the
// wire.Runner seam reused per AP-11 — the same transcript style T10's
// wiring tests use). check validates the argv; deliver, when set, writes
// bytes to the call's --output path (the download leg's side effect).
type ghStep struct {
	check   func(t *testing.T, args []string)
	stdout  string
	stderr  string
	exit    int
	deliver []byte
}

type fakeGH struct {
	t     *testing.T
	steps []ghStep
	i     int
}

func (f *fakeGH) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	f.t.Helper()
	if f.i >= len(f.steps) {
		f.t.Fatalf("unexpected gh call #%d: gh %s", f.i+1, strings.Join(args, " "))
	}
	s := f.steps[f.i]
	f.i++
	s.check(f.t, args)
	if s.deliver != nil {
		out := ""
		for j, a := range args {
			if a == "--output" && j+1 < len(args) {
				out = args[j+1]
			}
		}
		if out == "" {
			f.t.Fatalf("deliver step without --output in gh %s", strings.Join(args, " "))
		}
		if err := os.WriteFile(out, s.deliver, 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	return s.stdout, s.stderr, s.exit, nil
}

func (f *fakeGH) done() {
	f.t.Helper()
	if f.i != len(f.steps) {
		f.t.Fatalf("gh transcript incomplete: %d of %d calls made", f.i, len(f.steps))
	}
}

func wantExact(want ...string) func(*testing.T, []string) {
	return func(t *testing.T, args []string) {
		t.Helper()
		if !slices.Equal(args, want) {
			t.Fatalf("gh args = %v, want %v", args, want)
		}
	}
}

func viewStep(stdout, stderr string, exit int) ghStep {
	return ghStep{check: wantExact("release", "view", fixtureVersion, "--json", "assets"), stdout: stdout, stderr: stderr, exit: exit}
}

func uploadStep(tag, outDir, name string) ghStep {
	return ghStep{check: wantExact("release", "upload", tag, filepath.Join(outDir, name))}
}

func downloadStep(name string, deliver []byte) ghStep {
	return ghStep{
		check: func(t *testing.T, args []string) {
			t.Helper()
			if len(args) < 5 || args[0] != "release" || args[1] != "download" || args[3] != "--pattern" || args[4] != name {
				t.Fatalf("gh args = %v, want a release download --pattern %s", args, name)
			}
		},
		deliver: deliver,
	}
}

func assetsJSON(names ...string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("{\"name\":%q}", n)
	}
	return "{\"assets\":[" + strings.Join(parts, ",") + "]}"
}

// Absent release: one create call with --verify-tag and the whole sorted
// artifact set — the first publish leg of upload-or-verify.
func TestPublishCreatesAbsentRelease(t *testing.T) {
	out, res := packFixture(t, fixtureRepo(t))
	gh := &fakeGH{t: t, steps: []ghStep{
		viewStep("", "release not found", 1),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if args[0] != "release" || args[1] != "create" || args[2] != fixtureVersion {
				t.Fatalf("gh args = %v, want a release create", args)
			}
			if !slices.Contains(args, "--verify-tag") {
				t.Fatalf("create without --verify-tag: %v", args)
			}
			for _, name := range res.Artifacts {
				if !slices.Contains(args, filepath.Join(out, name)) {
					t.Fatalf("create is missing artifact %s: %v", name, args)
				}
			}
		}},
	}}
	pres, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	gh.done()
	if !pres.Created || len(pres.Actions) != len(res.Artifacts) {
		t.Fatalf("result = %+v, want created with every artifact uploaded", pres)
	}
}

// Existing release: missing assets upload, an existing byte-identical asset
// downloads and verifies — the re-run CONVERGES (AP-22), no clobber ever.
func TestPublishUploadOrVerifyConverges(t *testing.T) {
	out, res := packFixture(t, fixtureRepo(t))
	existing := "payload-" + fixtureVersion + ".tar.gz"
	existingBytes, err := os.ReadFile(filepath.Join(out, existing))
	if err != nil {
		t.Fatal(err)
	}
	steps := []ghStep{viewStep(assetsJSON(existing), "", 0)}
	for _, name := range res.Artifacts { // sorted walk order
		if name == existing {
			steps = append(steps, downloadStep(name, existingBytes))
			continue
		}
		steps = append(steps, uploadStep(fixtureVersion, out, name))
	}
	gh := &fakeGH{t: t, steps: steps}
	pres, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	gh.done()
	if pres.Created || pres.Actions[existing] != "verified" {
		t.Fatalf("result = %+v, want the existing asset verified, not re-created", pres)
	}
	uploaded := 0
	for _, action := range pres.Actions {
		if action == "uploaded" {
			uploaded++
		}
	}
	if uploaded != len(res.Artifacts)-1 {
		t.Fatalf("uploaded %d, want %d", uploaded, len(res.Artifacts)-1)
	}
}

// A byte-divergent existing asset REFUSES by name — verify-not-clobber is
// the whole point (never a second, different set under one tag).
func TestPublishRefusesDivergentAsset(t *testing.T) {
	out, res := packFixture(t, fixtureRepo(t))
	divergent := res.Artifacts[0] // SHA256SUMS sorts first — walked first
	steps := []ghStep{
		viewStep(assetsJSON(divergent), "", 0),
		downloadStep(divergent, []byte("different bytes under the same tag")),
	}
	gh := &fakeGH{t: t, steps: steps}
	_, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err == nil || !strings.Contains(err.Error(), divergent) || !strings.Contains(err.Error(), "never clobbered") {
		t.Fatalf("publish over a divergent asset = %v, want the named no-clobber refusal", err)
	}
	gh.done()
}

// A remote asset outside the packed set refuses before any operation — the
// set under a tag is closed.
func TestPublishRefusesForeignAsset(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	gh := &fakeGH{t: t, steps: []ghStep{
		viewStep(assetsJSON("stray.bin"), "", 0),
	}}
	_, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err == nil || !strings.Contains(err.Error(), "stray.bin") {
		t.Fatalf("publish with a foreign remote asset = %v, want it named", err)
	}
	gh.done()
}

// A gh failure that is not release-absence is a hard, named error.
func TestPublishSurfacesGHFailure(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	gh := &fakeGH{t: t, steps: []ghStep{
		viewStep("", "HTTP 500 upstream sad", 1),
	}}
	_, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("publish with gh failing = %v, want the stderr surfaced", err)
	}
	gh.done()
}

// Publish runs the local verify half FIRST: an incoherent local set never
// reaches the remote (zero gh calls).
func TestPublishRefusesIncoherentLocalSet(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	p := filepath.Join(out, "toola-"+fixtureVersion+"-linux-amd64")
	if err := os.WriteFile(p, []byte("tampered after pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{t: t} // any call fails the test
	_, err := release.Publish(context.Background(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Runner: gh})
	if err == nil || !strings.Contains(err.Error(), "failed verification first") {
		t.Fatalf("publish of a tampered local set = %v, want the local-verify refusal", err)
	}
	gh.done()
}
