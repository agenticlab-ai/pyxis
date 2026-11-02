package diffset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// initRepo creates a hermetic git repo at path with an isolated identity, so
// the host environment can never leak in (mirrors invoke_test.go's helper).
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitOK(t, dir, "init", "-q", "-b", "main")
	return dir
}

func gitOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "-m", msg)
	return strings.TrimSpace(gitOK(t, dir, "rev-parse", "HEAD"))
}

func findFile(t *testing.T, ds *DiffSet, path string) *FileChange {
	t.Helper()
	for i := range ds.Files {
		if ds.Files[i].Path == path {
			return &ds.Files[i]
		}
	}
	t.Fatalf("path %q not found in diffset; files: %+v", path, ds.Files)
	return nil
}

func TestExtract_AddedFileLineNumbers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "line1\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "new.txt", "alpha\nbeta\ngamma\n")
	head := commit(t, dir, "add new.txt")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	f := findFile(t, ds, "new.txt")
	if f.Status != StatusAdded {
		t.Errorf("status = %q, want added", f.Status)
	}
	want := []AddedLine{{Line: 1, Content: "alpha"}, {Line: 2, Content: "beta"}, {Line: 3, Content: "gamma"}}
	if len(f.AddedLines) != len(want) {
		t.Fatalf("added lines = %+v, want %+v", f.AddedLines, want)
	}
	for i, w := range want {
		if f.AddedLines[i] != w {
			t.Errorf("added line %d = %+v, want %+v", i, f.AddedLines[i], w)
		}
	}
	if f.SizeBytes != int64(len("alpha\nbeta\ngamma\n")) {
		t.Errorf("size = %d, want %d", f.SizeBytes, len("alpha\nbeta\ngamma\n"))
	}
}

func TestExtract_ModifiedFileOnlyAddedLinesCounted(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "app.go", "package app\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	base := commit(t, dir, "base")

	// Replace B with two new functions, leaving A and C untouched — the
	// removed line must not shift the added lines' new-file numbering.
	writeFile(t, dir, "app.go", "package app\n\nfunc A() {}\nfunc B2() {}\nfunc B3() {}\nfunc C() {}\n")
	head := commit(t, dir, "swap B")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "app.go")
	if f.Status != StatusModified {
		t.Errorf("status = %q, want modified", f.Status)
	}
	if len(f.AddedLines) != 2 {
		t.Fatalf("added lines = %+v, want 2 entries", f.AddedLines)
	}
	if f.AddedLines[0].Content != "func B2() {}" || f.AddedLines[0].Line != 4 {
		t.Errorf("added[0] = %+v, want {4 func B2() {}}", f.AddedLines[0])
	}
	if f.AddedLines[1].Content != "func B3() {}" || f.AddedLines[1].Line != 5 {
		t.Errorf("added[1] = %+v, want {5 func B3() {}}", f.AddedLines[1])
	}
}

func TestExtract_PreExistingContentNotInDiff(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "secrets.txt", "password: hunter2\n")
	writeFile(t, dir, "other.txt", "irrelevant\n")
	base := commit(t, dir, "base with a pre-existing secret")

	// Touch an unrelated file only; secrets.txt is untouched by this diff.
	writeFile(t, dir, "other.txt", "irrelevant\nmore\n")
	head := commit(t, dir, "unrelated change")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, f := range ds.Files {
		if f.Path == "secrets.txt" {
			t.Fatalf("pre-existing file outside the diff must not appear: %+v", f)
		}
	}
	if len(ds.Files) != 1 || ds.Files[0].Path != "other.txt" {
		t.Fatalf("expected only other.txt in the diff, got %+v", ds.Files)
	}
}

func TestExtract_DeletedFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "gone.txt", "bye\n")
	base := commit(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "delete gone.txt")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "gone.txt")
	if f.Status != StatusDeleted {
		t.Errorf("status = %q, want deleted", f.Status)
	}
	if len(f.AddedLines) != 0 || f.SizeBytes != 0 || f.Binary {
		t.Errorf("deleted file should carry no content fields, got %+v", f)
	}
	if f.ContentIntroduced() {
		t.Error("a deletion must never report ContentIntroduced")
	}
}

