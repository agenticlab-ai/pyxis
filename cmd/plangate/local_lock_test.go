package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
)

func TestDraftPlanningPreservesLegacyAndHostedLockReaders(t *testing.T) {
	for _, binding := range []struct{ name, body string }{
		{"legacy", ""},
		{"hosted", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n"},
	} {
		t.Run(binding.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(binding.body), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", t.TempDir())
			calls := 0
			reader := func(_ context.Context, dir string, _ []string, program string, args ...string) ([]byte, error) {
				calls++
				if dir != root || program != "git" || len(args) == 0 || args[0] != "ls-remote" {
					t.Fatalf("unexpected lock read: %s %s %v", dir, program, args)
				}
				return nil, errors.New("lock reader stopped before transport")
			}
			basis, defects := configuredPlanningPrecheck(t.Context(), root, "widget", reader)
			if basis != "" || calls != 1 || len(defects) != 1 || defects[0].Op != "precheck/locked-readiness" || !strings.Contains(defects[0].Reason, "lock reader stopped before transport") {
				t.Fatalf("%s draft planning did not use its existing lock reader: basis=%q calls=%d defects=%v", binding.name, basis, calls, defects)
			}
		})
	}
}

func TestDocumentPlanningDoesNotRequireLocalOrigin(t *testing.T) {
	for _, state := range []string{"empty origin", "unavailable origin", "mismatched origin", "base ledger present"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			root, origin := filepath.Join(dir, "work"), filepath.Join(dir, "origin.git")
			localPlanningGit(t, dir, "init", "--initial-branch=trunk", root)
			if err := os.CopyFS(root, os.DirFS("../../internal/plangate/plan/testdata/precheck-clean")); err != nil {
				t.Fatal(err)
			}
			if state != "unavailable origin" {
				localPlanningGit(t, dir, "init", "--bare", "--initial-branch=trunk", origin)
			}
			remote := origin
			if state == "mismatched origin" {
				remote = filepath.Join(dir, "different.git")
			}
			localPlanningGit(t, root, "remote", "add", "upstream", remote)
			if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf("schema_version: pyxis-config/v2\nsystem:\n  mode: source\nbranch: trunk\nremote: upstream\nintegrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: %s\n", origin)
			if state == "base ledger present" {
				if err := os.WriteFile(filepath.Join(root, "docs/prds/demo/000-base.md"), []byte("# Existing document feature\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			// No Git executable is available after setup: contract readiness must
			// remain independent of origin inspection and lock verification.
			t.Setenv("PATH", t.TempDir())
			never := func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Fatal("document planning reached the Git process port")
				return nil, nil
			}
			basis, defects := configuredPlanningPrecheck(t.Context(), root, "demo", never)
			if basis != "" || len(defects) != 0 {
				t.Fatalf("document contract blocked by local origin: basis=%q defects=%v", basis, defects)
			}
		})
	}
}

func TestDocumentPlanningDoesNotResolveMalformedCodeHostBindings(t *testing.T) {
	for _, binding := range []struct{ name, body string }{
		{"missing local root", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n"},
		{"invalid code host provider", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: carina\n"},
		{"missing connection", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: forgejo\n      connection: missing\n      repository: o/r\n"},
	} {
		t.Run(binding.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../../internal/plangate/plan/testdata/precheck-clean")); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(binding.body), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", t.TempDir())
			never := func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Fatal("document planning reached the code host")
				return nil, nil
			}
			basis, defects := configuredPlanningPrecheck(t.Context(), root, "demo", never)
			if basis != "" || len(defects) != 0 {
				t.Fatalf("document contract blocked by unused binding: basis=%q defects=%v", basis, defects)
			}
			// Draft-born planning still needs its selected binding to be valid.
			basis, defects = configuredPlanningPrecheck(t.Context(), root, "widget", never)
			if basis != "" || len(defects) == 0 || defects[0].Op != "precheck/local-code-host" {
				t.Fatalf("draft planning ignored malformed binding: basis=%q defects=%v", basis, defects)
			}
		})
	}
}

func TestPlanningSelectsLocalOriginAndRefusesMissingLocks(t *testing.T) {
	localgittest.Skip(t)
	for _, state := range []string{"ready origin", "empty origin", "wrong origin", "invalid config schema"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			root, origin := filepath.Join(dir, "work"), filepath.Join(dir, "origin.git")
			localPlanningGit(t, dir, "init", "--initial-branch=trunk", root)
			localPlanningGit(t, dir, "init", "--bare", "--initial-branch=trunk", origin)
			localPlanningGit(t, root, "config", "user.name", "Local planning fixture")
			localPlanningGit(t, root, "config", "user.email", "planning@example.test")
			localPlanningGit(t, root, "remote", "add", "upstream", origin)
			if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf("schema_version: pyxis-config/v2\nsystem:\n  mode: source\nbranch: trunk\nremote: upstream\nintegrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: local-git\n      root: %s\n", origin)
			if state == "invalid config schema" {
				body = strings.Replace(body, "pyxis-config/v2", "pyxis-config/unknown", 1)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			localPlanningGit(t, root, "add", "--", ".pyxis/config.yaml")
			localPlanningGit(t, root, "commit", "-m", "configured local planning")
			if state != "empty origin" {
				localPlanningGit(t, root, "push", "upstream", "trunk")
			}
			if state == "wrong origin" {
				localPlanningGit(t, root, "remote", "set-url", "upstream", filepath.Join(dir, "missing.git"))
			}
			before := localPlanningGit(t, origin, "for-each-ref", "--format=%(refname) %(objectname)")
			basis, defects := configuredPlanningPrecheck(t.Context(), root, "widget", nil)
			if basis != "" || len(defects) == 0 {
				t.Fatalf("missing local lock accepted: %q %v", basis, defects)
			}
			if state == "ready origin" && !strings.Contains(defects[0].Reason, "no binding locked version") {
				t.Fatalf("local planning entered hosted route: %v", defects)
			}
			if state == "empty origin" && !strings.Contains(defects[0].Reason, "first configured-mainline push") {
				t.Fatalf("missing first-push remedy: %v", defects)
			}
			if localPlanningGit(t, origin, "for-each-ref", "--format=%(refname) %(objectname)") != before {
				t.Fatal("planning precheck changed origin refs")
			}
		})
	}
}

func TestPlanningPreservesHostedSelectionAndRejectsMalformedBinding(t *testing.T) {
	for _, body := range []string{"", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n", "[invalid YAML", "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: carina\n"} {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		never := func(context.Context, string, []string, string, ...string) ([]byte, error) {
			t.Fatal("invalid feature or binding reached Git")
			return nil, nil
		}
		basis, defects := configuredPlanningPrecheck(t.Context(), root, "INVALID", never)
		if basis != "" || len(defects) == 0 {
			t.Fatal("invalid planning input accepted")
		}
	}
}

func localPlanningGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
