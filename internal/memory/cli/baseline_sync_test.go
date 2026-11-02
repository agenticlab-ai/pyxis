package cli

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestPolicySynchronizationPreflightRefusalsDoNotWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		prepare func(*harness) string
		migrate bool
		want    string
	}{
		{name: "absent policy", prepare: func(h *harness) string {
			h.git("rm", "-q", reconcile.PolicyPath)
			h.commit("remove marker")
			return "HEAD"
		}, migrate: true, want: "cannot bootstrap"},
		{name: "migration flag required", prepare: func(*harness) string { return "HEAD" }, want: "explicit --migrate-v2"},
		{name: "no retained mainline", prepare: func(h *harness) string { h.git("branch", "-m", "topic"); return "HEAD" }, migrate: true, want: "retained trusted mainline"},
		{name: "topic is not mainline", prepare: func(h *harness) string {
			h.git("checkout", "-q", "-b", "topic")
			h.git("commit", "--allow-empty", "-q", "-m", "advance")
			return "HEAD"
		}, migrate: true, want: "only at the trusted mainline"},
		{name: "missing ref", prepare: func(*harness) string { return "absent" }, migrate: true, want: "does not name a commit"},
		{name: "bad marker", prepare: func(h *harness) string {
			writeFixtureFile(t, h.dir, reconcile.PolicyPath, "{}", 0644)
			h.commit("bad marker")
			return "HEAD"
		}, migrate: true, want: "required field"},
		{name: "inventory changed", prepare: func(h *harness) string {
			h.ok("reconcile", "--write")
			h.commit("reduce legacy debt before migration")
			return "HEAD"
		}, migrate: true, want: "preserve the exact current v1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := legacyPolicyFixture(t)
			ref := tc.prepare(h)
			before := policyFilesOnDisk(t, h)
			env, _, _ := h.env("")
			code, err := baselineSync(t.Context(), env, ref, "", tc.migrate, false, nil)
			if code != exitRefused || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("preflight: %d %v", code, err)
			}
			if !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
				t.Fatal("failed preflight changed policy")
			}
		})
	}
	t.Run("immutable record", func(t *testing.T) {
		h, _ := partitionedPolicyFixture(t)
		id := h.add("--kind", "Learning", "--subject", "meta/x", "--statement", "original")
		h.commit("trusted record")
		base := h.git("rev-parse", "HEAD")
		name := filepath.Join(h.dir, "memory/meta/x", id+".json")
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, bytes.ReplaceAll(body, []byte("original"), []byte("forged")), 0644); err != nil {
			t.Fatal(err)
		}
		h.commit("rewrite immutable record")
		before := policyFilesOnDisk(t, h)
		code, _, errout := h.run("reconcile", "--baseline-sync", "--baseline-ref", base)
		if code != exitRefused || !strings.Contains(errout, "record transition") || !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
			t.Fatalf("immutable bypass: %d %s", code, errout)
		}
	})
	t.Run("missing repo", func(t *testing.T) {
		h := newHarness(t, t.TempDir())
		env, _, _ := h.env("")
		if _, err := baselineSync(t.Context(), env, "HEAD", "", true, false, nil); err == nil {
			t.Fatal("missing repository accepted")
		}
	})
}

func TestPolicyWorkingFilesRefuseRedirectsAndUnavailableIO(t *testing.T) {
	t.Parallel()
	allowance := reconcile.BaselineAllowance{Code: "gap", Scope: "scope", Count: 1}
	member, err := reconcile.AllowancePath(allowance)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "marker-symlink", "member-symlink", "member-directory", "root-file", "unexpected-member", "unreadable-member", "unreadable-directory", "closed"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer closePolicyRoot(root)
			if kind == "missing" {
				got, err := workingPolicyFiles(root)
				if err != nil || len(got) != 0 {
					t.Fatalf("absent policy: %v %v", got, err)
				}
				if _, err := workingPolicyFile(root, member); err != nil {
					t.Fatal(err)
				}
				if err := atomicPolicyFile(root, member, []byte("first")); err != nil {
					t.Fatal(err)
				}
				if err := atomicPolicyFile(root, member, []byte("second")); err != nil {
					t.Fatal(err)
				}
				got, err = workingPolicyFile(root, member)
				if err != nil || string(got[member]) != "second" {
					t.Fatalf("atomic replacement: %v %v", got, err)
				}
				return
			}
			writeFixtureFile(t, dir, member, "member", 0644)
			writeFixtureFile(t, dir, reconcile.PolicyPath, "marker", 0644)
			switch kind {
			case "marker-symlink", "member-symlink":
				name := member
				if kind == "marker-symlink" {
					name = reconcile.PolicyPath
				}
				if err := os.Remove(filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../outside", filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
			case "member-directory":
				if err := os.Remove(filepath.Join(dir, member)); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, member), 0755); err != nil {
					t.Fatal(err)
				}
			case "root-file":
				if err := os.RemoveAll(filepath.Join(dir, reconcile.PolicyDirectory)); err != nil {
					t.Fatal(err)
				}
				writeFixtureFile(t, dir, reconcile.PolicyDirectory, "file", 0644)
			case "unexpected-member":
				writeFixtureFile(t, dir, reconcile.PolicyDirectory+"/foreign.txt", "keep", 0644)
			case "unreadable-member":
				if err := os.Chmod(filepath.Join(dir, member), 0000); err != nil {
					t.Fatal(err)
				}
			case "unreadable-directory":
				if err := os.Chmod(filepath.Join(dir, reconcile.PolicyDirectory), 0100); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(filepath.Join(dir, reconcile.PolicyDirectory), 0755); err != nil {
						t.Error(err)
					}
				})
			case "closed":
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := workingPolicyFiles(root); err == nil {
				t.Fatalf("accepted unsafe/unavailable policy %s", kind)
			}
			if kind == "member-symlink" || kind == "member-directory" || kind == "unreadable-member" || kind == "closed" || kind == "root-file" {
				if _, err := workingPolicyFile(root, member); err == nil {
					t.Fatalf("unsafe direct member read: %s", kind)
				}
			}
			if kind == "closed" || kind == "root-file" {
				if err := atomicPolicyFile(root, member, []byte("must not write")); err == nil {
					t.Fatal("unsafe atomic write")
				}
			}
		})
	}
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	if _, err := policyParent(root, "../outside", true); err == nil {
		t.Fatal("unowned policy parent accepted")
	}
}

