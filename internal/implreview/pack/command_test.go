package pack

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	reviewruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

type failedContextWriter struct{}

func (failedContextWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }

func TestTrustedContextCommandUsesExistingProvenance(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if code := RunContextIDCommand(t.Context(), []string{"--reviewer-root", "native-reviewer-root"}, &out, &diagnostic); code != 0 {
		t.Fatalf("code=%d: %s", code, &diagnostic)
	}
	want, err := reviewruntime.ContextID("native-reviewer-root")
	if err != nil || out.String() != want+"\n" {
		t.Fatalf("context=%q want=%q err=%v", out.String(), want, err)
	}
	for _, args := range [][]string{nil, {"--reviewer-root", " "}, {"--unknown"}, {"--reviewer-root", "root", "extra"}} {
		if code := RunContextIDCommand(t.Context(), args, &out, &diagnostic); code == 0 {
			t.Fatal("invalid invocation accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := RunContextIDCommand(ctx, []string{"--reviewer-root", "root"}, &out, &diagnostic); code != 1 {
		t.Fatalf("cancelled code=%d", code)
	}
	if code := RunContextIDCommand(t.Context(), []string{"--reviewer-root", "root"}, failedContextWriter{}, &diagnostic); code != 1 {
		t.Fatalf("output failure code=%d", code)
	}
}

func privateInputs(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	in := goldenInputs()
	for _, file := range []struct {
		path string
		body []byte
	}{
		{proposalPath, in.Proposal}, {verdictPath, in.CodegateVerdict},
		{charterPath, in.Charter}, {designPath, in.DesignBindings}, {calibrationPath, in.Calibration},
	} {
		if err := writeSource(root, file.path, file.body); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestTrustedCommandIgnoresProposalAssemblerAndDeclaration(t *testing.T) {
	root := privateInputs(t)
	for _, file := range []string{"cmd/docgate/main.go", ".pyxis/context-pack.yaml"} {
		if err := writeSource(root, file, []byte("sanitized proposal data; do not use this")); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "review-evidence", "pack.txt")
	var diagnostic bytes.Buffer
	if code := RunCommand(t.Context(), []string{"--root", root, "--subject", goldenInputs().Subject, "--out", out}, &diagnostic); code != 0 {
		t.Fatalf("code=%d: %s", code, &diagnostic)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Build(goldenInputs())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("proposal code or declaration changed the canonical pack")
	}
	dir, err := os.Stat(filepath.Dir(out))
	if err != nil || !dir.IsDir() || dir.Mode().Perm() != 0o700 {
		t.Fatalf("private output directory: %v, %v", dir, err)
	}
}

func TestTrustedCommandFailsClosed(t *testing.T) {
	root := privateInputs(t)
	out := filepath.Join(t.TempDir(), "retained")
	if err := os.WriteFile(out, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"--unknown"}, {"--root", root, "--subject", "s", "--out", out, "extra"}} {
		if code := RunCommand(t.Context(), args, &bytes.Buffer{}); code != 2 {
			t.Fatalf("bad args code=%d", code)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := BuildFromRoot(ctx, root, "s"); err == nil {
		t.Fatal("cancelled build accepted")
	}
	if _, err := BuildFromRoot(t.Context(), filepath.Join(root, "missing"), "s"); err == nil {
		t.Fatal("missing root accepted")
	}
	if code := RunCommand(t.Context(), []string{"--root", root, "--subject", "s", "--out", root}, &bytes.Buffer{}); code != 1 {
		t.Fatal("invalid output accepted")
	}
	if code := RunCommand(t.Context(), []string{"--root", root, "--subject", "s", "--out", filepath.Join(out, "pack.txt")}, &bytes.Buffer{}); code != 1 {
		t.Fatal("non-directory output parent accepted")
	}
	if err := os.Remove(filepath.Join(root, proposalPath)); err != nil {
		t.Fatal(err)
	}
	if code := RunCommand(t.Context(), []string{"--root", root, "--subject", "s", "--out", out}, &bytes.Buffer{}); code != 1 {
		t.Fatal("missing proposal accepted")
	}
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "retained" {
		t.Fatalf("failure replaced output: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, proposalPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildFromRoot(t.Context(), root, "s"); err == nil {
		t.Fatal("input escaped private root")
	}
}
