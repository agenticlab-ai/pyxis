package codehost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	TaskRequestSchema      = "local-task-review-request/v1"
	TaskAggregate          = "task-review-aggregate"
	TaskRequest            = "task-review-request"
	MaxReviewArtifactBytes = 1 << 20
)

var ErrReviewArtifactNotFound = errors.New("retained local review artifact is absent")

// ArtifactRef pins both the origin ref and its complete canonical object/body.
type ArtifactRef struct {
	Ref    string `json:"ref"`
	OID    string `json:"oid"`
	Digest string `json:"digest"`
}

func (a ArtifactRef) Validate() error {
	if !strings.HasPrefix(a.Ref, "refs/pyxis/") || strings.ContainsAny(a.Ref, "\r\n\x00") ||
		(!lowerHex(a.OID, 40) && !lowerHex(a.OID, 64)) || !strings.HasPrefix(a.Digest, "sha256:") || !lowerHex(strings.TrimPrefix(a.Digest, "sha256:"), 64) {
		return ErrReviewInvalid
	}
	return nil
}

type ReviewArtifact struct {
	Identity ArtifactRef `json:"identity"`
	Kind     string      `json:"kind"`
	Body     []byte      `json:"body"`
	Trust    Trust       `json:"trust"`
}

// ReviewArtifacts creates immutable objects after the prepared generation.
// It grants neither receipt publication nor a landing operation.
type ReviewArtifacts interface {
	Scope(context.Context) (ProposalScope, error)
	Retain(context.Context, ProposalGeneration, string, []byte) (ReviewArtifact, error)
	ReadRetained(context.Context, ProposalGeneration, string, ArtifactRef) (ReviewArtifact, error)
	ReadRequest(context.Context, ProposalGeneration) (ReviewArtifact, error)
}

// TaskReviewRequest precedes its receipt. Binding.Request is intentionally
// empty: including this request's own object id would create a cycle.
type TaskReviewRequest struct {
	SchemaVersion string        `json:"schema_version"`
	Binding       ReviewBinding `json:"binding"`
}

func (r TaskReviewRequest) Validate() error {
	if r.SchemaVersion != TaskRequestSchema || r.Binding.Request != (ArtifactRef{}) {
		return ErrReviewInvalid
	}
	return r.Binding.validateTaskIntent()
}

func (r TaskReviewRequest) Matches(g ProposalGeneration) error {
	if err := r.Validate(); err != nil {
		return err
	}
	p, b := g.Proposal, r.Binding
	if p.SchemaVersion != ProposalSchema || p.Kind != "implementation" || p.State != "prepared" || p.Task == "" || p.ReviewRequest != nil || p.TaskPreparation == nil || p.TaskPreparation.Validate() != nil ||
		b.ProposalID != p.ID || b.ProposalOID != g.OID || b.Sequence != p.Sequence || b.Kind != p.Kind || b.Branch != p.Branch || b.Head != p.Head || b.TargetRef != p.TargetRef || b.TargetOID != p.TargetOID || b.Nonce != p.TaskPreparation.Nonce {
		return ErrReviewInvalid
	}
	return nil
}

func (r TaskReviewRequest) Canonical() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func ParseTaskRequest(body []byte) (TaskReviewRequest, error) {
	var r TaskReviewRequest
	if len(body) > MaxReviewArtifactBytes {
		return r, ErrReviewInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return TaskReviewRequest{}, fmt.Errorf("%w: task request: %w", ErrReviewInvalid, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return TaskReviewRequest{}, ErrReviewInvalid
	}
	canonical, err := r.Canonical()
	if err != nil || !bytes.Equal(body, canonical) {
		return TaskReviewRequest{}, ErrReviewInvalid
	}
	return r, nil
}
