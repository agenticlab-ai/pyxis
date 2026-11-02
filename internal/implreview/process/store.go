package process

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

var ErrProcessActive = errors.New("impl-review process: subject already has a live coordinator")

const (
	DefaultEvidenceRef = "refs/pyxis/impl-review-process-state"
	processStatePath   = "impl-review/process-state.jsonl"
	processRowSchema   = "impl-review-process-state/v1"
	processAdmission   = "admitted"
	processTerminal    = "terminal"
)

type processRow struct {
	SchemaVersion    string                   `json:"schema_version"`
	Kind             string                   `json:"kind"`
	ReviewProcessID  string                   `json:"review_process_id"`
	Subject          implreview.ReviewSubject `json:"subject"`
	Status           implreview.HandoffStatus `json:"status,omitempty"`
	DiagnosticDigest string                   `json:"diagnostic_digest,omitempty"`
}

// GitStore persists process admission and completion on a fetched, CAS-updated
// evidence ref. The ref is outside every reviewer workspace.
type GitStore struct {
	repo string
	log  *gitrefstore.Store
	mu   sync.Mutex
	held map[string]*fileLock
}

func NewGitStore(repoDir, remote, ref string) (*GitStore, error) {
	log, err := gitrefstore.New(repoDir, remote, ref)
	if err != nil {
		return nil, err
	}
	return &GitStore{repo: repoDir, log: log, held: map[string]*fileLock{}}, nil
}

func (s *GitStore) Preflight(ctx context.Context) error {
	data, err := s.log.Read(ctx, processStatePath)
	if err != nil {
		return err
	}
	_, err = parseRows(data)
	return err
}

func (s *GitStore) MissingProcesses(ctx context.Context, subject implreview.ReviewSubject) ([]implreview.AdmittedProcess, error) {
	if err := validateSubject(subject); err != nil {
		return nil, err
	}
	data, err := s.log.Read(ctx, processStatePath)
	if err != nil {
		return nil, err
	}
	rows, err := parseRows(data)
	if err != nil {
		return nil, err
	}
	admissions, terminals := map[string]processRow{}, map[string]processRow{}
	for _, row := range rows {
		dest := admissions
		if row.Kind == processTerminal {
			dest = terminals
		}
		if prior, found := dest[row.ReviewProcessID]; found && prior != row {
			return nil, errors.New("impl-review process: conflicting process evidence")
		}
		dest[row.ReviewProcessID] = row
	}
	for id, terminal := range terminals {
		admission, found := admissions[id]
		if !found || admission.Subject != terminal.Subject {
			return nil, errors.New("impl-review process: terminal lacks matching admission")
		}
	}
	var missing []implreview.AdmittedProcess
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Kind != processAdmission || row.Subject != subject || seen[row.ReviewProcessID] {
			continue
		}
		seen[row.ReviewProcessID] = true
		if terminal, found := terminals[row.ReviewProcessID]; found && terminal.Status == implreview.HandoffMissing {
			missing = append(missing, implreview.AdmittedProcess{ReviewProcessID: row.ReviewProcessID, Subject: subject})
		}
	}
	return missing, nil
}

func (s *GitStore) Prepare(ctx context.Context, subject implreview.ReviewSubject) (implreview.AdmittedProcess, error) {
	lockPath, err := s.subjectLockPath(ctx, subject)
	if err != nil {
		return implreview.AdmittedProcess{}, err
	}
	lease, err := tryLockExclusive(lockPath)
	if err != nil {
		return implreview.AdmittedProcess{}, err
	}
	keepLease := false
	defer func() {
		if !keepLease {
			_ = lease.Close()
		}
	}()
	processID, err := newProcessID(subject)
	if err != nil {
		return implreview.AdmittedProcess{}, err
	}
	var selected implreview.AdmittedProcess
	_, err = s.log.Update(ctx, processStatePath, "pyxis: admit impl-review process", func(current []byte) ([]byte, bool, error) {
		rows, err := parseRows(current)
		if err != nil {
			return nil, false, err
		}
		terminal := map[string]bool{}
		for _, row := range rows {
			if row.Kind == processTerminal {
				terminal[row.ReviewProcessID] = true
			}
		}
		for _, row := range rows {
			if row.Kind == processAdmission && row.Subject == subject && !terminal[row.ReviewProcessID] {
				// A clone-local file lock cannot prove that an owner in another
				// clone or machine died. The fetched-ref CAS is the cross-host
				// fence: a live admission remains active until an explicit durable
				// terminal exists, rather than being guessed abandoned here.
				return nil, false, ErrProcessActive
			}
		}
		selected = implreview.AdmittedProcess{ReviewProcessID: processID, Subject: subject}
		return appendRow(current, processRow{SchemaVersion: processRowSchema, Kind: processAdmission, ReviewProcessID: processID, Subject: subject})
	})
	if err != nil {
		return implreview.AdmittedProcess{}, fmt.Errorf("impl-review process: admit: %w", err)
	}
	s.mu.Lock()
	s.held[selected.ReviewProcessID] = lease
	s.mu.Unlock()
	keepLease = true
	return selected, nil
}

