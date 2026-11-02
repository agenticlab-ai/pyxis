package converge_test

// Hermetic tests for the T5 converge engine: a complete fixture release
// served over file://, an operand tree built per test, and the per-class
// drift matrix / idempotency / kill-and-re-run / preview honesty /
// source-mode legs the charter names (installer-v1 A-4; prd:installer
// FR-5, FR-6, FR-19, UB-5).

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// ---- fixture scaffolding ----------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func writeFile(t *testing.T, path string, b []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	// Fixture modes model the release manifest, independent of the runner's umask.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// pf is one payload file of the fixture release: tree content, archive
// content, and (for engine-verbatim) the manifest pin all derive from b.
type pf struct {
	path  string
	b     []byte
	class relmanifest.Class
	exec  bool
}

// stdFiles mirrors the real class-policy shapes: repo-owned config + stamp,
// engine-verbatim templates/scripts/workflow (one executable), generated
// design indexes.
func stdFiles() []pf {
	return []pf{
		{path: ".pyxis/config.yaml", b: []byte("# the packed SOURCE-repo config; never extracted\n"), class: relmanifest.RepoOwned},
		{path: ".pyxis/stamp/origin.json", b: []byte("{\"stamp\":true}\n"), class: relmanifest.RepoOwned},
		{path: ".pyxis/templates/prd-template.md", b: []byte("# PRD template\nbody\n"), class: relmanifest.EngineVerbatim},
		{path: ".pyxis/scripts/hook.sh", b: []byte("#!/bin/sh\nexit 0\n"), class: relmanifest.EngineVerbatim, exec: true},
		{path: ".github/workflows/lint.yml", b: []byte("name: lint\n"), class: relmanifest.EngineVerbatim},
		{path: "docs/tech-design/index.md", b: []byte("stale packed index\n"), class: relmanifest.Generated},
		{path: "docs/tech-design/index.json", b: []byte("{}\n"), class: relmanifest.Generated},
	}
}

