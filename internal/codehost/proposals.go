package codehost

import (
	"context"
	"errors"
)

const ProposalSchema = "local-proposal/v1"

var (
	ErrProposalInvalid   = errors.New("invalid code-host proposal")
	ErrProposalConflict  = errors.New("code-host proposal moved")
	ErrProposalNotFound  = errors.New("code-host proposal not found")
	ErrAtomicUnavailable = errors.New("code host does not support atomic publication")
)

// Proposal is an origin-retained generation. Branch and TargetRef are fully
// qualified refs; an adapter derives TargetRef from the workflow, never the
// change request. Its Git object retains the head, target, and prior generation.
type Proposal struct {
	SchemaVersion        string                `json:"schema_version"`
	ID                   string                `json:"id"`
	Kind                 string                `json:"kind"`
	Feature              string                `json:"feature"`
	Task                 string                `json:"task,omitempty"`
	Branch               string                `json:"branch"`
	Head                 string                `json:"head"`
	TargetRef            string                `json:"target_ref"`
	TargetOID            string                `json:"target_oid"`
	SourceRef            string                `json:"source_ref,omitempty"`
	State                string                `json:"state"`
	Sequence             uint64                `json:"sequence"`
	PreviousOID          string                `json:"previous_oid,omitempty"`
	ArchiveRef           string                `json:"archive_ref,omitempty"`
	ReviewRequest        *DraftReviewRequest   `json:"review_request,omitempty"`
	TaskPreparation      *TaskPreparation      `json:"task_preparation,omitempty"`
	CandidatePreparation *CandidatePreparation `json:"candidate_preparation,omitempty"`
	TaskLanding          *TaskLanding          `json:"task_landing,omitempty"`
	CandidateLanding     *CandidateLanding     `json:"candidate_landing,omitempty"`
	AbandonmentReason    string                `json:"abandonment_reason,omitempty"`
	DraftLock            *LocalDraftLock       `json:"draft_lock,omitempty"`
}

type ProposalGeneration struct {
	OID      string   `json:"oid"`
	Proposal Proposal `json:"proposal"`
}

// ProposalChange pins all observed old refs. Retrying the identical change
// recognizes its already-published generation without minting another event.
// It conveys no approval or landing authority.
type ProposalChange struct {
	ID, Feature, Task, Branch, Head, State                    string
	ExpectedProposalOID, ExpectedBranchOID, ExpectedTargetOID string
	ReviewRequest                                             *DraftReviewRequest
	TaskPreparation                                           *TaskPreparation
	CandidatePreparation                                      *CandidatePreparation
	AbandonmentReason                                         string
}

// Proposals is the consumer-side semantic port. Git commands, authentication,
// transport, and serialization belong to the selected adapter.
type Proposals interface {
	List(context.Context) ([]ProposalGeneration, error)
	Read(context.Context, string) (ProposalGeneration, error)
	Publish(context.Context, ProposalChange) (ProposalGeneration, error)
}

const ProposalScopeSchema = "code-host-proposal-scope/v1"

// ProposalScope exposes immutable adapter bindings so a consumer can verify
// the injected proposal destinations before requesting an origin write.
type ProposalScope struct {
	SchemaVersion string `json:"schema_version"`
	Origin        string `json:"origin"`
	Kind          string `json:"kind"`
	MainlineRef   string `json:"mainline_ref"`
	TargetRef     string `json:"target_ref"`
	HeadRef       string `json:"head_ref,omitempty"`
}

type ScopedProposals interface {
	Proposals
	Scope(context.Context) (ProposalScope, error)
}

// DraftProposals also resolves and fetches the configured origin's current
// mainline object. Consumers read that immutable object, never a cached ref.
type DraftProposals interface {
	Proposals
	Mainline(context.Context) (Readiness, error)
}
