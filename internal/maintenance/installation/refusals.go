package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

const RefusalSchemaVersion = "pyxis-maintenance-refusals/v1"
const refusalName = "refusals.json"
const maxRefusalBytes = 64 << 10

var ErrRefusalsAbsent = errors.New("maintenance refusal counters are absent")

type RefusalCause string

const (
	CauseInstallationFailed  RefusalCause = "installation_failed"
	CauseAdmissionRefused    RefusalCause = "admission_refused"
	CauseClaimUnproved       RefusalCause = "claim_unproved"
	CauseAuditFailed         RefusalCause = "audit_failed"
	CauseWorkspaceFailed     RefusalCause = "workspace_failed"
	CauseExecutionRefused    RefusalCause = "execution_refused"
	CauseExecutionFailed     RefusalCause = "execution_failed"
	CauseExecutionTimedOut   RefusalCause = "execution_timed_out"
	CauseExecutionIncomplete RefusalCause = "execution_incomplete"
	CauseTerminalFailed      RefusalCause = "terminal_metadata_failed"
)

var refusalCauses = map[RefusalCause]bool{
	CauseInstallationFailed: true, CauseAdmissionRefused: true, CauseClaimUnproved: true,
	CauseAuditFailed: true, CauseWorkspaceFailed: true, CauseExecutionRefused: true,
	CauseExecutionFailed: true, CauseExecutionTimedOut: true, CauseExecutionIncomplete: true,
	CauseTerminalFailed: true,
}

type RefusalCounter struct {
	JobID          string       `json:"job_id"`
	Cause          RefusalCause `json:"cause"`
	Lifetime       uint64       `json:"lifetime"`
	Consecutive    uint64       `json:"consecutive"`
	FirstObserved  time.Time    `json:"first_observed"`
	LastObserved   time.Time    `json:"last_observed"`
	LastInvocation string       `json:"last_invocation"`
}

type RefusalState struct {
	SchemaVersion string           `json:"schema_version"`
	RepositoryID  string           `json:"repository_id"`
	Counters      []RefusalCounter `json:"counters"`
}

func (store *Store) LoadRefusals(ctx context.Context) (RefusalState, error) {
	if err := ctx.Err(); err != nil {
		return RefusalState{}, err
	}
	file, err := os.OpenFile(filepath.Join(filepath.Dir(store.ManifestPath()), refusalName), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return RefusalState{}, ErrRefusalsAbsent
	}
	if err != nil {
		return RefusalState{}, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return RefusalState{}, errors.New("maintenance refusal state must be a 0600 regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRefusalBytes+1))
	if err != nil || len(data) > maxRefusalBytes {
		return RefusalState{}, errors.Join(errors.New("read bounded maintenance refusal state"), err)
	}
	return decodeRefusals(data, store.repositoryID)
}

func (store *Store) RecordRefusal(ctx context.Context, invocation, jobID string, cause RefusalCause, at time.Time) error {
	if !validRefusalInput(invocation, jobID, cause, at) {
		return errors.New("invalid maintenance refusal counter input")
	}
	return store.WithTransaction(ctx, func(*Transaction) error {
		state, err := store.LoadRefusals(ctx)
		if errors.Is(err, ErrRefusalsAbsent) {
			state = RefusalState{SchemaVersion: RefusalSchemaVersion, RepositoryID: store.repositoryID, Counters: []RefusalCounter{}}
		} else if err != nil {
			return err
		}
		for index := range state.Counters {
			counter := &state.Counters[index]
			if counter.JobID == jobID && counter.Cause == cause {
				if counter.LastInvocation == invocation {
					return nil
				}
				counter.Lifetime, counter.Consecutive, counter.LastObserved, counter.LastInvocation = counter.Lifetime+1, counter.Consecutive+1, at.UTC(), invocation
				return store.publishRefusals(ctx, state)
			}
		}
		state.Counters = append(state.Counters, RefusalCounter{JobID: jobID, Cause: cause, Lifetime: 1, Consecutive: 1, FirstObserved: at.UTC(), LastObserved: at.UTC(), LastInvocation: invocation})
		return store.publishRefusals(ctx, state)
	})
}

func (store *Store) ResetRefusals(ctx context.Context, jobID string) error {
	return store.WithTransaction(ctx, func(*Transaction) error {
		state, err := store.LoadRefusals(ctx)
		if errors.Is(err, ErrRefusalsAbsent) {
			return nil
		}
		if err != nil {
			return err
		}
		changed := false
		for index := range state.Counters {
			if (jobID == "" || state.Counters[index].JobID == jobID) && state.Counters[index].Consecutive != 0 {
				state.Counters[index].Consecutive, changed = 0, true
			}
		}
		if !changed {
			return nil
		}
		return store.publishRefusals(ctx, state)
	})
}

func validRefusalInput(invocation, jobID string, cause RefusalCause, at time.Time) bool {
	return validHex(invocation, 32) && (jobID == "installation" || jobs.ValidID(jobID)) && refusalCauses[cause] && !at.IsZero()
}

func decodeRefusals(data []byte, repositoryID string) (RefusalState, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var state RefusalState
	if err := decoder.Decode(&state); err != nil {
		return RefusalState{}, fmt.Errorf("decode maintenance refusal state: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) || state.SchemaVersion != RefusalSchemaVersion || state.RepositoryID != repositoryID {
		return RefusalState{}, errors.New("invalid maintenance refusal state schema or repository")
	}
	seen := map[string]bool{}
	for _, counter := range state.Counters {
		key := counter.JobID + "\x00" + string(counter.Cause)
		if !validRefusalInput(counter.LastInvocation, counter.JobID, counter.Cause, counter.LastObserved) || counter.Lifetime == 0 || counter.Consecutive > counter.Lifetime || counter.FirstObserved.IsZero() || counter.FirstObserved.Location() != time.UTC || counter.LastObserved.Location() != time.UTC || counter.FirstObserved.After(counter.LastObserved) || seen[key] {
			return RefusalState{}, errors.New("invalid maintenance refusal counter")
		}
		seen[key] = true
	}
	encoded, _ := json.Marshal(state)
	if !bytes.Equal(data, append(encoded, '\n')) {
		return RefusalState{}, errors.New("maintenance refusal state is noncanonical")
	}
	return state, nil
}

func (store *Store) publishRefusals(ctx context.Context, state RefusalState) error {
	data, err := json.Marshal(state)
	if err != nil || len(data)+1 > maxRefusalBytes {
		return errors.Join(errors.New("encode bounded maintenance refusal state"), err)
	}
	_, err = store.publishFile(ctx, refusalName, append(data, '\n'))
	return err
}
