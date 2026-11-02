package preflight

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

const ProcessBoundaryID = "process-boundary"

var probeProcessBoundary = runtime.ProbeProcessAccess

var processBoundaryCheck = Check{ID: ProcessBoundaryID, Probe: func(ctx context.Context, _ Input) (Outcome, error) {
	return ProcessBoundary(ctx), nil
}}

// ProcessBoundary is shared by init's pre-write report and doctor's health
// findings. Missing enforcement does not prevent documentation-only setup, but
// is reported before installation proceeds; native launch itself fails closed.
func ProcessBoundary(ctx context.Context) Outcome {
	if err := probeProcessBoundary(ctx); err != nil {
		remedy := runtime.ProcessAccessRemedy()
		return Outcome{Check: ProcessBoundaryID, Status: StatusAdvise, Reason: err.Error(), Remedy: remedy,
			Warning: "native implementation/review unavailable: " + err.Error() + "; " + remedy}
	}
	return Outcome{Check: ProcessBoundaryID, Status: StatusPass}
}