func TestPolicyPlanDetectsInterveningChangesAndOutputFailure(t *testing.T) {
	t.Parallel()
	h, base := legacyPolicyFixture(t)
	before := policyFilesOnDisk(t, h)
	env, out, _ := h.env("")
	code, err := baselineSync(t.Context(), env, base, "", true, true, func(index int, name string) error {
		if index == 1 {
			writeFixtureFile(t, h.dir, name, "concurrent policy edit", 0644)
		}
		return nil
	})
	if code != exitRefused || err == nil || !strings.Contains(err.Error(), "changed during synchronization") {
		t.Fatalf("intervening change: %d %v", code, err)
	}
	var receipt policySyncOutput
	decodeJSON(t, out.String(), &receipt)
	if len(receipt.Written) != 1 || len(receipt.Completed) != 1 {
		t.Fatalf("partial paths: %+v", receipt)
	}
	if got := policyFilesOnDisk(t, h)[reconcile.PolicyPath]; string(got) != "concurrent policy edit" {
		t.Fatal("overwrote intervening edit")
	}
	writeFixtureFile(t, h.dir, reconcile.PolicyPath, string(before[reconcile.PolicyPath]), 0644)
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
	decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2", "--json"), &receipt)
	if len(receipt.Written) != 0 {
		t.Fatal("completed retry wrote policy again")
	}
	for _, asJSON := range []bool{false, true} {
		if err := writePolicySyncOutput(Env{Stdout: rejectingWriter{errors.New("output unavailable")}}, asJSON, receipt); err == nil {
			t.Fatal("lost sync output swallowed")
		}
	}
	repo, err := gitsrc.Open(t.Context(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := &memory{repo: repo}
	root, err := os.OpenRoot(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePolicyRoot(root)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := preflightPolicyWrite(canceled, m, root, before, before); err == nil {
		t.Fatal("canceled preflight accepted")
	}
	if err := applyPolicyWrite(canceled, m, root, base, before, before, []policyChange{{Path: reconcile.PolicyPath}}, &receipt, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write: %v", err)
	}
	if _, err := workingPolicyFile(root, reconcile.PolicyDirectory+"/"+strings.Repeat("0", 64)+".json"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestPolicySyncReportsLostReceiptAfterCompletedWrite(t *testing.T) {
	t.Parallel()
	h, base := legacyPolicyFixture(t)
	env, _, _ := h.env("")
	env.Stdout = rejectingWriter{err: errors.New("receipt lost")}
	code, err := baselineSync(t.Context(), env, base, "", true, true, nil)
	if code != exitRefused || err == nil || !strings.Contains(err.Error(), "receipt lost") {
		t.Fatalf("lost receipt reported success: %d %v", code, err)
	}
	var retry policySyncOutput
	decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2", "--json"), &retry)
	if len(retry.Written) != 0 || retry.Status != "complete" {
		t.Fatalf("completed write was repeated: %+v", retry)
	}
}

func TestPolicySynchronizationWriteFailurePreservesInputAndCanRetry(t *testing.T) {
	t.Parallel()
	h, base := legacyPolicyFixture(t)
	dir := filepath.Join(h.dir, reconcile.PolicyDirectory)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0755); err != nil {
			t.Error(err)
		}
	})
	before := policyFilesOnDisk(t, h)
	env, out, _ := h.env("")
	code, err := baselineSync(t.Context(), env, base, "", true, true, func(index int, _ string) error {
		if index == 0 {
			return os.Chmod(dir, 0555)
		}
		return nil
	})
	if code != exitRefused || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("unwritable policy directory: %d %v", code, err)
	}
	var receipt policySyncOutput
	decodeJSON(t, out.String(), &receipt)
	if receipt.Status != "incomplete" || len(receipt.Completed) != 0 || len(receipt.Written) != 0 {
		t.Fatalf("failed write reported progress: %+v", receipt)
	}
	if !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
		t.Fatal("failed write changed policy inputs")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2", "--json"), &receipt)
	if receipt.Status != "complete" || len(receipt.Completed) != len(receipt.Planned) || len(receipt.Written) == 0 {
		t.Fatalf("restored permissions did not allow retry: %+v", receipt)
	}
}