func (s *GitStore) Finish(ctx context.Context, admitted implreview.AdmittedProcess, status implreview.HandoffStatus, digest string) error {
	defer s.release(admitted.ReviewProcessID)
	_, err := s.log.Update(ctx, processStatePath, "pyxis: finish impl-review process", func(current []byte) ([]byte, bool, error) {
		rows, err := parseRows(current)
		if err != nil {
			return nil, false, err
		}
		found := false
		for _, row := range rows {
			if row.Kind == processAdmission && row.ReviewProcessID == admitted.ReviewProcessID {
				if row.Subject != admitted.Subject {
					return nil, false, errors.New("impl-review process: admitted subject conflict")
				}
				found = true
			}
			if row.Kind == processTerminal && row.ReviewProcessID == admitted.ReviewProcessID {
				if row.Subject == admitted.Subject && row.Status == status && row.DiagnosticDigest == digest {
					return current, false, nil
				}
				return nil, false, errors.New("impl-review process: conflicting terminal replay")
			}
		}
		if !found {
			return nil, false, errors.New("impl-review process: terminal references unknown process")
		}
		return appendRow(current, processRow{SchemaVersion: processRowSchema, Kind: processTerminal, ReviewProcessID: admitted.ReviewProcessID, Subject: admitted.Subject, Status: status, DiagnosticDigest: digest})
	})
	if err != nil {
		return fmt.Errorf("impl-review process: finish: %w", err)
	}
	return nil
}

func (s *GitStore) release(processID string) {
	s.mu.Lock()
	lease := s.held[processID]
	delete(s.held, processID)
	s.mu.Unlock()
	if lease != nil {
		_ = lease.Close()
	}
}

func (s *GitStore) subjectLockPath(ctx context.Context, subject implreview.ReviewSubject) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", s.repo, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("impl-review process: resolve git common directory: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(s.repo, common)
	}
	b, err := json.Marshal(subject)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(b)
	return filepath.Join(filepath.Clean(common), "pyxis", fmt.Sprintf("impl-review-process-%x.lock", digest)), nil
}

func appendRow(current []byte, row processRow) ([]byte, bool, error) {
	encoded, err := json.Marshal(row)
	if err != nil {
		return nil, false, err
	}
	next := append(append([]byte(nil), current...), encoded...)
	next = append(next, '\n')
	return next, true, nil
}

func parseRows(data []byte) ([]processRow, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rows []processRow
	for {
		var row processRow
		if err := dec.Decode(&row); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode process state: %w", err)
		}
		if row.SchemaVersion != processRowSchema || strings.TrimSpace(row.ReviewProcessID) == "" {
			return nil, errors.New("impl-review process: malformed process-state row")
		}
		switch row.Kind {
		case processAdmission:
			if row.Status != "" || row.DiagnosticDigest != "" || validateSubject(row.Subject) != nil {
				return nil, errors.New("impl-review process: malformed admission row")
			}
		case processTerminal:
			if !validHandoffStatus(row.Status) || row.DiagnosticDigest == "" || validateSubject(row.Subject) != nil {
				return nil, errors.New("impl-review process: malformed terminal row")
			}
		default:
			return nil, errors.New("impl-review process: unknown row kind")
		}
		rows = append(rows, row)
	}
	return rows, nil
}
