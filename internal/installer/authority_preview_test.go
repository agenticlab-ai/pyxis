package installer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// Git admission deliberately permits ignored objects. Preview and apply must
// nevertheless refuse the same authority conflict, before writing anything.
func TestInitAuthorityConflictsRefusePreviewAndApply(t *testing.T) {
	for _, kind := range []string{"file", "dangling symlink", "directory", "blocked parent", ".pyxis symlink", "skills symlink", "argus symlink"} {
		t.Run(kind, func(t *testing.T) {
			isolateGit(t)
			// Protected verification uses an empty Git template directory. The
			// fixture owns its exclude file; git init need not create .git/info.
			t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			r := newRelease(t)
			root, outside := t.TempDir(), t.TempDir()
			git(t, root, "init", "-b", "main")
			writeFile(t, at(root, ".git/info/exclude"), []byte(".pyxis/\n"), 0o644)
			target := at(root, payload.AuthorityContractTarget)
			switch kind {
			case ".pyxis symlink":
				target = at(root, ".pyxis")
			case "skills symlink", "blocked parent":
				target = at(root, ".pyxis/skills")
			case "argus symlink":
				target = at(root, ".pyxis/skills/argus")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file", "blocked parent":
				if err := os.WriteFile(target, []byte("preserve operator content\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink(filepath.Join(outside, "absent-policy"), target); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
			}
			before, externalBefore := snap(t, root), snap(t, outside)
			argus := true
			for _, preview := range []bool{true, false} {
				opts := r.carinaOptions(root)
				opts.Preview, opts.Answers.Argus = preview, &argus
				rep, err := installer.Init(context.Background(), opts)
				if err == nil || len(rep.WouldSeed) != 0 || len(rep.T8.Seeded) != 0 {
					t.Fatalf("preview=%t admitted authority conflict: %+v, %v", preview, rep, err)
				}
				sameSnap(t, before, snap(t, root), "authority conflict refusal")
				sameSnap(t, externalBefore, snap(t, outside), "external authority directory")
			}
		})
	}
}
