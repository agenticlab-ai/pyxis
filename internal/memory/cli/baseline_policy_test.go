package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func legacyPolicyFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	h.commit("initialize memory")
	writeFixtureFile(t, h.dir, reconcile.PolicyPath, h.ok("reconcile", "--baseline-generate"), 0644)
	h.commit("trust legacy debt")
	return h, h.git("rev-parse", "HEAD")
}

func partitionedPolicyFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h, base := legacyPolicyFixture(t)
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
	h.commit("migrate debt representation")
	h.gateOK("--base", base)
	return h, h.git("rev-parse", "HEAD")
}

func policyFilesOnDisk(t *testing.T, h *harness) map[string][]byte {
	t.Helper()
	root, err := os.OpenRoot(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	files, err := workingPolicyFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPartitionedPolicyMigrationAndUnchangedDebt(t *testing.T) {
	t.Parallel()
	h, base := legacyPolicyFixture(t)
	old, err := os.ReadFile(filepath.Join(h.dir, reconcile.PolicyPath))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := reconcile.ParseBaseline(old)
	if err != nil {
		t.Fatal(err)
	}
	out := h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2", "--json")
	var sync policySyncOutput
	decodeJSON(t, out, &sync)
	if sync.Status != "complete" || len(sync.Planned) != len(legacy.Allowances)+1 || sync.Planned[len(sync.Planned)-1].Path != reconcile.PolicyPath {
		t.Fatalf("migration plan: %+v", sync)
	}
	first := policyFilesOnDisk(t, h)
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
	if !reflect.DeepEqual(first, policyFilesOnDisk(t, h)) {
		t.Fatal("migration retry changed bytes")
	}
	h.commit("partition legacy policy")
	h.gateOK("--base", base)
	v2base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-q", "-b", "independent")
	h.ok("add", "--kind", "Learning", "--subject", "meta/independent", "--statement", "Independent durable knowledge")
	h.commit("add independent knowledge")
	h.gateOK("--base", v2base)
	out = h.ok("reconcile", "--baseline-sync", "--baseline-ref", v2base, "--json")
	decodeJSON(t, out, &sync)
	if len(sync.Planned) != 0 || !reflect.DeepEqual(first, policyFilesOnDisk(t, h)) {
		t.Fatalf("ordinary memory changed policy: %+v", sync.Planned)
	}
	code, _, errout := h.run("reconcile", "--baseline-generate")
	if code != exitRefused || !strings.Contains(errout, "--baseline-sync") {
		t.Fatalf("v2 generation: %d %s", code, errout)
	}
	var report reconciliationRatchetOutput
	decodeJSON(t, h.ok("reconcile", "--check", "--baseline", reconcile.PolicyPath, "--baseline-ref", v2base, "--json"), &report)
	if report.Policy.Schema != reconcile.PolicySchema || report.Policy.Commit != h.git("rev-parse", "HEAD") || len(report.Policy.Files) != len(first) || report.BaselineRef.Policy.Commit != v2base || report.Report.MemoryClosureSHA256 == legacy.MemoryClosureSHA256 {
		t.Fatalf("pinned evidence missing: %+v", report)
	}
	for _, event := range []string{"pull_request", "push", "schedule"} {
		h.envs["PYXIS_CI_EVENT"] = event
		h.envs["PYXIS_CI_BASE"] = v2base
		if event == "schedule" {
			h.envs["PYXIS_CI_BASE"] = ""
		}
		h.gateOK()
	}
}

func TestPartitionedPolicyTighteningAndReintroduction(t *testing.T) {
	t.Parallel()
	h, base := partitionedPolicyFixture(t)
	before := policyFilesOnDisk(t, h)
	h.ok("reconcile", "--write")
	h.commit("capture README debt")
	code, out, _ := h.gate("--base", base)
	if code != exitRefused || !strings.Contains(out, "baseline update required") {
		t.Fatalf("stale allowance passed: %d %s", code, out)
	}
	var sync policySyncOutput
	decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--json"), &sync)
	if len(sync.Planned) != 1 || sync.Planned[0].Path == reconcile.PolicyPath || sync.Planned[0].AfterSHA256 != "" {
		t.Fatalf("reduction footprint: %+v", sync.Planned)
	}
	if !bytes.Equal(before[reconcile.PolicyPath], policyFilesOnDisk(t, h)[reconcile.PolicyPath]) {
		t.Fatal("reduction rewrote marker")
	}
	h.commit("retire exact debt allowance")
	h.gateOK("--base", base)
	tightened := h.git("rev-parse", "HEAD")
	// The previously trusted files cannot be restored as a new grant.
	for name, body := range before {
		writeFixtureFile(t, h.dir, name, string(body), 0644)
	}
	h.commit("attempt allowance reintroduction")
	code, out, errout := h.gate("--base", tightened)
	if code != exitRefused || !strings.Contains(out+errout, "baseline-regression") {
		t.Fatalf("retired allowance returned: %d %s %s", code, out, errout)
	}
}

func TestPolicyWriterInterruptionAndExactRetry(t *testing.T) {
	t.Parallel()
	h, base := legacyPolicyFixture(t)
	before := policyFilesOnDisk(t, h)
	env, out, _ := h.env("")
	code, err := baselineSync(context.Background(), env, base, "", true, true, func(index int, _ string) error {
		if index == 1 {
			return errors.New("injected interruption")
		}
		return nil
	})
	if code != exitRefused || err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("interruption: %d %v", code, err)
	}
	var partial policySyncOutput
	decodeJSON(t, out.String(), &partial)
	if partial.Status != "incomplete" || len(partial.Completed) != 1 || len(partial.Planned) != 2 {
		t.Fatalf("partial receipt: %+v", partial)
	}
	if !bytes.Equal(before[reconcile.PolicyPath], policyFilesOnDisk(t, h)[reconcile.PolicyPath]) {
		t.Fatal("marker was written before its members")
	}
	code, _, _ = h.gate()
	if code != exitRefused {
		t.Fatal("partial working policy passed the gate")
	}
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
	h.commit("complete interrupted migration")
	h.gateOK("--base", base)
}

func TestPolicyWriterProtectsDirtyFilesAndMovedHead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*harness)
		want   string
	}{
		{name: "nonpolicy", mutate: func(h *harness) { writeFixtureFile(t, h.dir, "unrelated.txt", "keep me", 0644) }, want: "commit nonpolicy work"},
		{name: "marker", mutate: func(h *harness) { writeFixtureFile(t, h.dir, reconcile.PolicyPath, "arbitrary input", 0644) }, want: "differs from both"},
		{name: "staged", mutate: func(h *harness) {
			writeFixtureFile(t, h.dir, reconcile.PolicyPath, "staged input", 0644)
			h.git("add", reconcile.PolicyPath)
			h.git("checkout", "HEAD", "--", reconcile.PolicyPath)
			writeFixtureFile(t, h.dir, reconcile.PolicyPath, "staged input", 0644)
			h.git("add", reconcile.PolicyPath)
			h.git("restore", "--worktree", "--source=HEAD", reconcile.PolicyPath)
		}, want: "staged policy"},
		{name: "root symlink", mutate: func(h *harness) {
			if err := os.Symlink(t.TempDir(), filepath.Join(h.dir, reconcile.PolicyDirectory)); err != nil {
				t.Fatal(err)
			}
		}, want: "commit nonpolicy work"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, base := legacyPolicyFixture(t)
			tc.mutate(h)
			code, _, errout := h.run("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
			if code != exitRefused || !strings.Contains(errout, tc.want) {
				t.Fatalf("protected input: %d %s", code, errout)
			}
			entries, err := os.ReadDir(filepath.Join(h.dir, ".github"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".memory-policy-") {
					t.Fatal("preflight wrote a temporary file")
				}
			}
		})
	}
	h, base := legacyPolicyFixture(t)
	before := policyFilesOnDisk(t, h)
	env, _, _ := h.env("")
	code, err := baselineSync(context.Background(), env, base, "", true, false, func(index int, _ string) error {
		if index == 0 {
			h.git("commit", "--allow-empty", "-q", "-m", "head moved")
		}
		return nil
	})
	if code != exitRefused || err == nil || !strings.Contains(err.Error(), "HEAD moved") || !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
		t.Fatalf("HEAD race: %d %v", code, err)
	}
}

