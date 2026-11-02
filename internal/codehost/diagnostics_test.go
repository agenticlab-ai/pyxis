package codehost_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type probe struct {
	readiness codehost.Readiness
	err       error
}

func (p probe) Inspect(context.Context) (codehost.Readiness, error) { return p.readiness, p.err }

func TestRequireWorkflowPreservesReadinessAndRefusals(t *testing.T) {
	broken := errors.New("origin probe failed")
	for _, tc := range []struct {
		name  string
		probe codehost.Diagnostics
		want  error
	}{
		{"uncomposed", nil, codehost.ErrOrigin},
		{"probe failure", probe{err: broken}, broken},
		{"missing mainline", probe{readiness: codehost.Readiness{MainlineRef: "refs/heads/trunk", MainlineRemedy: "first push required"}}, codehost.ErrOrigin},
		{"unavailable proposals", probe{readiness: codehost.Readiness{MainlineOID: "origin-head", ProposalRemedy: "proposal support pending"}}, codehost.ErrWorkflowUnavailable},
		{"available adapter", probe{readiness: codehost.Readiness{MainlineOID: "origin-head", ProposalReady: true}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := codehost.RequireWorkflow(t.Context(), tc.probe); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}
