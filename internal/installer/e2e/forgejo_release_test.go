package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// The live Forgejo scenario needs the execution tools and installed Git
// hook stamper beyond TestMain's installer and docs gate. Build only on
// demand, once per test process. Cache bytes rather
// than temporary paths so one test's cleanup cannot invalidate another leg.
var forgejoExecutionBinaries struct {
	once sync.Once
	data map[string][]byte
	err  error
}

func forgejoExecutionTools(t *testing.T) map[string][]byte {
	t.Helper()
	forgejoExecutionBinaries.once.Do(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
		defer cancel()
		dir, err := os.MkdirTemp("", "pyxis-forgejo-e2e-tools-")
		if err != nil {
			forgejoExecutionBinaries.err = err
			return
		}
		defer func() {
			forgejoExecutionBinaries.err = errors.Join(forgejoExecutionBinaries.err, os.RemoveAll(dir))
		}()
		forgejoExecutionBinaries.data = make(map[string][]byte)
		for _, tool := range []string{"argus-stamp", "code-gate", "implement-now"} {
			out := filepath.Join(dir, tool)
			cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./cmd/"+tool)
			cmd.Dir = repoRoot()
			if output, err := cmd.CombinedOutput(); err != nil {
				forgejoExecutionBinaries.err = fmt.Errorf("build %s: %w\n%s", tool, err, output)
				return
			}
			data, err := os.ReadFile(out)
			if err != nil {
				forgejoExecutionBinaries.err = fmt.Errorf("read built %s: %w", tool, err)
				return
			}
			forgejoExecutionBinaries.data[tool] = data
		}
	})
	if forgejoExecutionBinaries.err != nil {
		t.Fatalf("Forgejo execution fixture tools: %v", forgejoExecutionBinaries.err)
	}
	return forgejoExecutionBinaries.data
}

// buildForgejoFixtureRelease reuses the existing platform-local release and
// adds genuine execution/review assets. No cross compilation, service setup,
// repository creation, or network access occurs here. The live test calls it
// only after its PYXIS_E2E_FORGEJO opt-in; default tests keep their two builds.
func buildForgejoFixtureRelease(t *testing.T) fixtureRelease {
	t.Helper()
	r := buildFixtureRelease(t)
	manifestPath := filepath.Join(r.src, "release-manifest-"+r.version+".json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := relmanifest.ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	for tool, binary := range forgejoExecutionTools(t) {
		manifest.Binaries[tool] = map[string]string{r.plat: hexOf(binary)}
		writeFile(t, filepath.Join(r.src, tool+"-"+r.version+"-"+r.plat), binary, 0o755)
	}
	files := forgejoExecutionPayload(t)
	archive := tarGz(t, files)
	manifest.Payload = relmanifest.Payload{ArchiveSHA256: hexOf(archive)}
	for _, file := range files {
		entry := relmanifest.PayloadFile{Path: file.path, Class: file.class}
		if file.class == relmanifest.EngineVerbatim {
			digest := hexOf(file.b)
			entry.SHA256 = &digest
		}
		manifest.Payload.Files = append(manifest.Payload.Files, entry)
	}
	data, err = manifest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, manifestPath, data, 0o644)
	writeFile(t, filepath.Join(r.src, "payload-"+r.version+".tar.gz"), archive, 0o644)
	r.pin = hexOf(data)
	return r
}

