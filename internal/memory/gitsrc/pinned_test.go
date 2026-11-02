package gitsrc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// gitCalls records every Git process a handle starts.
type gitCalls struct {
	args  [][]string
	stdin []string
}

func observe(r *Repo) *gitCalls {
	calls := &gitCalls{}
	real := r.run
	r.run = func(ctx context.Context, in []byte, args ...string) ([]byte, error) {
		calls.args = append(calls.args, args)
		calls.stdin = append(calls.stdin, string(in))
		return real(ctx, in, args...)
	}
	return calls
}

func (c *gitCalls) reset() { c.args, c.stdin = nil, nil }

// history is newRepo plus three commits: one adds a.txt and b.txt, one
// rewrites b.txt and adds c.txt with a.txt's bytes, one deletes b.txt.
func history(t *testing.T) (dir string, commits []string) {
	t.Helper()
	dir = newRepo(t)
	commit := func(message string) {
		git(t, dir, "add", "-A")
		git(t, dir, "commit", "-q", "-m", message)
		commits = append(commits, git(t, dir, "rev-parse", "HEAD"))
	}
	commits = append(commits, git(t, dir, "rev-parse", "HEAD"))
	write(t, dir, "a.txt", "same\n")
	write(t, dir, "b.txt", "one\n")
	commit("add")
	write(t, dir, "b.txt", "two\n")
	write(t, dir, "c.txt", "same\n")
	commit("rewrite")
	git(t, dir, "rm", "-q", "b.txt")
	commit("delete")
	return dir, commits
}

func open(t *testing.T, dir string, keep bool) (*Repo, *gitCalls) {
	t.Helper()
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if keep {
		r.KeepPinnedReads()
	}
	return r, observe(r)
}

type answer struct {
	value any
	err   string
}

func answerOf(value any, err error) answer {
	if err != nil {
		return answer{err: err.Error()}
	}
	return answer{value: value}
}

func TestKeptReadsAnswerEachPinnedReadOnce(t *testing.T) {
	dir, commits := history(t)
	base, added, rewritten, head := commits[0], commits[1], commits[2], commits[3]
	ctx := t.Context()
	reads := []struct {
		name string
		// probes is how many repository probes a repeat still sends to Git.
		probes int
		read   func(*Repo) answer
	}{
		{name: "commit", read: func(r *Repo) answer { return answerOf(r.ResolveCommit(ctx, head)) }},
		{name: "first parent", read: func(r *Repo) answer { return answerOf(r.ResolveCommit(ctx, head+"^1")) }},
		{name: "tree", read: func(r *Repo) answer { return answerOf(r.ListTree(ctx, head, "memory")) }},
		{name: "tree nodes", read: func(r *Repo) answer { return answerOf(r.TreeNodes(ctx, head, "memory")) }},
		{name: "evidence tree", read: func(r *Repo) answer { return answerOf(r.EvidenceTree(ctx, head)) }},
		{name: "first-parent changes", read: func(r *Repo) answer { return answerOf(r.FirstParentChanges(ctx, head, "memory")) }},
		{name: "merge base", read: func(r *Repo) answer { return answerOf(r.MergeBase(ctx, base, head)) }},
		{name: "ancestor", read: func(r *Repo) answer { return answerOf(r.Ancestor(ctx, base, head)) }},
		{name: "first-parent ancestor", read: func(r *Repo) answer { return answerOf(nil, r.FirstParentAncestor(ctx, base, head)) }},
		{name: "introduction", probes: 1, read: func(r *Repo) answer { return answerOf(r.SourceIntroduction(ctx, head, "a.txt")) }},
		{name: "before deletion", probes: 1, read: func(r *Repo) answer { return answerOf(r.BeforeSourceDeletion(ctx, head, "b.txt")) }},
		{name: "files", read: func(r *Repo) answer { return answerOf(r.Files(ctx, added, []string{"a.txt", "b.txt"})) }},
		{name: "memory tree", read: func(r *Repo) answer {
			files, irregular, err := TreeSource{Repo: r, Ref: rewritten, Prefix: "memory"}.Files(ctx)
			return answerOf([]any{files, irregular}, err)
		}},
	}
	for _, tc := range reads {
		t.Run(tc.name, func(t *testing.T) {
			plain, plainCalls := open(t, dir, false)
			want := tc.read(plain)
			first := len(plainCalls.args)
			if want.err != "" || first == 0 {
				t.Fatalf("plain read: %+v after %d Git processes", want, first)
			}
			if again := tc.read(plain); !reflect.DeepEqual(again, want) || len(plainCalls.args) != 2*first {
				t.Fatalf("a plain handle must ask Git every time: %d then %d processes", first, len(plainCalls.args)-first)
			}
			kept, keptCalls := open(t, dir, true)
			if got := tc.read(kept); !reflect.DeepEqual(got, want) {
				t.Fatalf("first kept read = %+v, want %+v", got, want)
			}
			keptCalls.reset()
			if got := tc.read(kept); !reflect.DeepEqual(got, want) {
				t.Fatalf("repeated kept read = %+v, want %+v", got, want)
			}
			if len(keptCalls.args) != tc.probes {
				t.Fatalf("repeat started %d Git processes, want %d: %v", len(keptCalls.args), tc.probes, keptCalls.args)
			}
			for _, args := range keptCalls.args {
				if !reflect.DeepEqual(args, []string{"rev-parse", "--is-shallow-repository"}) {
					t.Fatalf("repeat re-read a pinned answer: %v", args)
				}
			}
		})
	}
}

