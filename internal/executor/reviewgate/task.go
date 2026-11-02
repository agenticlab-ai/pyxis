// Package reviewgate verifies the complete current task review and check
// evidence before a local operator request can be presented. It grants no
// approval or landing authority.
package reviewgate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

const (
	BindingSchema  = "local-task-review-binding/v1"
	PolicySchema   = "local-task-review-policy/v1"
	SnapshotSchema = "local-task-review-snapshot/v1"
)

var ErrIncomplete = errors.New("task review gate is incomplete or stale")

// ErrPolicyStale identifies valid retained proof that needs a fresh review
// under current policy. Malformed or unavailable proof does not match it.
var ErrPolicyStale = fmt.Errorf("%w: current review policy changed", ErrIncomplete)

type Binding struct {
	SchemaVersion string `json:"schema_version"`
	Origin        string `json:"origin"`
	ProposalID    string `json:"proposal_id"`
	ProposalOID   string `json:"proposal_oid"`
	Sequence      uint64 `json:"sequence"`
	Feature       string `json:"feature"`
	Task          string `json:"task"`
	Branch        string `json:"branch"`
	Head          string `json:"head"`
	TargetRef     string `json:"target_ref"`
	TargetOID     string `json:"target_oid"`
}

func Bind(origin string, g codehost.ProposalGeneration) (Binding, error) {
	p := g.Proposal
	b := Binding{BindingSchema, origin, p.ID, g.OID, p.Sequence, p.Feature, p.Task, p.Branch, p.Head, p.TargetRef, p.TargetOID}
	if p.SchemaVersion != codehost.ProposalSchema || p.Kind != "implementation" || p.State != "prepared" || p.ReviewRequest != nil || p.TaskPreparation == nil || p.TaskPreparation.Validate() != nil {
		return Binding{}, fmt.Errorf("%w: expected a prepared implementation task", ErrIncomplete)
	}
	return b, b.Validate()
}

func (b Binding) Validate() error {
	if b.SchemaVersion != BindingSchema || b.Origin == "" || b.ProposalID == "" || b.Feature == "" || b.Task == "" || b.Sequence == 0 ||
		strings.ContainsAny(b.Origin+b.ProposalID+b.Feature+b.Task+b.Branch+b.TargetRef, "\r\n\x00") ||
		!strings.HasPrefix(b.Branch, "refs/heads/") || !strings.HasPrefix(b.TargetRef, "refs/heads/") || b.Branch == b.TargetRef {
		return fmt.Errorf("%w: invalid task binding", ErrIncomplete)
	}
	for _, oid := range []string{b.ProposalOID, b.Head, b.TargetOID} {
		if len(oid) != len(b.Head) || (len(oid) != 40 && len(oid) != 64) || strings.Trim(oid, "0123456789abcdef") != "" {
			return fmt.Errorf("%w: expected full canonical object ids", ErrIncomplete)
		}
	}
	return nil
}

// Subject includes the prepared generation. A later H1→H2→H1 generation
// cannot reuse the older process/report simply because the head matches.
func (b Binding) Subject() string {
	return (&url.URL{Scheme: "local-git", Path: b.Origin, Fragment: b.ProposalID + "@" + b.ProposalOID}).String()
}

type Policy struct {
	SchemaVersion string     `json:"schema_version"`
	CodeGate      GatePolicy `json:"code_gate"`
	Required      []string   `json:"required_checks"`
	ReviewBand    string     `json:"review_band"`
	NativeModel   string     `json:"native_model"`
	Prompts       string     `json:"prompts_version"`
	Calibration   string     `json:"calibration"`
}

type Check struct {
	Name  string          `json:"name"`
	State port.CheckState `json:"state"`
}

// Snapshot carries the whole aggregate and each check outcome, together with
// the policy actually evaluated. A summary verdict alone cannot satisfy it.
type Snapshot struct {
	SchemaVersion string                      `json:"schema_version"`
	Binding       Binding                     `json:"binding"`
	Policy        Policy                      `json:"policy"`
	Review        implreview.AggregateHandoff `json:"review"`
	CodeGate      GateReport                  `json:"code_gate"`
	Checks        []Check                     `json:"checks"`
}

// Reader is host-owned: it exact-reads current policy and completed process/
// gate artifacts for the requested generation. Reviewer terminal output is
// passed through the existing trusted coordinator before becoming a Snapshot.
type Reader interface {
	Policy(context.Context, Binding) (Policy, error)
	Snapshot(context.Context, Binding) (Snapshot, error)
}

// SnapshotLease is required for approval publication and landing. The host
// rejects an obsolete policy, aggregate or check result and keeps all their
// authoritative sources unchanged until the callback returns.
type SnapshotLease interface {
	WithSnapshot(context.Context, Binding, Snapshot, func() error) error
}
