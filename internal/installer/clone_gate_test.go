package installer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/installer"
)

// A hosted runner receives tracked files, so init's green proof must survive
// a fresh Git clone even before the first PRD exists. In particular, Git must
// retain the empty registry that LedgerDirs requires before running the gate.
func TestFreshInstalledClonePassesDocsGate(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	// A Git hook can put the relocatable Git binary ahead of its system
	// launcher. Resolve TestMain's restricted-PATH symlink before discovering
	// the helpers needed by the real clone transport.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err = filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	helperPath, err := exec.CommandContext(t.Context(), gitPath, "--exec-path").Output()
	if err != nil || strings.TrimSpace(string(helperPath)) == "" {
		t.Fatalf("discover Git helper path: %q, %v", helperPath, err)
	}
	t.Setenv("GIT_EXEC_PATH", strings.TrimSpace(string(helperPath)))
	r := newRelease(t)
	installed := t.TempDir()
	if _, err := installer.Init(t.Context(), r.carinaOptions(installed)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	cloned := filepath.Join(t.TempDir(), "runner-checkout")
	git(t, installed, "clone", "--no-hardlinks", installed, cloned)
	cfg, defects := config.Load(cloned)
	if len(defects) != 0 {
		t.Fatalf("cloned config: %v", defects)
	}
	dirs, err := gate.LedgerDirs(cloned, cfg)
	if err != nil {
		t.Fatalf("fresh clone lost the empty PRD registry: %v", err)
	}
	if defects := gate.Gate(cloned, "HEAD", dirs, cfg); len(defects) != 0 {
		t.Fatalf("fresh installed clone fails its real docs gate: %v", defects)
	}
}

func TestInitPreservesExistingIgnoredRegistryMarker(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	installed := t.TempDir()
	git(t, installed, "init", "-b", "main")
	writeFile(t, filepath.Join(installed, ".git/info/exclude"), []byte("docs/prds/.gitkeep\n"), 0o644)
	marker := filepath.Join(installed, "docs/prds/.gitkeep")
	const owned = "operator-owned directory marker\n"
	writeFile(t, marker, []byte(owned), 0o644)
	if _, err := installer.Init(t.Context(), r.carinaOptions(installed)); err == nil || !strings.Contains(err.Error(), "preserve empty PRD registry") {
		t.Fatalf("preexisting ignored marker was not refused: %v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != owned {
		t.Fatalf("init or its rollback changed the operator's ignored marker: %q, %v", got, err)
	}
}
