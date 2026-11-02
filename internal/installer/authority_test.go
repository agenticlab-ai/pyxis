package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func authoritySeedRun(t *testing.T) *initRun {
	t.Helper()
	root := t.TempDir()
	template, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(payload.AuthorityContractTemplate)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(payload.AuthorityContractTemplate))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, template, 0o644); err != nil {
		t.Fatal(err)
	}
	return &initRun{opts: Options{Target: root}, repo: config.Defaults(), j: &journal{target: root}}
}

func TestAuthoritySeedBoundaryFailuresPreserveTarget(t *testing.T) {
	for _, kind := range []string{"missing root", "cancelled", "unreadable parent", "unwritable parent", "unwritable policy directory"} {
		t.Run(kind, func(t *testing.T) {
			if strings.HasPrefix(kind, "un") && os.Geteuid() == 0 {
				t.Skip("permission boundaries require an unprivileged process")
			}
			run := authoritySeedRun(t)
			ctx := context.Background()
			parent := filepath.Join(run.opts.Target, ".pyxis")
			switch kind {
			case "missing root":
				run.opts.Target = filepath.Join(t.TempDir(), "absent")
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "unwritable policy directory":
				parent = filepath.Join(run.opts.Target, ".pyxis", "skills", "argus")
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(kind, "un") {
				mode := os.FileMode(0o555)
				if kind == "unreadable parent" {
					mode = 0
				}
				if err := os.Chmod(parent, mode); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(parent, 0o755); err != nil {
						t.Error(err)
					}
				})
			}
			if kind != "unwritable parent" && kind != "unwritable policy directory" {
				if err := probeAuthoritySeed(ctx, run.opts.Target); err == nil || (kind == "cancelled" && !errors.Is(err, context.Canceled)) {
					t.Fatalf("invalid authority path passed probe: %v", err)
				}
			}
			if err := seedAuthorityContract(ctx, run); err == nil || (kind == "cancelled" && !errors.Is(err, context.Canceled)) {
				t.Fatalf("invalid authority path was seeded: %v", err)
			}
			if len(run.j.entries) != 0 {
				t.Fatalf("failed boundary check journaled foreign paths: %+v", run.j.entries)
			}
		})
	}
}

func TestAuthorityRollbackCannotFollowReplacedParentOutside(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary rollback", true: "replaced parent"}[redirect], func(t *testing.T) {
			run := authoritySeedRun(t)
			run.rep = &Report{}
			ctx := context.Background()
			if _, err := gitOutput(ctx, run.opts.Target, "init"); err != nil {
				t.Fatal(err)
			}
			run.j.recordGitInit()
			if err := seedAuthorityContract(ctx, run); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(run.opts.Target, ".pyxis", "skills", "argus")
			outside := t.TempDir()
			foreign := filepath.Join(outside, "authority-contract.md")
			original := []byte("external operator policy\n")
			if err := os.WriteFile(foreign, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if redirect {
				if err := os.Rename(parent, parent+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, parent); err != nil {
					t.Fatal(err)
				}
			}
			rb := run.unwind(ctx, "seed")
			if rb.Restored == redirect {
				t.Fatalf("rollback did not report its bounded removal outcome: %+v", rb)
			}
			actual, err := os.ReadFile(foreign)
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatalf("rollback touched external policy: %q, %v", actual, err)
			}
			if redirect {
				if link, err := os.Readlink(parent); err != nil || link != outside {
					t.Fatalf("rollback removed foreign parent replacement: %q, %v", link, err)
				}
			} else if _, err := os.Lstat(parent); !os.IsNotExist(err) {
				t.Fatalf("ordinary authority rollback left created parents: %v", err)
			}
		})
	}
}