// tarGz packs the fixture payload archive (modes by the exec bit, the
// packer's normalization).
func tarGz(t *testing.T, files []pf) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		mode := int64(0o644)
		if f.exec {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: f.path, Mode: mode, Size: int64(len(f.b))}); err != nil {
			t.Fatalf("tar header %s: %v", f.path, err)
		}
		if _, err := tw.Write(f.b); err != nil {
			t.Fatalf("tar write %s: %v", f.path, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release is a complete fixture release directory a file:// source serves.
type release struct {
	src   string
	ver   string
	pin   string
	files []pf
	plat  string
	bin   []byte // the one fixture tool's bytes ("docgate")
}

func newRelease(t *testing.T, files []pf) release {
	t.Helper()
	const ver = "v0.4.0"
	src := t.TempDir()
	plat := resolve.Platform()
	bin := []byte("#!/bin/sh\necho docgate fixture\n")
	archive := tarGz(t, files)

	man := relmanifest.Manifest{
		SchemaVersion:    relmanifest.ManifestSchemaVersion,
		SystemVersion:    ver,
		SourceCommit:     strings.Repeat("cd", 20),
		Binaries:         map[string]map[string]string{"docgate": {plat: hexOf(bin)}},
		Payload:          relmanifest.Payload{ArchiveSHA256: hexOf(archive)},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	// A release that generates the hosted workflow pins what it bootstraps.
	generatesWorkflow := slices.ContainsFunc(files, func(f pf) bool {
		return f.path == payload.InstalledWorkflowTarget && f.class == relmanifest.Generated
	})
	if generatesWorkflow {
		man.Binaries["pyxis"] = map[string]string{"linux-arm64": hexOf(bin), "linux-amd64": hexOf(bin)}
		writeFile(t, filepath.Join(src, "pyxis-"+ver+"-"+plat), bin, 0o644)
	}
	for _, f := range files {
		entry := relmanifest.PayloadFile{Path: f.path, Class: f.class}
		if f.class == relmanifest.EngineVerbatim {
			sum := hexOf(f.b)
			entry.SHA256 = &sum
		}
		man.Payload.Files = append(man.Payload.Files, entry)
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(src, "release-manifest-"+ver+".json"), mb, 0o644)
	writeFile(t, filepath.Join(src, "payload-"+ver+".tar.gz"), archive, 0o644)
	writeFile(t, filepath.Join(src, "docgate-"+ver+"-"+plat), bin, 0o644)
	return release{src: src, ver: ver, pin: hexOf(mb), files: files, plat: plat, bin: bin}
}

// operandConfig is the operand's own VALID v2 config, pinning this release.
func (r release) operandConfig() []byte {
	return fmt.Appendf(nil, `schema_version: pyxis-config/v2
system:
  version: %q
  manifest_sha256: %q
  artifact_source: %q
  mode: release
`, r.ver, r.pin, "file://"+r.src)
}

// newOperand builds a PRISTINE converged operand tree: repo-owned files
// present and valid, engine-verbatim files byte-exact with their modes, and
// the generated indexes exactly as the sole generator renders them.
func (r release) newOperand(t *testing.T) (string, resolve.Config) {
	t.Helper()
	root := t.TempDir()
	cfgBytes := r.operandConfig()
	for _, f := range r.files {
		switch {
		case f.path == ".pyxis/config.yaml":
			writeFile(t, filepath.Join(root, filepath.FromSlash(f.path)), cfgBytes, 0o644)
		case f.class == relmanifest.Generated:
			// written below, after the directory exists, via the generator
		default:
			mode := fs.FileMode(0o644)
			if f.exec {
				mode = 0o755
			}
			writeFile(t, filepath.Join(root, filepath.FromSlash(f.path)), f.b, mode)
		}
	}
	rc := r.config(t, root)
	dir := filepath.Join(root, filepath.FromSlash(rc.Repo.TechDesigns))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md, js, defects := gate.GenIndexBytes(root, rc.Repo)
	if len(defects) > 0 {
		t.Fatalf("fixture generator defects: %v", defects)
	}
	writeFile(t, filepath.Join(dir, "index.md"), md, 0o644)
	writeFile(t, filepath.Join(dir, "index.json"), js, 0o644)
	return root, rc
}

// config parses the operand config into the resolver operand.
func (r release) config(t *testing.T, root string) resolve.Config {
	t.Helper()
	repo, defects := config.Parse(r.operandConfig())
	if len(defects) > 0 {
		t.Fatalf("fixture config defects: %v", defects)
	}
	return resolve.Config{Root: root, Repo: repo, Logger: quiet()}
}

// find returns the fixture spec for a payload path.
func (r release) find(t *testing.T, path string) pf {
	t.Helper()
	for _, f := range r.files {
		if f.path == path {
			return f
		}
	}
	t.Fatalf("no fixture file %s", path)
	return pf{}
}

// ---- probes -------------------------------------------------------------------

// countingFetcher counts fetches around the real scheme dispatch.
type countingFetcher struct {
	mu   sync.Mutex
	urls []string
}

func (c *countingFetcher) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	c.mu.Lock()
	c.urls = append(c.urls, url)
	c.mu.Unlock()
	return resolve.ByScheme{}.Fetch(ctx, url)
}

func (c *countingFetcher) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.urls)
}

// denyFetcher fails the test on ANY fetch — the zero-external-operations probe.
type denyFetcher struct{ t *testing.T }

func (d denyFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	d.t.Error("a fetch was performed on a path contracted to perform none")
	return nil, errors.New("denied")
}

type failingFetcher struct{ err error }

func (f failingFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return nil, f.err
}

// snap captures every path under root as rel → sha256+mode (dirs as "dir",
// symlinks by their target), the zero-mutations / identical-end-state probe.
func snap(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			out[rel] = "link " + target
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out[rel] = hexOf(b) + " " + info.Mode().Perm().String()
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

func sameSnap(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s: %s changed (%s → %s)", what, k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s appeared", what, k)
		}
	}
}

func actionPaths(as []converge.Action) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Path
	}
	return out
}

