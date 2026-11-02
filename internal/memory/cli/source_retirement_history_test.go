package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestStandaloneRetirementHistoryRefusesUnverifiableSnapshots(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"no mainline", "missing ref", "unrelated ref", "topic issuance", "missing source history", "invalid issuance config", "invalid issuance index", "invalid trusted config", "invalid trusted deletion parent", "invalid candidate deletion parent", "trusted symlink", "candidate symlink"} {
		t.Run(mode, func(t *testing.T) {
			h := standaloneRetirementFixture(t)
			var sealed retirementOutput
			decodeJSON(t, h.ok(retirementArgs("--write")...), &sealed)
			h.commit("seals")
			head := h.git("rev-parse", "HEAD")
			r := sealed.Sources[0].Receipt
			ref := "main"
			want := ""
			switch mode {
			case "missing ref":
				ref = "nonexistent"
				want = "does not name a commit"
			case "unrelated ref":
				h.git("checkout", "--orphan", "unrelated")
				h.git("rm", "-rf", ".")
				writeFixtureFile(t, h.dir, "other", "other", 0644)
				h.commit("unrelated root")
				ref = h.git("rev-parse", "HEAD")
				h.git("checkout", "--detach", head)
				want = "merge-base"
			case "topic issuance":
				r.SourceCommit = head
				want = "not on the retained"
			case "missing source history":
				r.Path = ".pyxis/skills/missing/SKILL.md"
				want = "no retained A event"
			case "invalid issuance config", "invalid issuance index", "invalid trusted config", "invalid trusted deletion parent", "invalid candidate deletion parent":
				name := ".pyxis/config.yaml"
				body := "unknown-key: true\n"
				want = "configuration"
				if mode == "invalid issuance index" {
					name = "docs/tech-design/index.json"
					body = "{"
					want = "index"
				}
				writeFixtureFile(t, h.dir, name, body, 0644)
				h.commit("invalid historical input")
				bad := h.git("rev-parse", "HEAD")
				h.git("rm", name)
				if strings.Contains(mode, "deletion parent") {
					h.git("rm", r.Path)
				}
				h.commit("restore candidate metadata")
				ref = h.git("rev-parse", "HEAD")
				if strings.Contains(mode, "issuance") {
					r.SourceCommit = bad
				}
				if mode == "invalid trusted config" {
					ref = bad
				}
				if mode == "invalid candidate deletion parent" {
					ref = head
				}
			case "trusted symlink", "candidate symlink":
				if err := os.Remove(filepath.Join(h.dir, r.Path)); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/missing", filepath.Join(h.dir, r.Path)); err != nil {
					t.Fatal(err)
				}
				h.commit("replace regular source with symlink")
				ref = h.git("rev-parse", "HEAD")
				if mode == "candidate symlink" {
					ref = head
				}
				want = "no retained D event"
			}
			env, _, _ := h.env("")
			m, err := open(t.Context(), env)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "no mainline" {
				m.branch = "absent"
				ref = ""
				want = "configured mainline"
			}
			snapshot, _, err := retirementStateAt(t.Context(), m, h.git("rev-parse", "HEAD"))
			if strings.Contains(mode, "symlink") {
				if err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("non-regular authored source was accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = sourceRetirementEvidence(t.Context(), m, snapshot, r, ref)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("history refusal: %v; want %s", err, want)
			}
			// The public analyzer receives no invented proof after a loader
			// failure, and must keep the missing-history finding blocking.
			snapshot.SourceRetirements = map[string]reconcile.SourceRetirementProof{sealed.Sources[0].RecordID: {Error: err.Error()}}
			report := reconcile.VerifyOffline(snapshot, m.set, m.proj, reconcile.Options{})
			if !hasFinding(report.Findings, "source-retirement-history-unavailable") {
				t.Fatalf("reader hid history failure: %+v", report.Findings)
			}
		})
	}
}