// These source assets are consumed by the installed executor, its review
// factory and model prompts. They are never substitutes for the production
// implementation: only the model-facing CLI is stubbed by the live scenario.
func forgejoExecutionPayload(t *testing.T) []pf {
	t.Helper()
	files := payloadFiles(t)
	byPath := make(map[string]pf, len(files))
	for _, file := range files {
		if file.path == ".github/workflows/docs-gate.yml" {
			// Converge owns this output and maps it to the selected code host.
			// The fast fixture selects local CI, so it needs no Linux cross-builds.
			file.class = relmanifest.Generated
		}
		if file.class == relmanifest.Generated {
			file.b = nil
		}
		byPath[file.path] = file
	}
	for _, path := range []string{
		".pyxis/pipeline.md",
		".pyxis/model-routing.md",
		".pyxis/context-pack.yaml",
		".pyxis/skills/help/SKILL.md",
		".pyxis/authoring/help.md",
		".pyxis/skills/implement-now/SKILL.md",
		".pyxis/authoring/implement-now.md",
		".pyxis/skills/impl-review/SKILL.md",
		".pyxis/authoring/impl-review.md",
		".pyxis/skills/impl-review/calibration.md",
		".pyxis/skills/impl-review/lenses/spec-fidelity.md",
		".pyxis/skills/impl-review/lenses/design-conformance.md",
		".pyxis/skills/impl-review/lenses/security-abuse.md",
		".pyxis/skills/impl-review/lenses/correctness-critic.md",
		".pyxis/templates/plan-template.md",
		".pyxis/templates/prd-template.md",
		".pyxis/templates/tech-design-template.md",
		".pyxis/dist/github-workflows/docs-gate.yml",
		".pyxis/dist/forgejo-workflows/docs-gate.yml",
		".pyxis/scripts/pyxis-bootstrap.sh",
		".pyxis/scripts/docgate-resolve.sh",
		".pyxis/scripts/code-gate-resolve.sh",
	} {
		info, err := os.Stat(at(repoRoot(), path))
		if err != nil {
			t.Fatal(err)
		}
		byPath[path] = pf{path: path, b: readSourceFile(t, path), class: relmanifest.EngineVerbatim, exec: info.Mode().Perm()&0o111 != 0}
	}
	files = files[:0]
	for _, file := range byPath {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files
}

func TestForgejoExecutionPayloadUsesRealSourceAssets(t *testing.T) {
	files := forgejoExecutionPayload(t)
	seen := make(map[string]bool)
	for _, file := range files {
		if seen[file.path] {
			t.Fatalf("duplicate payload path %s", file.path)
		}
		seen[file.path] = true
		if file.class == relmanifest.EngineVerbatim && !bytes.Equal(file.b, readSourceFile(t, file.path)) {
			t.Errorf("payload %s differs from its real source asset", file.path)
		}
	}
	for _, path := range []string{".pyxis/skills/impl-review/calibration.md", ".pyxis/authoring/impl-review.md", ".pyxis/model-routing.md", ".pyxis/security/bwrap.apparmor"} {
		if !seen[path] {
			t.Errorf("required execution asset %s missing", path)
		}
	}
}

// This opt-in check needs no Forgejo server or token: it validates the fast
// release independently before the scenario is allowed to touch its host.
func TestForgejoFixtureReleaseIsPlatformLocal(t *testing.T) {
	if os.Getenv("PYXIS_E2E_FORGEJO") != "1" {
		t.Skip("set PYXIS_E2E_FORGEJO=1 to build the extra execution tools")
	}
	r := buildForgejoFixtureRelease(t)
	data, err := os.ReadFile(filepath.Join(r.src, "release-manifest-"+r.version+".json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := relmanifest.ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Binaries) != 5 || hexOf(data) != r.pin {
		t.Fatalf("fixture must pin exactly five real tools: %v", manifest.Binaries)
	}
	for _, tool := range []string{"pyxis", "docgate", "argus-stamp", "code-gate", "implement-now"} {
		platforms := manifest.Binaries[tool]
		if len(platforms) != 1 || platforms[r.plat] == "" {
			t.Fatalf("%s is not limited to host platform %s: %v", tool, r.plat, platforms)
		}
		if actual := hashFile(t, filepath.Join(r.src, tool+"-"+r.version+"-"+r.plat)); actual != platforms[r.plat] {
			t.Errorf("%s bytes differ from the release pin", tool)
		}
	}
	if actual := hashFile(t, filepath.Join(r.src, "payload-"+r.version+".tar.gz")); actual != manifest.Payload.ArchiveSHA256 {
		t.Error("payload bytes differ from the release pin")
	}
}
