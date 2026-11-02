package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestE2ESessionLogsAreOptionalAndHistorical(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.ok("reconcile", "--write")
	h.commit("capture the actual authored source")
	const source = "logs/optional.md"
	const body = "# Session log — quasarlogtoken\n\nStarted: 2026-09-09 18:10 · CLI: Codex\n\nNothing escalated this session.\n"
	writeFixtureFile(t, h.dir, source, body, 0o644)
	h.commit("optional operational log")
	code, out, errout := h.run("reconcile", "--write", "--json")
	if code != exitOK || decodeReconciliationRun(t, out).Written != 0 {
		t.Fatalf("optional log created work: %d %s %s", code, out, errout)
	}
	id := h.add("--kind", "Document", "--subject", "session-log/optional", "--family", "record", "--statement", "quasarlogtoken history", "--literal-body", "--body-file", source, "--source", source)
	h.add("--kind", "Decision", "--subject", "design/widget", "--family", "D-1", "--statement", "quasarlogtoken durable decision", "--source", source)
	h.commit("legacy import and a real decision citing it")
	search := h.ok("search", "quasarlogtoken")
	if strings.Contains(search, id) || !strings.Contains(search, "durable decision") {
		t.Fatalf("default retrieval = %s", search)
	}
	if all := h.ok("search", "quasarlogtoken", "--all"); !strings.Contains(all, id) {
		t.Fatalf("explicit history omitted legacy import: %s", all)
	}
	if packed := h.ok("pack", "--subject", "session-log/optional"); strings.Contains(packed, "Nothing escalated") {
		t.Fatalf("pack leaked session history: %s", packed)
	}
	if rendered := h.ok("render", "session-log/optional", "--slot", "record"); !strings.Contains(rendered, "Nothing escalated") {
		t.Fatalf("explicit render lost history: %s", rendered)
	}
	if err := os.Remove(filepath.Join(h.dir, source)); err != nil {
		t.Fatal(err)
	}
	h.commit("drop optional raw log")
	h.ok("reconcile", "--check")
	h.ok("revoke", id, "--reason", "history was retired")
	h.commit("retire legacy import")
	h.ok("reconcile", "--check")
	if err := os.Remove(filepath.Join(h.dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	h.commit("attempt to drop durable source")
	code, out, errout = h.run("reconcile", "--check", "--json")
	if code != exitRefused || !hasFinding(decodeReconciliationRun(t, out).Report.Findings, "orphan-document-source") {
		t.Fatalf("durable source deletion escaped: %d %s %s", code, out, errout)
	}
}

func TestE2EV1BaselineReplaysLogsBeforeRetirement(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "logs/old.md", "historical operational evidence\n", 0o644)
	h.commit("old log")
	h.ok("reconcile", "--write")
	h.add("--kind", "Document", "--subject", "session-log/old", "--family", "record", "--statement", "old import", "--literal-body", "--body-file", "logs/old.md", "--source", "logs/old.md")
	h.commit("v1 captured all authored sources")
	_, out, _ := h.run("reconcile", "--check", "--json")
	baseline := reconcile.NewBaseline(decodeReconciliationRun(t, out).Report)
	env, _, _ := h.env("")
	m, err := open(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	opts := reconciliationOptions(m)
	opts.LegacySessionLogs = true
	legacy, err := reconcile.BuildSnapshot(context.Background(), m.repo, "HEAD", opts)
	if err != nil {
		t.Fatal(err)
	}
	baseline.Generator = "memory-reconcile/v1"
	baseline.SourceClosureSHA256 = legacy.SourceClosureSHA256
	b, err := reconcile.MarshalBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, string(b), 0o644)
	h.commit("trusted v1 baseline")
	trusted := h.git("rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(h.dir, "logs/old.md")); err != nil {
		t.Fatal(err)
	}
	h.commit("retire optional source under v2")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("tighten inventory under v2")
	h.ok("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", trusted)
}

func TestE2EOnlyPinnedEmptyImportsCanBeDeleted(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.clock = time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	h.ok("init")
	envelope, err := os.ReadFile("../sessionlog/testdata/empty.json")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../sessionlog/testdata/empty.md")
	if err != nil {
		t.Fatal(err)
	}
	var r record.Record
	if err := json.Unmarshal(envelope, &r); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, "memory/"+r.Path(), string(envelope), 0o644)
	writeFixtureFile(t, h.dir, "memory/"+r.BodyPath(), string(body), 0o644)
	writeFixtureFile(t, h.dir, r.Provenance.Source, string(body), 0o644)
	h.commit("approved historical empty import")
	base := h.git("rev-parse", "HEAD")
	for _, p := range []string{"memory/" + r.Path(), "memory/" + r.BodyPath(), r.Provenance.Source} {
		if err := os.Remove(filepath.Join(h.dir, p)); err != nil {
			t.Fatal(err)
		}
	}
	h.commit("remove exact empty pair")
	h.ok("check", "--base", base)
	writeFixtureFile(t, h.dir, "logs/new.md", string(body), 0o644)
	h.commit("pin the new import source")
	id := h.add("--kind", "Document", "--subject", "session-log/new", "--family", "record", "--statement", "unapproved new import", "--literal-body", "--body-file", "logs/new.md", "--source", "logs/new.md")
	h.commit("another empty import outside the migration")
	base = h.git("rev-parse", "HEAD")
	for _, ext := range []string{".json", ".md"} {
		if err := os.Remove(filepath.Join(h.dir, "memory/session-log/new", id+ext)); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errout := h.run("check", "--base", base)
	if code != exitRefused || !strings.Contains(out+errout, "record-deleted") {
		t.Fatalf("new record received a deletion exemption: %d %s %s", code, out, errout)
	}
}
