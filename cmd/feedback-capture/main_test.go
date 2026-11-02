package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

type failingCommentReader struct{}

func (failingCommentReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// fixedNow is an injected clock so the default-timestamp path is hermetic.
func fixedNow() time.Time { return time.Date(2026, 7, 13, 9, 30, 0, 0, time.UTC) }

// A-2 end-to-end: the binary reads the verbatim comment on stdin and its
// metadata from flags, and a confirmed comment produces one ledger entry
// with all fields; the binary exits 0.
func TestRunCapturesOneEntry(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "feedback")
	queue := filepath.Join(tmp, "queue")

	var out, errb bytes.Buffer
	code := run([]string{
		"--author=@reviewer-one",
		"--timestamp=2026-07-13T09:30:00Z",
		"--subject-anchor=review-feedback-capture#3.1",
		"--root=" + root,
		"--queue=" + queue,
	}, strings.NewReader("Name the concrete artifact in §3."), &out, &errb, fixedNow)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, errb.String())
	}
	dir := filepath.Join(root, "review-feedback-capture")
	b, err := os.ReadFile(filepath.Join(dir, "001-conversation.md"))
	if err != nil {
		t.Fatalf("expected an entry file: %v", err)
	}
	got, defs := feedback.ParseEntry("001-conversation.md", b)
	if len(defs) != 0 {
		t.Fatalf("entry has fold defects: %v", defs)
	}
	if got.Author != "@reviewer-one" || got.Timestamp != "2026-07-13T09:30:00Z" ||
		got.SubjectAnchor != "review-feedback-capture#3.1" || got.Source != "conversation" ||
		got.Comment != "Name the concrete artifact in §3." {
		t.Fatalf("entry field drift: %+v", got)
	}
	if !strings.Contains(out.String(), "captured:") {
		t.Errorf("expected a 'captured:' line on stdout, got %q", out.String())
	}
}

// The default timestamp comes from the injected clock when --timestamp is
// omitted — the binary stamps it (the pure core has no clock).
func TestRunDefaultsTimestampFromClock(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "feedback")
	var out, errb bytes.Buffer
	code := run([]string{
		"--author=@r", "--subject-anchor=some-doc#1",
		"--root=" + root, "--queue=" + filepath.Join(tmp, "q"),
	}, strings.NewReader("a comment"), &out, &errb, fixedNow)
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, errb.String())
	}
	b, err := os.ReadFile(filepath.Join(root, "some-doc", "001-conversation.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "2026-07-13T09:30:00Z") {
		t.Fatalf("expected the injected clock's timestamp in:\n%s", b)
	}
}

// UB-1 at the binary boundary: a simulated write failure exits 0 (blocks no
// review) and queues the comment (loses none). The ledger root is a regular
// file, so every write fails.
func TestRunFailsOpenAndQueues(t *testing.T) {
	tmp := t.TempDir()
	fileRoot := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(tmp, "queue")

	var out, errb bytes.Buffer
	code := run([]string{
		"--author=@r", "--timestamp=2026-07-13T09:30:00Z",
		"--subject-anchor=some-doc#1", "--root=" + fileRoot, "--queue=" + queue,
	}, strings.NewReader("a comment that must not be lost"), &out, &errb, fixedNow)

	if code != 0 {
		t.Fatalf("a write failure must not block the review: exit = %d", code)
	}
	entries, err := os.ReadDir(queue)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one queued comment, got %d (err=%v)", len(entries), err)
	}
	if !strings.Contains(errb.String(), "queued for retry") {
		t.Errorf("expected a 'queued for retry' notice on stderr, got %q", errb.String())
	}
}

// A malformed flag set exits non-zero (a shim-authoring bug), but that path
// never runs on the review critical path — the shim passes well-formed flags.
func TestRunRejectsBadFlags(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--nonsense"}, strings.NewReader(""), &out, &errb, fixedNow); code == 0 {
		t.Fatal("expected a non-zero exit on an unknown flag")
	}
}

func TestRunFailsOpenWhenCommentCannotBeRead(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, failingCommentReader{}, &out, &errb, fixedNow); code != 0 {
		t.Fatalf("read failure exit = %d", code)
	}
	if !strings.Contains(errb.String(), "read comment") || !strings.Contains(errb.String(), "review not blocked") {
		t.Fatalf("read failure diagnostic = %q", errb.String())
	}
}

func TestMainDelegatesUsageExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"feedback-capture", "--bad"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 2 {
		t.Fatalf("main exit = %d, want 2", got)
	}
}

func TestRunFailsOpenWhenLedgerAndQueueBothFail(t *testing.T) {
	root := t.TempDir()
	ledgerFile := filepath.Join(root, "ledger")
	queueFile := filepath.Join(root, "queue")
	for _, path := range []string{ledgerFile, queueFile} {
		if err := os.WriteFile(path, []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--author=@r", "--subject-anchor=doc#1", "--root=" + ledgerFile, "--queue=" + queueFile}, strings.NewReader("comment"), &stdout, &stderr, fixedNow)
	if code != 0 || !strings.Contains(stderr.String(), "queueing failed") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
