package reviewobligation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

const DefaultEvidenceRef = "refs/pyxis/implement-now-review-state"

const (
	gitObligationsPath = "review-state/obligations.jsonl"
	gitAttemptsPath    = "review-state/attempts.jsonl"
	gitPairsPath       = "review-state/publication-pairs.jsonl"
	gitTerminalsPath   = "review-state/terminals.jsonl"
	gitHandoffsPath    = "review-state/handoffs.jsonl"
)

// GitStore is the production canonical review-state adapter. Each append is
// a CAS commit on a fetched evidence ref, so a fresh clone reduces the same
// obligation/attempt/terminal family without relying on workspace files.
type GitStore struct {
	repo string
	log  *gitrefstore.Store
}

func NewGitStore(repoDir, remote, ref string) (*GitStore, error) {
	log, err := gitrefstore.New(repoDir, remote, ref)
	if err != nil {
		return nil, err
	}
	return &GitStore{repo: repoDir, log: log}, nil
}

func (s *GitStore) Preflight(ctx context.Context) error {
	_, err := s.log.ReadMany(ctx, gitObligationsPath, gitAttemptsPath, gitPairsPath, gitTerminalsPath, gitHandoffsPath)
	return err
}

func (s *GitStore) Append(ctx context.Context, in Obligation) error {
	if err := validate(in); err != nil {
		return err
	}
	_, err := s.log.Update(ctx, gitObligationsPath, "pyxis: append configured-review obligation", func(current []byte) ([]byte, bool, error) {
		rows, err := parseObligations(current)
		if err != nil {
			return nil, false, err
		}
		for _, existing := range rows {
			if existing.ID != in.ID {
				continue
			}
			if existing == in {
				return current, false, nil
			}
			return nil, false, fmt.Errorf("review obligation: conflicting replay for %s", in.ID)
		}
		return appendJSONLine(current, in)
	})
	return err
}

func (s *GitStore) Get(ctx context.Context, id string) (Obligation, bool, error) {
	data, err := s.log.Read(ctx, gitObligationsPath)
	if err != nil {
		return Obligation{}, false, err
	}
	rows, err := parseObligations(data)
	if err != nil {
		return Obligation{}, false, err
	}
	var match Obligation
	found := false
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		if found && match != row {
			return Obligation{}, false, fmt.Errorf("review obligation: conflicting rows for %s", id)
		}
		match, found = row, true
	}
	return match, found, nil
}

func (s *GitStore) State(ctx context.Context, id string) (State, bool, error) {
	files, err := s.log.ReadMany(ctx, gitObligationsPath, gitAttemptsPath, gitPairsPath, gitTerminalsPath, gitHandoffsPath)
	if err != nil {
		return State{}, false, err
	}
	obligations, err := parseObligations(files[gitObligationsPath])
	if err != nil {
		return State{}, false, err
	}
	var selected []Obligation
	for _, obligation := range obligations {
		if obligation.ID == id {
			selected = append(selected, obligation)
		}
	}
	if len(selected) == 0 {
		return State{}, false, nil
	}
	attempts, err := parseAttempts(files[gitAttemptsPath])
	if err != nil {
		return State{}, false, err
	}
	terminals, err := parseTerminals(files[gitTerminalsPath])
	if err != nil {
		return State{}, false, err
	}
	handoffs, err := parseHandoffs(files[gitHandoffsPath])
	if err != nil {
		return State{}, false, err
	}
	selectedAttempts := attempts[:0]
	attemptIDs := map[string]bool{}
	for _, attempt := range attempts {
		if attempt.ObligationID == id {
			selectedAttempts = append(selectedAttempts, attempt)
			attemptIDs[attempt.AttemptID] = true
		}
	}
	selectedTerminals := terminals[:0]
	for _, terminal := range terminals {
		if attemptIDs[terminal.AttemptID] {
			selectedTerminals = append(selectedTerminals, terminal)
		}
	}
	selectedHandoffs := handoffs[:0]
	for _, handoff := range handoffs {
		if handoff.ObligationID == id {
			selectedHandoffs = append(selectedHandoffs, handoff)
		}
	}
	pairs, err := parsePairs(files[gitPairsPath])
	if err != nil {
		return State{}, false, err
	}
	selectedPairs := pairs[:0]
	for _, pair := range pairs {
		if attemptIDs[pair.AttemptID] {
			selectedPairs = append(selectedPairs, pair)
		}
	}
	states, err := reduceStates(selected[0].Scope, selected, selectedAttempts, selectedPairs, selectedTerminals, selectedHandoffs)
	if err != nil {
		return State{}, false, err
	}
	if len(states) != 1 {
		return State{}, false, fmt.Errorf("review obligation: expected one reduced state for %s", id)
	}
	return states[0], true, nil
}