func TestKeptReadsNeverHoldAMovingRefOrProbe(t *testing.T) {
	dir, commits := history(t)
	ctx := t.Context()
	reads := []struct {
		name string
		read func(*Repo) answer
	}{
		{name: "HEAD", read: func(r *Repo) answer { return answerOf(r.Head(ctx)) }},
		{name: "resolve HEAD", read: func(r *Repo) answer { return answerOf(r.ResolveCommit(ctx, "HEAD")) }},
		{name: "resolve branch", read: func(r *Repo) answer { return answerOf(r.ResolveCommit(ctx, "main")) }},
		{name: "parent of branch", read: func(r *Repo) answer { return answerOf(r.ResolveCommit(ctx, "main^1")) }},
		{name: "branch exists", read: func(r *Repo) answer { return answerOf(r.RefExists(ctx, "main"), nil) }},
		{name: "tree of branch", read: func(r *Repo) answer { return answerOf(r.ListTree(ctx, "main", ".")) }},
		{name: "tree nodes of HEAD", read: func(r *Repo) answer { return answerOf(r.TreeNodes(ctx, "HEAD", ".")) }},
		{name: "files of branch", read: func(r *Repo) answer { return answerOf(r.Files(ctx, "main", []string{"a.txt", "d.txt"})) }},
		{name: "merge base with HEAD", read: func(r *Repo) answer { return answerOf(r.MergeBase(ctx, commits[1], "HEAD")) }},
		{name: "introduction at HEAD", read: func(r *Repo) answer { return answerOf(r.SourceIntroduction(ctx, "HEAD", "a.txt")) }},
		{name: "first-parent line of HEAD", read: func(r *Repo) answer { return answerOf(nil, r.FirstParentAncestor(ctx, commits[3], "HEAD")) }},
		{name: "full history", read: func(r *Repo) answer { return answerOf(nil, r.FullHistory(ctx)) }},
		{name: "dirty", read: func(r *Repo) answer { return answerOf(r.Dirty(ctx, ".")) }},
	}
	kept, calls := open(t, dir, true)
	for _, tc := range reads {
		if got := tc.read(kept); got.err != "" {
			t.Fatalf("%s before the move: %v", tc.name, got.err)
		}
	}
	// Both HEAD and main now name a different commit with different files.
	git(t, dir, "rm", "-q", "a.txt")
	write(t, dir, "a.txt", "moved\n")
	write(t, dir, "d.txt", "new\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "move")
	plain, _ := open(t, dir, false)
	for _, tc := range reads {
		calls.reset()
		got, want := tc.read(kept), tc.read(plain)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s after the move = %+v, want Git's current answer %+v", tc.name, got, want)
		}
		if len(calls.args) == 0 {
			t.Fatalf("%s was answered without asking Git", tc.name)
		}
	}
}

