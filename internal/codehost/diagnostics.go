// Package codehost defines the code-host diagnostics consumed by draft,
// implementation, and doctor. Composition roots choose the medium adapter.
package codehost

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrOrigin              = errors.New("code-host origin is not ready")
	ErrWorkflowUnavailable = errors.New("code-host workflow is not available")
)

// Diagnostics is read-only. Inspection must not initialize an origin, fetch,
// publish a proposal, or infer approval from a branch's existence.
type Diagnostics interface {
	Inspect(context.Context) (Readiness, error)
}

type Trust struct {
	SharedAccount             bool `json:"shared_account"`
	ApproverAuthenticated     bool `json:"approver_authenticated"`
	MainlineProtected         bool `json:"mainline_protected"`
	IndependentOriginIdentity bool `json:"independent_origin_identity"`
}

// Readiness describes the configured origin, not the checkout's cached refs.
// ProposalReady describes implemented capability, never human approval.
type Readiness struct {
	SchemaVersion  string `json:"schema_version"`
	Provider       string `json:"provider"`
	Origin         string `json:"configured_origin"`
	Remote         string `json:"remote"`
	MainlineRef    string `json:"mainline_ref"`
	MainlineOID    string `json:"mainline_oid,omitempty"`
	MainlineRemedy string `json:"mainline_remedy,omitempty"`
	ProposalReady  bool   `json:"proposal_ready"`
	ProposalRemedy string `json:"proposal_remedy,omitempty"`
	Trust          Trust  `json:"trust"`
}

// RequireWorkflow is the common pre-write admission for commands that need
// proposal semantics. An incomplete adapter cannot fall through to a host.
func RequireWorkflow(ctx context.Context, probe Diagnostics) error {
	if probe == nil {
		return fmt.Errorf("%w: no diagnostics adapter was composed", ErrOrigin)
	}
	r, err := probe.Inspect(ctx)
	if err != nil {
		return err
	}
	if r.MainlineOID == "" {
		return fmt.Errorf("%w: %s is absent; %s", ErrOrigin, r.MainlineRef, r.MainlineRemedy)
	}
	if !r.ProposalReady {
		return fmt.Errorf("%w: %s", ErrWorkflowUnavailable, r.ProposalRemedy)
	}
	return nil
}
