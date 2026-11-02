package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/implreview/process"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

// The real coordinator, publication adapter and recovery verifier must all
// select the configured stores. Existing shared default refs are deliberately
// present: isolation means preserving them, not merely creating new names.
func TestReviewConfiguredEvidenceRefsIsolatePublicationAndRepair(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper-process E2E currently targets POSIX Git environments")
	}
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true, ReviewPostGreenOnly: true})
	const obligationRef = "refs/pyxis/isolated-review/obligations"
	const processRef = "refs/pyxis/isolated-review/processes"
	const publicationRef = "refs/pyxis/isolated-review/publications"
	path := filepath.Join(fixture.repo, ".pyxis/config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("    evidence_ref: "+obligationRef+"\n    process_evidence_ref: "+processRef+"\n    publication_evidence_ref: "+publicationRef+"\n")...)
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fixture.gitAt(fixture.repo, "add", ".pyxis/config.yaml")
	fixture.gitAt(fixture.repo, "commit", "-m", "test: isolate review evidence references")
	fixture.gitAt(fixture.repo, "push", "origin", "main")
	seed := fixture.gitBare("rev-parse", "refs/heads/main")
	defaults := []string{reviewobligation.DefaultEvidenceRef, process.DefaultEvidenceRef, publication.DefaultEvidenceRef}
	for _, ref := range defaults {
		fixture.gitBare("update-ref", ref, seed)
	}
	var stdout, stderr bytes.Buffer
	if exit := run(fixture.args(), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "review-completed: 1") {
		t.Fatalf("configured review flow exit=%d\nstdout:\n%s\nstderr:\n%s", exit, stdout.String(), stderr.String())
	}
	for _, ref := range []string{obligationRef, processRef, publicationRef} {
		if fixture.gitBare("rev-parse", ref) == seed {
			t.Fatalf("configured evidence store %s contains no review state", ref)
		}
	}
	obligationID := ""
	journal, err := os.ReadFile(fixture.journal)
	if err != nil {
		t.Fatal(err)
	}
	for line := range bytes.SplitSeq(bytes.TrimSpace(journal), []byte("\n")) {
		var event report.Event
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == report.KindImplReviewTerminal {
			obligationID = event.ReviewObligationID
		}
	}
	if obligationID == "" {
		t.Fatal("configured review emitted no obligation identity")
	}
	store, err := reviewobligation.NewGitStore(fixture.repo, "origin", cfg.ReviewEvidenceRef())
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := store.State(t.Context(), obligationID)
	if err != nil || !found || state.Pair == nil {
		t.Fatalf("configured obligation lacks durable publication pair: found=%t state=%+v err=%v", found, state, err)
	}
	repairer, err := newReviewPairRepairer(cfg, fixture.repo, e2eRepository, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if err := repairer.Preflight(t.Context(), e2eRepository); err != nil {
		t.Fatal(err)
	}
	result, err := repairer.Repair(t.Context(), state.Obligation, *state.Pair, "")
	if err != nil || result.Status != report.ReviewStatusCompleted {
		t.Fatalf("recovery did not authenticate the configured publication store: %+v %v", result, err)
	}
	for _, ref := range defaults {
		if actual := fixture.gitBare("rev-parse", ref); actual != seed {
			t.Errorf("shared default evidence ref %s changed: %s != %s", ref, actual, seed)
		}
	}
}
