package readiness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func sourceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func sourceFixture(t *testing.T) (string, *gitsrc.Repo, string) {
	t.Helper()
	root := t.TempDir()
	sourceGit(t, root, "init", "-b", "main")
	sourceGit(t, root, "config", "user.name", "Source Test")
	sourceGit(t, root, "config", "user.email", "source@example.test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceGit(t, root, "add", "--", "README.md")
	sourceGit(t, root, "commit", "-m", "base")
	repo, err := gitsrc.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, repo, sourceGit(t, root, "rev-parse", "HEAD")
}

func TestLoadSourcesRejectsMissingInputsAndInvalidBase(t *testing.T) {
	_, repo, base := sourceFixture(t)
	if _, err := LoadSources(t.Context(), nil, base, config.Defaults()); err == nil || !strings.Contains(err.Error(), "repository and mainline commit") {
		t.Fatalf("nil repository: %v", err)
	}
	if _, err := LoadSources(t.Context(), repo, "", config.Defaults()); err == nil || !strings.Contains(err.Error(), "repository and mainline commit") {
		t.Fatalf("empty base: %v", err)
	}
	if sources, err := LoadSources(t.Context(), repo, base, config.Defaults()); err != nil || sources.Mainline == nil || len(sources.Documents) != 0 {
		t.Fatalf("empty mainline sources: %+v, %v", sources, err)
	}
	if _, err := LoadSources(t.Context(), repo, "absent", config.Defaults()); err == nil || !strings.Contains(err.Error(), "load mainline memory") {
		t.Fatalf("invalid base: %v", err)
	}
}

func TestLoadSourcesRejectsNonregularMainlineDocument(t *testing.T) {
	root, repo, _ := sourceFixture(t)
	path := filepath.Join(root, "docs", "prds", "linked.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../README.md", path); err != nil {
		t.Fatal(err)
	}
	sourceGit(t, root, "add", "--", "docs/prds/linked.md")
	sourceGit(t, root, "commit", "-m", "add document link")
	base := sourceGit(t, root, "rev-parse", "HEAD")
	if _, err := LoadSources(t.Context(), repo, base, config.Defaults()); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("linked document: %v", err)
	}
}

func TestLoadSourcesReadsPinnedAuthorityContract(t *testing.T) {
	root, repo, _ := sourceFixture(t)
	path := filepath.Join(root, schema.AuthorityContractPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("pinned authority\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(root, "docs", "prds", "example.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docPath, []byte("# Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "prds", "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceGit(t, root, "add", "--", schema.AuthorityContractPath, "docs/prds/example.md", "docs/prds/ignored.txt")
	sourceGit(t, root, "commit", "-m", "add authority")
	base := sourceGit(t, root, "rev-parse", "HEAD")
	sources, err := LoadSources(t.Context(), repo, base, config.Defaults())
	if err != nil || string(sources.Documents[schema.AuthorityContractPath]) != "pinned authority\n" || string(sources.Documents["docs/prds/example.md"]) != "# Example\n" || sources.Mainline == nil {
		t.Fatalf("sources=%+v error=%v", sources, err)
	}
}

func TestLoadSourcesRejectsBrokenMainlineMemory(t *testing.T) {
	root, repo, _ := sourceFixture(t)
	path := filepath.Join(root, "memory", "broken.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{bad json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceGit(t, root, "add", "--", "memory/broken.json")
	sourceGit(t, root, "commit", "-m", "add broken memory")
	base := sourceGit(t, root, "rev-parse", "HEAD")
	if _, err := LoadSources(t.Context(), repo, base, config.Defaults()); err == nil || !strings.Contains(err.Error(), "unresolved store defects") {
		t.Fatalf("broken mainline memory: %v", err)
	}
}
