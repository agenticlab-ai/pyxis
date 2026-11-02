package release_test

import (
	"context"
	"debug/buildinfo"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
)

// buildFixtureModule is a self-contained module the real toolchain can
// cross-compile: stdlib only, so the build is hermetic (no network, no
// module downloads).
func buildFixtureModule(t *testing.T, mains map[string]string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("build invokes the real Go toolchain")
	}
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", 0o644, "module buildfixture\n\ngo 1.26\n")
	for tool, src := range mains {
		writeFile(t, dir, "cmd/"+tool+"/main.go", 0o644, src)
	}
	// The build-only tool is present and must never be built.
	writeFile(t, dir, "cmd/"+release.BuildOnlyTool+"/main.go", 0o644, "package main\n\nfunc main() {}\n")
	return dir
}

const helloMain = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"

// frontDoorMain mirrors the real front door's version contract: a stampable
// version var, reported through the version verb — what Build's attestation
// exercises.
const frontDoorMain = "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nvar version = \"source\"\n\nfunc main() {\n\tif len(os.Args) > 1 && os.Args[1] == \"version\" {\n\t\tfmt.Println(\"pyxis \" + version)\n\t}\n}\n"

// Build produces exactly the scan-derived set × platforms, named as Pack
// expects, and each artifact really is a binary for its target.
func TestBuildProducesTheMatrix(t *testing.T) {
	repo := buildFixtureModule(t, map[string]string{"alpha": helloMain})
	out := t.TempDir()

	res, err := release.Build(context.Background(), release.BuildOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: out, Jobs: 2,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if want := []string{"alpha"}; len(res.Tools) != 1 || res.Tools[0] != want[0] {
		t.Fatalf("tools = %v, want %v (the build-only tool must be excluded)", res.Tools, want)
	}
	if len(res.Binaries) != len(release.Platforms) {
		t.Fatalf("binaries = %v, want one per platform", res.Binaries)
	}
	for _, plat := range release.Platforms {
		name := "alpha-" + fixtureVersion + "-" + plat
		path := filepath.Join(out, name)
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			t.Fatalf("%s is not a Go binary: %v", name, err)
		}
		var goos, goarch string
		for _, s := range info.Settings {
			switch s.Key {
			case "GOOS":
				goos = s.Value
			case "GOARCH":
				goarch = s.Value
			case "CGO_ENABLED":
				if s.Value != "0" {
					t.Errorf("%s: CGO_ENABLED = %s, want 0 (a release binary is static)", name, s.Value)
				}
			case "-trimpath":
				if s.Value != "true" {
					t.Errorf("%s: -trimpath = %s, want true (no build-machine paths in a release artifact)", name, s.Value)
				}
			}
		}
		if got := goos + "-" + goarch; got != plat {
			t.Errorf("%s targets %s, want %s", name, got, plat)
		}
	}
	if _, err := os.Stat(filepath.Join(out, release.BuildOnlyTool+"-"+fixtureVersion+"-linux-amd64")); !os.IsNotExist(err) {
		t.Error("the build-only tool was built into the release matrix")
	}
}

// The matrix Build writes is exactly what Pack accepts — the two halves of
// the runbook agree on the naming, with no glue in between.
func TestBuildOutputFeedsPack(t *testing.T) {
	if testing.Short() {
		t.Skip("build invokes the real Go toolchain")
	}
	repo := fixtureRepo(t)
	bin := t.TempDir()
	// The fixture repo's cmd/ tree has no module, so build from a module
	// whose tool names match it exactly.
	mains := map[string]string{}
	for _, tool := range fixtureTools {
		mains[tool] = helloMain
	}
	mains["pyxis"] = frontDoorMain // the front door must attest its version
	src := buildFixtureModule(t, mains)
	if _, err := release.Build(context.Background(), release.BuildOptions{
		Version: fixtureVersion, RepoRoot: src, OutDir: bin,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: bin,
	}); err != nil {
		t.Fatalf("pack over the built matrix: %v", err)
	}
}

// A broken tool fails the whole build and names every target it broke —
// one run tells the operator the full story.
func TestBuildNamesEveryFailure(t *testing.T) {
	repo := buildFixtureModule(t, map[string]string{
		"alpha":  helloMain,
		"broken": "package main\n\nfunc main() { this is not go }\n",
	})
	_, err := release.Build(context.Background(), release.BuildOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("err = nil, want the build refusal")
	}
	for _, plat := range release.Platforms {
		if !strings.Contains(err.Error(), "broken "+plat) {
			t.Errorf("refusal does not name broken/%s:\n%v", plat, err)
		}
	}
	if strings.Contains(err.Error(), "alpha ") {
		t.Errorf("refusal blames the tool that built fine:\n%v", err)
	}
}

// The stamp reaches an executed binary: the front door built for this host
// reports the version being built, not its source default (#868 — every
// release through v0.1.3 shipped binaries answering "pyxis source").
func TestBuildStampsTheFrontDoorVersion(t *testing.T) {
	host := runtime.GOOS + "-" + runtime.GOARCH
	if !slices.Contains(release.Platforms, host) {
		t.Skipf("host %s is outside the release matrix; nothing built here can run", host)
	}
	repo := buildFixtureModule(t, map[string]string{"pyxis": frontDoorMain})
	out := t.TempDir()
	if _, err := release.Build(context.Background(), release.BuildOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: out, Jobs: 2,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}
	got, err := exec.Command(filepath.Join(out, "pyxis-"+fixtureVersion+"-"+host), "version").Output()
	if err != nil {
		t.Fatalf("run the built front door: %v", err)
	}
	if want := "pyxis " + fixtureVersion; strings.TrimSpace(string(got)) != want {
		t.Errorf("version = %q, want %q", strings.TrimSpace(string(got)), want)
	}
}

// A front door that cannot attest the built version fails the whole build —
// the drift that shipped #868 is now a named refusal, not a shipped set.
func TestBuildRefusesAnUnattestedFrontDoor(t *testing.T) {
	host := runtime.GOOS + "-" + runtime.GOARCH
	if !slices.Contains(release.Platforms, host) {
		t.Skipf("host %s is outside the release matrix; nothing built here can run", host)
	}
	repo := buildFixtureModule(t, map[string]string{"pyxis": helloMain})
	_, err := release.Build(context.Background(), release.BuildOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(), Jobs: 2,
	})
	if err == nil || !strings.Contains(err.Error(), "self-reports") {
		t.Fatalf("err = %v, want the attestation refusal", err)
	}
}

// A version outside the tag grammar refuses before the toolchain is touched.
func TestBuildRefusesBadVersion(t *testing.T) {
	_, err := release.Build(context.Background(), release.BuildOptions{
		Version: "0.1.2", RepoRoot: t.TempDir(), OutDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "system tag") {
		t.Fatalf("err = %v, want the version-grammar refusal", err)
	}
}
