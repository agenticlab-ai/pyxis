// Package publication owns impl-review's durable host-publication state.
// Medium adapters use it to fence one externally observable create and to
// replay receipt/event completion without republishing.
package publication

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

const (
	DefaultEvidenceRef   = "refs/pyxis/impl-review-publication-state"
	publicationStatePath = "impl-review/publication-state.jsonl"
	rowSchemaV1          = "review-publication-state/v1"
	rowSchema            = "review-publication-state/v2"
)

type Stage string

const (
	StageAccepted        Stage = "accepted"
	StageArmed           Stage = "armed"
	StageReceiptRecorded Stage = "receipt_recorded"
	StageTerminal        Stage = "terminal"
)

type Admission struct {
	Binding            implreview.VerifiedPublicationBinding `json:"binding"`
	HandoffDigest      string                                `json:"handoff_digest"`
	ArtifactBodyDigest string                                `json:"artifact_body_digest,omitempty"`
	Medium             string                                `json:"medium"`
}

type Record struct {
	Admission
	PublicationAttemptID string                          `json:"publication_attempt_id"`
	IdempotencyID        string                          `json:"idempotency_id"`
	Stage                Stage                           `json:"stage"`
	CreateArmed          bool                            `json:"create_armed,omitempty"`
	NativeReceipt        string                          `json:"native_receipt,omitempty"`
	Terminal             *implreview.PublicationTerminal `json:"terminal,omitempty"`
}

type stateRow struct {
	SchemaVersion string `json:"schema_version"`
	Record
}

type Store interface {
	Preflight(context.Context) error
	Admit(context.Context, Admission) (Record, error)
	Arm(context.Context, Record) (Record, bool, error)
	RecordReceipt(context.Context, Record, string) (Record, error)
	Finish(context.Context, Record, implreview.PublicationTerminal) (Record, error)
}

// Reader is the protected read side used by the executor to authenticate the
// EVT-13 value returned by a host adapter. It is deliberately narrower than
// Store: pair verification cannot acquire publication authority.
type Reader interface {
	Record(context.Context, string) (Record, bool, error)
}

// Recoverer finds the sole publication that crossed the externally visible
// create fence for an exact review subject. Accepted-only attempts are omitted:
// they own no side effect and a replacement process may safely supersede them.
type Recoverer interface {
	Recover(context.Context, implreview.ReviewSubject) (Record, bool, error)
}

type GitStore struct{ log *gitrefstore.Store }

func NewGitStore(repoDir, remote, ref string) (*GitStore, error) {
	log, err := gitrefstore.New(repoDir, remote, ref)
	if err != nil {
		return nil, err
	}
	return &GitStore{log: log}, nil
}

func (s *GitStore) Preflight(ctx context.Context) error {
	data, err := s.log.Read(ctx, publicationStatePath)
	if err != nil {
		return err
	}
	_, err = reduce(data)
	return err
}

// Record exact-reads one publication attempt from the canonical evidence ref.
// Conflicting histories fail during reduction before a value is returned.
func (s *GitStore) Record(ctx context.Context, publicationAttemptID string) (Record, bool, error) {
	publicationAttemptID = strings.TrimSpace(publicationAttemptID)
	if publicationAttemptID == "" {
		return Record{}, false, errors.New("impl-review publication: empty publication attempt identity")
	}
	data, err := s.log.Read(ctx, publicationStatePath)
	if err != nil {
		return Record{}, false, err
	}
	states, err := reduce(data)
	if err != nil {
		return Record{}, false, err
	}
	record, ok := states[publicationAttemptID]
	return record, ok, nil
}