func refusalPaths(rs []converge.Refusal) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Path
	}
	return out
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func at(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

// ---- the per-class drift matrix (A-4; FR-5) ----------------------------------

func TestRunRepairsDriftPerOwnershipClass(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// Inject the matrix: verbatim tampered + verbatim deleted (the executable),
	// generated stale + generated deleted, repo-owned VALIDLY edited.
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered by a malicious PR\n"), 0o644)
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, at(root, "docs/tech-design/index.json"), []byte("{\"stale\":true}\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}
	editedConfig := append(slices.Clone(r.operandConfig()), []byte("branch: main\n")...)
	writeFile(t, at(root, ".pyxis/config.yaml"), editedConfig, 0o644)
	// A stale cache entry for the janitor leg (payload grammar included).
	writeFile(t, at(root, ".pyxis/bin/docgate-v0.1.0-"+r.plat), []byte("old"), 0o755)
	writeFile(t, at(root, ".pyxis/bin/payload-v0.1.0.tar.gz"), []byte("old"), 0o644)

	rep, err := converge.Run(ctx, cfg, converge.Options{Repair: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rep.Mode != converge.ModeRepair {
		t.Errorf("mode = %q, want repair (the --repair naming)", rep.Mode)
	}
	if want := []string{".pyxis/scripts/hook.sh", ".pyxis/templates/prd-template.md"}; !slices.Equal(actionPaths(rep.Repaired), want) {
		t.Errorf("Repaired = %v, want %v", actionPaths(rep.Repaired), want)
	}
	if want := []string{"docs/tech-design/index.json", "docs/tech-design/index.md"}; !slices.Equal(actionPaths(rep.Regenerated), want) {
		t.Errorf("Regenerated = %v, want %v", actionPaths(rep.Regenerated), want)
	}
	if len(rep.RefusedByName) != 0 {
		t.Errorf("RefusedByName = %v, want none (a valid repo-owned edit is the operand's own)", rep.RefusedByName)
	}
	if want := []string{"engine-verbatim", "generated"}; !slices.Equal(rep.DriftClasses, want) {
		t.Errorf("DriftClasses = %v, want %v", rep.DriftClasses, want)
	}

	// Byte-exact repair, exec bit preserved (0755 from the archive).
	if got := mustRead(t, at(root, ".pyxis/templates/prd-template.md")); !bytes.Equal(got, r.find(t, ".pyxis/templates/prd-template.md").b) {
		t.Error("tampered engine-verbatim file not restored byte-exact")
	}
	hook, err := os.Stat(at(root, ".pyxis/scripts/hook.sh"))
	if err != nil {
		t.Fatalf("repaired hook missing: %v", err)
	}
	if hook.Mode().Perm() != 0o755 {
		t.Errorf("repaired hook mode = %v, want 0755 (the archive's exec bit)", hook.Mode().Perm())
	}
	if !bytes.Equal(mustRead(t, at(root, ".pyxis/scripts/hook.sh")), r.find(t, ".pyxis/scripts/hook.sh").b) {
		t.Error("deleted engine-verbatim file not restored byte-exact")
	}

	// Regenerated == the sole generator's output, never the packed bytes.
	md, js, defects := gate.GenIndexBytes(root, cfg.Repo)
	if len(defects) > 0 {
		t.Fatalf("generator defects: %v", defects)
	}
	if !bytes.Equal(mustRead(t, at(root, "docs/tech-design/index.md")), md) {
		t.Error("index.md not regenerated to the generator's bytes")
	}
	if !bytes.Equal(mustRead(t, at(root, "docs/tech-design/index.json")), js) {
		t.Error("index.json not regenerated to the generator's bytes")
	}

	// The valid repo-owned edit survives byte-exact (DINV-3).
	if !bytes.Equal(mustRead(t, at(root, ".pyxis/config.yaml")), editedConfig) {
		t.Error("a VALID repo-owned edit was rewritten — converge must never touch repo-owned content")
	}

	// Pre-warm + janitor orchestration (§3.4).
	if want := []string{"docgate"}; !slices.Equal(rep.PreWarmed, want) {
		t.Errorf("PreWarmed = %v, want %v", rep.PreWarmed, want)
	}
	if !bytes.Equal(mustRead(t, at(root, ".pyxis/bin/docgate-"+r.ver+"-"+r.plat)), r.bin) {
		t.Error("pre-warm did not cache the pinned binary")
	}
	if want := []string{"docgate-v0.1.0-" + r.plat, "payload-v0.1.0.tar.gz"}; !slices.Equal(rep.Reaped, want) {
		t.Errorf("Reaped = %v, want %v", rep.Reaped, want)
	}
}

// ---- repo-owned violations: named refusal, file untouched (DINV-3) -----------

func TestRepoOwnedViolationsRefusedByNameFileUntouched(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	invalid := []byte("bogus:\n  key: value\n")
	writeFile(t, at(root, ".pyxis/config.yaml"), invalid, 0o644)
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {
		t.Fatal(err)
	}
	// A verbatim tamper in the same run: converge continues the other files.
	writeFile(t, at(root, ".github/workflows/lint.yml"), []byte("tampered\n"), 0o644)

	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err == nil {
		t.Fatal("Run = nil error, want the summarizing refusal error")
	}
	for _, frag := range []string{".pyxis/config.yaml", ".pyxis/stamp/origin.json", "DINV-3"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("summarizing error lacks %q: %v", frag, err)
		}
	}
	if want := []string{".pyxis/config.yaml", ".pyxis/stamp/origin.json"}; !slices.Equal(refusalPaths(rep.RefusedByName), want) {
		t.Fatalf("RefusedByName = %v, want %v", refusalPaths(rep.RefusedByName), want)
	}
	if !strings.Contains(rep.RefusedByName[0].Reason, "closed-schema parse failed") {
		t.Errorf("config refusal reason = %q, want the named parse failure", rep.RefusedByName[0].Reason)
	}
	if !strings.Contains(rep.RefusedByName[1].Reason, "never writes repo-owned") {
		t.Errorf("stamp refusal reason = %q, want the never-writes posture", rep.RefusedByName[1].Reason)
	}
	// The invalid file is untouched, and the missing one was NOT created.
	if !bytes.Equal(mustRead(t, at(root, ".pyxis/config.yaml")), invalid) {
		t.Error("schema-invalid repo-owned file was modified")
	}
	if _, serr := os.Stat(at(root, ".pyxis/stamp/origin.json")); !errors.Is(serr, fs.ErrNotExist) {
		t.Error("missing repo-owned file was created — converge never writes repo-owned content")
	}
	// The other classes still converged.
	if want := []string{".github/workflows/lint.yml"}; !slices.Equal(actionPaths(rep.Repaired), want) {
		t.Errorf("Repaired = %v, want %v (converge continues past refusals)", actionPaths(rep.Repaired), want)
	}
	if want := []string{"engine-verbatim", "repo-owned"}; !slices.Equal(rep.DriftClasses, want) {
		t.Errorf("DriftClasses = %v, want %v", rep.DriftClasses, want)
	}
}

// ---- fresh-tree materialization + the empty-corpus generator leg -------------

func TestFreshTreeMaterializesEverythingWritable(t *testing.T) {
	r := newRelease(t, stdFiles())
	root := t.TempDir()
	cfg := r.config(t, root)
	// Only the repo-owned files exist (init's seeding turf); no docs/ at all.
	writeFile(t, at(root, ".pyxis/config.yaml"), r.operandConfig(), 0o644)
	writeFile(t, at(root, ".pyxis/stamp/origin.json"), []byte("{}\n"), 0o644)

	rep, err := converge.Run(context.Background(), cfg, converge.Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Mode != converge.ModeInstall {
		t.Errorf("mode = %q, want install", rep.Mode)
	}
	wantRepaired := []string{".github/workflows/lint.yml", ".pyxis/scripts/hook.sh", ".pyxis/templates/prd-template.md"}
	if !slices.Equal(actionPaths(rep.Repaired), wantRepaired) {
		t.Errorf("Repaired = %v, want %v", actionPaths(rep.Repaired), wantRepaired)
	}
	wantGen := []string{"docs/tech-design/index.json", "docs/tech-design/index.md"}
	if !slices.Equal(actionPaths(rep.Regenerated), wantGen) {
		t.Errorf("Regenerated = %v, want %v", actionPaths(rep.Regenerated), wantGen)
	}
	// The empty-corpus indexes equal the generator's output over the now-real dir.
	md, js, defects := gate.GenIndexBytes(root, cfg.Repo)
	if len(defects) > 0 {
		t.Fatalf("generator defects: %v", defects)
	}
	if !bytes.Equal(mustRead(t, at(root, "docs/tech-design/index.md")), md) ||
		!bytes.Equal(mustRead(t, at(root, "docs/tech-design/index.json")), js) {
		t.Error("fresh-tree indexes differ from the sole generator's output")
	}

	// And the very next run is a noop (FR-6 from a cold start).
	rep2, err := converge.Run(context.Background(), cfg, converge.Options{})
	if err != nil || rep2.Mode != converge.ModeNoop {
		t.Fatalf("second Run = (mode %q, %v), want (noop, nil)", rep2.Mode, err)
	}
}

// ---- idempotency: second run mutates nothing (FR-6, DINV-4) -------------------

func TestSecondRunIsNoopWithZeroMutationsAndZeroFetches(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("drift\n"), 0o644)
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t} // converged + warm cache: zero fetches
	rep, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if rep.Mode != converge.ModeNoop {
		t.Errorf("mode = %q, want noop", rep.Mode)
	}
	if len(rep.Repaired)+len(rep.Regenerated)+len(rep.RefusedByName)+len(rep.PreWarmed)+len(rep.Reaped) != 0 {
		t.Errorf("second run reported work: %+v", rep)
	}
	if len(rep.DriftClasses) != 0 {
		t.Errorf("DriftClasses = %v, want none", rep.DriftClasses)
	}
	sameSnap(t, before, snap(t, root), "second run")
}

// ---- kill-and-re-run: partial trees converge to the identical end state (UB-5) -

func TestKillAndRerunConvergesToIdenticalEndState(t *testing.T) {
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	// drift injects the same damage into any fresh operand.
	drift := func(t *testing.T, root string) {
		writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
		if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, at(root, "docs/tech-design/index.json"), []byte("stale\n"), 0o644)
	}

	// The reference end state: one uninterrupted converge.
	refRoot, refCfg := r.newOperand(t)
	drift(t, refRoot)
	if _, err := converge.Run(ctx, refCfg, converge.Options{}); err != nil {
		t.Fatalf("reference Run: %v", err)
	}
	want := snap(t, refRoot)

	// Partial states a kill could leave: nothing applied; one verbatim repair
	// applied; the generated pair half-applied.
	partials := map[string]func(t *testing.T, root string){
		"nothing applied": func(*testing.T, string) {},
		"one verbatim repaired": func(t *testing.T, root string) {
			writeFile(t, at(root, ".pyxis/templates/prd-template.md"), r.find(t, ".pyxis/templates/prd-template.md").b, 0o644)
		},
		"generated pair half applied": func(t *testing.T, root string) {
			md, _, defects := gate.GenIndexBytes(root, r.config(t, root).Repo)
			if len(defects) > 0 {
				t.Fatalf("generator defects: %v", defects)
			}
			writeFile(t, at(root, "docs/tech-design/index.md"), md, 0o644)
		},
	}
	for name, apply := range partials {
		t.Run(name, func(t *testing.T) {
			root, cfg := r.newOperand(t)
			drift(t, root)
			apply(t, root)
			if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			sameSnap(t, want, snap(t, root), "end state")
		})
	}
}

// ---- preview, cached leg: the exact write set, zero operations (FR-19) --------

func TestPreviewCachedLegEnumeratesExactWriteSetPerformsNothing(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// Warm the cache with one real (noop) run, then drift the tree.
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	if err := os.Remove(at(root, ".github/workflows/lint.yml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, at(root, "docs/tech-design/index.md"), []byte("stale\n"), 0o644)

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t}
	rep, err := converge.Run(ctx, cfg, converge.Options{Preview: true})
	if err != nil {
		t.Fatalf("preview Run: %v", err)
	}
	if rep.Mode != converge.ModePreview {
		t.Errorf("mode = %q, want preview", rep.Mode)
	}
	wantWrites := []string{".github/workflows/lint.yml", ".pyxis/templates/prd-template.md", "docs/tech-design/index.md"}
	if !slices.Equal(actionPaths(rep.WouldWrite), wantWrites) {
		t.Errorf("WouldWrite = %v, want %v", actionPaths(rep.WouldWrite), wantWrites)
	}
	if len(rep.WouldFetch)+len(rep.PreWarmed)+len(rep.Reaped)+len(rep.Incomplete) != 0 {
		t.Errorf("warm-cache preview reported external work: %+v", rep)
	}
	if len(rep.Repaired)+len(rep.Regenerated) != 0 {
		t.Errorf("preview reported performed writes: %+v", rep)
	}
	sameSnap(t, before, snap(t, root), "preview") // repo tree AND cache untouched

	// The preview's write set IS the subsequent real run's write set — and
	// its empty WouldFetch is the real run's zero fetches.
	counter := &countingFetcher{}
	cfg.Fetcher = counter
	real, err := converge.Run(ctx, cfg, converge.Options{})
	if err != nil {
		t.Fatalf("real Run after preview: %v", err)
	}
	performed := append(actionPaths(real.Repaired), actionPaths(real.Regenerated)...)
	slices.Sort(performed)
	if !slices.Equal(performed, wantWrites) {
		t.Errorf("real run performed %v, preview enumerated %v", performed, wantWrites)
	}
	if counter.count() != 0 {
		t.Errorf("real run fetched %d times, preview enumerated zero fetches", counter.count())
	}
}

func TestConvergePreservesUnlistedOperatorAuthority(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Repo.Knobs.Argus = true
	policy := []byte("# Operand authority\n\n| Field | Value |\n|---|---|\n| **Current tier** | tier-2 |\n| **Template version** | authority-contract/v1 |\n\n## The ladder\n\n| Tier | Bundle |\n|---|---|\n| revoked | ∅ |\n| floor | dispose-advisory-finding |\n| tier-2 | + dispose-blocking-finding · approve-merge |\n\n## Change history\n\n| Date | Move | Tier | Evidence |\n|---|---|---|---|\n| 2026-10-05 | none | floor | initialized at floor |\n| 2026-10-06 | promotion | tier-2 | actual operator-directed grant |\n")
	const authority = ".pyxis/skills/argus/authority-contract.md"
	writeFile(t, at(root, authority), policy, 0o600)
	trackAuthority(t, root)
	if defects := gate.CheckAuthorityContract(root); len(defects) != 0 {
		t.Fatalf("fixture policy invalid: %v", defects)
	}
	if _, err := converge.Run(context.Background(), cfg, converge.Options{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("drift\n"), 0o644)
	preview, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(actionPaths(preview.WouldWrite), authority) {
		t.Fatal("preview proposed replacing operator authority")
	}
	for range 2 {
		if _, err := converge.Run(context.Background(), cfg, converge.Options{Repair: true}); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(at(root, authority))
		if err != nil || !bytes.Equal(actual, policy) {
			t.Fatalf("converge changed operator policy: %q, %v", actual, err)
		}
		info, err := os.Stat(at(root, authority))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("converge changed policy mode: %v, %v", info, err)
		}
	}
}

// ---- preview, uncached leg: fetches reported, none performed (FR-19) ----------

func TestPreviewUncachedLegReportsFetchesPerformsNone(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t}
	rep, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true})
	if err != nil {
		t.Fatalf("preview Run: %v", err)
	}
	if rep.Mode != converge.ModePreview {
		t.Errorf("mode = %q, want preview", rep.Mode)
	}
	if len(rep.WouldFetch) != 1 || rep.WouldFetch[0].Artifact != "release-manifest-"+r.ver+".json" {
		t.Fatalf("WouldFetch = %v, want the release manifest", rep.WouldFetch)
	}
	if want := "file://" + r.src + "/release-manifest-" + r.ver + ".json"; rep.WouldFetch[0].URL != want {
		t.Errorf("WouldFetch URL = %q, want %q (the artifact source surfaced)", rep.WouldFetch[0].URL, want)
	}
	if len(rep.Incomplete) != 1 || !strings.Contains(rep.Incomplete[0], "payload-"+r.ver+".tar.gz") {
		t.Errorf("Incomplete = %v, want the honest computable-after-fetch note naming the payload", rep.Incomplete)
	}
	if len(rep.WouldWrite) != 0 {
		t.Errorf("WouldWrite = %v, want none — the write set is not computable without the manifest", rep.WouldWrite)
	}
	sameSnap(t, before, snap(t, root), "uncached preview")
}

