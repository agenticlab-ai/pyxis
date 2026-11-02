package gitsrc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSourceHistoryDistinguishesReintroductionAndSquash(t *testing.T) {
	dir := newRepo(t)
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	name := ".pyxis/skills/review/SKILL.md"
	write(t, dir, name, "exact\n")
	git(t, dir, "add", name)
	git(t, dir, "commit", "-qm", "introduce")
	first := git(t, dir, "rev-parse", "HEAD")
	got, err := r.SourceIntroduction(t.Context(), first, name)
	if err != nil || got != first {
		t.Fatalf("introduction: %q %v", got, err)
	}
	git(t, dir, "checkout", "-qb", "retire")
	git(t, dir, "rm", name)
	git(t, dir, "commit", "-qm", "delete on topic")
	topic := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "main")
	git(t, dir, "merge", "--squash", "retire")
	git(t, dir, "commit", "-qm", "squash deletion")
	deleted := git(t, dir, "rev-parse", "HEAD")
	before, err := r.BeforeSourceDeletion(t.Context(), deleted, name)
	if err != nil || before != first {
		t.Fatalf("deletion parent: %q %v", before, err)
	}
	if err := r.FirstParentAncestor(t.Context(), topic, deleted); err == nil {
		t.Fatal("topic commit treated as retained mainline")
	}
	if err := r.FirstParentAncestor(t.Context(), first, deleted); err != nil {
		t.Fatal(err)
	}
	write(t, dir, name, "exact\n")
	git(t, dir, "add", name)
	git(t, dir, "commit", "-qm", "reintroduce identical bytes")
	latest := git(t, dir, "rev-parse", "HEAD")
	got, err = r.SourceIntroduction(t.Context(), latest, name)
	if err != nil || got != latest || got == first {
		t.Fatalf("reintroduction reused old identity: %q %v", got, err)
	}
	if _, err := r.SourceIntroduction(t.Context(), latest, "missing"); err == nil {
		t.Fatal("missing history accepted")
	}
	write(t, dir, "untracked", "new")
	git(t, dir, "mv", name, "renamed")
	write(t, dir, "README.md", "edited")
	dirty, err := r.DirtyPathNames(t.Context(), []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{name, "README.md", "renamed", "untracked"}
	if !reflect.DeepEqual(dirty, want) {
		t.Fatalf("dirty paths: %v", dirty)
	}
}

func TestSourceHistoryFailsClosedOnUnavailableInputs(t *testing.T) {
	for _, mode := range []string{"shallow query", "shallow", "log failure", "missing event", "revision failure"} {
		t.Run(mode, func(t *testing.T) {
			r := &Repo{run: func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					if mode == "shallow query" {
						return nil, errors.New("unavailable shallow state")
					}
					if mode == "shallow" {
						return []byte("true\n"), nil
					}
					return []byte("false\n"), nil
				}
				if args[0] == "log" {
					if mode == "missing event" {
						return nil, nil
					}
					return nil, errors.New("unavailable log")
				}
				return nil, errors.New("unavailable " + strings.Join(args, " "))
			}}
			var err error
			switch mode {
			case "revision failure":
				err = r.FirstParentAncestor(t.Context(), "old", "HEAD")
			default:
				_, err = r.BeforeSourceDeletion(t.Context(), "HEAD", "source")
			}
			if err == nil {
				t.Fatal("unavailable input accepted")
			}
		})
	}
}

func TestPathCommitsPinsFirstParentHistory(t *testing.T) {
	dir := newRepo(t)
	r, err := Open(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	base := git(t, dir, "rev-parse", "HEAD")
	for _, name := range []string{"", "../outside", "/outside", "a/../b"} {
		if _, err := r.PathCommits(t.Context(), base, base, name); err == nil {
			t.Errorf("accepted invalid path %q", name)
		}
	}
	if commits, err := r.PathCommits(t.Context(), base, base, "drafts/widget/prd.md"); err != nil || len(commits) != 0 {
		t.Fatalf("unchanged path: %v, %v", commits, err)
	}
	name := "drafts/widget/prd.md"
	write(t, dir, name, "first\n")
	git(t, dir, "add", name)
	git(t, dir, "commit", "-qm", "add draft")
	first := git(t, dir, "rev-parse", "HEAD")
	write(t, dir, "unrelated.md", "other\n")
	git(t, dir, "add", "unrelated.md")
	git(t, dir, "commit", "-qm", "unrelated edit")
	write(t, dir, name, "second\n")
	git(t, dir, "add", name)
	git(t, dir, "commit", "-qm", "revise draft")
	last := git(t, dir, "rev-parse", "HEAD")
	commits, err := r.PathCommits(t.Context(), base, last, name)
	if err != nil || !reflect.DeepEqual(commits, []string{first, last}) {
		t.Fatalf("path history: %v, %v", commits, err)
	}
	if _, err := r.PathCommits(t.Context(), last, base, name); err == nil {
		t.Fatal("accepted a base outside the head's history")
	}
}
