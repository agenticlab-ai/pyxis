package preflight

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProcessBoundaryReportsWorkingMissingAndBlockedHosts(t *testing.T) {
	prior := probeProcessBoundary
	t.Cleanup(func() { probeProcessBoundary = prior })
	for _, failure := range []error{nil, errors.New("bwrap not found"), errors.New("bwrap: setting up uid map: Permission denied")} {
		probeProcessBoundary = func(context.Context) error { return failure }
		out, err := processBoundaryCheck.Probe(t.Context(), Input{})
		if err != nil || out.Check != ProcessBoundaryID {
			t.Fatalf("probe = %+v, %v", out, err)
		}
		if failure == nil {
			if out.Status != StatusPass {
				t.Fatalf("working boundary = %+v", out)
			}
		} else if out.Status != StatusAdvise || out.Remedy == "" || !strings.Contains(out.Warning, failure.Error()) {
			t.Fatalf("unavailable boundary must name its cause and setup remedy: %+v", out)
		}
	}
}