func TestPreviewClassifiesGeneratedAndRepoOwnedRefusals(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	if _, err := resolve.EnsureManifest(t.Context(), cfg); err != nil {
		t.Fatalf("warm manifest: %v", err)
	}
	writeFile(t, at(root, "docs/tech-design/broken.md"), []byte("not a design\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {
		t.Fatal(err)
	}

	rep, err := converge.Run(t.Context(), cfg, converge.Options{Preview: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	classes := map[relmanifest.Class]bool{}
	for _, refusal := range rep.RefusedByName {
		classes[refusal.Class] = true
	}
	if !classes[relmanifest.Generated] || !classes[relmanifest.RepoOwned] {
		t.Fatalf("preview refusals = %+v", rep.RefusedByName)
	}
}

func TestClassifyWithArchiveSurfacesPayloadAcquisitionFailure(t *testing.T) {
	r := newRelease(t, stdFiles())
	_, cfg := r.newOperand(t)
	if _, err := resolve.EnsureManifest(t.Context(), cfg); err != nil {
		t.Fatalf("warm manifest: %v", err)
	}
	wantErr := errors.New("payload unavailable")
	cfg.Fetcher = failingFetcher{err: wantErr}
	if _, _, err := converge.ClassifyWithArchive(t.Context(), cfg); !errors.Is(err, wantErr) {
		t.Fatalf("ClassifyWithArchive error = %v", err)
	}
}

// ---- source mode: refuse by name, nothing warmed or reaped --------------------

func TestSourceModeRefusesByName(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Repo.System.Mode = "source"
	cfg.Fetcher = denyFetcher{t}

	before := snap(t, root)
	for name, call := range map[string]func() error{
		"run": func() error { _, err := converge.Run(context.Background(), cfg, converge.Options{}); return err },
		"preview": func() error {
			_, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true})
			return err
		},
		"drift": func() error { _, err := converge.Drift(context.Background(), cfg); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if !errors.Is(err, converge.ErrSourceMode) {
				t.Fatalf("err = %v, want ErrSourceMode", err)
			}
			if !strings.Contains(err.Error(), "source tree is authoritative") {
				t.Errorf("refusal not named: %v", err)
			}
		})
	}
	sameSnap(t, before, snap(t, root), "source mode") // no cache dir, no reap, no writes
	if _, err := os.Stat(at(root, ".pyxis/bin")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("source-mode refusal still created the cache dir")
	}
}