func (s *GitStore) Open(ctx context.Context, scope string) ([]State, error) {
	files, err := s.log.ReadMany(ctx, gitObligationsPath, gitAttemptsPath, gitPairsPath, gitTerminalsPath, gitHandoffsPath)
	if err != nil {
		return nil, err
	}
	obligations, err := parseObligations(files[gitObligationsPath])
	if err != nil {
		return nil, err
	}
	attempts, err := parseAttempts(files[gitAttemptsPath])
	if err != nil {
		return nil, err
	}
	terminals, err := parseTerminals(files[gitTerminalsPath])
	if err != nil {
		return nil, err
	}
	handoffs, err := parseHandoffs(files[gitHandoffsPath])
	if err != nil {
		return nil, err
	}
	pairs, err := parsePairs(files[gitPairsPath])
	if err != nil {
		return nil, err
	}
	return reduceStates(scope, obligations, attempts, pairs, terminals, handoffs)
}

func (s *GitStore) CompleteHandoff(ctx context.Context, obligationID string) error {
	if obligationID == "" {
		return errors.New("review obligation: empty handoff identity")
	}
	if _, found, err := s.Get(ctx, obligationID); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("review obligation: handoff references unknown obligation %s", obligationID)
	}
	in := Handoff{SchemaVersion: HandoffSchema, ObligationID: obligationID}
	_, err := s.log.Update(ctx, gitHandoffsPath, "pyxis: complete configured-review handoff", func(current []byte) ([]byte, bool, error) {
		rows, err := parseHandoffs(current)
		if err != nil {
			return nil, false, err
		}
		for _, existing := range rows {
			if existing.ObligationID == obligationID {
				return current, false, nil
			}
		}
		return appendJSONLine(current, in)
	})
	return err
}

func (s *GitStore) Begin(ctx context.Context, in Attempt) (AttemptLease, error) {
	if err := validateAttempt(in); err != nil {
		return nil, err
	}
	lockBase, err := s.lockBase(ctx)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(in.ObligationID))
	lease, err := tryLockExclusive(fmt.Sprintf("%s.attempt-%x.lock", lockBase, digest))
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = lease.Close()
		}
	}()
	files, err := s.log.ReadMany(ctx, gitObligationsPath, gitAttemptsPath, gitPairsPath, gitTerminalsPath, gitHandoffsPath)
	if err != nil {
		return nil, err
	}
	obligations, err := parseObligations(files[gitObligationsPath])
	if err != nil {
		return nil, err
	}
	found := false
	for _, obligation := range obligations {
		found = found || obligation.ID == in.ObligationID
	}
	if !found {
		return nil, fmt.Errorf("review obligation: attempt references unknown obligation %s", in.ObligationID)
	}
	handoffs, err := parseHandoffs(files[gitHandoffsPath])
	if err != nil {
		return nil, err
	}
	handedOff := false
	for _, handoff := range handoffs {
		handedOff = handedOff || handoff.ObligationID == in.ObligationID
	}
	if !handedOff {
		return nil, ErrHandoffPending
	}
	terminals, err := parseTerminals(files[gitTerminalsPath])
	if err != nil {
		return nil, err
	}
	pairs, err := parsePairs(files[gitPairsPath])
	if err != nil {
		return nil, err
	}
	terminalByAttempt := map[string]Terminal{}
	for _, terminal := range terminals {
		terminalByAttempt[terminal.AttemptID] = terminal
		if terminal.ObligationID == in.ObligationID && terminal.Status == "verified" {
			return nil, ErrAlreadyVerified
		}
	}
	for _, pair := range pairs {
		terminal, settled := terminalByAttempt[pair.AttemptID]
		if pair.ObligationID == in.ObligationID && (!settled || terminal.Status == "verified") {
			return nil, ErrPairVerified
		}
	}
	// The local flock proves nothing about another clone or machine. The Update
	// callback below is the sole attempt-ownership fence because it evaluates the
	// freshly fetched canonical ref on every CAS retry.
	_, err = s.log.Update(ctx, gitAttemptsPath, "pyxis: admit configured-review attempt", func(current []byte) ([]byte, bool, error) {
		rows, err := parseAttempts(current)
		if err != nil {
			return nil, false, err
		}
		for _, existing := range rows {
			if existing.AttemptID == in.AttemptID {
				return nil, false, fmt.Errorf("review obligation: duplicate attempt identity %s", in.AttemptID)
			}
			if existing.ObligationID == in.ObligationID {
				if _, settled := terminalByAttempt[existing.AttemptID]; !settled {
					return nil, false, ErrAttemptActive
				}
			}
		}
		return appendJSONLine(current, in)
	})
	if err != nil {
		return nil, err
	}
	ok = true
	return lease, nil
}

