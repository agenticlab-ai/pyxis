package localgit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestCheckRefFormatRemembersOnlyAcceptedNames(t *testing.T) {
	o := gitFixture(t, false)
	ref := "refs/heads/check-ref-format-cache-" + strings.ReplaceAll(t.Name(), "/", "-")
	if _, cached := acceptedRefs.Load(ref); cached {
		t.Fatal("fixture ref is already cached")
	}
	if err := checkRefFormat(t.Context(), o.Root, ref); err != nil {
		t.Fatalf("valid ref refused: %v", err)
	}
	if _, cached := acceptedRefs.Load(ref); !cached {
		t.Fatal("accepted ref was not remembered")
	}
	if err := checkRefFormat(t.Context(), o.Root, ref); err != nil {
		t.Fatalf("remembered ref refused: %v", err)
	}
	if err := checkRefFormat(t.Context(), o.Root, "refs/heads/bad..ref"); err == nil {
		t.Fatal("invalid ref accepted")
	}
	if _, cached := acceptedRefs.Load("refs/heads/bad..ref"); cached {
		t.Fatal("refused ref was remembered")
	}
	// A remembered name still fails where Git itself could not run.
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := checkRefFormat(cancelled, o.Root, ref); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled check = %v", err)
	}
	if err := checkRefFormat(t.Context(), filepath.Join(o.Root, "missing"), ref); err == nil {
		t.Fatal("check in a missing root succeeded")
	}
}

func TestReadCatObjectsParsesBatchAnswers(t *testing.T) {
	oid := strings.Repeat("a", 40)
	read := func(out string, limit, count int) ([]catObject, bool, error) {
		return readCatObjects(bufio.NewReader(strings.NewReader(out)), limit, count)
	}
	got, complete, err := read(oid+" blob 3\nabc\nname missing\n"+oid+" tree 0\n\n", 8, 3)
	want := []catObject{{Type: "blob", Size: 3, Body: []byte("abc")}, {}, {Type: "tree", Size: 0, Body: []byte{}}}
	if err != nil || !complete || !reflect.DeepEqual(got, want) {
		t.Fatalf("answers = %+v, %t, %v", got, complete, err)
	}
	if got, complete, err := read("", 8, 0); err != nil || !complete || len(got) != 0 {
		t.Fatalf("no specs = %+v, %t, %v", got, complete, err)
	}
	// An oversized object ends the read before its body.
	got, complete, err = read(oid+" blob 9\n", 8, 2)
	if err != nil || complete || !reflect.DeepEqual(got, []catObject{{Type: "blob", Size: 9}}) {
		t.Fatalf("oversized answer = %+v, %t, %v", got, complete, err)
	}
	for name, out := range map[string]string{
		"no answer":         "",
		"unparsable size":   oid + " blob x\n",
		"negative size":     oid + " blob -1\n",
		"short body":        oid + " blob 3\nab",
		"unterminated body": oid + " blob 3\nabcd",
	} {
		if _, _, err := read(out, 8, 1); !errors.Is(err, errCatFileResponse) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestCatObjectsReadsFromOneProcess(t *testing.T) {
	o := gitFixture(t, true)
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	tree := fixtureGit(t, o.Root, "rev-parse", "HEAD^{tree}")
	got, err := catObjects(t.Context(), o.Origin, 1<<20, head, head+":absent", tree)
	if err != nil || len(got) != 3 || got[0].Type != "commit" || !bytes.HasPrefix(got[0].Body, []byte("tree "+tree+"\n")) || got[1].Type != "" || got[2].Type != "tree" {
		t.Fatalf("answers = %+v, %v", got, err)
	}
	// A bound below the first object stops and reaps the process.
	got, err = catObjects(t.Context(), o.Origin, 1, head, tree)
	if err != nil || len(got) != 1 || got[0].Type != "commit" || got[0].Body != nil {
		t.Fatalf("bounded answers = %+v, %v", got, err)
	}
	// So does an oversized last answer, whose body Git is still writing.
	got, err = catObjects(t.Context(), o.Origin, 1, head)
	if err != nil || len(got) != 1 || got[0].Type != "commit" || got[0].Body != nil {
		t.Fatalf("bounded last answer = %+v, %v", got, err)
	}
	if _, err := catObjects(t.Context(), o.Origin, 1<<20, head+"\n"+tree); err == nil {
		t.Fatal("multi-line object name accepted")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := catObjects(cancelled, o.Origin, 1<<20, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read = %v", err)
	}
	if _, err := catObjects(t.Context(), t.TempDir(), 1<<20, head); !errors.Is(err, errCatFileResponse) {
		t.Fatalf("read outside a repository = %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := catObjects(t.Context(), o.Origin, 1<<20, head); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing Git = %v", err)
	}
}

func TestProposalRecordRefusesLikeSeparateReads(t *testing.T) {
	o := gitFixture(t, true)
	commit := func(t *testing.T, tree string) string {
		t.Helper()
		cmd := exec.Command("git", "commit-tree", tree, "-m", "record")
		cmd.Dir = o.Origin
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=f", "GIT_AUTHOR_EMAIL=f@invalid", "GIT_COMMITTER_NAME=f", "GIT_COMMITTER_EMAIL=f@invalid")
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	blob := func(t *testing.T, body []byte) string {
		t.Helper()
		cmd := exec.Command("git", "hash-object", "-w", "--stdin")
		cmd.Dir = o.Origin
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	mktree := func(t *testing.T, entry string) string {
		t.Helper()
		cmd := exec.Command("git", "mktree")
		cmd.Dir = o.Origin
		cmd.Stdin = strings.NewReader(entry)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	record := blob(t, []byte("{}\n"))
	empty := mktree(t, "")
	valid := commit(t, mktree(t, "100644 blob "+record+"\tproposal.json\n"))
	if body, err := proposalRecord(t.Context(), o.Origin, valid); err != nil || string(body) != "{}\n" {
		t.Fatalf("record = %q, %v", body, err)
	}
	for name, tc := range map[string]struct {
		oid, refusal string
	}{
		"not a commit":       {record, "is not a readable commit"},
		"absent record":      {commit(t, empty), "has no readable record"},
		"directory record":   {commit(t, mktree(t, "040000 tree "+empty+"\tproposal.json\n")), "too large"},
		"oversized record":   {commit(t, mktree(t, "100644 blob "+blob(t, bytes.Repeat([]byte("x"), maxTaskProposalBytes+1))+"\tproposal.json\n")), "too large"},
		"unknown commit oid": {strings.Repeat("e", len(valid)), "is not a readable commit"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := proposalRecord(t.Context(), o.Origin, tc.oid); !errors.Is(err, codehost.ErrProposalInvalid) || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("refusal = %v, want %q", err, tc.refusal)
			}
		})
	}
}
