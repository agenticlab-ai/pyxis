package gitrefstore

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseBlobsKeepsOnlyWellFormedBlobAnswers(t *testing.T) {
	oid := strings.Repeat("a", 40)
	paths := []string{"one", "two"}
	for _, tc := range []struct {
		name string
		out  string
		want map[string][]byte
	}{
		{"blob and missing", oid + " blob 3\nabc\nhead:two missing\n", map[string][]byte{"one": []byte("abc")}},
		// An empty blob maps to nil, as the exact read's empty output does.
		{"tree is not a blob", oid + " tree 0\n\n" + oid + " blob 0\n\n", map[string][]byte{"two": nil}},
		{"unterminated header", oid + " blob 3", nil},
		{"unparsable size", oid + " blob x\nabc\nhead:two missing\n", nil},
		{"negative size", oid + " blob -1\nabc\nhead:two missing\n", nil},
		{"short body", oid + " blob 9\nabc\n", nil},
		{"unterminated body", oid + " blob 2\nabc\nhead:two missing\n", nil},
		{"trailing output", oid + " blob 3\nabc\nhead:two missing\nextra\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseBlobs([]byte(tc.out), paths); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseBlobs = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseHistoryRefusesMalformedWalks(t *testing.T) {
	a, b, c, tree := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40)
	got, err := parseHistory(c, []byte("commit "+a+"\n"+tree+"\ncommit "+b+" "+a+"\n"+tree+"\ncommit "+c+" "+a+" "+b+"\n"+tree+"\n"))
	want := []Commit{{Object: a, Tree: tree}, {Object: b, Parents: []string{a}, Tree: tree}, {Object: c, Parents: []string{a, b}, Tree: tree}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHistory = %+v, %v", got, err)
	}
	for name, out := range map[string]string{
		"empty":          "",
		"missing tree":   "commit " + a + "\n",
		"missing header": a + "\n" + tree + "\n",
		"bad object":     "commit not-an-object\n" + tree + "\n",
		"bad parent":     "commit " + b + " not-a-parent\n" + tree + "\n",
		"bad tree":       "commit " + a + "\nnot-a-tree\n",
	} {
		if _, err := parseHistory(c, []byte(out)); err == nil {
			t.Fatalf("%s: parseHistory accepted %q", name, out)
		}
	}
}

func TestReadManyAtKeepsExactReadsForNonBlobEntries(t *testing.T) {
	_, repo := cloneFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, "events"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "events", "a"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "events/a")
	runGit(t, repo, "commit", "-qm", "events")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	store, err := New(repo, "origin", "refs/pyxis/batch-read")
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	// A directory path is listed by ls-tree, so the exact read shows the
	// tree listing rather than failing as an absent path.
	files, err := store.readManyAt(ctx, head, []string{"README", "events"})
	if err != nil || string(files["README"]) != "base\n" || string(files["events"]) != runGit(t, repo, "show", head+":events") {
		t.Fatalf("readManyAt = %q, %v", files, err)
	}
	if _, err := store.readManyAt(ctx, head, []string{"README", "events/missing"}); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("absent path = %v", err)
	}
	if _, err := store.readManyAt(ctx, head, []string{"README", "../outside"}); err == nil {
		t.Fatal("readManyAt accepted an unsafe path after a valid one")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.readManyAt(cancelled, head, []string{"README"}); err == nil {
		t.Fatal("cancelled readManyAt succeeded")
	}
	if blobs := store.readBlobs(ctx, nil); blobs != nil {
		t.Fatalf("empty batch = %q", blobs)
	}
}