func TestKeptReadsHoldOnlySuccess(t *testing.T) {
	dir, commits := history(t)
	head := commits[3]
	ctx := t.Context()
	kept, calls := open(t, dir, true)
	plain, _ := open(t, dir, false)

	// A failed read is asked again, and its later success is then kept.
	failure := errors.New("read denied")
	real, failing := kept.run, true
	kept.run = func(ctx context.Context, in []byte, args ...string) ([]byte, error) {
		if failing {
			return nil, failure
		}
		return real(ctx, in, args...)
	}
	if _, err := kept.ListTree(ctx, head, "."); !errors.Is(err, failure) {
		t.Fatalf("lost failure: %v", err)
	}
	if _, err := kept.Files(ctx, head, []string{"a.txt"}); !errors.Is(err, failure) {
		t.Fatalf("lost blob failure: %v", err)
	}
	failing = false
	want, err := plain.ListTree(ctx, head, ".")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := kept.ListTree(ctx, head, "."); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("read after a failure = %v %v", got, err)
	}
	calls.reset()
	if got, err := kept.ListTree(ctx, head, "."); err != nil || !reflect.DeepEqual(got, want) || len(calls.args) != 0 {
		t.Fatalf("kept success = %v %v after %d Git processes", got, err, len(calls.args))
	}

	// A blob Git could not produce is asked for again; one it produced is not.
	if got, err := kept.Files(ctx, head, []string{"a.txt", "absent.txt"}); err != nil || len(got) != 1 || string(got["a.txt"]) != "same\n" {
		t.Fatalf("partly missing read = %v %v", got, err)
	}
	calls.reset()
	if got, err := kept.Files(ctx, head, []string{"a.txt", "absent.txt"}); err != nil || len(got) != 1 {
		t.Fatalf("repeated missing read = %v %v", got, err)
	}
	if len(calls.stdin) != 1 || calls.stdin[0] != head+":absent.txt\n" {
		t.Fatalf("a missing blob must be the only one asked for again: %q", calls.stdin)
	}

	// Whatever the commit's tree does not list as a blob fails as it always has.
	git(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
	git(t, dir, "commit", "-q", "-m", "gitlink")
	linked := git(t, dir, "rev-parse", "HEAD")
	unknown := strings.Repeat("e", 40)
	for _, tc := range []struct {
		name, commit, path string
	}{
		{"directory", head, "memory"},
		{"gitlink", linked, "module"},
		{"unknown commit", unknown, "a.txt"},
	} {
		got := answerOf(kept.Files(ctx, tc.commit, []string{tc.path}))
		if want := answerOf(plain.Files(ctx, tc.commit, []string{tc.path})); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %+v, want Git's own answer %+v", tc.name, got, want)
		}
	}
	if _, err := kept.Files(ctx, head, []string{"memory"}); err == nil {
		t.Fatal("a directory was read as a file")
	}

	// A finished context reaches Git even for an answer already kept.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for name, read := range map[string]func(context.Context, *Repo) error{
		"tree":  func(ctx context.Context, r *Repo) error { _, err := r.ListTree(ctx, head, "."); return err },
		"files": func(ctx context.Context, r *Repo) error { _, err := r.Files(ctx, head, []string{"a.txt"}); return err },
	} {
		got, want := read(cancelled, kept), read(cancelled, plain)
		if got == nil || want == nil || got.Error() != want.Error() {
			t.Fatalf("%s under a finished context = %v, want Git's own failure %v", name, got, want)
		}
		calls.reset()
		if err := read(ctx, kept); err != nil || len(calls.args) != 0 {
			t.Fatalf("%s afterwards = %v after %d Git processes", name, err, len(calls.args))
		}
	}
}

func TestKeptFilesReadEachBlobOnce(t *testing.T) {
	dir, commits := history(t)
	added, rewritten := commits[1], commits[2]
	ctx := t.Context()
	kept, calls := open(t, dir, true)

	first, err := kept.Files(ctx, added, []string{"a.txt", "b.txt"})
	if err != nil || string(first["a.txt"]) != "same\n" || string(first["b.txt"]) != "one\n" {
		t.Fatalf("first commit = %q %v", first, err)
	}
	calls.reset()
	// a.txt is unchanged and c.txt holds the same bytes, so both are the blob
	// already read; only the rewritten b.txt is new to this handle.
	second, err := kept.Files(ctx, rewritten, []string{"a.txt", "b.txt", "c.txt"})
	if err != nil || string(second["a.txt"]) != "same\n" || string(second["b.txt"]) != "two\n" || string(second["c.txt"]) != "same\n" {
		t.Fatalf("second commit = %q %v", second, err)
	}
	var asked []string
	for i, args := range calls.args {
		if args[0] == "cat-file" {
			asked = append(asked, calls.stdin[i])
		}
	}
	if !reflect.DeepEqual(asked, []string{rewritten + ":b.txt\n"}) {
		t.Fatalf("blobs asked for = %q, want only the rewritten file", asked)
	}
	calls.reset()
	if again, err := kept.Files(ctx, rewritten, []string{"a.txt", "b.txt", "c.txt"}); err != nil || !reflect.DeepEqual(again, second) || len(calls.args) != 0 {
		t.Fatalf("repeat = %q %v after %d Git processes", again, err, len(calls.args))
	}
	// Shared bytes are capped, so appending to one result cannot reach another.
	for name, body := range second {
		if cap(body) != len(body) {
			t.Fatalf("%s leaves %d spare bytes in shared storage", name, cap(body)-len(body))
		}
	}
}
