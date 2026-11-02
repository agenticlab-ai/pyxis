package runtime

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

func TestCaptureOutputPreservesStreamsWithoutStorage(t *testing.T) {
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".pyxis"), []byte("existing content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", blocked} {
		var out, diagnostic bytes.Buffer
		stdout, stderr, closeLogs := CaptureOutput(t.Context(), root, "codex", &out, &diagnostic)
		if _, err := stdout.Write([]byte("response")); err != nil {
			t.Fatal(err)
		}
		if _, err := stderr.Write([]byte("diagnostic")); err != nil {
			t.Fatal(err)
		}
		closeLogs()
		if out.String() != "response" || diagnostic.String() != "diagnostic" {
			t.Fatal("storage failure changed the child output streams")
		}
	}
}

func TestCaptureOutputWarnsOnceAndContinuesAfterSinkFails(t *testing.T) {
	var output, warnings bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&warnings, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	stdout, _, closeLogs := CaptureOutput(t.Context(), t.TempDir(), "codex", &output, &output)
	// Closing the file injects a real I/O failure while the child-facing
	// destination remains writable, as with storage loss during a run.
	if err := stdout.(*outputStream).file.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if n, err := stdout.Write([]byte("response")); n != len("response") || err != nil {
			t.Fatalf("storage failure reached the child: n=%d err=%v", n, err)
		}
	}
	if output.String() != "responseresponse" {
		t.Fatal(output.String())
	}
	if n := strings.Count(warnings.String(), "cannot append agent output"); n != 1 {
		t.Fatalf("wanted one storage warning, got %d: %s", n, warnings.String())
	}
	closeLogs()
}

func TestCaptureOutputSurvivesChildExitAndSeparatesStreams(t *testing.T) {
	root := t.TempDir()
	var combined bytes.Buffer
	var reference OutputReference
	ctx := WithOutputObserver(t.Context(), func(ref OutputReference) { reference = ref })
	stdout, stderr, closeLogs := CaptureOutput(ctx, root, "codex", &combined, &combined)
	if reference.Harness != "codex" || reference.Stdout == "" || reference.Stderr == "" || reference.Stdout == reference.Stderr {
		t.Fatalf("missing private log links: %+v", reference)
	}
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "printf response; printf diagnostic >&2; exit 7")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("expected child exit 7")
	}
	closeLogs()
	if !strings.Contains(combined.String(), "response") || !strings.Contains(combined.String(), "diagnostic") {
		t.Fatal(combined.String())
	}
	for name, want := range map[string]string{"stdout.log": "response", "stderr.log": "diagnostic"} {
		files, err := filepath.Glob(filepath.Join(root, runlogs.Agents, "codex", "*", name))
		if err != nil || len(files) != 1 {
			t.Fatalf("%s: %v %v", name, files, err)
		}
		got, err := os.ReadFile(files[0])
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q %v", name, got, err)
		}
	}
}