func TestPartitionedPolicyRefusesMigrationLaunderingAndDowngrade(t *testing.T) {
	t.Parallel()
	t.Run("data only", func(t *testing.T) {
		h, base := legacyPolicyFixture(t)
		h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--migrate-v2")
		writeFixtureFile(t, h.dir, "unrelated.txt", "not representation", 0644)
		h.commit("migration with unrelated changes")
		code, _, errout := h.gate("--base", base)
		if code != exitRefused || !strings.Contains(errout, "change only policy representation") {
			t.Fatalf("mixed migration passed: %d %s", code, errout)
		}
	})
	t.Run("downgrade", func(t *testing.T) {
		h, legacyBase := legacyPolicyFixture(t)
		old := policyFilesOnDisk(t, h)[reconcile.PolicyPath]
		h.ok("reconcile", "--baseline-sync", "--baseline-ref", legacyBase, "--migrate-v2")
		h.commit("migrate")
		base := h.git("rev-parse", "HEAD")
		if err := os.RemoveAll(filepath.Join(h.dir, reconcile.PolicyDirectory)); err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, h.dir, reconcile.PolicyPath, string(old), 0644)
		h.commit("downgrade")
		code, _, errout := h.gate("--base", base)
		if code != exitRefused || !strings.Contains(errout, "cannot downgrade") {
			t.Fatalf("downgrade passed: %d %s", code, errout)
		}
	})
	t.Run("CI base override", func(t *testing.T) {
		h, base := partitionedPolicyFixture(t)
		h.git("commit", "--allow-empty", "-q", "-m", "advance")
		h.envs["PYXIS_CI_EVENT"] = "pull_request"
		h.envs["PYXIS_CI_BASE"] = base
		code, _, errout := h.run("reconcile", "--baseline-sync", "--baseline-ref", "HEAD")
		if code != exitRefused || !strings.Contains(errout, "cannot override") {
			t.Fatalf("CI override: %d %s", code, errout)
		}
	})
}