func (s *GitStore) Recover(ctx context.Context, subject implreview.ReviewSubject) (Record, bool, error) {
	if strings.TrimSpace(subject.ReviewObligationID) == "" || strings.TrimSpace(subject.ProposalID) == "" || strings.TrimSpace(subject.ReviewedRevision) == "" {
		return Record{}, false, errors.New("impl-review publication: incomplete recovery subject")
	}
	data, err := s.log.Read(ctx, publicationStatePath)
	if err != nil {
		return Record{}, false, err
	}
	states, err := reduce(data)
	if err != nil {
		return Record{}, false, err
	}
	var candidates []Record
	for _, state := range states {
		if !state.CreateArmed || state.Binding.ReviewObligationID != subject.ReviewObligationID || state.Binding.ProposalID != subject.ProposalID || state.Binding.ReviewedRevision != subject.ReviewedRevision ||
			(state.Stage == StageTerminal && state.Terminal != nil && state.Terminal.Status == implreview.PublicationFailed) {
			continue
		}
		candidates = append(candidates, state)
	}
	if len(candidates) == 0 {
		return Record{}, false, nil
	}
	// Historical v1 rows could infer more than one armed attempt for a subject.
	// Reconcile them deterministically one at a time instead of turning that
	// migrated state into a permanent hard error. Failed terminals are skipped
	// above, so each recovery makes forward progress.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Stage != candidates[j].Stage {
			return recoveryPriority(candidates[i].Stage) > recoveryPriority(candidates[j].Stage)
		}
		return candidates[i].PublicationAttemptID < candidates[j].PublicationAttemptID
	})
	return candidates[0], true, nil
}

func recoveryPriority(stage Stage) int {
	switch stage {
	case StageTerminal:
		return 3
	case StageReceiptRecorded:
		return 2
	case StageArmed:
		return 1
	default:
		return 0
	}
}

func (s *GitStore) Admit(ctx context.Context, admission Admission) (Record, error) {
	if err := validateAdmission(admission); err != nil {
		return Record{}, err
	}
	attemptID, err := freshID("publication-attempt", admission.HandoffDigest)
	if err != nil {
		return Record{}, err
	}
	// A publication identity is stable for an exact process/handoff replay while
	// a later process generation receives a fresh identity. This lets a failed or
	// acknowledgement-ambiguous attempt remain immutable without blocking the
	// contract's required later fresh review.
	idempotencyID := digest("impl-review-publication/v1", admission.Binding.ReviewObligationID,
		admission.Binding.ReviewProcessID, attemptID, admission.Binding.ProposalID,
		admission.Binding.ReviewedRevision, admission.HandoffDigest)
	want := Record{Admission: admission, PublicationAttemptID: attemptID, IdempotencyID: idempotencyID, Stage: StageAccepted}
	var got Record
	_, err = s.log.Update(ctx, publicationStatePath, "pyxis: admit impl-review publication", func(current []byte) ([]byte, bool, error) {
		states, err := reduce(current)
		if err != nil {
			return nil, false, err
		}
		for _, state := range states {
			if state.Binding.ReviewProcessID == admission.Binding.ReviewProcessID && state.Binding.ReviewObligationID == admission.Binding.ReviewObligationID {
				if state.Admission != admission {
					return nil, false, errors.New("impl-review publication: conflicting handoff for process")
				}
				got = state
				return current, false, nil
			}
		}
		got = want
		return appendState(current, want)
	})
	if err != nil {
		return Record{}, fmt.Errorf("impl-review publication: admit: %w", err)
	}
	return got, nil
}

func (s *GitStore) Arm(ctx context.Context, current Record) (Record, bool, error) {
	if current.Stage == StageArmed || current.Stage == StageReceiptRecorded || current.Stage == StageTerminal {
		return current, false, nil
	}
	if current.Stage != StageAccepted {
		return Record{}, false, errors.New("impl-review publication: only accepted publication can arm")
	}
	next := current
	next.Stage, next.CreateArmed = StageArmed, true
	return s.transition(ctx, current, next, "pyxis: arm impl-review publication")
}

func (s *GitStore) RecordReceipt(ctx context.Context, current Record, receipt string) (Record, error) {
	receipt = strings.TrimSpace(receipt)
	if receipt == "" {
		return Record{}, errors.New("impl-review publication: empty native receipt")
	}
	if current.Stage == StageReceiptRecorded || current.Stage == StageTerminal {
		if current.NativeReceipt != receipt {
			return Record{}, errors.New("impl-review publication: conflicting native receipt replay")
		}
		return current, nil
	}
	if current.Stage != StageArmed {
		return Record{}, errors.New("impl-review publication: receipt requires armed publication")
	}
	next := current
	next.Stage, next.NativeReceipt = StageReceiptRecorded, receipt
	got, _, err := s.transition(ctx, current, next, "pyxis: record impl-review native receipt")
	return got, err
}