func TestAuthorityRollbackPreservesReplacedAndModifiedObjects(t *testing.T) {
	for _, kind := range []string{"removed", "replacement with same bytes", "modified in place", "changed mode", "replacement directory"} {
		t.Run(kind, func(t *testing.T) {
			run := authoritySeedRun(t)
			run.rep = &Report{}
			ctx := context.Background()
			if _, err := gitOutput(ctx, run.opts.Target, "init"); err != nil {
				t.Fatal(err)
			}
			run.j.recordGitInit()
			if err := seedAuthorityContract(ctx, run); err != nil {
				t.Fatal(err)
			}
			policyPath := filepath.Join(run.opts.Target, filepath.FromSlash(payload.AuthorityContractTarget))
			original, err := os.ReadFile(policyPath)
			if err != nil {
				t.Fatal(err)
			}
			want := original
			switch kind {
			case "removed":
				if err := os.Remove(policyPath); err != nil {
					t.Fatal(err)
				}
			case "replacement with same bytes", "replacement directory":
				if err := os.Rename(policyPath, policyPath+"-original"); err != nil {
					t.Fatal(err)
				}
				if kind == "replacement directory" {
					if err := os.Mkdir(policyPath, 0o755); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(policyPath, original, 0o644); err != nil {
					t.Fatal(err)
				}
			case "modified in place":
				want = []byte("operator changed the seeded policy\n")
				if err := os.WriteFile(policyPath, want, 0o644); err != nil {
					t.Fatal(err)
				}
			case "changed mode":
				// Removing owner write changes the mode even when umask 077 created it.
				if err := os.Chmod(policyPath, 0o400); err != nil {
					t.Fatal(err)
				}
			}
			rb := run.unwind(ctx, "hooks")
			if kind == "removed" {
				if !rb.Restored {
					t.Fatalf("already removed authority was treated as a foreign replacement: %+v", rb)
				}
				return
			}
			preservedPath := payload.AuthorityContractTarget
			if kind == "replacement directory" {
				preservedPath += "/" // rollback reports empty foreign directories with their slash
			}
			if rb.Restored || !slices.Contains(rb.ForeignPreserved, preservedPath) {
				t.Fatalf("rollback did not retain and report foreign authority: %+v", rb)
			}
			if kind == "replacement directory" {
				if info, err := os.Lstat(policyPath); err != nil || !info.IsDir() {
					t.Fatalf("rollback deleted replacement policy directory: %v, %v", info, err)
				}
			} else if actual, err := os.ReadFile(policyPath); err != nil || !bytes.Equal(actual, want) {
				t.Fatalf("rollback deleted or changed foreign policy: %q, %v", actual, err)
			}
			if kind == "changed mode" {
				if info, err := os.Stat(policyPath); err != nil || info.Mode().Perm() != 0o400 {
					t.Fatalf("rollback changed foreign policy mode: %v, %v", info, err)
				}
			}
		})
	}
}

func TestSeedAuthorityRefusesExistingPolicyObjects(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			run := authoritySeedRun(t)
			target := filepath.Join(run.opts.Target, filepath.FromSlash(payload.AuthorityContractTarget))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			original := []byte("operator-owned grant and history\n")
			switch kind {
			case "file":
				if err := os.WriteFile(target, original, 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				other := filepath.Join(t.TempDir(), "policy.md")
				if err := os.WriteFile(other, original, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, target); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := seedAuthorityContract(context.Background(), run); err == nil {
				t.Fatal("existing operator object was replaced")
			}
			if len(run.j.entries) != 0 {
				t.Fatalf("existing object was journaled for deletion: %+v", run.j.entries)
			}
			if kind != "directory" {
				actual, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(actual, original) {
					t.Fatalf("existing policy changed: %q, %v", actual, err)
				}
			}
		})
	}
}

func TestSeedAuthorityRefusesMissingInvalidTemplateAndBlockedParent(t *testing.T) {
	for _, kind := range []string{"missing template", "invalid template", "blocked parent"} {
		t.Run(kind, func(t *testing.T) {
			run := authoritySeedRun(t)
			template := filepath.Join(run.opts.Target, filepath.FromSlash(payload.AuthorityContractTemplate))
			switch kind {
			case "missing template":
				if err := os.Remove(template); err != nil {
					t.Fatal(err)
				}
			case "invalid template":
				if err := os.WriteFile(template, []byte("malformed template\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "blocked parent":
				if err := os.WriteFile(filepath.Join(run.opts.Target, ".pyxis", "skills"), []byte("preserve me\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := seedAuthorityContract(context.Background(), run); err == nil {
				t.Fatal("invalid seed input admitted")
			}
			if _, err := os.Lstat(filepath.Join(run.opts.Target, filepath.FromSlash(payload.AuthorityContractTarget))); err == nil {
				t.Fatal("failed seed created an authority")
			}
		})
	}
}

func TestStageSeedAuthorityRefusalRollsBackEarlierSkeletons(t *testing.T) {
	for _, kind := range []string{"existing policy", "invalid template"} {
		t.Run(kind, func(t *testing.T) {
			run := authoritySeedRun(t)
			run.rep = &Report{}
			run.knobs.Argus = true
			ctx := context.Background()
			if _, err := gitOutput(ctx, run.opts.Target, "init"); err != nil {
				t.Fatal(err)
			}
			run.j.recordGitInit()
			for template := range payload.SkeletonTargets(run.repo) {
				body, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(template)))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(run.payloadTemplatesDir(), filepath.Base(template)), body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			policyPath := filepath.Join(run.opts.Target, filepath.FromSlash(payload.AuthorityContractTarget))
			original := []byte("operator-owned grant and history\n")
			if kind == "existing policy" {
				if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(policyPath, original, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(run.payloadTemplatesDir(), filepath.Base(payload.AuthorityContractTemplate)), []byte("malformed template\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := stageSeed(ctx, run); err == nil {
				t.Fatal("authority refusal did not stop the seed stage")
			}
			for _, target := range payload.SkeletonTargets(run.repo) {
				if _, err := os.Stat(filepath.Join(run.opts.Target, filepath.FromSlash(target))); err != nil {
					t.Fatalf("earlier skeleton %s was not created before refusal: %v", target, err)
				}
			}
			rollback := run.unwind(ctx, "seed")
			if !rollback.Restored || !rollback.RepoRemoved {
				t.Fatalf("seed refusal did not restore the target: %+v", rollback)
			}
			for _, target := range payload.SkeletonTargets(run.repo) {
				if _, err := os.Lstat(filepath.Join(run.opts.Target, filepath.FromSlash(target))); !os.IsNotExist(err) {
					t.Fatalf("failed initialization left skeleton %s: %v", target, err)
				}
			}
			if kind == "existing policy" {
				actual, err := os.ReadFile(policyPath)
				if err != nil || !bytes.Equal(actual, original) {
					t.Fatalf("rollback changed existing policy: %q, %v", actual, err)
				}
				info, err := os.Stat(policyPath)
				if err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("rollback changed existing policy mode: %v, %v", info, err)
				}
				if !slices.Contains(rollback.ForeignPreserved, payload.AuthorityContractTarget) {
					t.Fatalf("rollback omitted the preserved policy: %+v", rollback)
				}
			} else if _, err := os.Lstat(policyPath); !os.IsNotExist(err) {
				t.Fatalf("invalid template left an authority contract: %v", err)
			}
		})
	}
}