func TestPolicyGitTreeIsClosed(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"missing-marker", "missing-blob", "nested", "symlink-member", "symlink-root", "symlink-github", "submodule", "unknown-file", "mixed"} {
		t.Run(kind, func(t *testing.T) {
			h := initRepo(t)
			p := reconcile.NewPolicySet(reconcile.Report{Generator: reconcile.Generator, Findings: []reconcile.Finding{{Code: "gap", Scope: "scope"}}})
			files, err := reconcile.MarshalPolicySet(p)
			if err != nil {
				t.Fatal(err)
			}
			var member string
			for name, body := range files {
				writeFixtureFile(t, h.dir, name, string(body), 0644)
				if name != reconcile.PolicyPath {
					member = name
				}
			}
			switch kind {
			case "missing-marker":
				if err := os.Remove(filepath.Join(h.dir, reconcile.PolicyPath)); err != nil {
					t.Fatal(err)
				}
			case "nested":
				writeFixtureFile(t, h.dir, reconcile.PolicyDirectory+"/nested/member.json", "{}", 0644)
			case "unknown-file":
				writeFixtureFile(t, h.dir, reconcile.PolicyDirectory+"/README.md", "unexpected", 0644)
			case "mixed":
				writeFixtureFile(t, h.dir, reconcile.PolicyPath, h.ok("reconcile", "--baseline-generate"), 0644)
			case "symlink-member", "symlink-root", "symlink-github":
				name := member
				if kind == "symlink-root" {
					name = reconcile.PolicyDirectory
				}
				if kind == "symlink-github" {
					name = ".github"
				}
				if err := os.RemoveAll(filepath.Join(h.dir, name)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../README.md", filepath.Join(h.dir, name)); err != nil {
					t.Fatal(err)
				}
			case "submodule":
				if err := os.Remove(filepath.Join(h.dir, member)); err != nil {
					t.Fatal(err)
				}
				h.git("add", reconcile.PolicyPath)
				h.git("update-index", "--add", "--cacheinfo", "160000,"+h.git("rev-parse", "HEAD")+","+member)
			}
			if kind == "submodule" {
				h.git("commit", "-q", "-m", "invalid policy tree")
			} else {
				h.commit("invalid policy tree")
			}
			env, _, _ := h.env("")
			m, err := open(t.Context(), env)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "missing-blob" {
				oid := h.git("rev-parse", "HEAD:"+member)
				if err := os.Remove(filepath.Join(h.dir, ".git", "objects", oid[:2], oid[2:])); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := readPolicyAt(t.Context(), m, h.git("rev-parse", "HEAD")); err == nil {
				t.Fatalf("accepted %s tree", kind)
			}
		})
	}
}