// TestExtract_ModeOnlyChangeNoContentChange proves a content-preserving
// in-place modify (e.g. chmod +x, reported by git as status M with zero
// hunks and no binary marker) reports no introduced content — the same
// promise TestExtract_PureRenameNoContentChange makes for renames. Without
// this, a diff that only flips a permission bit on a pre-existing file would
// re-trip forbidden-artifacts' floors for content the diff never touched.
func TestExtract_ModeOnlyChangeNoContentChange(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "script.sh", "#!/bin/bash\necho hi\n")
	base := commit(t, dir, "base")

	if err := os.Chmod(filepath.Join(dir, "script.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "chmod +x, no content change")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "script.sh")
	if f.Status != StatusModified {
		t.Fatalf("status = %q, want modified (git may not have detected a mode-only change)", f.Status)
	}
	if len(f.AddedLines) != 0 || f.Binary {
		t.Errorf("a mode-only change must carry no content fields, got %+v", f)
	}
	if f.ContentIntroduced() {
		t.Error("a mode-only change must not report ContentIntroduced")
	}
}

func TestExtract_PureRenameNoContentChange(t *testing.T) {
	dir := initRepo(t)
	content := "identical content across the rename, long enough for -M to detect it as a rename rather than an unrelated add+delete pair\n"
	writeFile(t, dir, "old/name.txt", content)
	base := commit(t, dir, "base")

	// Move via the OS (git mv requires the destination dir to pre-exist);
	// -M detects the rename at diff time from content similarity, not from
	// how the working-tree change was made.
	if err := os.Remove(filepath.Join(dir, "old/name.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "new/name.txt", content)
	head := commit(t, dir, "pure rename")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "new/name.txt")
	if f.Status != StatusRenamed {
		t.Fatalf("status = %q, want renamed (git may not have detected the rename)", f.Status)
	}
	if f.OldPath != "old/name.txt" {
		t.Errorf("old path = %q, want old/name.txt", f.OldPath)
	}
	if len(f.AddedLines) != 0 || f.Binary {
		t.Errorf("a pure rename must carry no added content, got %+v", f)
	}
	if f.ContentIntroduced() {
		t.Error("a pure rename must not report ContentIntroduced")
	}
}

func TestExtract_RenameWithContentChange(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "old/name.txt", "line one\nline two\nline three\nline four\nline five\n")
	base := commit(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "old/name.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "new/name.txt", "line one\nline two\nline three\nline four\nline five\nline six\n")
	head := commit(t, dir, "rename and extend")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "new/name.txt")
	if f.Status != StatusRenamed {
		t.Fatalf("status = %q, want renamed", f.Status)
	}
	if len(f.AddedLines) != 1 || f.AddedLines[0].Content != "line six" || f.AddedLines[0].Line != 6 {
		t.Errorf("added lines = %+v, want a single 'line six' at line 6", f.AddedLines)
	}
	if !f.ContentIntroduced() {
		t.Error("a content-changing rename must report ContentIntroduced")
	}
}

func TestExtract_BinaryFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "text\n")
	base := commit(t, dir, "base")

	binPath := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 'P', 'N', 'G', 0x00, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "add binary")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "blob.bin")
	if !f.Binary {
		t.Error("expected Binary=true for a NUL-containing file")
	}
	if len(f.AddedLines) != 0 {
		t.Errorf("a binary file must carry no added lines, got %+v", f.AddedLines)
	}
	if f.SizeBytes != 8 {
		t.Errorf("size = %d, want 8", f.SizeBytes)
	}
	if !f.ContentIntroduced() {
		t.Error("a newly added binary file must report ContentIntroduced")
	}
}

// TestExtract_GitattributesDiffOverrideDoesNotHideTextContent proves a
// `path -diff` .gitattributes entry — which makes git itself print "Binary
// files ... differ" for genuinely textual content — cannot blind Extract to
// that content. Without --text (-a) forcing git's textual diff algorithm,
// this file's AddedLines would come back empty and Binary=true, exactly as
// for real binary content — an attribute-driven blind spot in a
// diff-scoped anti-gaming scanner.
func TestExtract_GitattributesDiffOverrideDoesNotHideTextContent(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "text\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, ".gitattributes", "secret.py -diff\n")
	writeFile(t, dir, "secret.py", "api_key: sk-live-abcdef0123456789\n")
	head := commit(t, dir, "add secret.py marked -diff")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "secret.py")
	if f.Binary {
		t.Error("a -diff attribute must not make genuinely textual content report Binary=true")
	}
	if len(f.AddedLines) != 1 || f.AddedLines[0].Content != "api_key: sk-live-abcdef0123456789" {
		t.Errorf("added lines = %+v, want the secret line captured despite the -diff attribute", f.AddedLines)
	}
}

