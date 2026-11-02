package cli

import (
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestStandaloneRetirementRefusesLostOutputAndWriterPreflightFailures(t *testing.T) {
	t.Parallel()
	for _, asJSON := range []bool{false, true} {
		assertEveryOutputFailure(t, func(w io.Writer) error {
			return writeRetirementOutput(Env{Stdout: w}, asJSON, "ready", []retirementPlan{{receipt: reconcile.SourceRetirementReceipt{Path: retirementSources[0]}}})
		})
	}
	t.Run("missing repository", func(t *testing.T) {
		h := newHarness(t, t.TempDir())
		if code, _, _ := h.run(retirementArgs("--write")...); code != exitUsage {
			t.Fatalf("missing repo: %d", code)
		}
	})
	for _, mode := range []string{"missing actor", "entropy unavailable", "unrelated complete retry", "changed retry reason", "read-only check retry"} {
		t.Run(mode, func(t *testing.T) {
			h := standaloneRetirementFixture(t)
			args := retirementArgs("--write")
			if mode == "missing actor" {
				h.git("config", "--unset", "user.email")
			}
			if mode == "unrelated complete retry" || mode == "changed retry reason" || mode == "read-only check retry" {
				h.ok(args...)
				if mode == "unrelated complete retry" {
					writeFixtureFile(t, h.dir, "unrelated", "dirty", 0644)
				}
				if mode == "changed retry reason" {
					args[6] = "A different retirement reason"
				}
				if mode == "read-only check retry" {
					h.ok(retirementArgs("--check")...)
					return
				}
			}
			before := h.git("status", "--porcelain=v1", "--untracked-files=all")
			env, _, _ := h.env("")
			if mode == "entropy unavailable" {
				env.Entropy = func() ([10]byte, error) { return [10]byte{}, errors.New("entropy unavailable") }
			}
			if code := Run(t.Context(), args, env); code == exitOK {
				t.Fatal("failed preflight accepted")
			}
			if after := h.git("status", "--porcelain=v1", "--untracked-files=all"); before != after {
				t.Fatal("failed preflight wrote new files")
			}
		})
	}
}

func TestStandaloneRetirementRejectsChangedSourceAndReintroduction(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"edit before deletion", "mode before deletion", "reintroduced identical source", "revoked seal", "shallow history"} {
		t.Run(mode, func(t *testing.T) {
			h := standaloneRetirementFixture(t)
			var sealed retirementOutput
			decodeJSON(t, h.ok(retirementArgs("--write")...), &sealed)
			h.commit("seals")
			h.git("checkout", "main")
			h.git("merge", "--squash", "seal")
			h.commit("land seals")
			h.git("branch", "-D", "seal")
			base := h.git("rev-parse", "HEAD")
			h.git("checkout", "-qb", "deletion")
			source := retirementSources[0]
			switch mode {
			case "edit before deletion":
				writeFixtureFile(t, h.dir, source, "unsealed bytes", 0644)
				h.commit("edit before deletion")
			case "mode before deletion":
				h.git("update-index", "--chmod=+x", source)
				h.git("commit", "-m", "unsealed mode")
			case "reintroduced identical source":
				h.git("rm", source)
				h.commit("first deletion")
				writeFixtureFile(t, h.dir, source, "# Exact historical source\n", 0644)
				h.commit("new introduction with identical bytes")
			case "revoked seal":
				h.ok("revoke", sealed.Sources[0].RecordID, "--reason", "withdraw retirement authority")
				h.commit("revoke seal")
			}
			h.git("rm", "-f", source)
			h.commit("delete source")
			if mode == "shallow history" {
				h.git("checkout", "main")
				h.git("merge", "--squash", "deletion")
				h.commit("land deletion")
				h.git("branch", "-D", "deletion")
				dir := filepath.Join(t.TempDir(), "shallow")
				h.git("clone", "--depth=1", "--no-local", "--branch", "main", "file://"+h.dir, dir)
				h = newHarness(t, dir)
			}
			if code, _, _ := h.run("verify", "--offline"); code == exitOK {
				t.Fatal("invalid retirement passed offline replay")
			}
			if mode != "shallow history" {
				if code, _, _ := h.gate("--base", base); code == exitOK {
					t.Fatal("invalid retirement passed proposal gate")
				}
			}
		})
	}
}
