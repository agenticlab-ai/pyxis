package gitdrive

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

// fixtureTemplates holds one pristine source clone, origin, and genesis per
// object format. Git builds each once; every fixture copies the files.
var fixtureTemplates = struct {
	sync.Mutex
	dir   string
	built map[string]fixtureTemplate
}{built: make(map[string]fixtureTemplate)}

type fixtureTemplate struct {
	root, sourceHead string
	genesis          gitrefstore.Genesis
}

func TestMain(m *testing.M) {
	code := m.Run()
	if fixtureTemplates.dir != "" {
		// The templates are scratch files; a failed removal leaves only
		// temporary files behind and must not change the test verdict.
		_ = os.RemoveAll(fixtureTemplates.dir)
	}
	os.Exit(code)
}

func templateFor(t *testing.T, format string) fixtureTemplate {
	t.Helper()
	fixtureTemplates.Lock()
	defer fixtureTemplates.Unlock()
	if built, ok := fixtureTemplates.built[format]; ok {
		return built
	}
	if fixtureTemplates.dir == "" {
		dir, err := os.MkdirTemp("", "featuredrive-fixtures-")
		if err != nil {
			t.Fatal(err)
		}
		fixtureTemplates.dir = dir
	}
	root := filepath.Join(fixtureTemplates.dir, format)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	repo, origin := filepath.Join(root, "work"), filepath.Join(root, "origin.git")
	git(t, root, "init", "--bare", "--object-format="+format, origin)
	git(t, root, "clone", "--quiet", origin, repo)
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("source remains unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "README")
	git(t, repo, "commit", "-m", "source baseline")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "push", "origin", "HEAD:refs/heads/main")
	built := fixtureTemplate{root: root, sourceHead: git(t, repo, "rev-parse", "HEAD")}
	raw, err := gitrefstore.New(repo, "origin", "refs/pyxis/feature-drives/sample-feature")
	if err != nil {
		t.Fatal(err)
	}
	built.genesis, err = raw.EstablishGenesis(t.Context(), "registered-initialization")
	if err != nil {
		t.Fatal(err)
	}
	fixtureTemplates.built[format] = built
	return built
}

// copyTemplate copies template's origin and clone into root, then points the
// clone at the copied origin, which its config names by absolute path.
func copyTemplate(t *testing.T, template fixtureTemplate, root string) {
	t.Helper()
	err := filepath.WalkDir(template.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(template.root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode().IsRegular():
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, body, info.Mode().Perm())
		default:
			return fmt.Errorf("fixture template holds a non-regular file %s", path)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "work", ".git", "config")
	body, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	from, to := []byte("url = "+filepath.Join(template.root, "origin.git")+"\n"), []byte("url = "+filepath.Join(root, "origin.git")+"\n")
	if bytes.Count(body, from) != 1 {
		t.Fatalf("template clone config does not name its origin once:\n%s", body)
	}
	if err := os.WriteFile(config, bytes.Replace(body, from, to, 1), 0o644); err != nil {
		t.Fatal(err)
	}
}
