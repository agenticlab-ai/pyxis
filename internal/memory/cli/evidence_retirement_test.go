package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestEvidenceSharesExactBookkeepingRetirement(t *testing.T) {
	t.Parallel()
	const latestAnchor = "Need-01M31W585PD5F0KTWARECF191Y"
	const legacyAnchor = "Need-01M26FGPM9DGT3K9Z3YG65YD6J"
	for _, mode := range []string{
		"complete", "partial", "unrelated deletion", "reintroduction",
		"reintroduction without evidence", "reintroduction at different path", "reintroduction after stripping evidence",
	} {
		t.Run(mode, func(t *testing.T) {
			reintroduce := strings.HasPrefix(mode, "reintroduction")
			h := initRepo(t)
			h.ok("init")
			files, err := os.ReadDir("../packet/testdata/bookkeeping")
			if err != nil {
				t.Fatal(err)
			}
			paths := map[string]string{}
			bodies := map[string][]byte{}
			for _, file := range files {
				body, err := os.ReadFile(filepath.Join("../packet/testdata/bookkeeping", file.Name()))
				if err != nil {
					t.Fatal(err)
				}
				var r record.Record
				if err := json.Unmarshal(body, &r); err != nil {
					t.Fatal(err)
				}
				paths[r.ID] = store.Dir + "/" + r.Path()
				bodies[r.ID] = body
				writeFixtureFile(t, h.dir, paths[r.ID], string(body), 0o644)
			}
			h.commit("approved historical bookkeeping")
			// This independent record has real evidence and must remain protected
			// even when every approved bookkeeping record disappears beside it.
			kept := h.add("--kind", "Learning", "--subject", "learning/retained", "--statement", "Retain an independent reason.", "--source", "README.md")
			h.commit("retain independent evidence")
			base := h.git("rev-parse", "HEAD")
			h.git("checkout", "-qb", "cleanup")
			for id, path := range paths {
				if mode != "partial" || id == latestAnchor {
					h.git("rm", "--", path)
				}
			}
			wantFailure := latestAnchor
			if mode == "unrelated deletion" {
				r, _ := readEvidenceRecord(t, h, kept)
				h.git("rm", "--", store.Dir+"/"+r.Path())
				wantFailure = kept
			}
			h.commit("bounded cleanup")
			code, out, errout := h.run("evidence", "--check", "--base", base, "--json")
			var report evidenceReport
			decodeJSON(t, out, &report)
			if mode == "complete" || reintroduce {
				if code != exitOK || report.blocking() {
					t.Fatalf("exact retirement refused: %d %s %s", code, out, errout)
				}
				h.ok("check", "--base", base)
			} else {
				found := false
				for _, entry := range report.Entries {
					found = found || entry.RecordID == wantFailure && entry.Code == "evidence-record-rewritten"
				}
				if code != exitRefused || !found {
					t.Fatalf("unapproved removal passed: %d %s %s", code, out, errout)
				}
			}
			if reintroduce {
				h.git("checkout", "main")
				h.git("merge", "--ff-only", "cleanup")
				h.git("checkout", "-qb", "reuse-identity")
				id := latestAnchor
				if mode == "reintroduction without evidence" || mode == "reintroduction at different path" {
					id = legacyAnchor
				}
				if mode == "reintroduction" {
					writeFixtureFile(t, h.dir, paths[id], string(bodies[id]), 0o644)
				} else {
					var r record.Record
					if err := json.Unmarshal(bodies[id], &r); err != nil {
						t.Fatal(err)
					}
					if id == legacyAnchor && r.Provenance.Evidence != nil {
						t.Fatal("legacy fixture unexpectedly has durable evidence")
					}
					// Reuse the identity for a different claim with no repository
					// evidence requirement, including a move to another subject.
					r.Statement = "Release ownership changes require delegated approval."
					r.Intent, r.Rationale, r.FitCriterion = "", "", ""
					r.Tags, r.Relations = nil, nil
					r.Provenance = record.Provenance{}
					if mode == "reintroduction at different path" {
						r.Subject = "release/ownership"
					}
					writeEvidenceRecord(t, h, &r)
				}
				h.commit("reintroduce removed identity")
				code, out, errout := h.run("evidence", "--check", "--json")
				if code != exitRefused || !strings.Contains(out, "a removed mainline identity cannot be reintroduced") {
					t.Fatalf("retired identity was reintroduced: %d %s %s", code, out, errout)
				}
			}
		})
	}
}