// TestExtract_GitattributesDiffOverrideStillDetectsRealBinary proves the fix
// above is not a blanket "always treat as text": genuinely binary content
// (well past any embedded-newline edge case) is still classified Binary,
// even under the same -diff attribute and even under --text forcing.
func TestExtract_GitattributesDiffOverrideStillDetectsRealBinary(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "text\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, ".gitattributes", "blob.bin -diff\n")
	raw := make([]byte, 4096)
	for i := range raw {
		raw[i] = byte(i % 256) // deterministic, includes NUL bytes and embedded 0x0A
	}
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "add real binary marked -diff")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "blob.bin")
	if !f.Binary {
		t.Error("genuinely binary content must still report Binary=true")
	}
	if len(f.AddedLines) != 0 {
		t.Errorf("a binary file must carry no added lines, got %d entries", len(f.AddedLines))
	}
}

func TestExtract_DoublePlusPrefixedLineCaptured(t *testing.T) {
	// An added line whose own text starts with "++" is rendered "+++…" in the
	// hunk body; it must NOT be dropped as a "+++ b/file" header — that would be
	// a silent secret-scan bypass triggered by typing "++" before the secret.
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "hello\n")
	base := commit(t, dir, "base")
	writeFile(t, dir, "config.txt", "++api_key = sk-live-abcdef0123456789\n")
	head := commit(t, dir, "add a ++-prefixed secret line")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "config.txt")
	if len(f.AddedLines) != 1 || f.AddedLines[0].Content != "++api_key = sk-live-abcdef0123456789" {
		t.Errorf("a ++-prefixed added line was dropped or altered: %+v", f.AddedLines)
	}
}

func TestExtract_MultipleHunks(t *testing.T) {
	dir := initRepo(t)
	lines := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		lines = append(lines, "orig")
	}
	writeFile(t, dir, "big.txt", strings.Join(lines, "\n")+"\n")
	base := commit(t, dir, "base")

	lines[2] = "changed-top"     // 1-based line 3
	lines[17] = "changed-bottom" // 1-based line 18
	writeFile(t, dir, "big.txt", strings.Join(lines, "\n")+"\n")
	head := commit(t, dir, "two separate hunks")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "big.txt")
	if len(f.AddedLines) != 2 {
		t.Fatalf("added lines = %+v, want exactly 2 (one per hunk)", f.AddedLines)
	}
	if f.AddedLines[0].Line != 3 || f.AddedLines[0].Content != "changed-top" {
		t.Errorf("hunk 1 = %+v, want {3 changed-top}", f.AddedLines[0])
	}
	if f.AddedLines[1].Line != 18 || f.AddedLines[1].Content != "changed-bottom" {
		t.Errorf("hunk 2 = %+v, want {18 changed-bottom}", f.AddedLines[1])
	}
}

func TestExtract_FilenameWithSpaces(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "text\n")
	base := commit(t, dir, "base")

	writeFile(t, dir, "a file with spaces.txt", "hello world\n")
	head := commit(t, dir, "add spaced filename")

	ds, err := Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	f := findFile(t, ds, "a file with spaces.txt")
	if len(f.AddedLines) != 1 || f.AddedLines[0].Content != "hello world" {
		t.Errorf("added lines = %+v, want a single 'hello world'", f.AddedLines)
	}
}

func TestTouchedPaths(t *testing.T) {
	ds := &DiffSet{Files: []FileChange{
		{Status: StatusAdded, Path: "a.txt"},
		{Status: StatusDeleted, Path: "b.txt"},
		{Status: StatusRenamed, OldPath: "c-old.txt", Path: "c-new.txt"},
		{Status: StatusModified, Path: "a.txt"}, // duplicate path, must dedup
	}}
	got := ds.TouchedPaths()
	want := map[string]bool{"a.txt": true, "b.txt": true, "c-old.txt": true, "c-new.txt": true}
	if len(got) != len(want) {
		t.Fatalf("TouchedPaths() = %v, want %d unique entries", got, len(want))
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected path %q", p)
		}
	}
}

