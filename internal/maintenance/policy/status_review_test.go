package policy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/audit"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func TestJournalReaderErrorPreservesKnownEvidence(t *testing.T) {
	for _, failure := range []error{errors.New("injected read error"), context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			at := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)
			inspection := audit.Inspection{Malformed: 1, Events: []audit.Event{{
				Event: audit.EventRun, Phase: audit.PhaseTerminal, JobID: "daily", Outcome: audit.OutcomeIncomplete,
				InvocationID: strings.Repeat("1", 32), Timestamp: at, CauseCode: "fixture_incomplete",
				References: []audit.Reference{{Kind: "retained_workspace", Location: "maintenance-attempt:kept"}},
			}}}
			var output strings.Builder
			renderJournalObservation(jobs.Document{Jobs: []jobs.Job{{ID: "daily"}, {ID: "unknown"}}}, inspection, failure, &output)
			for _, want := range []string{"journal health: unavailable (reader error)", "malformed_complete=1", `attempt: job="daily" outcome="incomplete"`, `reference="maintenance-attempt:kept"`, `attempt: job="unknown" outcome=unavailable`} {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("partial evidence lost %q:\n%s", want, output.String())
				}
			}
		})
	}
}