func (s *GitStore) Resume(ctx context.Context, obligationID, attemptID string) (AttemptLease, error) {
	if strings.TrimSpace(obligationID) == "" || strings.TrimSpace(attemptID) == "" {
		return nil, errors.New("review obligation: recovery requires obligation and attempt identities")
	}
	lockBase, err := s.lockBase(ctx)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(obligationID))
	lease, err := tryLockExclusive(fmt.Sprintf("%s.attempt-%x.lock", lockBase, digest))
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = lease.Close()
		}
	}()
	files, err := s.log.ReadMany(ctx, gitAttemptsPath, gitTerminalsPath)
	if err != nil {
		return nil, err
	}
	attempts, err := parseAttempts(files[gitAttemptsPath])
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
	terminals, err := parseTerminals(files[gitTerminalsPath])
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

func (s *GitStore) AppendTerminal(ctx context.Context, in Terminal) error {
	return s.appendTerminal(ctx, in, true)
}

func (s *GitStore) Attempt(ctx context.Context, obligationID, attemptID string) (Attempt, string, bool, error) {
	data, err := s.log.Read(ctx, gitAttemptsPath)
	if err != nil {
		return Attempt{}, "", false, err
	}
	rows, err := parseAttempts(data)
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

func (s *GitStore) Terminal(ctx context.Context, obligationID, attemptID string) (Terminal, bool, error) {
	data, err := s.log.Read(ctx, gitTerminalsPath)
	if err != nil {
		return Terminal{}, false, err
	}
	rows, err := parseTerminals(data)
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

func (s *GitStore) RecordPair(ctx context.Context, in PublicationPair) error {
	if err := validatePair(in); err != nil {
		return err
	}
	attempt, _, found, err := s.Attempt(ctx, in.ObligationID, in.AttemptID)
	if err != nil {
		return err
	}
	if !found || attempt.AttemptProofRef != in.AttemptProofRef {
		return fmt.Errorf("review obligation: publication pair references unknown or mismatched attempt %s", in.AttemptID)
	}
	_, err = s.log.Update(ctx, gitPairsPath, "pyxis: append configured-review publication pair", func(current []byte) ([]byte, bool, error) {
		rows, err := parsePairs(current)
		if err != nil {
			return nil, false, err
		}
		for _, existing := range rows {
			if existing.AttemptID != in.AttemptID {
				continue
			}
			if existing == in {
				return current, false, nil
			}
			return nil, false, fmt.Errorf("review obligation: conflicting publication pair for %s", in.AttemptID)
		}
		return appendJSONLine(current, in)
	})
	return err
}

func (s *GitStore) appendTerminal(ctx context.Context, in Terminal, requireAttempt bool) error {
	if err := validateTerminal(in); err != nil {
		return err
	}
	if requireAttempt {
		data, err := s.log.Read(ctx, gitAttemptsPath)
		if err != nil {
			return err
		}
		attempts, err := parseAttempts(data)
		if err != nil {
			return err
		}
		found := false
		for _, attempt := range attempts {
			found = found || (attempt.AttemptID == in.AttemptID && attempt.ObligationID == in.ObligationID)
		}
		if !found {
			return fmt.Errorf("review obligation: terminal references unadmitted attempt %s", in.AttemptID)
		}
	}
	_, err := s.log.Update(ctx, gitTerminalsPath, "pyxis: append configured-review terminal", func(current []byte) ([]byte, bool, error) {
		rows, err := parseTerminals(current)
		if err != nil {
			return nil, false, err
		}
		for _, existing := range rows {
			if existing.AttemptID != in.AttemptID {
				continue
			}
			if existing == in {
				return current, false, nil
			}
			return nil, false, fmt.Errorf("review obligation: conflicting terminal replay for %s", in.AttemptID)
		}
		return appendJSONLine(current, in)
	})
	return err
}

func (s *GitStore) Verified(ctx context.Context, obligationID string) (bool, error) {
	state, ok, err := s.State(ctx, obligationID)
	return ok && state.Status == "verified", err
}

func (s *GitStore) lockBase(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", s.repo, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("review obligation: resolve git common directory: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(s.repo, common)
	}
	return filepath.Join(filepath.Clean(common), "pyxis", "canonical-review-state"), nil
}

func parseObligations(data []byte) ([]Obligation, error) {
	var rows []Obligation
	err := parseLines(data, func(line []byte) error {
		var row Obligation
		if err := decodeRecordJSON(line, &row); err != nil {
			return fmt.Errorf("review obligation: malformed row: %w", err)
		}
		if err := validate(row); err != nil {
			return err
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

func parseAttempts(data []byte) ([]Attempt, error) {
	var rows []Attempt
	err := parseLines(data, func(line []byte) error {
		var row Attempt
		if err := decodeRecordJSON(line, &row); err != nil {
			return fmt.Errorf("review obligation: malformed attempt: %w", err)
		}
		if err := validateAttempt(row); err != nil {
			return err
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

func parseTerminals(data []byte) ([]Terminal, error) {
	var rows []Terminal
	err := parseLines(data, func(line []byte) error {
		var row Terminal
		if err := decodeRecordJSON(line, &row); err != nil {
			return fmt.Errorf("review obligation: malformed terminal: %w", err)
		}
		if err := validateTerminal(row); err != nil {
			return err
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

func parsePairs(data []byte) ([]PublicationPair, error) {
	var rows []PublicationPair
	err := parseLines(data, func(line []byte) error {
		var row PublicationPair
		if err := decodeRecordJSON(line, &row); err != nil {
			return fmt.Errorf("review obligation: malformed publication pair: %w", err)
		}
		if err := validatePair(row); err != nil {
			return err
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

func parseHandoffs(data []byte) ([]Handoff, error) {
	var rows []Handoff
	err := parseLines(data, func(line []byte) error {
		var row Handoff
		if err := decodeRecordJSON(line, &row); err != nil || row.SchemaVersion != HandoffSchema || row.ObligationID == "" {
			return errors.New("review obligation: malformed handoff row")
		}
		rows = append(rows, row)
		return nil
	})
	return rows, err
}

func parseLines(data []byte, parse func([]byte) error) error {
	scan := recordScanner(data)
	for scan.Scan() {
		if err := parse(scan.Bytes()); err != nil {
			return err
		}
	}
	return scan.Err()
}

func appendJSONLine[T any](current []byte, row T) ([]byte, bool, error) {
	b, err := marshalRecord(row)
	if err != nil {
		return nil, false, err
	}
	return append(current, append(b, '\n')...), true, nil
}

func reduceStates(scope string, obligations []Obligation, attempts []Attempt, pairs []PublicationPair, terminals []Terminal, handoffs []Handoff) ([]State, error) {
	obligationByID := make(map[string]Obligation, len(obligations))
	for _, obligation := range obligations {
		if prior, exists := obligationByID[obligation.ID]; exists && prior != obligation {
			return nil, fmt.Errorf("review obligation: conflicting rows for %s", obligation.ID)
		}
		obligationByID[obligation.ID] = obligation
	}
	attemptByID := make(map[string]Attempt, len(attempts))
	for _, attempt := range attempts {
		if _, exists := obligationByID[attempt.ObligationID]; !exists {
			return nil, fmt.Errorf("review obligation: attempt %s references unknown obligation", attempt.AttemptID)
		}
		if prior, exists := attemptByID[attempt.AttemptID]; exists && prior != attempt {
			return nil, fmt.Errorf("review obligation: conflicting attempt rows for %s", attempt.AttemptID)
		}
		attemptByID[attempt.AttemptID] = attempt
	}
	terminalByAttempt := make(map[string]Terminal, len(terminals))
	for _, terminal := range terminals {
		attempt, exists := attemptByID[terminal.AttemptID]
		if !exists || attempt.ObligationID != terminal.ObligationID {
			return nil, fmt.Errorf("review obligation: terminal %s has no matching admitted attempt", terminal.AttemptID)
		}
		if prior, exists := terminalByAttempt[terminal.AttemptID]; exists && prior != terminal {
			return nil, fmt.Errorf("review obligation: conflicting terminal rows for %s", terminal.AttemptID)
		}
		terminalByAttempt[terminal.AttemptID] = terminal
	}
	pairByAttempt := make(map[string]PublicationPair, len(pairs))
	for _, pair := range pairs {
		attempt, exists := attemptByID[pair.AttemptID]
		if !exists || attempt.ObligationID != pair.ObligationID || attempt.AttemptProofRef != pair.AttemptProofRef {
			return nil, fmt.Errorf("review obligation: publication pair %s has no matching admitted attempt", pair.AttemptID)
		}
		obligation := obligationByID[pair.ObligationID]
		if pair.ProposalRef != obligation.ProposalRef || pair.Revision != obligation.Revision {
			return nil, fmt.Errorf("review obligation: publication pair %s mismatches proposal or revision", pair.AttemptID)
		}
		if prior, exists := pairByAttempt[pair.AttemptID]; exists && prior != pair {
			return nil, fmt.Errorf("review obligation: conflicting publication pairs for %s", pair.AttemptID)
		}
		pairByAttempt[pair.AttemptID] = pair
	}
	var out []State
	handoffByObligation := map[string]bool{}
	for _, handoff := range handoffs {
		if _, exists := obligationByID[handoff.ObligationID]; !exists {
			return nil, fmt.Errorf("review obligation: handoff references unknown obligation %s", handoff.ObligationID)
		}
		handoffByObligation[handoff.ObligationID] = true
	}
	for _, attempt := range attempts {
		if !handoffByObligation[attempt.ObligationID] {
			return nil, fmt.Errorf("review obligation: attempt %s precedes implementation terminal handoff", attempt.AttemptID)
		}
	}
	for _, obligation := range obligations {
		if obligation.Scope != scope {
			continue
		}
		state := State{Obligation: obligation, Status: "handoff_pending"}
		if handoffByObligation[obligation.ID] {
			state.Status = "review_pending"
		}
		for _, attempt := range attempts {
			if attempt.ObligationID != obligation.ID {
				continue
			}
			state.AttemptCount++
			if pair, ok := pairByAttempt[attempt.AttemptID]; ok {
				pairCopy := pair
				state.Pair = &pairCopy
				state.Status = "pair_verified"
			}
			if terminal, ok := terminalByAttempt[attempt.AttemptID]; ok {
				if terminal.Status == "verified" {
					pair, paired := pairByAttempt[attempt.AttemptID]
					if !paired || terminal.ArtifactRef != pair.ArtifactRef || terminal.NativeReceipt != pair.NativeReceipt || terminal.AttemptProofRef != pair.AttemptProofRef || terminal.EVT13Ref != pair.EVT13Ref || terminal.EVT17Ref != pair.EVT17Ref {
						return nil, fmt.Errorf("review obligation: verified terminal %s lacks its exact publication pair", attempt.AttemptID)
					}
					state.Status = "verified"
				}
				if terminal.Status == "failed" {
					if _, paired := pairByAttempt[attempt.AttemptID]; paired {
						// Historical recovery code could append a verified pair and then
						// lose a race to an already-durable failed lifecycle terminal. The
						// immutable failed terminal wins; the orphaned pair is nonaccepting
						// and cannot block the contract's required fresh process generation.
						state.Pair = nil
						state.Status = "review_pending"
					}
					state.LastFailureClass = terminal.FailureClass
				}
			}
		}
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Obligation.ID < out[j].Obligation.ID })
	return out, nil
}