func TestV2TrustAndTransitionsCannotHideBehindUnchangedAllowances(t *testing.T) {
	t.Parallel()
	t.Run("unreproducible trusted allowances", func(t *testing.T) {
		h, base := partitionedPolicyFixture(t)
		for name := range policyFilesOnDisk(t, h) {
			if name != reconcile.PolicyPath {
				if err := os.Remove(filepath.Join(h.dir, name)); err != nil {
					t.Fatal(err)
				}
			}
		}
		h.commit("corrupt trusted inventory")
		badBase := h.git("rev-parse", "HEAD")
		h.ok("reconcile", "--write")
		h.commit("candidate now has zero debt")
		code, _, errout := h.run("reconcile", "--check", "--baseline", reconcile.PolicyPath, "--baseline-ref", badBase)
		if code != exitRefused || !strings.Contains(errout, "does not reproduce its exact allowances") {
			t.Fatalf("bad trusted policy passed: %d %s (original %s)", code, errout, base)
		}
	})
	t.Run("code transition", func(t *testing.T) {
		h := initRepo(t)
		h.ok("init")
		writeFixtureFile(t, h.dir, "internal/x/x.go", "package x\nconst Value = 1\n", 0644)
		h.commit("code without semantic memory")
		code, out, errout := h.run("reconcile", "--write")
		if code != exitOK {
			t.Fatalf("fixture: %d %s %s", code, out, errout)
		}
		h.commit("capture source")
		writeFixtureFile(t, h.dir, reconcile.PolicyPath, h.ok("reconcile", "--baseline-generate"), 0644)
		h.commit("trust current inventory")
		h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
		h.commit("migrate")
		base := h.git("rev-parse", "HEAD")
		before := policyFilesOnDisk(t, h)
		writeFixtureFile(t, h.dir, "internal/x/x.go", "package x\nconst Value = 2\n", 0644)
		h.commit("changed behavior")
		code, _, _ = h.run("reconcile", "--write")
		if code != exitOK {
			t.Fatal("code change required semantic debt")
		}
		code, _, errout = h.run("reconcile", "--baseline-sync", "--baseline-ref", base)
		if code != exitOK || !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
			t.Fatalf("code-only change modified semantic policy: %d %s", code, errout)
		}
		code, out, errout = h.gate("--base", base)
		if code != exitOK {
			t.Fatalf("v2 gate required invented knowledge: %d %s %s", code, out, errout)
		}
	})
	t.Run("legacy replay still mandatory", func(t *testing.T) {
		h, _ := legacyPolicyFixture(t)
		h.ok("add", "--kind", "Learning", "--subject", "meta/new", "--statement", "changes legacy memory identity")
		h.commit("stale global digest")
		before := policyFilesOnDisk(t, h)
		code, _, errout := h.run("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
		if code != exitRefused || !strings.Contains(errout, "--baseline-replay-ref") || !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
			t.Fatalf("legacy identity bypass: %d %s", code, errout)
		}
	})
}

func TestV2GatePreservesSourceAnchorAndRecordBoundaries(t *testing.T) {
	t.Parallel()
	t.Run("source deletion cannot erase debt", func(t *testing.T) {
		h, base := partitionedPolicyFixture(t)
		h.git("rm", "README.md")
		// Make the candidate inventory exact, so the source-retirement guard
		// must refuse independently of allowance exactness or monotonicity.
		for name := range policyFilesOnDisk(t, h) {
			if name != reconcile.PolicyPath {
				h.git("rm", name)
			}
		}
		h.commit("attempt to delete the source and its allowance")
		code, out, errout := h.gate("--base", base)
		if code != exitRefused || !strings.Contains(out+errout, "authored-source-retirement README.md") {
			t.Fatalf("v2 gate accepted source deletion: %d %s %s", code, out, errout)
		}
	})
	t.Run("code changes reuse existing knowledge", func(t *testing.T) {
		h := initRepo(t)
		h.ok("init")
		writeFixtureFile(t, h.dir, "internal/x/x.go", "package x\nconst Value = 1\n", 0644)
		h.commit("initial code to witness")
		h.ok("add", "--kind", "Learning", "--subject", "meta/x", "--statement", "Offline execution requires a local default.", "--rationale", "The target deployment has no configuration service.", "--tag", "code:internal/x")
		h.commit("initial code and semantic memory")
		h.ok("reconcile", "--write")
		h.commit("exact initial evidence")
		writeFixtureFile(t, h.dir, reconcile.PolicyPath, h.ok("reconcile", "--baseline-generate"), 0644)
		h.commit("exact legacy inventory")
		h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
		h.commit("partition policy")
		base := h.git("rev-parse", "HEAD")
		writeFixtureFile(t, h.dir, "internal/x/x.go", "package x\nconst Value = 2\n", 0644)
		h.commit("changed code to witness")
		code, out, errout := h.gate("--base", base)
		if code != exitOK {
			t.Fatalf("v2 gate rejected existing governing knowledge: %d %s %s", code, out, errout)
		}
	})
	for _, mutation := range []string{"modified", "deleted"} {
		t.Run("record "+mutation, func(t *testing.T) {
			h, _ := partitionedPolicyFixture(t)
			id := h.add("--kind", "Learning", "--subject", "meta/x", "--statement", "original")
			h.commit("trusted immutable memory")
			base := h.git("rev-parse", "HEAD")
			name := filepath.Join(h.dir, "memory/meta/x", id+".json")
			if mutation == "deleted" {
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			} else {
				body, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, bytes.ReplaceAll(body, []byte("original"), []byte("forged")), 0644); err != nil {
					t.Fatal(err)
				}
			}
			h.commit("attempt record mutation")
			code, out, errout := h.gate("--base", base)
			want := "record-" + mutation
			if mutation == "modified" {
				want = "evidence-record-rewritten"
			}
			if code != exitRefused || !strings.Contains(out+errout, want) {
				t.Fatalf("v2 gate accepted record mutation: %d %s %s", code, out, errout)
			}
		})
	}
}

