// Package candidatereview validates complete, generation-bound final evidence.
// It grants no approval or landing authority. Native runner and human
// attribution are revalidated through the host-owned Reader.
package candidatereview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

const (
	BindingSchema  = "local-candidate-review-binding/v1"
	SnapshotSchema = "local-candidate-review-snapshot/v1"
)

var ErrIncomplete = errors.New("final candidate evidence is incomplete or stale")

// ErrEvidenceChanged identifies complete, currently authorized results that
// require a fresh review request. Negative or unavailable evidence cannot
// authorize replacement of a retained request.
var ErrEvidenceChanged = fmt.Errorf("%w: accepted current results changed", ErrIncomplete)

type Binding struct {
	SchemaVersion string                      `json:"schema_version"`
	Origin        string                      `json:"origin"`
	Generation    codehost.ProposalGeneration `json:"generation"`
}

func Bind(origin string, g codehost.ProposalGeneration) (Binding, error) {
	b := Binding{BindingSchema, origin, g}
	return b, b.Validate()
}

func (b Binding) Validate() error {
	g, p := b.Generation, b.Generation.Proposal
	if b.SchemaVersion != BindingSchema || b.Origin == "" || strings.ContainsAny(b.Origin, "\r\n\x00") ||
		p.SchemaVersion != codehost.ProposalSchema || p.Kind != "implementation" || p.State != "prepared" || p.Feature == "" || p.ID == "" || p.Task != "" || p.Sequence == 0 ||
		p.ReviewRequest != nil || p.TaskPreparation != nil || p.TaskLanding != nil || p.CandidateLanding != nil || p.CandidatePreparation == nil || p.CandidatePreparation.Validate() != nil ||
		!strings.HasPrefix(p.SourceRef, "refs/heads/") || !strings.HasPrefix(p.Branch, "refs/heads/") || !strings.HasPrefix(p.TargetRef, "refs/heads/") ||
		p.Branch == p.SourceRef || p.Branch == p.TargetRef || p.SourceRef == p.TargetRef || strings.ContainsAny(p.Feature+p.ID+p.Branch+p.SourceRef+p.TargetRef, "\r\n\x00") {
		return ErrIncomplete
	}
	c := p.CandidatePreparation.Candidate
	if c.FeatureID != p.Feature || c.FeatureHead != p.Head || c.MainHead != p.TargetOID {
		return ErrIncomplete
	}
	for _, oid := range []string{g.OID, p.Head, p.TargetOID} {
		if len(oid) != len(c.Commit) || strings.Trim(oid, "0123456789abcdef") != "" {
			return ErrIncomplete
		}
	}
	return nil
}

type Snapshot struct {
	SchemaVersion string                         `json:"schema_version"`
	Binding       Binding                        `json:"binding"`
	Plan          featuredrive.ScenarioPlan      `json:"plan"`
	Evidence      featuredrive.CandidateEvidence `json:"evidence"`
}

// Reader owns current scenario policy and source attribution. Authorize must
// verify every retained result through its existing runner/manual-human
// authority. A declared local landing receipt cannot satisfy that method.
type Reader interface {
	Plan(context.Context, Binding) (featuredrive.ScenarioPlan, error)
	Evidence(context.Context, Binding) (featuredrive.CandidateEvidence, error)
	Authorize(context.Context, Binding, featuredrive.ScenarioPlan, featuredrive.CandidateEvidence) error
}

// EvidenceLease is required for approval publication and landing. The host
// checks the expected scenario policy, complete result set and attribution,
// then keeps their authoritative sources unchanged until the callback returns.
type EvidenceLease interface {
	WithEvidence(context.Context, Binding, Snapshot, func() error) error
}

func Validate(binding Binding, snapshot Snapshot, current featuredrive.ScenarioPlan) error {
	if binding.Validate() != nil || snapshot.SchemaVersion != SnapshotSchema || !reflect.DeepEqual(snapshot.Binding, binding) ||
		current.Validate() != nil || !reflect.DeepEqual(snapshot.Plan, current) {
		return ErrIncomplete
	}
	preparation := binding.Generation.Proposal.CandidatePreparation
	candidate := featuredrive.CandidateIdentity(preparation.Candidate)
	if current.FeatureID != candidate.FeatureID || current.Candidate != candidate || current.ScenarioSetFingerprint != preparation.ScenarioSetFingerprint {
		return ErrIncomplete
	}
	verdict, err := featuredrive.EvaluateCandidateEvidence(current, snapshot.Evidence)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrIncomplete, err)
	}
	if !verdict.Accepted {
		return fmt.Errorf("%w: every required scenario needs one accepted result", ErrIncomplete)
	}
	return nil
}

func Capture(ctx context.Context, binding Binding, reader Reader) (Snapshot, error) {
	if reader == nil || binding.Validate() != nil {
		return Snapshot{}, ErrIncomplete
	}
	plan, err := reader.Plan(ctx, binding)
	if err != nil {
		return Snapshot{}, err
	}
	evidence, err := reader.Evidence(ctx, binding)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{SnapshotSchema, binding, plan, evidence}
	if err := Verify(ctx, binding, snapshot, reader); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// Verify exact-reads current policy, results and attribution for presentation
// and retained recovery. It does not rerun scenarios or publish an approval.
func Verify(ctx context.Context, binding Binding, snapshot Snapshot, reader Reader) error {
	if reader == nil || ctx.Err() != nil {
		return ErrIncomplete
	}
	plan, err := reader.Plan(ctx, binding)
	if err != nil {
		return err
	}
	if err := Validate(binding, snapshot, plan); err != nil {
		return err
	}
	evidence, err := reader.Evidence(ctx, binding)
	if err != nil {
		return err
	}
	current := Snapshot{snapshot.SchemaVersion, binding, plan, evidence}
	currentBody, err := current.Canonical()
	if err != nil {
		return err
	}
	retainedBody, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if err := reader.Authorize(ctx, binding, plan, evidence); err != nil {
		return err
	}
	if !bytes.Equal(currentBody, append(retainedBody, '\n')) {
		return ErrEvidenceChanged
	}
	return nil
}

func (s Snapshot) Canonical() ([]byte, error) {
	if err := Validate(s.Binding, s, s.Plan); err != nil {
		return nil, err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	if len(body)+1 > codehost.MaxReviewArtifactBytes {
		return nil, ErrIncomplete
	}
	return append(body, '\n'), nil
}

func Parse(body []byte) (Snapshot, error) {
	if len(body) == 0 || len(body) > codehost.MaxReviewArtifactBytes {
		return Snapshot{}, ErrIncomplete
	}
	var s Snapshot
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return Snapshot{}, err
	}
	var extra json.RawMessage
	if err := d.Decode(&extra); err != io.EOF {
		return Snapshot{}, ErrIncomplete
	}
	canonical, err := s.Canonical()
	if err != nil || !bytes.Equal(canonical, body) {
		return Snapshot{}, ErrIncomplete
	}
	return s, nil
}