func (s *GitStore) Finish(ctx context.Context, current Record, terminal implreview.PublicationTerminal) (Record, error) {
	if current.Stage == StageTerminal {
		if current.Terminal != nil && *current.Terminal == terminal {
			return current, nil
		}
		return Record{}, errors.New("impl-review publication: conflicting terminal replay")
	}
	if current.Stage != StageAccepted && current.Stage != StageArmed && current.Stage != StageReceiptRecorded {
		return Record{}, errors.New("impl-review publication: terminal from invalid state")
	}
	if err := validateTerminal(current, terminal); err != nil {
		return Record{}, err
	}
	next := current
	next.Stage, next.Terminal = StageTerminal, &terminal
	if terminal.NativeReceipt != "" {
		next.NativeReceipt = terminal.NativeReceipt
	}
	got, _, err := s.transition(ctx, current, next, "pyxis: finish impl-review publication")
	return got, err
}

func (s *GitStore) transition(ctx context.Context, prior, next Record, message string) (Record, bool, error) {
	var got Record
	var changed bool
	_, err := s.log.Update(ctx, publicationStatePath, message, func(current []byte) ([]byte, bool, error) {
		// Update may replay this callback after losing a remote CAS race. The
		// ownership result belongs to the latest exact-read generation, not an
		// earlier callback that prepared a commit but failed to publish it.
		changed = false
		states, err := reduce(current)
		if err != nil {
			return nil, false, err
		}
		state, ok := states[prior.PublicationAttemptID]
		if !ok {
			return nil, false, errors.New("impl-review publication: transition references unknown publication attempt")
		}
		if recordsEqual(state, next) {
			got = state
			return current, false, nil
		}
		if !recordsEqual(state, prior) {
			return nil, false, errors.New("impl-review publication: stale or conflicting transition")
		}
		got = next
		changed = true
		return appendState(current, next)
	})
	if err != nil {
		return Record{}, false, err
	}
	return got, changed, nil
}

func appendState(current []byte, record Record) ([]byte, bool, error) {
	encoded, err := json.Marshal(stateRow{SchemaVersion: rowSchema, Record: record})
	if err != nil {
		return nil, false, err
	}
	next := append(append([]byte(nil), current...), encoded...)
	next = append(next, '\n')
	return next, true, nil
}

func reduce(data []byte) (map[string]Record, error) {
	out := map[string]Record{}
	if len(bytes.TrimSpace(data)) == 0 {
		return out, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	for {
		var row stateRow
		if err := dec.Decode(&row); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("impl-review publication: decode state: %w", err)
		}
		switch row.SchemaVersion {
		case rowSchemaV1:
			// v1 predates the explicit CreateArmed bit. Armed and receipted rows
			// crossed the boundary. A terminal inherits that fact from its prior
			// row; a direct accepted->failed terminal (for example a malformed
			// handoff) did not arm and must remain retryable. Historical bytes stay
			// immutable.
			if row.Stage == StageArmed || row.Stage == StageReceiptRecorded {
				row.CreateArmed = true
			}
			if row.Stage == StageTerminal {
				prior := out[row.PublicationAttemptID]
				row.CreateArmed = prior.CreateArmed || row.NativeReceipt != "" || (row.Terminal != nil && row.Terminal.Status == implreview.PublicationVerified)
			}
		case rowSchema:
		default:
			return nil, errors.New("impl-review publication: malformed state row")
		}
		if validateRecord(row.Record) != nil {
			return nil, errors.New("impl-review publication: malformed state row")
		}
		prior, exists := out[row.PublicationAttemptID]
		if exists {
			if prior.Admission != row.Admission || prior.PublicationAttemptID != row.PublicationAttemptID {
				return nil, errors.New("impl-review publication: immutable identity conflict")
			}
			if !validSuccessor(prior, row.Record) {
				return nil, errors.New("impl-review publication: invalid state transition")
			}
		}
		out[row.PublicationAttemptID] = row.Record
	}
	return out, nil
}

