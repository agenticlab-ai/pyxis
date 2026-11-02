package e2e

// prd:init-hardening/UB-4 (amendment 001, the operator-ruled permission
// hand-off; review:impl #812 FND-1's escalation resolved): a wiring
// operation that fails ONLY for permission — here the branch-protection
// PUT answered with GitHub's admin-rights 403 — completes everything the
// credential allows, ends GREEN, records the handed-off operation with its
// runnable command, and performs NO rollback. The matrix previously
// substituted a transport abort for this class; this leg drives the real
// classification path end to end.

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func TestPermissionFailureHandsOffGreenNoRollback(t *testing.T) {
	r := buildFixtureRelease(t)
	target := t.TempDir()
	transcript := &ghTranscript{repoExists: true, protectionForbidden: true}

	argus := false
	opts := installer.Options{
		Target:         target,
		Pin:            installer.Pin{Version: r.version, ManifestSHA256: r.pin},
		ArtifactSource: r.source(),
		Answers: installer.Answers{
			Tracker:          "github",
			CIMode:           "both",
			WiringRepo:       "use",
			WiringProtection: "required-checks",
			CLISurfaces:      []string{},
			Argus:            &argus,
			GitName:          "E2E Operator",
			GitEmail:         "e2e@example.com",
			Coordinate:       "octo/widget",
		},
		AnswersProvided: true,
		Fetcher:         resolve.FileFetcher{},
		WireRunner:      transcript,
		PublicationGit:  localPublicationTransport(t),
		Stdout:          discardWriter{},
		Logger:          slog.New(slog.NewTextHandler(discardWriter{}, nil)),
	}

	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init = %v, want GREEN with the protection op handed off (UB-4)", err)
	}
	if len(rep.Commits) == 0 || !rep.T8.GreenProof.Green {
		t.Fatal("not a green install — the permission class must not degrade the run")
	}
	if _, serr := os.Stat(filepath.Join(target, ".git")); serr != nil {
		t.Fatalf("the target was rolled back (%v) — UB-4 forbids rollback for the permission class", serr)
	}
	if rep.Rollback != nil {
		t.Errorf("Report.Rollback = %+v, want none (no rollback occurred)", rep.Rollback)
	}
	if rep.T8.Wiring == nil {
		t.Fatal("no wiring audit on the report")
	}
	if !rep.T8.Wiring.Partial {
		t.Error("wiring audit not marked partial despite a handed-off operation")
	}
	var handedOff bool
	for _, op := range rep.T8.Wiring.Operations {
		if op.Name == wire.OpSetProtection && op.Outcome == wire.OutcomeHandedOff {
			handedOff = true
		}
	}
	if !handedOff {
		t.Errorf("set-protection not classed handed-off: %+v", rep.T8.Wiring.Operations)
	}
	if !strings.Contains(rep.T8.Wiring.HandOff, "/protection") {
		t.Errorf("the hand-off block does not carry the remedial protection command:\n%s", rep.T8.Wiring.HandOff)
	}
}
