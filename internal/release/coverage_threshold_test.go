package release

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type coverageGHResult struct {
	stdout string
	stderr string
	code   int
	err    error
}

type coverageGHRunner struct {
	results []coverageGHResult
	next    int
}

func (r *coverageGHRunner) RunGH(context.Context, ...string) (string, string, int, error) {
	if r.next >= len(r.results) {
		return "", "", 0, errors.New("unexpected gh call")
	}
	result := r.results[r.next]
	r.next++
	return result.stdout, result.stderr, result.code, result.err
}

func TestCheckPinnedVersionBranches(t *testing.T) {
	if err := checkPinnedVersion(nil, "v1.2.3"); err == nil {
		t.Fatal("accepted a missing PINNED_VERSION")
	}
	if err := checkPinnedVersion([]byte("PINNED_VERSION=\"v1.2.2\"\n"), "v1.2.3"); err == nil {
		t.Fatal("accepted a mismatched PINNED_VERSION")
	}
	if err := checkPinnedVersion([]byte("PINNED_VERSION=\"v1.2.3\"\n"), "v1.2.3"); err != nil {
		t.Fatalf("matching PINNED_VERSION: %v", err)
	}
}

func TestCheckWorkflowToolchainFreeRejectsSourceTooling(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(overlayDstDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs.yml"), []byte("uses: actions/setup-go@v6\nrun: go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckWorkflowToolchainFree(root)
	if err == nil || !strings.Contains(err.Error(), "source-toolchain") {
		t.Fatalf("CheckWorkflowToolchainFree error = %v", err)
	}
}

func TestTarballRefusesDuplicateBlockedAndInvalidEntries(t *testing.T) {
	out := filepath.Join(t.TempDir(), "archive.tgz")
	if err := writeDeterministicTarGz(out, []tarEntry{{name: "same", data: []byte("a")}, {name: "same", data: []byte("b")}}); err == nil {
		t.Fatal("duplicate archive member was accepted")
	}
	if err := writeDeterministicTarGz(filepath.Join(t.TempDir(), "missing", "archive.tgz"), nil); err == nil {
		t.Fatal("blocked archive output was accepted")
	}
	if err := writeMember(tar.NewWriter(io.Discard), tarEntry{name: "missing", src: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing source member was accepted")
	}

	closed := tar.NewWriter(io.Discard)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writeMember(closed, tarEntry{name: "literal", data: []byte("data")}); err == nil {
		t.Fatal("member write through closed tar was accepted")
	}
	source := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(source, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMember(closed, tarEntry{name: "source", src: source}); err == nil {
		t.Fatal("source header through closed tar was accepted")
	}
	if err := os.Chmod(source, 0o000); err != nil {
		t.Fatal(err)
	}
	if err := writeDeterministicTarGz(out, []tarEntry{{name: "source", src: source}}); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "open source") {
		t.Fatalf("unreadable archive member must fail with its source name: %v", err)
	}
}

func TestTreeEntriesPreservesPathsAndRejectsNonRegularFiles(t *testing.T) {
	if _, err := treeEntries(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing tree root was accepted")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kept"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := treeEntries(root)
	if err != nil || len(entries) != 2 || entries[0].name != "kept" || !entries[0].exec || entries[1].name != "nested/file" || entries[1].exec {
		t.Fatalf("tree entries = %+v, %v", entries, err)
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink("kept", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := treeEntries(root); err == nil {
		t.Fatal("symlink archive member was accepted")
	}
}

func TestStagePayloadAndHelpersRefuseInvalidFilesystemShapes(t *testing.T) {
	if _, err := StagePayload(t.Context(), filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
		t.Fatal("missing repository was accepted")
	}
	emptyRepo := t.TempDir()
	if out, err := exec.Command("git", "-C", emptyRepo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init empty repository: %v: %s", err, out)
	}
	if _, err := StagePayload(t.Context(), emptyRepo, t.TempDir()); err == nil || !strings.Contains(err.Error(), "no tracked payload") {
		t.Fatalf("empty repository error = %v", err)
	}

	if mapped, err := overlayMap(t.TempDir()); err != nil || mapped != nil {
		t.Fatalf("missing overlay = %v, %v", mapped, err)
	}
	notDirRoot := t.TempDir()
	notDir := filepath.Join(notDirRoot, filepath.FromSlash(overlaySrcDir))
	if err := os.MkdirAll(filepath.Dir(notDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notDir, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := overlayMap(notDirRoot); err == nil {
		t.Fatal("non-directory overlay home was accepted")
	}
	directoryRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directoryRoot, filepath.FromSlash(overlaySrcDir), "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := overlayMap(directoryRoot); err == nil || !strings.Contains(err.Error(), "contains a directory") {
		t.Fatalf("nested overlay error = %v", err)
	}

	if err := copyStaged(t.TempDir(), t.TempDir(), "missing", "out"); err == nil {
		t.Fatal("missing staged source was accepted")
	}
	sourceRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(sourceRoot, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyStaged(sourceRoot, t.TempDir(), "directory", "out"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory source error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "source"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	destinationRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(destinationRoot, "blocked"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyStaged(sourceRoot, destinationRoot, "source", "blocked/out"); err == nil {
		t.Fatal("blocked staged destination was accepted")
	}
	if _, err := sourceCommit(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing source repository produced a commit")
	}
}

func TestPackAndArtifactHelpersRefuseIncompleteInputs(t *testing.T) {
	valid := PackOptions{Version: "v1.2.3"}
	if _, err := Pack(t.Context(), valid); err == nil || !strings.Contains(err.Error(), "--out") {
		t.Fatalf("missing out error = %v", err)
	}
	valid.OutDir = t.TempDir()
	if _, err := Pack(t.Context(), valid); err == nil || !strings.Contains(err.Error(), "--binaries") {
		t.Fatalf("missing binaries error = %v", err)
	}
	valid.RepoRoot = t.TempDir()
	valid.BinariesDir = t.TempDir()
	if _, err := Pack(t.Context(), valid); err == nil || !strings.Contains(err.Error(), "class policy") {
		t.Fatalf("missing class policy error = %v", err)
	}
	if _, _, err := collectBinaries(filepath.Join(t.TempDir(), "missing"), "v1.2.3", []string{"pyxis"}); err == nil {
		t.Fatal("missing binaries directory was accepted")
	}
	binaries := t.TempDir()
	if err := os.Mkdir(filepath.Join(binaries, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := collectBinaries(binaries, "v1.2.3", []string{"pyxis"}); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory binary error = %v", err)
	}

	artifact := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(artifact, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyArtifact(artifact, artifact); err != nil {
		t.Fatalf("same artifact copy: %v", err)
	}
	if err := copyArtifact(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("missing artifact source was accepted")
	}
	if err := copyArtifact(artifact, t.TempDir()); err == nil {
		t.Fatal("directory artifact destination was accepted")
	}
	if _, err := writeSums(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing sums directory was accepted")
	}
	sumsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sumsDir, SumsName), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sumsDir, "artifact"), []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if names, err := writeSums(sumsDir); err != nil || len(names) != 2 {
		t.Fatalf("rewrite sums = %v, %v", names, err)
	}
}

func TestMirrorValidatesClosedInputsBeforeIO(t *testing.T) {
	runner := &coverageGHRunner{}
	cases := []MirrorOptions{
		{},
		{Runner: runner},
		{Runner: runner, Tag: "v1.2.3", SourceRepo: "owner/repo"},
	}
	for _, opts := range cases {
		if _, err := Mirror(t.Context(), opts); err == nil {
			t.Errorf("Mirror(%+v) succeeded", opts)
		}
	}
}

func TestCutAndRemoteHelpersNameBoundaryFailures(t *testing.T) {
	if _, err := Cut(t.Context(), CutOptions{Version: "not-a-version"}); err == nil {
		t.Fatal("invalid cut version was accepted")
	}
	if _, err := Cut(t.Context(), CutOptions{Version: "v1.2.3"}); err == nil || !strings.Contains(err.Error(), "runner") {
		t.Fatalf("missing runner error = %v", err)
	}
	runner := &coverageGHRunner{}
	if _, err := Cut(t.Context(), CutOptions{Version: "v1.2.3", Runner: runner}); err == nil || !strings.Contains(err.Error(), "source repo") {
		t.Fatalf("missing source repo error = %v", err)
	}
	t.Run("default root reaches pack refusal", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := Cut(t.Context(), CutOptions{
			Version: "v1.2.3", SourceRepo: "owner/repo", Runner: runner,
			OutDir: t.TempDir(), BinariesDir: t.TempDir(),
		})
		if err == nil || !strings.Contains(err.Error(), "class policy") {
			t.Fatalf("default-root pack error = %v", err)
		}
	})
	t.Run("default builder fails closed", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := Cut(t.Context(), CutOptions{
			Version: "v1.2.3", SourceRepo: "owner/repo", Runner: runner, OutDir: t.TempDir(),
		})
		if err == nil {
			t.Fatal("cut without a buildable repository succeeded")
		}
	})

	out := t.TempDir()
	if _, err := packedSourceCommit(out, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("missing manifest error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, manifestName("v1.2.3")), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packedSourceCommit(out, "v1.2.3"); err == nil || !strings.Contains(err.Error(), "release-manifest") {
		t.Fatalf("malformed manifest error = %v", err)
	}

	transport := errors.New("transport")
	for _, tc := range []struct {
		name   string
		result coverageGHResult
	}{
		{name: "transport", result: coverageGHResult{err: transport}},
		{name: "api failure", result: coverageGHResult{stderr: "forbidden", code: 1}},
		{name: "malformed response", result: coverageGHResult{stdout: "{"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &coverageGHRunner{results: []coverageGHResult{tc.result}}
			if _, err := remoteTagCommit(t.Context(), r, "owner/repo", "v1.2.3"); err == nil {
				t.Fatal("remote tag boundary failure was accepted")
			}
		})
	}
	annotated := &coverageGHRunner{results: []coverageGHResult{
		{stdout: `{"object":{"sha":"abc","type":"tag"}}`},
		{stderr: "not found", code: 1},
	}}
	if _, err := remoteTagCommit(t.Context(), annotated, "owner/repo", "v1.2.3"); err == nil || !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("missing annotated object error = %v", err)
	}
	annotatedTransport := &coverageGHRunner{results: []coverageGHResult{
		{stdout: `{"object":{"sha":"abc","type":"tag"}}`},
		{err: transport},
	}}
	if _, err := remoteTagCommit(t.Context(), annotatedTransport, "owner/repo", "v1.2.3"); !errors.Is(err, transport) {
		t.Fatalf("annotated transport error = %v", err)
	}
	createFailure := &coverageGHRunner{results: []coverageGHResult{{stderr: "forbidden", code: 1}}}
	if err := createRemoteTag(t.Context(), createFailure, "owner/repo", "v1.2.3", strings.Repeat("a", 40)); err == nil {
		t.Fatal("remote tag create failure was accepted")
	}
	if err := requireCommittedPins(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repository passed the committed-pins gate")
	}
}
