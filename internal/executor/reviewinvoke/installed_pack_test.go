package reviewinvoke

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	reviewruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// Exercise the trusted host command in a private root with no Go module,
// installed binary cache or PATH. Proposal code and declarations cannot select
// the assembler or alter the five-source mapping compiled into the host.
func TestReviewBriefBuildsCanonicalPackWithoutSource(t *testing.T) {
	brief := reviewDebugBrief(t.TempDir(), "/trusted-host")
	const fence = "```yaml\n"
	_, declaration, found := strings.Cut(brief, fence)
	if !found {
		t.Fatal("source-less review brief has no supplied declaration")
	}
	declaration, _, found = strings.Cut(declaration, "```")
	if !found {
		t.Fatal("supplied declaration is incomplete")
	}
	root := t.TempDir()
	for _, dir := range []string{".pyxis", "docs/prds", "review"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	inputs := pack.ReviewInputs{Subject: "github:owner/repo#1", Proposal: []byte("proposal\n"), CodegateVerdict: []byte("green\n"), Charter: []byte("charter\n"), DesignBindings: []byte("design\n"), Calibration: []byte("calibration\n")}
	files := map[string][]byte{
		".pyxis/config.yaml":        {},
		".pyxis/context-pack.yaml":  []byte(declaration),
		"review/proposal.diff":      inputs.Proposal,
		"review/codegate.txt":       inputs.CodegateVerdict,
		"review/charter.md":         inputs.Charter,
		"review/design-bindings.md": inputs.DesignBindings,
		"review/calibration.md":     inputs.Calibration,
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "trusted-host")
	build := exec.CommandContext(t.Context(), "go", "build", "-mod=readonly", "-o", bin, "./cmd/implement-now")
	build.Dir = repo
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build released Go command: %v: %s", err, out)
	}
	// Inputs and output may use separate private roots; the brief's evidence
	// directory does not exist until the trusted command creates it.
	outPath := filepath.Join(t.TempDir(), "review-evidence", "pack.txt")
	if err := os.WriteFile(filepath.Join(root, ".pyxis/context-pack.yaml"), []byte("impl-review:\n  - subject review/sanitized.diff\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bin, "--internal-review-pack", "--root", root, "--subject", inputs.Subject, "--out", outPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH=", "GITHUB_TOKEN=", "GH_TOKEN=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("source-less public command: %v: %s", err, out)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := pack.Build(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("installed command differs from canonical five-source builder")
	}
	ctxCmd := exec.CommandContext(t.Context(), bin, "--internal-review-context", "--reviewer-root", "native-root")
	ctxCmd.Dir, ctxCmd.Env = root, cmd.Env
	contextOut, err := ctxCmd.Output()
	wantContext, contextErr := reviewruntime.ContextID("native-root")
	if err != nil || contextErr != nil || string(contextOut) != wantContext+"\n" {
		t.Fatalf("source-less context derivation: %q %v %v", contextOut, err, contextErr)
	}
}
