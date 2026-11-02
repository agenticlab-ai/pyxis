package reviewinvoke

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustedAssemblerPathAndBrief(t *testing.T) {
	work, state := t.TempDir(), t.TempDir()
	path, err := trustedAssemblerPath(t.Context(), work, state)
	if err != nil {
		t.Fatal(err)
	}
	brief := reviewDebugBrief(state, path)
	if !strings.Contains(brief, path) || !strings.Contains(brief, "--internal-review-pack") || strings.Contains(brief, "go build") || strings.Contains(brief, ".pyxis/bin") {
		t.Fatal("reviewer must receive its exact trusted executable without proposal or cache discovery")
	}
	if !strings.Contains(brief, "--internal-review-schema") || !strings.Contains(brief, "schema_version is the envelope key") || !strings.Contains(brief, "context_id belongs directly to each round") {
		t.Fatal("installed native reviewer must receive the compiled schema and correct envelope/context hierarchy")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := trustedAssemblerPath(ctx, work, state); err == nil {
		t.Fatal("cancelled preparation accepted")
	}
	for _, root := range []string{work, state} {
		bad := filepath.Join(root, "executable")
		if err := os.WriteFile(bad, []byte("bad"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := validateAssemblerPath(bad, work, state); err == nil {
			t.Fatal("untrusted/writable executable accepted")
		}
	}
	if _, err := validateAssemblerPath(filepath.Join(work, "absent"), work, state); err == nil {
		t.Fatal("absent executable accepted")
	}
	if _, err := validateAssemblerPath(path, filepath.Join(work, "absent"), state); err == nil {
		t.Fatal("absent boundary accepted")
	}
	nonexec := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(nonexec, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateAssemblerPath(nonexec, work, state); err == nil {
		t.Fatal("nonexecutable host accepted")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateAssemblerPath(relative, work, state); err == nil {
		t.Fatal("relative executable accepted")
	}
}