func TestSnapshotsAtAnswersEachCommitLikeSnapshotAt(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			repo := t.TempDir()
			runGit(t, repo, "init", "-q", "--object-format="+format)
			plumb := func(stdin string, args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
				cmd.Stdin = strings.NewReader(stdin)
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("git %v: %v", args, err)
				}
				return strings.TrimSpace(string(out))
			}
			blob := func(body string) string { return plumb(body, "hash-object", "-w", "--stdin") }
			commit := func(entries string, missing bool) string {
				args := []string{"mktree"}
				if missing {
					args = append(args, "--missing")
				}
				return plumb("", "commit-tree", plumb(entries, args...), "-m", "fixture")
			}
			drive, other := blob("drive\n"), blob("other\n")
			nested := plumb("100644 blob "+other+"\tleaf\n100755 blob "+drive+"\ttool\n", "mktree")
			deeper := plumb("040000 tree "+nested+"\tinner\n120000 blob "+other+"\tlink\n", "mktree")
			absent := strings.Repeat("0", len(drive)-1) + "1"
			commits := []string{
				commit("", false),
				commit("100644 blob "+drive+"\tdrive.json\n", false),
				commit("100644 blob "+drive+"\tdrive.json\n040000 tree "+deeper+"\tevents\n", false),
				commit("100644 blob "+blob("")+"\tempty\n", false),
				commit("160000 commit "+absent+"\tmodule\n", true),
				commit("040000 tree "+absent+"\tgone\n", true),
				absent,
				"not-an-object",
			}
			commits = append(commits, commits[2])
			store, err := New(repo, "origin", "refs/pyxis/snapshots")
			if err != nil {
				t.Fatal(err)
			}
			for _, prefix := range []string{"", "events"} {
				got, errs := store.SnapshotsAt(t.Context(), commits, prefix)
				for i, commit := range commits {
					want, wantErr := store.SnapshotAt(t.Context(), commit, prefix)
					if !reflect.DeepEqual(got[i], want) || (errs[i] == nil) != (wantErr == nil) || errs[i] != nil && errs[i].Error() != wantErr.Error() {
						t.Fatalf("prefix %q commit %d: SnapshotsAt = %+v, %v; SnapshotAt = %+v, %v", prefix, i, got[i], errs[i], want, wantErr)
					}
				}
			}
			if got, errs := store.SnapshotsAt(t.Context(), commits[1:3], ""); errs[0] != nil || errs[1] != nil || len(got[1].Files) != 4 || string(got[1].Files["events/inner/tool"]) != "drive\n" {
				t.Fatalf("nested snapshot = %+v, %v", got, errs)
			}
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			if _, errs := store.SnapshotsAt(cancelled, commits[:2], ""); errs[0] == nil || errs[1] == nil {
				t.Fatalf("cancelled SnapshotsAt = %v", errs)
			}
		})
	}
}

func TestParseTreeAcceptsOnlyCanonicalEntries(t *testing.T) {
	oid := strings.Repeat("ab", 20)
	raw := string(bytes.Repeat([]byte{0xab}, 20))
	tree := func(body string) batchObject { return batchObject{Object: oid, Type: "tree", Body: []byte(body)} }
	got, ok := parseTree(tree("100644 a\x00" + raw + "40000 b\x00" + raw + "160000 c\x00" + raw))
	want := []treeEntry{{name: "a", object: oid}, {name: "b", object: oid, tree: true}, {name: "c", object: oid}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTree = %+v, %t", got, ok)
	}
	if got, ok := parseTree(tree("")); !ok || len(got) != 0 {
		t.Fatalf("empty tree = %+v, %t", got, ok)
	}
	for name, object := range map[string]batchObject{
		"not a tree":     {Object: oid, Type: "blob"},
		"no mode":        tree("100644"),
		"no name end":    tree("100644 a"),
		"short id":       tree("100644 a\x00" + raw[:19]),
		"empty name":     tree("100644 \x00" + raw),
		"dot":            tree("100644 .\x00" + raw),
		"dot dot":        tree("40000 ..\x00" + raw),
		"slash":          tree("100644 a/b\x00" + raw),
		"duplicate":      tree("100644 a\x00" + raw + "40000 a\x00" + raw),
		"noncanonical":   tree("100664 a\x00" + raw),
		"padded subtree": tree("040000 a\x00" + raw),
	} {
		if _, ok := parseTree(object); ok {
			t.Fatalf("%s: parseTree accepted %q", name, object.Body)
		}
	}
}
