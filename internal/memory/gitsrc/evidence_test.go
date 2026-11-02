package gitsrc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceGitReadsExactHistoryWithoutExecution(t *testing.T) {
	dir := newRepo(t)
	ctx := t.Context()
	root := git(t, dir, "rev-parse", "HEAD")
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FullHistory(ctx); err != nil {
		t.Fatal(err)
	}
	entries, err := r.EvidenceTree(ctx, root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("tree: %+v %v", entries, err)
	}
	changes, err := r.FirstParentChanges(ctx, root, "memory")
	if err != nil || len(changes) != 2 || changes[0].OldMode != "000000" {
		t.Fatalf("root additions: %+v %v", changes, err)
	}
	git(t, dir, "checkout", "-qb", "topic")
	write(t, dir, "README.md", "topic source")
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "-qm", "topic")
	topic := git(t, dir, "rev-parse", "HEAD")
	if ok, err := r.Ancestor(ctx, root, topic); err != nil || !ok {
		t.Fatalf("ancestor: %v %v", ok, err)
	}
	if ok, err := r.Ancestor(ctx, topic, root); err != nil || ok {
		t.Fatalf("reverse ancestor: %v %v", ok, err)
	}
	// Replacement refs and graft configuration must not alter pinned reads.
	git(t, dir, "replace", root, topic)
	marker := filepath.Join(dir, "executed")
	write(t, dir, ".gitattributes", "README.md filter=trip diff=trip\n")
	git(t, dir, "add", ".gitattributes")
	git(t, dir, "commit", "-qm", "attribute declarations")
	git(t, dir, "config", "core.fsmonitor", "!touch "+marker)
	git(t, dir, "config", "filter.trip.smudge", "touch "+marker)
	git(t, dir, "config", "filter.trip.clean", "touch "+marker)
	git(t, dir, "config", "diff.trip.textconv", "touch "+marker)
	write(t, dir, "README.md", "uncommitted raw content must not invoke a clean filter")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "not-the-repository"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "!touch "+marker)
	files, err := r.Files(ctx, root, []string{"README.md"})
	if err != nil || string(files["README.md"]) != "hi" {
		t.Fatalf("replaced read: %s %v", files, err)
	}
	if _, err := r.Dirty(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("evidence read executed a Git extension")
	}
}

func TestEvidenceGitFailsClosedOnUnavailableOrMalformedInput(t *testing.T) {
	ctx := t.Context()
	pin := strings.Repeat("a", 40)
	failure := errors.New("object unavailable")
	for _, verb := range []string{"full", "tree", "history", "ancestor"} {
		t.Run(verb, func(t *testing.T) {
			r := &Repo{run: func(context.Context, []byte, ...string) ([]byte, error) { return nil, failure }}
			var err error
			switch verb {
			case "full":
				err = r.FullHistory(ctx)
			case "tree":
				_, err = r.EvidenceTree(ctx, pin)
			case "history":
				_, err = r.FirstParentChanges(ctx, pin, "memory")
			case "ancestor":
				_, err = r.Ancestor(ctx, pin, pin)
			}
			if !errors.Is(err, failure) {
				t.Fatalf("lost read failure: %v", err)
			}
		})
	}
	r := &Repo{run: func(context.Context, []byte, ...string) ([]byte, error) {
		t.Fatal("invalid pin invoked Git")
		return nil, nil
	}}
	if _, err := r.EvidenceTree(ctx, "HEAD"); err == nil {
		t.Fatal("moving tree pin")
	}
	if _, err := r.FirstParentChanges(ctx, pin, "../memory"); err == nil {
		t.Fatal("invalid prefix")
	}
	if _, err := r.Ancestor(ctx, "HEAD", pin); err == nil {
		t.Fatal("moving ancestry pin")
	}
	for _, raw := range []string{"true\n", "unknown\n"} {
		r.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte(raw), nil }
		if err := r.FullHistory(ctx); err == nil {
			t.Fatal("history was assumed complete")
		}
	}
	for _, raw := range []string{"bad\x00", "100644 blob " + pin + "\t../escape\x00"} {
		r.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte(raw), nil }
		if _, err := r.EvidenceTree(ctx, pin); err == nil {
			t.Fatal("malformed tree accepted")
		}
	}
	for _, raw := range []string{"bad\x00", "\x00" + pin + "\x00:000000 100644 " + pin + " " + pin + " A\x00../escape\x00"} {
		r.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte(raw), nil }
		if _, err := r.FirstParentChanges(ctx, pin, "memory"); err == nil {
			t.Fatal("malformed history accepted")
		}
	}
}
