package gitsrc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestPolicyTreeBatchReadHasConstantProcessCount(t *testing.T) {
	const count = 700
	var listing, blobs strings.Builder
	paths := make([]string, 0, count)
	for i := range count {
		name := fmt.Sprintf(".github/memory-reconciliation-baseline/%064x.json", i)
		paths = append(paths, name)
		fmt.Fprintf(&listing, "100644 blob %040x\t%s%c", i, name, 0)
		fmt.Fprintf(&blobs, "%040x blob 2\n{}\n", i)
	}
	calls := 0
	r := &Repo{run: func(_ context.Context, in []byte, args ...string) ([]byte, error) {
		calls++
		switch args[0] {
		case "ls-tree":
			if !reflect.DeepEqual(args, []string{"ls-tree", "-r", "-t", "-z", "pinned", "--", ".github"}) {
				t.Fatalf("tree args: %v", args)
			}
			return []byte(listing.String()), nil
		case "cat-file":
			if !reflect.DeepEqual(args, []string{"cat-file", "--batch"}) || strings.Count(string(in), "\n") != count {
				t.Fatalf("not one batch for all members: %v", args)
			}
			return []byte(blobs.String()), nil
		default:
			t.Fatalf("unexpected Git operation: %v", args)
			return nil, nil
		}
	}}
	entries, err := r.TreeNodes(t.Context(), "pinned", ".github")
	if err != nil {
		t.Fatal(err)
	}
	files, err := r.Files(t.Context(), "pinned", paths)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(entries) != count || len(files) != count {
		t.Fatalf("700-member read: %d calls, %d nodes, %d blobs", calls, len(entries), len(files))
	}
}

func TestPolicyGitPathsRemainExactAndIndexConflictsRefuse(t *testing.T) {
	tests := []struct {
		name   string
		output string
		run    func(*Repo) error
		bad    bool
	}{
		{name: "quoted tree names", output: "100644 blob abc\t.github/a\n\"b\"\x00", run: func(r *Repo) error {
			entries, err := r.TreeNodes(t.Context(), "HEAD", ".github")
			if err == nil && entries[0].Path != ".github/a\n\"b\"" {
				t.Fatal("tree path was display-unquoted")
			}
			return err
		}},
		{name: "malformed tree", output: "bad\x00", run: func(r *Repo) error { _, err := r.TreeNodes(t.Context(), "HEAD", ".github"); return err }, bad: true},
		{name: "working paths", output: " M a\nquoted\x00?? new\x00M  staged\x00", run: func(r *Repo) error {
			got, err := r.WorkingChanges(t.Context())
			if err == nil && (!reflect.DeepEqual(got.Paths, []string{"a\nquoted", "new", "staged"}) || !reflect.DeepEqual(got.IndexPaths, []string{"staged"})) {
				t.Fatalf("paths: %+v", got)
			}
			return err
		}},
		{name: "malformed status", output: "bad\x00", run: func(r *Repo) error { _, err := r.WorkingChanges(t.Context()); return err }, bad: true},
		{name: "unmerged index", output: "100644 abc 2\tmember\x00", run: func(r *Repo) error { _, err := r.IndexEntries(t.Context(), ".github"); return err }, bad: true},
		{name: "regular index", output: "100644 abc 0\tmember\x00", run: func(r *Repo) error {
			got, err := r.IndexEntries(t.Context(), ".github")
			if err == nil && (len(got) != 1 || !got[0].Regular()) {
				t.Fatal("regular index lost")
			}
			return err
		}},
		{name: "both rename paths", output: "old\x00new\x00", run: func(r *Repo) error {
			got, err := r.ChangedPaths(t.Context(), "a", "b")
			if err == nil && !reflect.DeepEqual(got, []string{"new", "old"}) {
				t.Fatalf("diff paths: %v", got)
			}
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Repo{run: func(context.Context, []byte, ...string) ([]byte, error) { return []byte(tc.output), nil }}
			if err := tc.run(r); (err != nil) != tc.bad {
				t.Fatalf("path decode error: %v", err)
			}
			r.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, context.Canceled }
			if err := tc.run(r); err == nil {
				t.Fatal("Git error swallowed")
			}
		})
	}
}