// ---- the shared drift predicate (doctor's seam, T6) ----------------------------

func TestDriftClassifiesWithoutMutating(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()

	// Warm the manifest cache so Drift is a pure read.
	if _, err := resolve.EnsureManifest(ctx, cfg); err != nil {
		t.Fatalf("EnsureManifest: %v", err)
	}

	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644) // modified
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {                   // missing
		t.Fatal(err)
	}
	writeFile(t, at(root, "docs/tech-design/index.json"), []byte("stale\n"), 0o644) // stale
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {         // repo-owned missing
		t.Fatal(err)
	}
	// A VALID repo-owned edit must NOT appear as drift.
	writeFile(t, at(root, ".pyxis/config.yaml"), append(slices.Clone(r.operandConfig()), []byte("branch: main\n")...), 0o644)

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t}
	cls, err := converge.Drift(ctx, cfg)
	if err != nil {
		t.Fatalf("Drift: %v", err)
	}
	got := map[string]converge.State{}
	for _, f := range cls.Findings {
		got[f.Path] = f.State
	}
	want := map[string]converge.State{
		".pyxis/scripts/hook.sh":           converge.StateMissing,
		".pyxis/stamp/origin.json":         converge.StateMissing,
		".pyxis/templates/prd-template.md": converge.StateModified,
		"docs/tech-design/index.json":      converge.StateStale,
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want exactly %v", got, want)
	}
	for p, s := range want {
		if got[p] != s {
			t.Errorf("finding[%s] = %q, want %q", p, got[p], s)
		}
	}
	sameSnap(t, before, snap(t, root), "drift") // read-only, cache included
}