func TestIndependentDebtReductionsMergeAndSameAddressStillConflicts(t *testing.T) {
	t.Parallel()
	h, _ := legacyPolicyFixture(t)
	for _, name := range []string{"docs/a.md", "docs/b.md"} {
		writeFixtureFile(t, h.dir, name, "# Independent debt\n", 0644)
	}
	h.commit("two independent missing source captures")
	writeFixtureFile(t, h.dir, reconcile.PolicyPath, h.ok("reconcile", "--baseline-generate"), 0644)
	h.commit("exact legacy inventory")
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
	h.commit("partition debt")
	base := h.git("rev-parse", "HEAD")
	var commits, paths []string
	for i, source := range []string{"docs/a.md", "docs/b.md", "docs/a.md"} {
		h.git("checkout", "-q", "-b", fmt.Sprintf("reduction-%d", i), base)
		code, out, errout := h.run("reconcile", "--write", "--action", "capture-document|"+source, "--json")
		if code != exitRefused || decodeReconciliationRun(t, out).Written != 1 {
			t.Fatalf("targeted reduction: %d %s %s", code, out, errout)
		}
		h.commit("capture one source")
		var sync policySyncOutput
		decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--json"), &sync)
		if len(sync.Written) != 1 || sync.Written[0] == reconcile.PolicyPath {
			t.Fatalf("unbounded reduction footprint: %+v", sync)
		}
		paths = append(paths, sync.Written[0])
		h.commit("retire one address")
		commits = append(commits, h.git("rev-parse", "HEAD"))
	}
	if paths[0] == paths[1] || paths[0] != paths[2] {
		t.Fatalf("address partition: %v", paths)
	}
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		h.git("checkout", "-q", "--detach", commits[order[0]])
		h.git("merge", "--no-ff", "--no-edit", "-q", commits[order[1]])
		h.gateOK("--base", commits[order[0]])
	}
	h.git("checkout", "-q", "--detach", commits[0])
	h.git("merge", "--no-ff", "--no-edit", "-q", commits[2])
	code, out, errout := h.gate("--base", commits[0])
	if code != exitRefused || !strings.Contains(out, "conflict") {
		t.Fatalf("competing same-address captures disappeared: %d %s %s", code, out, errout)
	}
}

func TestPinnedPolicyInputsAndBaseErrorsFailClosed(t *testing.T) {
	t.Parallel()
	h, base := partitionedPolicyFixture(t)
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := pinnedReconciliation(t.Context(), m, "not-a-commit", reconcile.Generator); err == nil {
		t.Fatal("unavailable committed inputs accepted")
	}
	if err := migrationPathsOnly(t.Context(), m, base, "not-a-commit"); err == nil {
		t.Fatal("unavailable migration tree accepted")
	}
	for _, tc := range []struct{ event, base, ref string }{{"pull_request", "", ""}, {"schedule", "", base}, {"push", base, "missing-ref"}} {
		h.envs["PYXIS_CI_EVENT"] = tc.event
		h.envs["PYXIS_CI_BASE"] = tc.base
		env, _, _ = h.env("")
		if _, err := authoritativePolicyRef(t.Context(), m, tc.ref, env); err == nil {
			t.Fatalf("accepted invalid CI authority: %+v", tc)
		}
	}
}