func validateAdmission(in Admission) error {
	b := in.Binding
	if strings.TrimSpace(b.ReviewObligationID) == "" || strings.TrimSpace(b.ReviewProcessID) == "" || strings.TrimSpace(b.ProposalID) == "" || strings.TrimSpace(b.ReviewedRevision) == "" || strings.TrimSpace(b.AttemptProofRef) == "" || strings.TrimSpace(b.ProofEvidenceRef) == "" || strings.TrimSpace(in.HandoffDigest) == "" || strings.TrimSpace(in.Medium) == "" {
		return errors.New("impl-review publication: incomplete admission")
	}
	if b.AttemptProofRef == b.ProofEvidenceRef {
		return errors.New("impl-review publication: proof reference aliases proof evidence")
	}
	return nil
}

func validateRecord(record Record) error {
	if validateAdmission(record.Admission) != nil || record.PublicationAttemptID == "" || record.IdempotencyID == "" {
		return errors.New("invalid record")
	}
	switch record.Stage {
	case StageAccepted:
		if record.CreateArmed || record.NativeReceipt != "" || record.Terminal != nil {
			return errors.New("invalid accepted record")
		}
	case StageArmed:
		if !record.CreateArmed || record.NativeReceipt != "" || record.Terminal != nil {
			return errors.New("invalid pre-receipt record")
		}
	case StageReceiptRecorded:
		if !record.CreateArmed || record.NativeReceipt == "" || record.Terminal != nil {
			return errors.New("invalid receipt record")
		}
	case StageTerminal:
		if record.Terminal == nil || validateTerminal(record, *record.Terminal) != nil {
			return errors.New("invalid terminal record")
		}
	default:
		return errors.New("unknown stage")
	}
	return nil
}

func validateTerminal(record Record, terminal implreview.PublicationTerminal) error {
	b := record.Binding
	if terminal.ReviewProcessID != b.ReviewProcessID || terminal.ReviewObligationID != b.ReviewObligationID || terminal.PublicationAttemptID != record.PublicationAttemptID || terminal.ProposalID != b.ProposalID || terminal.ReviewedRevision != b.ReviewedRevision || terminal.Medium != record.Medium || terminal.IdempotencyID != record.IdempotencyID || terminal.EVT13Ref == "" {
		return errors.New("impl-review publication: terminal identity mismatch")
	}
	switch terminal.Status {
	case implreview.PublicationVerified:
		if terminal.ArtifactRef == "" || terminal.NativeReceipt == "" || terminal.AttemptProofRef != b.AttemptProofRef || terminal.FailureClass != "" || terminal.Reason != "" || terminal.NativeReceipt == terminal.AttemptProofRef {
			return errors.New("impl-review publication: invalid verified terminal")
		}
	case implreview.PublicationFailed:
		if terminal.FailureClass == "" || terminal.Reason == "" || terminal.ArtifactRef != "" {
			return errors.New("impl-review publication: invalid failed terminal")
		}
	default:
		return errors.New("impl-review publication: unknown terminal status")
	}
	return nil
}

func validSuccessor(prior, next Record) bool {
	if recordsEqual(prior, next) {
		return true
	}
	if next.Stage == StageTerminal {
		return prior.Stage == StageAccepted || prior.Stage == StageArmed || prior.Stage == StageReceiptRecorded
	}
	return (prior.Stage == StageAccepted && next.Stage == StageArmed) || (prior.Stage == StageArmed && next.Stage == StageReceiptRecorded)
}

func recordsEqual(left, right Record) bool {
	leftTerminal, rightTerminal := left.Terminal, right.Terminal
	left.Terminal, right.Terminal = nil, nil
	if left != right {
		return false
	}
	if leftTerminal == nil || rightTerminal == nil {
		return leftTerminal == nil && rightTerminal == nil
	}
	return *leftTerminal == *rightTerminal
}

func freshID(prefix, binding string) (string, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return prefix + "-" + digest(prefix, binding, string(nonce[:])), nil
}

func digest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func TerminalRef(terminal implreview.PublicationTerminal) (string, error) {
	copy := terminal
	copy.EVT13Ref = ""
	b, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "evidence:evt13:sha256:" + hex.EncodeToString(sum[:]), nil
}
