// Package git implements retained maintenance claims over gitrefstore.
package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type refStore interface {
	Update(context.Context, string, string, func([]byte) ([]byte, bool, error)) ([]byte, error)
	Read(context.Context, string) ([]byte, error)
}

type Control interface {
	RefStore(string) (*gitrefstore.Store, error)
}

type Store struct {
	refs  refStore
	jobID string
}

func New(control Control, jobID string) (*Store, error) {
	if control == nil || !jobs.ValidID(jobID) {
		return nil, errors.New("maintenance Git claim store requires control and valid job ID")
	}
	refs, err := control.RefStore("refs/pyxis/maintenance/" + jobID)
	if err != nil {
		return nil, fmt.Errorf("open maintenance claim ref: %w", err)
	}
	return &Store{refs: refs, jobID: jobID}, nil
}

func (s *Store) Claim(ctx context.Context, candidate claim.Record) (claim.ClaimResult, error) {
	if s == nil || s.refs == nil {
		return claim.ClaimResult{}, errors.New("maintenance Git claim store is required")
	}
	if err := candidate.Validate(); err != nil || candidate.Outcome != claim.OutcomeClaimed || candidate.JobID != s.jobID {
		return claim.ClaimResult{}, errors.Join(errors.New("invalid maintenance claim candidate"), err)
	}
	path := occurrencePath(candidate.JobDate)
	wanted, err := claim.Encode(candidate)
	if err != nil {
		return claim.ClaimResult{}, err
	}
	attemptedInsert, observedExisting := false, false
	_, writeErr := s.refs.Update(ctx, path, "claim maintenance occurrence "+candidate.JobDate, func(current []byte) ([]byte, bool, error) {
		if len(current) == 0 {
			attemptedInsert = true
			return wanted, true, nil
		}
		observedExisting = true
		existing, err := claim.Decode(current)
		if err != nil {
			return nil, false, err
		}
		if existing.JobID != candidate.JobID || existing.JobDate != candidate.JobDate {
			return nil, false, errors.New("maintenance occurrence identity does not match its remote key")
		}
		return current, false, nil
	})
	confirmed, readErr := s.confirm(ctx, path, candidate)
	if readErr != nil {
		return claim.ClaimResult{}, errors.Join(writeErr, fmt.Errorf("confirm remote maintenance claim: %w", readErr))
	}
	if confirmed.OwnerNonce == candidate.OwnerNonce && !claim.SameOwner(candidate, confirmed) {
		return claim.ClaimResult{}, errors.New("remote maintenance owner nonce has mismatched immutable proof")
	}
	if attemptedInsert && !observedExisting && claim.SameOwner(candidate, confirmed) {
		return claim.ClaimResult{Disposition: claim.DispositionOwned, Record: confirmed}, nil
	}
	return claim.ClaimResult{Disposition: claim.DispositionConsumed, Record: confirmed}, nil
}

func (s *Store) Transition(ctx context.Context, owned claim.Record, terminal claim.Terminal) (claim.Record, error) {
	if s == nil || s.refs == nil || owned.JobID != s.jobID {
		return claim.Record{}, errors.New("maintenance Git claim store is required")
	}
	next, err := claim.WithTerminal(owned, terminal)
	if err != nil {
		return claim.Record{}, err
	}
	path := occurrencePath(owned.JobDate)
	wanted, err := claim.Encode(next)
	if err != nil {
		return claim.Record{}, err
	}
	_, writeErr := s.refs.Update(ctx, path, "finish maintenance occurrence "+owned.JobDate, func(current []byte) ([]byte, bool, error) {
		existing, err := claim.Decode(current)
		if err != nil {
			return nil, false, err
		}
		if !claim.SameOwner(owned, existing) {
			return nil, false, errors.New("maintenance terminal transition owner does not match")
		}
		if existing.Outcome != claim.OutcomeClaimed {
			encoded, encodeErr := claim.Encode(existing)
			return encoded, false, encodeErr
		}
		return wanted, true, nil
	})
	confirmed, readErr := s.confirm(ctx, path, owned)
	if readErr != nil || !reflectTerminal(next, confirmed) {
		return claim.Record{}, errors.Join(writeErr, readErr, errors.New("remote maintenance terminal transition was not proved"))
	}
	return confirmed, nil
}

func (s *Store) confirm(ctx context.Context, path string, identity claim.Record) (claim.Record, error) {
	data, err := s.refs.Read(ctx, path)
	if err != nil {
		return claim.Record{}, err
	}
	record, err := claim.Decode(data)
	if err != nil {
		return claim.Record{}, err
	}
	if record.JobID != identity.JobID || record.JobDate != identity.JobDate {
		return claim.Record{}, errors.New("maintenance occurrence identity does not match its remote key")
	}
	return record, nil
}

func occurrencePath(date string) string {
	return filepath.ToSlash(filepath.Join("occurrences", date+".json"))
}

func reflectTerminal(want, got claim.Record) bool {
	wantBytes, wantErr := claim.Encode(want)
	gotBytes, gotErr := claim.Encode(got)
	return wantErr == nil && gotErr == nil && string(wantBytes) == string(gotBytes)
}
