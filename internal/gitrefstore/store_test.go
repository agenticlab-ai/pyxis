package gitrefstore

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func cloneFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	remote, seed, clone := filepath.Join(root, "remote.git"), filepath.Join(root, "seed"), filepath.Join(root, "clone")
	runGit(t, root, "init", "--bare", "-q", remote)
	if err := os.Mkdir(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "init", "-q")
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "README")
	runGit(t, seed, "commit", "-qm", "base")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "-q", "origin", "HEAD:refs/heads/main")
	runGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, root, "clone", "-q", remote, clone)
	return remote, clone
}

func TestStoreUpdateReplaysFromFreshClone(t *testing.T) {
	remote, first := cloneFixture(t)
	store, err := New(first, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Update(context.Background(), "state/log.jsonl", "append fixture", func(current []byte) ([]byte, bool, error) {
		return append(current, []byte("one\n")...), true, nil
	})
	if err != nil || string(got) != "one\n" {
		t.Fatalf("Update = %q, %v", got, err)
	}

	second := filepath.Join(t.TempDir(), "fresh")
	runGit(t, filepath.Dir(second), "clone", "-q", remote, second)
	fresh, err := New(second, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := fresh.Read(context.Background(), "state/log.jsonl")
	if err != nil || string(replayed) != "one\n" {
		t.Fatalf("fresh Read = %q, %v", replayed, err)
	}
}

func TestStoreUnchangedMutationDoesNotAdvanceRef(t *testing.T) {
	_, repo := cloneFixture(t)
	store, _ := New(repo, "origin", "refs/pyxis/test-evidence")
	if _, err := store.Update(context.Background(), "log", "first", func([]byte) ([]byte, bool, error) { return []byte("x"), true, nil }); err != nil {
		t.Fatal(err)
	}
	before := runGit(t, repo, "rev-parse", "refs/pyxis/test-evidence")
	if _, err := store.Update(context.Background(), "log", "noop", func(current []byte) ([]byte, bool, error) { return current, false, nil }); err != nil {
		t.Fatal(err)
	}
	after := runGit(t, repo, "rev-parse", "refs/pyxis/test-evidence")
	if before != after {
		t.Fatalf("no-op advanced ref: %s -> %s", before, after)
	}
}

func TestNewRejectsUnsafeEvidenceRefs(t *testing.T) {
	for _, ref := range []string{"refs/heads/main", "refs/pyxis/a..b", "refs/pyxis/a@{b", "refs/pyxis/a.lock", "refs/pyxis/a b", "refs/pyxis//b"} {
		if _, err := New("/repo", "origin", ref); err == nil {
			t.Errorf("New accepted unsafe ref %q", ref)
		}
	}
}

func TestGitKeepsSuccessfulStderrOutOfEvidenceBytes(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	// Git trace writes diagnostics to stderr while the command still exits 0.
	// Those diagnostics must never enter evidence reads or object identities.
	t.Setenv("GIT_TRACE", "1")
	out, err := store.git(context.Background(), nil, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != canonicalRepo {
		t.Fatalf("stdout was contaminated by git stderr: %q", out)
	}
}

func TestGitDropsRepositoryControlsAndPreservesTransportAuthentication(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	foreign := t.TempDir()
	runGit(t, foreign, "init", "-q")
	t.Setenv("GIT_DIR", filepath.Join(foreign, ".git"))
	t.Setenv("GIT_WORK_TREE", foreign)
	t.Setenv("GIT_OBJECT_DIRECTORY", filepath.Join(foreign, ".git", "objects"))
	t.Setenv("GIT_CONFIG_PARAMETERS", "'remote.origin.url=https://attacker.invalid/parameters'")
	t.Setenv("GIT_EXEC_PATH", filepath.Join(foreign, "exec"))
	t.Setenv("GIT_TEMPLATE_DIR", filepath.Join(foreign, "templates"))
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /credential")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	env := "\n" + strings.Join(gitEnvironment(), "\n") + "\n"
	for _, dropped := range []string{"\nGIT_DIR=", "\nGIT_WORK_TREE=", "\nGIT_OBJECT_DIRECTORY=", "\nGIT_CONFIG_PARAMETERS=", "\nGIT_EXEC_PATH=", "\nGIT_TEMPLATE_DIR="} {
		if strings.Contains(env, dropped) {
			t.Fatalf("canonical git environment retained redirect %q: %s", dropped, env)
		}
	}
	for _, kept := range []string{"\nGIT_SSH_COMMAND=ssh -i /credential\n", "\nGIT_TERMINAL_PROMPT=0\n"} {
		if !strings.Contains(env, kept) {
			t.Fatalf("canonical git environment dropped transport control %q: %s", kept, env)
		}
	}
	out, err := store.git(context.Background(), nil, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != canonicalRepo {
		t.Fatalf("ambient GIT_* redirected evidence command: got %q want %q", out, canonicalRepo)
	}
}

func TestApplyPublishesMultiplePathsAtomicallyAndExactReadsWinner(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := store.EstablishGenesis(context.Background(), "operator:review-1")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"events/aa/one": []byte("one"), "events/bb/two": []byte("two")}
	result, err := store.Apply(context.Background(), "events", "atomic pair", func(snapshot Snapshot) (Mutation, error) {
		if snapshot.Head != genesis.Commit || len(snapshot.Files) != 0 {
			t.Fatalf("initial snapshot = %+v", snapshot)
		}
		return Mutation{Files: files, Changed: true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Prior != genesis.Commit || string(result.Files["events/aa/one"]) != "one" || string(result.Files["events/bb/two"]) != "two" {
		t.Fatalf("Apply result = %+v", result)
	}
	snapshot, err := store.Fetch(context.Background(), "events")
	if err != nil || snapshot.Head != result.Head || len(snapshot.Files) != 2 {
		t.Fatalf("winning snapshot = %+v, %v", snapshot, err)
	}
}

func TestApplyConcurrentNonConflictingWritersReplayFromWinningHead(t *testing.T) {
	remote, firstRepo := cloneFixture(t)
	first, _ := New(firstRepo, "origin", "refs/pyxis/test-evidence")
	if _, err := first.EstablishGenesis(context.Background(), "operator:review-1"); err != nil {
		t.Fatal(err)
	}
	secondRepo := filepath.Join(t.TempDir(), "second")
	runGit(t, filepath.Dir(secondRepo), "clone", "-q", remote, secondRepo)
	second, _ := New(secondRepo, "origin", "refs/pyxis/test-evidence")

	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := func(store *Store, path, value string) {
		defer wg.Done()
		firstAttempt := true
		_, err := store.Apply(context.Background(), "events", "concurrent append", func(Snapshot) (Mutation, error) {
			if firstAttempt {
				firstAttempt = false
				ready <- struct{}{}
				<-release
			}
			return Mutation{Files: map[string][]byte{path: []byte(value)}, Changed: true}, nil
		})
		errs <- err
	}
	wg.Add(2)
	go start(first, "events/aa/one", "one")
	go start(second, "events/bb/two", "two")
	<-ready
	<-ready
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := first.Fetch(context.Background(), "events")
	if err != nil || len(snapshot.Files) != 2 {
		t.Fatalf("converged snapshot = %+v, %v", snapshot, err)
	}
}

func TestPushCASRejectsUpToDateStaleLease(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/test-evidence")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	genesis, err := store.EstablishGenesis(ctx, "operator:review-1")
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := store.commitFile(ctx, genesis.Commit, "events/one", []byte("one"), "candidate")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.pushCAS(ctx, genesis.Commit, candidate); err != nil {
		t.Fatalf("first CAS: %v", err)
	}
	if err := store.pushCAS(ctx, genesis.Commit, candidate); !errors.Is(err, errCASConflict) {
		t.Fatalf("stale identical-candidate CAS error = %v, want %v", err, errCASConflict)
	}
}

func TestUnpushedMultiPathCommitIsNotReachable(t *testing.T) {
	_, repo := cloneFixture(t)
	store, _ := New(repo, "origin", "refs/pyxis/test-evidence")
	genesis, err := store.EstablishGenesis(context.Background(), "operator:review-1")
	if err != nil {
		t.Fatal(err)
	}
	commit, err := store.commitFiles(context.Background(), genesis.Commit, map[string][]byte{"events/a": []byte("a"), "events/b": []byte("b")}, "interrupted composite")
	if err != nil || commit == "" {
		t.Fatalf("staged commit = %q, %v", commit, err)
	}
	snapshot, err := store.Fetch(context.Background(), "events")
	if err != nil || snapshot.Head != genesis.Commit || len(snapshot.Files) != 0 {
		t.Fatalf("interrupted paths became reachable: %+v, %v", snapshot, err)
	}
}

func TestEstablishGenesisIsOneTimeAndAuthorizationBound(t *testing.T) {
	_, repo := cloneFixture(t)
	store, _ := New(repo, "origin", "refs/pyxis/test-evidence")
	genesis, err := store.EstablishGenesis(context.Background(), "operator:review-1")
	if err != nil || genesis.Commit == "" || genesis.Tree == "" {
		t.Fatalf("genesis = %+v, %v", genesis, err)
	}
	if _, err := store.EstablishGenesis(context.Background(), "operator:review-2"); err == nil {
		t.Fatal("second genesis was accepted")
	}
}
