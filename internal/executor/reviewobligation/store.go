// Package reviewobligation owns the durable exact-green handoff between an
// implementation Worker and its independent reviewer.
package reviewobligation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const Schema = "review-obligation/v1"

// Obligation is immutable. A retry appends attempt/terminal evidence elsewhere;
// it never edits the exact implementation revision captured here.
type Obligation struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"review_obligation_id"`
	Repository    string         `json:"repository"`
	Scope         string         `json:"scope,omitempty"`
	TaskID        string         `json:"task_id"`
	TaskNumber    int            `json:"task_number,omitempty"`
	TaskCharter   string         `json:"task_charter,omitempty"`
	ProposalRef   string         `json:"proposal_ref"`
	Revision      string         `json:"revision"`
	Harness       entity.Harness `json:"backend"`

	DemandID      executiondemand.DemandID       `json:"demand_id,omitempty"`
	DemandProfile executiondemand.ProfileMinimum `json:"demand_profile"`
	NativeRouting *taskrouting.Spec              `json:"native_routing,omitempty"`
}

const TerminalSchema = "review-terminal/v1"

const AttemptSchema = "review-attempt/v1"

const HandoffSchema = "review-handoff/v1"

const PublicationPairSchema = "review-publication-pair/v1"

var (
	ErrAttemptActive   = errors.New("review obligation: attempt already active")
	ErrAlreadyVerified = errors.New("review obligation: already verified")
	ErrPairVerified    = errors.New("review obligation: publication pair verified; lifecycle completion required")
	ErrHandoffPending  = errors.New("review obligation: implementation terminal handoff pending")
	ErrAttemptSettled  = errors.New("review obligation: attempt already settled")
)

type Attempt struct {
	SchemaVersion   string `json:"schema_version"`
	ObligationID    string `json:"review_obligation_id"`
	AttemptID       string `json:"review_attempt_id"`
	RunID           string `json:"run_id"`
	AttemptProofRef string `json:"attempt_proof_ref"`
}

type Terminal struct {
	SchemaVersion   string `json:"schema_version"`
	ObligationID    string `json:"review_obligation_id"`
	AttemptID       string `json:"review_attempt_id"`
	Status          string `json:"status"`
	FailureClass    string `json:"failure_class,omitempty"`
	ArtifactRef     string `json:"artifact_ref,omitempty"`
	NativeReceipt   string `json:"native_receipt,omitempty"`
	AttemptProofRef string `json:"attempt_proof_ref,omitempty"`
	EVT13Ref        string `json:"evt13_ref,omitempty"`
	EVT17Ref        string `json:"evt17_ref,omitempty"`
}

// PublicationPair is the exact verified EVT-13↔EVT-17 join. It is durable
// before the lifecycle terminal so a crash exposes pair_verified and recovery
// can append only the missing terminal/EVT-15 without launching or publishing.
type PublicationPair struct {
	SchemaVersion        string `json:"schema_version"`
	ObligationID         string `json:"review_obligation_id"`
	AttemptID            string `json:"review_attempt_id"`
	PublicationAttemptID string `json:"publication_attempt_id"`
	ProposalRef          string `json:"proposal_ref"`
	Revision             string `json:"revision"`
	Medium               string `json:"medium"`
	IdempotencyID        string `json:"idempotency_id"`
	ArtifactRef          string `json:"artifact_ref"`
	NativeReceipt        string `json:"native_receipt"`
	AttemptProofRef      string `json:"attempt_proof_ref"`
	EVT13Ref             string `json:"evt13_ref"`
	EVT17Ref             string `json:"evt17_ref"`
}

// Handoff proves the succeeded implementation terminal was published after
// the obligation. Until this marker exists the reducer exposes
// handoff_pending and no reviewer is eligible.
type Handoff struct {
	SchemaVersion string `json:"schema_version"`
	ObligationID  string `json:"review_obligation_id"`
}

type Store interface {
	Preflight(context.Context) error
	Append(context.Context, Obligation) error
	Get(context.Context, string) (Obligation, bool, error)
	State(context.Context, string) (State, bool, error)
	Open(context.Context, string) ([]State, error)
	CompleteHandoff(context.Context, string) error
	Begin(context.Context, Attempt) (AttemptLease, error)
	Resume(context.Context, string, string) (AttemptLease, error)
	Attempt(context.Context, string, string) (Attempt, string, bool, error)
	Terminal(context.Context, string, string) (Terminal, bool, error)
	RecordPair(context.Context, PublicationPair) error
	AppendTerminal(context.Context, Terminal) error
	Verified(context.Context, string) (bool, error)
}

// State is the exact reduced provider view consumed by the unattended drive.
// AttemptCount is observational only; the wake consumer owns its independent
// recovery admission bound. Verified rows remain visible to the join so a
// retained consumer recovery row can be distinguished from corruption.
type State struct {
	Obligation       Obligation
	Status           string
	AttemptCount     int
	LastFailureClass string
	Pair             *PublicationPair
}

// AttemptLease is the process fence for one admitted attempt. The caller keeps
// it until the attempt's terminal is durable; process death releases the OS
// lock so a later recovery can settle the abandoned row before retrying.
type AttemptLease interface {
	Close() error
}

// FileStore is an append-only, process-safe store. Production places it under
// the repository's git common directory so disposable worktrees cannot remove
// pending review evidence and a later invocation can recover it.
type FileStore struct {
	path string
	mu   sync.Mutex
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("review obligation: empty store path")
	}
	return &FileStore{path: path}, nil
}

// NewRepositoryStore places review state in Git's common directory. Resolving
// the path through Git is required for linked worktrees, where repoDir/.git is
// a pointer file and the durable common directory lives in the parent clone.
func NewRepositoryStore(ctx context.Context, repoDir string) (*FileStore, error) {
	if strings.TrimSpace(repoDir) == "" {
		return nil, errors.New("review obligation: empty repository path")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return nil, fmt.Errorf("review obligation: resolve git common directory: %w", err)
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return nil, errors.New("review obligation: git returned an empty common directory")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoDir, commonDir)
	}
	commonDir, err = filepath.Abs(commonDir)
	if err != nil {
		return nil, fmt.Errorf("review obligation: resolve absolute common directory: %w", err)
	}
	return NewFileStore(filepath.Join(filepath.Clean(commonDir), "pyxis", "review-obligations.jsonl"))
}

func (s *FileStore) Preflight(ctx context.Context) error {
	_, err := s.Open(ctx, "")
	return err
}

func (s *FileStore) Append(ctx context.Context, in Obligation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validate(in); err != nil {
		return err
	}
	b, err := marshalRecord(in)
	if err != nil {
		return fmt.Errorf("review obligation: marshal: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	if got, ok, err := s.getLocked(in.ID); err != nil {
		return err
	} else if ok {
		if got == in {
			return nil
		}
		return fmt.Errorf("review obligation: conflicting replay for %s", in.ID)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("review obligation: create store directory: %w", err)
	}
	return appendSynced(s.path, b)
}

func (s *FileStore) Get(ctx context.Context, id string) (Obligation, bool, error) {
	if err := ctx.Err(); err != nil {
		return Obligation{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return Obligation{}, false, err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	return s.getLocked(id)
}

func (s *FileStore) State(ctx context.Context, id string) (State, bool, error) {
	obligation, ok, err := s.Get(ctx, id)
	if err != nil || !ok {
		return State{}, ok, err
	}
	states, err := s.Open(ctx, obligation.Scope)
	if err != nil {
		return State{}, false, err
	}
	for _, state := range states {
		if state.Obligation.ID == id {
			return state, true, nil
		}
	}
	return State{}, false, fmt.Errorf("review obligation: state disappeared for %s", id)
}

// Open exact-reduces every obligation in scope together with its admitted
// attempts and terminals. An empty scope is a closed request for legacy rows
// whose scope predates the field; production captures always carry scope.
func (s *FileStore) Open(ctx context.Context, scope string) ([]State, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided

	obligations, err := s.obligationsLocked()
	if err != nil {
		return nil, err
	}
	attempts, err := s.attemptsLocked()
	if err != nil {
		return nil, err
	}
	terminals, err := s.terminalsLocked()
	if err != nil {
		return nil, err
	}
	pairs, err := s.pairsLocked()
	if err != nil {
		return nil, err
	}
	handoffs, err := s.handoffsLocked()
	if err != nil {
		return nil, err
	}
	handoffRows := make([]Handoff, 0, len(handoffs))
	for obligationID := range handoffs {
		handoffRows = append(handoffRows, Handoff{SchemaVersion: HandoffSchema, ObligationID: obligationID})
	}
	return reduceStates(scope, obligations, attempts, pairs, terminals, handoffRows)
}

func (s *FileStore) CompleteHandoff(ctx context.Context, obligationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if obligationID == "" {
		return errors.New("review obligation: empty handoff identity")
	}
	b, err := marshalRecord(Handoff{SchemaVersion: HandoffSchema, ObligationID: obligationID})
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	if _, found, err := s.getLocked(obligationID); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("review obligation: handoff references unknown obligation %s", obligationID)
	}
	rows, err := s.handoffsLocked()
	if err != nil {
		return err
	}
	if rows[obligationID] {
		return nil
	}
	return appendSynced(s.path+".handoffs", b)
}

func (s *FileStore) handoffsLocked() (map[string]bool, error) {
	data, err := completeRows(s.path + ".handoffs")
	if err != nil {
		return nil, fmt.Errorf("review obligation: read handoff store: %w", err)
	}
	out := map[string]bool{}
	scan := recordScanner(data)
	for scan.Scan() {
		var row Handoff
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil || row.SchemaVersion != HandoffSchema || row.ObligationID == "" {
			return nil, errors.New("review obligation: malformed handoff row")
		}
		out[row.ObligationID] = true
	}
	return out, scan.Err()
}

func (s *FileStore) obligationsLocked() ([]Obligation, error) {
	data, err := completeRows(s.path)
	if err != nil {
		return nil, fmt.Errorf("review obligation: read store: %w", err)
	}
	byID := map[string]Obligation{}
	scan := recordScanner(data)
	for scan.Scan() {
		var row Obligation
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("review obligation: malformed row: %w", err)
		}
		if err := validate(row); err != nil {
			return nil, err
		}
		if prior, exists := byID[row.ID]; exists && prior != row {
			return nil, fmt.Errorf("review obligation: conflicting rows for %s", row.ID)
		}
		byID[row.ID] = row
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("review obligation: scan store: %w", err)
	}
	out := make([]Obligation, 0, len(byID))
	for _, row := range byID {
		out = append(out, row)
	}
	return out, nil
}

func (s *FileStore) getLocked(id string) (Obligation, bool, error) {
	data, err := completeRows(s.path)
	if err != nil {
		return Obligation{}, false, fmt.Errorf("review obligation: read store: %w", err)
	}
	var match Obligation
	found := false
	scan := recordScanner(data)
	for scan.Scan() {
		var row Obligation
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil {
			return Obligation{}, false, fmt.Errorf("review obligation: malformed row: %w", err)
		}
		if err := validate(row); err != nil {
			return Obligation{}, false, err
		}
		if row.ID == id {
			if found && row != match {
				return Obligation{}, false, fmt.Errorf("review obligation: conflicting rows for %s", id)
			}
			match, found = row, true
		}
	}
	if err := scan.Err(); err != nil {
		return Obligation{}, false, fmt.Errorf("review obligation: scan store: %w", err)
	}
	return match, found, nil
}

func validate(o Obligation) error {
	if o.SchemaVersion != Schema || o.ID == "" || o.Repository == "" || o.TaskID == "" || o.ProposalRef == "" || o.Revision == "" || o.Harness == "" {
		return errors.New("review obligation: incomplete or unknown-schema row")
	}
	return nil
}

func (s *FileStore) Begin(ctx context.Context, in Attempt) (AttemptLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateAttempt(in); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(in.ObligationID))
	attemptLock, err := tryLockExclusive(fmt.Sprintf("%s.attempt-%x.lock", s.path, digest))
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = attemptLock.Close()
		}
	}()

	s.mu.Lock()
	defer s.mu.Unlock()
	storeLock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return nil, err
	}
	defer func() { _ = storeLock.Close() }() // release is best-effort; the operation result is already decided
	if _, found, err := s.getLocked(in.ObligationID); err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("review obligation: attempt references unknown obligation %s", in.ObligationID)
	}
	handoffs, err := s.handoffsLocked()
	if err != nil {
		return nil, err
	}
	if !handoffs[in.ObligationID] {
		return nil, ErrHandoffPending
	}
	terminals, err := s.terminalsLocked()
	if err != nil {
		return nil, err
	}
	terminalByAttempt := make(map[string]Terminal, len(terminals))
	for _, terminal := range terminals {
		terminalByAttempt[terminal.AttemptID] = terminal
		if terminal.ObligationID == in.ObligationID && terminal.Status == "verified" {
			return nil, ErrAlreadyVerified
		}
	}
	pairs, err := s.pairsLocked()
	if err != nil {
		return nil, err
	}
	for _, pair := range pairs {
		terminal, settled := terminalByAttempt[pair.AttemptID]
		if pair.ObligationID == in.ObligationID && (!settled || terminal.Status == "verified") {
			return nil, ErrPairVerified
		}
	}
	attempts, err := s.attemptsLocked()
	if err != nil {
		return nil, err
	}
	for _, prior := range attempts {
		if prior.ObligationID != in.ObligationID {
			continue
		}
		if prior.AttemptID == in.AttemptID {
			return nil, fmt.Errorf("review obligation: duplicate attempt identity %s", in.AttemptID)
		}
		if _, settled := terminalByAttempt[prior.AttemptID]; settled {
			continue
		}
		// Acquiring the process fence proves the prior owner is gone. Settle its
		// abandoned row before admitting a fresh identity; a live owner would have
		// kept the non-blocking lock and returned ErrAttemptActive above.
		if err := s.appendTerminalLocked(Terminal{SchemaVersion: TerminalSchema, ObligationID: prior.ObligationID, AttemptID: prior.AttemptID, Status: "failed", FailureClass: "infrastructure"}); err != nil {
			return nil, fmt.Errorf("review obligation: settle abandoned attempt %s: %w", prior.AttemptID, err)
		}
	}
	if err := s.appendAttemptLocked(in); err != nil {
		return nil, err
	}
	ok = true
	return attemptLock, nil
}

func validateAttempt(in Attempt) error {
	if in.SchemaVersion != AttemptSchema || in.ObligationID == "" || in.AttemptID == "" || in.RunID == "" || in.AttemptProofRef == "" {
		return errors.New("review obligation: incomplete or unknown-schema attempt")
	}
	return nil
}

// Resume acquires the original attempt's process fence without creating or
// replacing evidence. Recovery uses it before any host-state reconciliation;
// an already-settled attempt is immutable and must be followed by a fresh
// process generation instead of rewriting its terminal.
func (s *FileStore) Resume(ctx context.Context, obligationID, attemptID string) (AttemptLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(obligationID) == "" || strings.TrimSpace(attemptID) == "" {
		return nil, errors.New("review obligation: recovery requires obligation and attempt identities")
	}
	digest := sha256.Sum256([]byte(obligationID))
	lease, err := tryLockExclusive(fmt.Sprintf("%s.attempt-%x.lock", s.path, digest))
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = lease.Close()
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	storeLock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return nil, err
	}
	defer func() { _ = storeLock.Close() }() // release is best-effort; the operation result is already decided
	attempts, err := s.attemptsLocked()
	if err != nil {
		return nil, err
	}
	found := false
	for _, attempt := range attempts {
		found = found || (attempt.ObligationID == obligationID && attempt.AttemptID == attemptID)
	}
	if !found {
		return nil, fmt.Errorf("review obligation: recovery references unadmitted attempt %s", attemptID)
	}
	terminals, err := s.terminalsLocked()
	if err != nil {
		return nil, err
	}
	for _, terminal := range terminals {
		if terminal.ObligationID == obligationID && terminal.AttemptID == attemptID {
			return nil, ErrAttemptSettled
		}
	}
	keep = true
	return lease, nil
}

func (s *FileStore) appendAttemptLocked(in Attempt) error {
	b, err := marshalRecord(in)
	if err != nil {
		return fmt.Errorf("review obligation: marshal attempt: %w", err)
	}
	return appendSynced(s.path+".attempts", b)
}

func (s *FileStore) attemptsLocked() ([]Attempt, error) {
	data, err := completeRows(s.path + ".attempts")
	if err != nil {
		return nil, fmt.Errorf("review obligation: read attempt store: %w", err)
	}
	var out []Attempt
	scan := recordScanner(data)
	for scan.Scan() {
		var row Attempt
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("review obligation: malformed attempt: %w", err)
		}
		if err := validateAttempt(row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *FileStore) Attempt(ctx context.Context, obligationID, attemptID string) (Attempt, string, bool, error) {
	if err := ctx.Err(); err != nil {
		return Attempt{}, "", false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return Attempt{}, "", false, err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	rows, err := s.attemptsLocked()
	if err != nil {
		return Attempt{}, "", false, err
	}
	var match Attempt
	found := false
	for _, row := range rows {
		if row.ObligationID != obligationID || row.AttemptID != attemptID {
			continue
		}
		if found && row != match {
			return Attempt{}, "", false, fmt.Errorf("review obligation: conflicting attempt rows for %s", attemptID)
		}
		match, found = row, true
	}
	if !found {
		return Attempt{}, "", false, nil
	}
	return match, attemptEvidenceRef(match), true, nil
}

func (s *FileStore) Terminal(ctx context.Context, obligationID, attemptID string) (Terminal, bool, error) {
	if err := ctx.Err(); err != nil {
		return Terminal{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return Terminal{}, false, err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	rows, err := s.terminalsLocked()
	if err != nil {
		return Terminal{}, false, err
	}
	for _, row := range rows {
		if row.ObligationID == obligationID && row.AttemptID == attemptID {
			return row, true, nil
		}
	}
	return Terminal{}, false, nil
}

func (s *FileStore) RecordPair(ctx context.Context, in PublicationPair) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePair(in); err != nil {
		return err
	}
	b, err := marshalRecord(in)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	attempts, err := s.attemptsLocked()
	if err != nil {
		return err
	}
	found := false
	for _, attempt := range attempts {
		found = found || (attempt.ObligationID == in.ObligationID && attempt.AttemptID == in.AttemptID && attempt.AttemptProofRef == in.AttemptProofRef)
	}
	if !found {
		return fmt.Errorf("review obligation: publication pair references unknown or mismatched attempt %s", in.AttemptID)
	}
	rows, err := s.pairsLocked()
	if err != nil {
		return err
	}
	for _, existing := range rows {
		if existing.AttemptID != in.AttemptID {
			continue
		}
		if existing == in {
			return nil
		}
		return fmt.Errorf("review obligation: conflicting publication pair for %s", in.AttemptID)
	}
	return appendSynced(s.path+".pairs", b)
}

func (s *FileStore) pairsLocked() ([]PublicationPair, error) {
	data, err := completeRows(s.path + ".pairs")
	if err != nil {
		return nil, fmt.Errorf("review obligation: read publication-pair store: %w", err)
	}
	var out []PublicationPair
	scan := recordScanner(data)
	for scan.Scan() {
		var row PublicationPair
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("review obligation: malformed publication pair: %w", err)
		}
		if err := validatePair(row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, scan.Err()
}

func (s *FileStore) AppendTerminal(ctx context.Context, in Terminal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateTerminal(in); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := lockExclusive(s.path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }() // release is best-effort; the operation result is already decided
	terminals, err := s.terminalsLocked()
	if err != nil {
		return err
	}
	for _, existing := range terminals {
		if existing.AttemptID != in.AttemptID {
			continue
		}
		if existing == in {
			return nil
		}
		return fmt.Errorf("review obligation: conflicting terminal replay for %s", in.AttemptID)
	}
	attempts, err := s.attemptsLocked()
	if err != nil {
		return err
	}
	found := false
	for _, attempt := range attempts {
		if attempt.AttemptID == in.AttemptID && attempt.ObligationID == in.ObligationID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("review obligation: terminal references unadmitted attempt %s", in.AttemptID)
	}
	return s.appendTerminalLocked(in)
}

func (s *FileStore) appendTerminalLocked(in Terminal) error {
	b, err := marshalRecord(in)
	if err != nil {
		return fmt.Errorf("review obligation: marshal terminal: %w", err)
	}
	return appendSynced(s.path+".terminals", b)
}

func (s *FileStore) Verified(ctx context.Context, obligationID string) (bool, error) {
	state, ok, err := s.State(ctx, obligationID)
	return ok && state.Status == "verified", err
}

func (s *FileStore) terminalsLocked() ([]Terminal, error) {
	data, err := completeRows(s.path + ".terminals")
	if err != nil {
		return nil, fmt.Errorf("review obligation: read terminal store: %w", err)
	}
	var out []Terminal
	scan := recordScanner(data)
	for scan.Scan() {
		var row Terminal
		if err := decodeRecordJSON(scan.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("review obligation: malformed terminal: %w", err)
		}
		if err := validateTerminal(row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validateTerminal(in Terminal) error {
	if in.SchemaVersion != TerminalSchema || in.ObligationID == "" || in.AttemptID == "" {
		return errors.New("review obligation: incomplete or unknown-schema terminal")
	}
	switch in.Status {
	case "verified":
		if in.ArtifactRef == "" || in.NativeReceipt == "" || in.AttemptProofRef == "" || in.EVT13Ref == "" || in.EVT17Ref == "" || in.FailureClass != "" || in.NativeReceipt == in.AttemptProofRef {
			return errors.New("review obligation: verified terminal requires the exact artifact, receipt, proof, event pair, and no failure class")
		}
	case "failed":
		if in.FailureClass != "artifact_unverified" && in.FailureClass != "artifact_untrusted" && in.FailureClass != "infrastructure" {
			return errors.New("review obligation: failed terminal requires one closed failure class")
		}
	default:
		return errors.New("review obligation: terminal has unknown status")
	}
	return nil
}

func validatePair(in PublicationPair) error {
	if in.SchemaVersion != PublicationPairSchema || in.ObligationID == "" || in.AttemptID == "" || in.PublicationAttemptID == "" || in.ProposalRef == "" || in.Revision == "" || in.Medium == "" || in.IdempotencyID == "" || in.ArtifactRef == "" || in.NativeReceipt == "" || in.AttemptProofRef == "" || in.EVT13Ref == "" || in.EVT17Ref == "" || in.NativeReceipt == in.AttemptProofRef {
		return errors.New("review obligation: incomplete or invalid publication pair")
	}
	return nil
}

func attemptEvidenceRef(in Attempt) string {
	b, _ := json.Marshal(in)
	sum := sha256.Sum256(b)
	return "evidence:review-attempt:sha256:" + fmt.Sprintf("%x", sum[:])
}

// completeRows returns only newline-committed JSONL records. A crash may leave
// one torn final write after at least one committed row; that tail is ignored
// until the next writer truncates it under the process lock. A non-empty file
// with no committed row is corruption, not an empty store: fail closed so an
// outstanding obligation can never be laundered into a clean frontier.
func completeRows(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data, nil
	}
	last := bytes.LastIndexByte(data, '\n')
	if last < 0 {
		return nil, errors.New("review obligation: evidence file has no newline-committed row")
	}
	return data[:last+1], nil
}

// appendSynced repairs an uncommitted torn tail, appends one complete JSONL
// row, syncs the file, and syncs the parent when the evidence file is first
// created so the directory entry is durable too. Callers hold the store lock.
func appendSynced(path string, row []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("review obligation: create store directory: %w", err)
	}
	_, statErr := os.Stat(path)
	created := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !created {
		return fmt.Errorf("review obligation: inspect store: %w", statErr)
	}
	if created {
		return createFirstRowAtomically(path, row)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("review obligation: open store: %w", err)
	}
	closeWith := func(prior error) error {
		if closeErr := f.Close(); prior == nil && closeErr != nil {
			return closeErr
		}
		return prior
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("review obligation: read append tail: %w", closeWith(err))
	}
	committed := len(data)
	if committed > 0 && data[committed-1] != '\n' {
		if last := bytes.LastIndexByte(data, '\n'); last >= 0 {
			committed = last + 1
		} else {
			return errors.New("review obligation: evidence file has no newline-committed row")
		}
		if err := f.Truncate(int64(committed)); err != nil {
			return fmt.Errorf("review obligation: truncate torn tail: %w", closeWith(err))
		}
	}
	if _, err := f.Seek(int64(committed), io.SeekStart); err != nil {
		return fmt.Errorf("review obligation: seek append position: %w", closeWith(err))
	}
	if _, err := f.Write(append(row, '\n')); err != nil {
		return fmt.Errorf("review obligation: append: %w", closeWith(err))
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("review obligation: sync: %w", closeWith(err))
	}
	if err := closeWith(nil); err != nil {
		return fmt.Errorf("review obligation: close: %w", err)
	}
	return nil
}

func createFirstRowAtomically(path string, row []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".review-evidence-*")
	if err != nil {
		return fmt.Errorf("review obligation: create first-row temp file: %w", err)
	}
	temp := f.Name()
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(temp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return fmt.Errorf("review obligation: chmod first-row temp file: %w", err)
	}
	if _, err := f.Write(append(append([]byte(nil), row...), '\n')); err != nil {
		return fmt.Errorf("review obligation: write first row: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("review obligation: sync first row: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("review obligation: close first row: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("review obligation: publish first row: %w", err)
	}
	keep = true
	if err := syncDirectory(dir); err != nil {
		return fmt.Errorf("review obligation: sync store directory: %w", err)
	}
	return nil
}

func decodeRecordJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("review obligation: trailing JSON value")
		}
		return err
	}
	return nil
}
