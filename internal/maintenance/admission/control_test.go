package admission

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func controlGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func treeDigest(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path) // WalkDir guarantees path is below root.
		result = append(result, fmt.Sprintf("%s %x", rel, sha256.Sum256(data)))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func TestControlStorageFetchesPublishedCommitWithoutEnrolledGitMutation(t *testing.T) {
	remote := filepath.Join(t.TempDir(), "remote.git")
	if output, err := exec.Command("git", "init", "--bare", "--quiet", remote).CombinedOutput(); err != nil {
		t.Fatalf("init remote: %v: %s", err, output)
	}
	repository := filepath.Join(t.TempDir(), "enrolled")
	if output, err := exec.Command("git", "init", "--quiet", "-b", "main", repository).CombinedOutput(); err != nil {
		t.Fatalf("init enrolled: %v: %s", err, output)
	}
	controlGit(t, repository, "remote", "add", "origin", remote)
	document := testDocument(testJob("codex"))
	policy, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{".pyxis/maintenance/jobs.yaml": policy, ".pyxis/config.yaml": []byte(harnessconfigtest.YAML)} {
		path := filepath.Join(repository, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	controlGit(t, repository, "add", ".pyxis/config.yaml", ".pyxis/maintenance/jobs.yaml")
	controlGit(t, repository, "commit", "-m", "published")
	controlGit(t, repository, "push", "origin", "main")
	publishedHead := controlGit(t, repository, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repository, ".pyxis/maintenance/jobs.yaml"), []byte("unpublished local prompt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".pyxis/config.yaml"), []byte("unpublished: catalog"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "untracked.txt"), []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	controlGit(t, repository, "config", "maintenance.fixture", "preserve")
	if err := os.WriteFile(filepath.Join(repository, ".git", "FETCH_HEAD"), []byte("preserve-fetch-head\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	common := controlGit(t, repository, "rev-parse", "--path-format=absolute", "--git-common-dir")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	relativeRemote, err := filepath.Rel(repository, remote)
	if err != nil {
		t.Fatal(err)
	}
	source := ControlSource{Repository: repository, GitCommon: common, ResolveRemote: func(_ context.Context, remoteName string) (string, error) {
		if remoteName != "origin" {
			t.Fatalf("resolved remote %q, want origin", remoteName)
		}
		return relativeRemote, nil
	}}
	before := treeDigest(t, repository)
	published, err := source.Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	if published.Commit != publishedHead || !reflect.DeepEqual(published.Jobs, policy) || string(published.Config) != harnessconfigtest.YAML {
		t.Fatalf("published snapshot commit=%s jobs=%q", published.Commit, published.Jobs)
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("successful capture changed enrolled tree\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := published.Control.RefStore("refs/pyxis/maintenance-test"); err != nil {
		t.Fatalf("open retained control ref store: %v", err)
	}
	firstStore, err := published.Control.RefStore("refs/pyxis/maintenance/daily")
	if err != nil {
		t.Fatal(err)
	}
	const occurrence = "occurrences/2026-10-20.json"
	if _, err := firstStore.Update(t.Context(), occurrence, "fixture occurrence", func(current []byte) ([]byte, bool, error) {
		if len(current) != 0 {
			t.Fatalf("unexpected existing occurrence: %q", current)
		}
		return []byte(`{"job_id":"daily","job_date":"2026-10-20"}`), true, nil
	}); err != nil {
		t.Fatal(err)
	}
	fileRemote := (&url.URL{Scheme: "file", Path: remote}).String()
	other, err := (ControlSource{Repository: repository, GitCommon: common, RemoteURL: fileRemote}).Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Control.Close() }()
	secondStore, err := other.Control.RefStore("refs/pyxis/maintenance/daily")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := secondStore.Read(t.Context(), occurrence); err != nil || string(got) != `{"job_id":"daily","job_date":"2026-10-20"}` {
		t.Fatalf("shared remote occurrence = %q, %v", got, err)
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("remote occurrence operations changed enrolled tree\nbefore=%v\nafter=%v", before, after)
	}
	if err := published.Control.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := published.Control.RefStore("refs/pyxis/maintenance-test"); err == nil {
		t.Fatal("closed control returned a ref store")
	}
	if err := published.Control.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	before = treeDigest(t, repository)
	_, err = source.Capture(t.Context(), jobs.SourceBinding{Remote: "origin", Branch: "missing"})
	if err == nil {
		t.Fatal("missing branch fetch succeeded")
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed capture changed enrolled tree\nbefore=%v\nafter=%v", before, after)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(repository, "forbidden-state"))
	before = treeDigest(t, repository)
	if _, err := source.Capture(t.Context(), document.Source); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("overlapping control storage error = %v", err)
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("overlap refusal changed enrolled tree\nbefore=%v\nafter=%v", before, after)
	}
}

func TestControlStoragePreservesSymlinkParentTransportMeaning(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "enrolled")
	actual := filepath.Join(root, "actual")
	if err := os.MkdirAll(filepath.Join(actual, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantedRemote := filepath.Join(actual, "backend.git")
	decoyRemote := filepath.Join(root, "backend.git")
	for _, remote := range []string{wantedRemote, decoyRemote} {
		controlGit(t, root, "init", "--bare", "--quiet", remote)
	}
	controlGit(t, root, "init", "--quiet", "-b", "main", repository)
	document := testDocument(testJob("codex"))
	policy, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		".pyxis/maintenance/jobs.yaml": policy,
		".pyxis/config.yaml":           []byte(harnessconfigtest.YAML),
	} {
		path := filepath.Join(repository, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	controlGit(t, repository, "add", ".pyxis/config.yaml", ".pyxis/maintenance/jobs.yaml")
	controlGit(t, repository, "commit", "-m", "wanted published snapshot")
	wanted := controlGit(t, repository, "rev-parse", "HEAD")
	controlGit(t, repository, "push", wantedRemote, "main")
	controlGit(t, repository, "commit", "--allow-empty", "-m", "decoy snapshot")
	decoy := controlGit(t, repository, "rev-parse", "HEAD")
	controlGit(t, repository, "push", decoyRemote, "main")
	if err := os.Symlink(filepath.Join(actual, "child"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	const transport = "../alias/../backend.git"
	controlGit(t, repository, "remote", "add", "origin", transport)
	// Establish Git's original enrolled-root meaning independently of Capture.
	if got := controlGit(t, repository, "ls-remote", "origin", "refs/heads/main"); got != wanted+"\trefs/heads/main" {
		t.Fatalf("fixture does not resolve through the symlink: %s", got)
	}
	common := controlGit(t, repository, "rev-parse", "--path-format=absolute", "--git-common-dir")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	source := ControlSource{Repository: repository, GitCommon: common, RemoteURL: transport}
	before := treeDigest(t, repository)
	published, err := source.Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := published.Control.Close(); err != nil {
			t.Error(err)
		}
	}()
	if published.Commit != wanted || !reflect.DeepEqual(published.Jobs, policy) || string(published.Config) != harnessconfigtest.YAML {
		t.Fatalf("capture changed transport meaning: got %s, want %s (decoy %s)", published.Commit, wanted, decoy)
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("capture changed enrolled files or Git state\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := source.Capture(t.Context(), jobs.SourceBinding{Remote: "origin", Branch: "missing"}); err == nil {
		t.Fatal("missing branch fetched successfully")
	}
	if after := treeDigest(t, repository); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed capture changed enrolled files or Git state\nbefore=%v\nafter=%v", before, after)
	}
}

func TestEnrolledRemoteURLPreservesNonFilesystemTransports(t *testing.T) {
	repository := filepath.Join(string(filepath.Separator), "enrolled", "repository")
	for _, test := range []struct{ input, want string }{
		{"../remote.git", repository + string(filepath.Separator) + "../remote.git"},
		{"nested/remote.git", filepath.Join(repository, "nested", "remote.git")},
		{"/srv/remote.git", "/srv/remote.git"},
		{"file:///srv/remote.git", "file:///srv/remote.git"},
		{"https://credential-canary@example.test/repo.git", "https://credential-canary@example.test/repo.git"},
		{"git@example.test:repo.git", "git@example.test:repo.git"},
		{"ext::transport helper", "ext::transport helper"},
		{"~/remote.git", "~/remote.git"},
	} {
		if got := enrolledRemoteURL(repository, test.input); got != test.want {
			t.Errorf("enrolledRemoteURL(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestControlStorageRefusalBranches(t *testing.T) {
	repository := t.TempDir()
	common := t.TempDir()
	source := jobs.SourceBinding{Remote: "origin", Branch: "main"}

	resolveErr := errors.New("remote unavailable")
	_, err := (ControlSource{Repository: repository, GitCommon: common, ResolveRemote: func(context.Context, string) (string, error) {
		return "", resolveErr
	}}).Capture(t.Context(), source)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("remote resolution error = %v", err)
	}
	if _, err := (ControlSource{}).Capture(t.Context(), source); err == nil {
		t.Fatal("invalid control source succeeded")
	}
	t.Setenv("XDG_STATE_HOME", "relative")
	valid := ControlSource{Repository: repository, GitCommon: common, RemoteURL: "fixture"}
	if _, err := valid.Capture(t.Context(), source); err == nil {
		t.Fatal("relative state directory succeeded")
	}

	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	base := filepath.Join(state, "pyxis", "maintenance", "control")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := valid.Capture(t.Context(), source); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("public control directory error = %v", err)
	}

	if err := (*control)(nil).Close(); err != nil {
		t.Fatalf("nil control close = %v", err)
	}
	c := &control{repository: t.TempDir(), owned: true}
	if err := c.git(t.Context(), "definitely-not-a-git-command"); err == nil {
		t.Fatal("invalid git operation succeeded")
	}
}