func TestExtract_UnknownBaseErrors(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	head := commit(t, dir, "base")

	if _, err := Extract(context.Background(), dir, "0000000000000000000000000000000000000000", head); err == nil {
		t.Error("expected an error diffing against a nonexistent base")
	}
}

func TestPopulateContent_BoundsConcurrencyAndPreservesOrder(t *testing.T) {
	files := make([]FileChange, maxConcurrentFileDiffs*2+3)
	for i := range files {
		files[i] = FileChange{Status: StatusAdded, Path: fmt.Sprintf("file-%02d", i)}
	}
	files[2].Status = StatusDeleted
	files[3] = FileChange{Status: StatusRenamed, OldPath: "old-name", Path: "new-name"}

	started := make(chan struct{}, len(files))
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseAll)

	var active atomic.Int32
	var peak atomic.Int32
	var calls atomic.Int32
	var pathsMu sync.Mutex
	pathspecsByPath := make(map[string][]string)
	diff := func(ctx context.Context, _, _, _ string, pathspecs []string) (bool, []AddedLine, error) {
		calls.Add(1)
		now := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
		}

		path := pathspecs[len(pathspecs)-1]
		pathsMu.Lock()
		pathspecsByPath[path] = append([]string(nil), pathspecs...)
		pathsMu.Unlock()
		started <- struct{}{}
		select {
		case <-release:
			return false, []AddedLine{{Line: 1, Content: path}}, nil
		case <-ctx.Done():
			return false, nil, ctx.Err()
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- populateContent(t.Context(), "workspace", "base", "head", files, diff)
	}()
	for range maxConcurrentFileDiffs {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("worker pool did not fill")
		}
	}
	select {
	case <-started:
		t.Fatalf("more than %d content diffs ran concurrently", maxConcurrentFileDiffs)
	default:
	}
	releaseAll()
	if err := <-done; err != nil {
		t.Fatalf("populateContent: %v", err)
	}

	if got := peak.Load(); got != maxConcurrentFileDiffs {
		t.Fatalf("peak concurrency = %d, want %d", got, maxConcurrentFileDiffs)
	}
	if got, want := calls.Load(), int32(len(files)-1); got != want {
		t.Fatalf("content diff calls = %d, want %d", got, want)
	}
	if len(files[2].AddedLines) != 0 {
		t.Fatalf("deleted file was populated: %+v", files[2])
	}
	for i := range files {
		if files[i].Status == StatusDeleted {
			continue
		}
		if got := files[i].AddedLines; len(got) != 1 || got[0].Content != files[i].Path {
			t.Errorf("file %d result = %+v, want content %q", i, got, files[i].Path)
		}
	}
	if got := pathspecsByPath["new-name"]; len(got) != 2 || got[0] != "old-name" || got[1] != "new-name" {
		t.Errorf("rename pathspecs = %v, want [old-name new-name]", got)
	}
}

func TestPopulateContent_FirstErrorCancelsAndJoinsPool(t *testing.T) {
	files := make([]FileChange, maxConcurrentFileDiffs*2)
	for i := range files {
		files[i] = FileChange{Status: StatusAdded, Path: fmt.Sprintf("file-%02d", i)}
	}

	wantErr := errors.New("injected content-diff failure")
	allStarted := make(chan struct{})
	var closeOnce sync.Once
	var calls atomic.Int32
	diff := func(ctx context.Context, _, _, _ string, pathspecs []string) (bool, []AddedLine, error) {
		if calls.Add(1) == maxConcurrentFileDiffs {
			closeOnce.Do(func() { close(allStarted) })
		}
		if pathspecs[0] == "file-00" {
			<-allStarted
			return false, nil, wantErr
		}
		<-ctx.Done()
		return false, nil, ctx.Err()
	}

	err := populateContent(t.Context(), "workspace", "base", "head", files, diff)
	if !errors.Is(err, wantErr) {
		t.Fatalf("populateContent error = %v, want injected error", err)
	}
	if !strings.Contains(err.Error(), `path "file-00"`) {
		t.Errorf("error %q does not identify the failing path", err)
	}
	if got := calls.Load(); got != maxConcurrentFileDiffs {
		t.Fatalf("content diff calls after cancellation = %d, want %d", got, maxConcurrentFileDiffs)
	}
}
