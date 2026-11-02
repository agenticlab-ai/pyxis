package installer

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

type noOpWireRunner struct{}

func (noOpWireRunner) RunGH(context.Context, ...string) (string, string, int, error) {
	return "", "", 0, nil
}

func tailBoundaryRun(t *testing.T) *initRun {
	t.Helper()
	root := t.TempDir()
	return &initRun{
		opts:   Options{Target: root},
		logger: slog.New(slog.DiscardHandler),
		rep:    &Report{RunID: "boundary"},
		j:      newJournal(root),
	}
}

func TestTailStagesHonorCancellationBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, stage := range map[string]stageFunc{
		"remote":      stageRemote,
		"seed":        stageSeed,
		"hooks":       stageHooks,
		"wiring":      stageWiring,
		"green-proof": stageGreenProof,
		"welcome":     stageWelcome,
	} {
		t.Run(name, func(t *testing.T) {
			if err := stage(ctx, tailBoundaryRun(t)); !errors.Is(err, context.Canceled) {
				t.Fatalf("stage error = %v", err)
			}
		})
	}
}

func TestGreenProofRefusesMissingPRDRegistry(t *testing.T) {
	run := tailBoundaryRun(t)
	run.repo = config.Defaults()
	if err := stageGreenProof(t.Context(), run); err == nil || !strings.Contains(err.Error(), "enumerate ledger dirs") {
		t.Fatalf("missing PRD registry did not fail the install green proof: %v", err)
	}
	if run.rep.T8.GreenProof.Green {
		t.Fatal("missing PRD registry was reported green")
	}
}

func TestTailStagesRejectClosedBoundaryValues(t *testing.T) {
	run := tailBoundaryRun(t)
	run.knobs.Tracker = "unknown"
	if err := stageWiring(t.Context(), run); err == nil {
		t.Fatal("unsupported tracker reached a silent wiring no-op")
	}

	run = tailBoundaryRun(t)
	run.opts.Answers.Coordinate = "not/a/valid/coordinate"
	run.repo.Remote = "origin"
	run.repo.Knobs = config.Knobs{Tracker: "github", CIMode: "both", Wiring: config.Wiring{Repo: "use", Protection: "required-checks"}}
	if err := stageRemote(t.Context(), run); err == nil {
		t.Fatal("invalid coordinate reached git argv")
	}

	run = tailBoundaryRun(t)
	run.repo.WorkItems = "work-items"
	if err := os.WriteFile(filepath.Join(run.opts.Target, run.repo.WorkItems), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := createCarinaItem(t.Context(), run, "title", "body"); err == nil {
		t.Fatal("carina item creation accepted a file-shaped store root")
	}
}

func TestTailPresentationAndAuditBoundaries(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	if audit, err := wireExecute(t.Context(), nil, noOpWireRunner{}, logger, "run-1", nil); err != nil || len(audit.Operations) != 0 {
		t.Fatalf("empty wire execution = %+v, %v", audit, err)
	}

	var out bytes.Buffer
	printWiringChecklist(&out, "owner/repo", wire.Audit{})
	printWiringChecklist(&out, "owner/repo", wire.Audit{Operations: []wire.OpResult{
		{Name: "apply", Outcome: wire.OutcomeApplied},
		{Name: "skip", Outcome: wire.OutcomeSkipped},
		{Name: "handoff", Outcome: wire.OutcomeHandedOff, Reason: "human step"},
		{Name: "failure", Outcome: wire.OutcomeFailed, Reason: "stopped"},
	}})
	for _, want := range []string{"waiting on you", "✗ failure: stopped"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("checklist missing %q: %s", want, out.String())
		}
	}

	for _, op := range []string{wire.OpCreateRepo, wire.OpEnsureLabels, wire.OpSetProtection, wire.OpWelcomeItem, wire.OpOnboardingItems, "custom"} {
		if got := wiringPlain(op, false); got == "" {
			t.Fatalf("wiringPlain(%q) returned empty", op)
		}
	}

	j := newJournal(t.TempDir())
	recordRemoteCreations(j, nil, wire.Audit{Operations: []wire.OpResult{{Name: wire.OpCreateRepo, Outcome: wire.OutcomeApplied}}})
	if len(j.entries) != 0 {
		t.Fatalf("out-of-range audit row was journaled: %+v", j.entries)
	}

	defects := make([]schema.Defect, 7)
	for i := range defects {
		defects[i] = schema.Defect{Verb: "test", Reason: "broken"}
	}
	if got := defectsLine(defects); !strings.Contains(got, "+2 more") {
		t.Fatalf("bounded defects = %q", got)
	}
	doc := []byte("body without a separator")
	if got := welcomeItemBody(doc); !bytes.Equal(got, doc) {
		t.Fatalf("separator-free body changed: %q", got)
	}
}