// ---- defective release metadata: refuse per file, write nothing ---------------

func TestDefectivePayloadArchiveRefusesByName(t *testing.T) {
	// The archive omits one manifest-listed verbatim file and carries wrong
	// bytes for another; the manifest pins the archive as shipped, so the
	// acquisition verifies — the per-file re-hash catches both.
	files := stdFiles()
	archiveFiles := make([]pf, 0, len(files))
	for _, f := range files {
		switch f.path {
		case ".pyxis/templates/prd-template.md":
			continue // omitted from the archive
		case ".github/workflows/lint.yml":
			f = pf{path: f.path, b: []byte("wrong bytes in the archive\n"), class: f.class}
		}
		archiveFiles = append(archiveFiles, f)
	}
	r := newReleaseWithArchive(t, files, archiveFiles)
	root, cfg := r.newOperand(t)

	// Drift both so repair needs both from the archive.
	if err := os.Remove(at(root, ".pyxis/templates/prd-template.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(at(root, ".github/workflows/lint.yml")); err != nil {
		t.Fatal(err)
	}

	rep, err := converge.Run(context.Background(), cfg, converge.Options{})
	if err == nil || !strings.Contains(err.Error(), "refused by name") {
		t.Fatalf("Run err = %v, want the summarizing refusal", err)
	}
	want := []string{".github/workflows/lint.yml", ".pyxis/templates/prd-template.md"}
	if !slices.Equal(refusalPaths(rep.RefusedByName), want) {
		t.Fatalf("RefusedByName = %v, want %v", refusalPaths(rep.RefusedByName), want)
	}
	if !strings.Contains(rep.RefusedByName[0].Reason, "defective") || !strings.Contains(rep.RefusedByName[1].Reason, "carries no such file") {
		t.Errorf("refusal reasons not named: %+v", rep.RefusedByName)
	}
	for _, p := range want {
		if _, serr := os.Stat(at(root, p)); !errors.Is(serr, fs.ErrNotExist) {
			t.Errorf("%s was written from a defective release (fail closed violated)", p)
		}
	}
}

// newReleaseWithArchive builds a release whose ARCHIVE content diverges from
// the manifest's per-file pins — the defective-metadata fixture.
func newReleaseWithArchive(t *testing.T, manifestFiles, archiveFiles []pf) release {
	t.Helper()
	const ver = "v0.4.0"
	src := t.TempDir()
	plat := resolve.Platform()
	bin := []byte("#!/bin/sh\necho docgate fixture\n")
	archive := tarGz(t, archiveFiles)
	man := relmanifest.Manifest{
		SchemaVersion:    relmanifest.ManifestSchemaVersion,
		SystemVersion:    ver,
		SourceCommit:     strings.Repeat("cd", 20),
		Binaries:         map[string]map[string]string{"docgate": {plat: hexOf(bin)}},
		Payload:          relmanifest.Payload{ArchiveSHA256: hexOf(archive)},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	for _, f := range manifestFiles {
		entry := relmanifest.PayloadFile{Path: f.path, Class: f.class}
		if f.class == relmanifest.EngineVerbatim {
			sum := hexOf(f.b)
			entry.SHA256 = &sum
		}
		man.Payload.Files = append(man.Payload.Files, entry)
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	writeFile(t, filepath.Join(src, "release-manifest-"+ver+".json"), mb, 0o644)
	writeFile(t, filepath.Join(src, "payload-"+ver+".tar.gz"), archive, 0o644)
	writeFile(t, filepath.Join(src, "docgate-"+ver+"-"+plat), bin, 0o644)
	return release{src: src, ver: ver, pin: hexOf(mb), files: manifestFiles, plat: plat, bin: bin}
}

// ---- ungeneratable + unregistered generated paths ------------------------------

func TestGeneratedRefusalsUngeneratableAndUnregistered(t *testing.T) {
	files := append(stdFiles(), pf{path: "docs/other/rollup.txt", b: []byte("x\n"), class: relmanifest.Generated})
	r := newRelease(t, files)
	root, cfg := r.newOperand(t)

	// A broken design makes the index corpus ungeneratable.
	writeFile(t, at(root, "docs/tech-design/broken.md"), []byte("not a design\n"), 0o644)
	if err := os.Remove(at(root, "docs/tech-design/index.md")); err != nil {
		t.Fatal(err)
	}

	rep, err := converge.Run(context.Background(), cfg, converge.Options{})
	if err == nil {
		t.Fatal("Run = nil error, want the summarizing refusal")
	}
	reasons := map[string]string{}
	for _, ref := range rep.RefusedByName {
		reasons[ref.Path] = ref.Reason
	}
	if !strings.Contains(reasons["docs/other/rollup.txt"], "no generator is registered") {
		t.Errorf("unregistered generated path not refused by name: %q", reasons["docs/other/rollup.txt"])
	}
	for _, p := range []string{"docs/tech-design/index.md", "docs/tech-design/index.json"} {
		if !strings.Contains(reasons[p], "generator refuses the design corpus") {
			t.Errorf("%s: reason = %q, want the ungeneratable naming", p, reasons[p])
		}
	}
	if _, serr := os.Stat(at(root, "docs/tech-design/index.md")); !errors.Is(serr, fs.ErrNotExist) {
		t.Error("an ungeneratable file was written")
	}
}

// ---- the EVT-2 event shape ------------------------------------------------------

func TestConvergeRunEventShape(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	var buf bytes.Buffer
	cfg.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	rep, err := converge.Run(context.Background(), cfg, converge.Options{Repair: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var evt map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == "converge_run" {
			evt = m
		}
	}
	if evt == nil {
		t.Fatalf("no converge_run event in:\n%s", buf.String())
	}
	if evt["mode"] != "repair" {
		t.Errorf("event mode = %v, want repair", evt["mode"])
	}
	if evt["repaired"] != float64(1) {
		t.Errorf("event repaired = %v, want 1", evt["repaired"])
	}
	dc, _ := evt["drift_classes"].([]any)
	if len(dc) != 1 || dc[0] != "engine-verbatim" {
		t.Errorf("event drift_classes = %v, want [engine-verbatim]", evt["drift_classes"])
	}
	if evt["run_id"] != rep.RunID || rep.RunID == "" {
		t.Errorf("event run_id = %v, report run_id = %q — one id on every event", evt["run_id"], rep.RunID)
	}
	if evt["artifact_source"] != "file://"+r.src {
		t.Errorf("artifact_source = %v, want the override surfaced loudly", evt["artifact_source"])
	}

	// Preview emits the same event with mode preview.
	buf.Reset()
	if _, err := converge.Run(context.Background(), cfg, converge.Options{Preview: true}); err != nil {
		t.Fatalf("preview Run: %v", err)
	}
	if !strings.Contains(buf.String(), `"msg":"converge_run"`) || !strings.Contains(buf.String(), `"mode":"preview"`) {
		t.Errorf("preview event missing:\n%s", buf.String())
	}
}

// ---- guards ---------------------------------------------------------------------

func TestRunGuards(t *testing.T) {
	if _, err := converge.Run(context.Background(), resolve.Config{Logger: quiet()}, converge.Options{}); err == nil || !strings.Contains(err.Error(), "Root is empty") {
		t.Errorf("empty root err = %v, want the named guard", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := converge.Run(ctx, resolve.Config{Root: "x", Logger: quiet()}, converge.Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("dead-context err = %v, want context.Canceled", err)
	}
}
