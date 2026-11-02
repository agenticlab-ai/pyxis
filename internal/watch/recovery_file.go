package watch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// FileRecoveryLedger is the process-safe append-only recovery adapter. Its
// production path is in Git's common directory, outside every disposable
// implementation/review workspace. The entry evidence is content-addressed;
// the store never edits or deletes a prior decision.
type FileRecoveryLedger struct {
	path string
	mu   sync.Mutex
}

func NewFileRecoveryLedger(path string) (*FileRecoveryLedger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("watch: empty review recovery ledger path")
	}
	return &FileRecoveryLedger{path: path}, nil
}

func NewRepositoryRecoveryLedger(ctx context.Context, repoDir string) (*FileRecoveryLedger, error) {
	commonDir, err := gitCommonDir(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	return NewFileRecoveryLedger(filepath.Join(commonDir, "pyxis", "review-recovery.jsonl"))
}

func gitCommonDir(ctx context.Context, repoDir string) (string, error) {
	if strings.TrimSpace(repoDir) == "" {
		return "", errors.New("watch: empty repository path")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("watch: resolve git common directory: %w", err)
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return "", errors.New("watch: git returned an empty common directory")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoDir, commonDir)
	}
	commonDir, err = filepath.Abs(commonDir)
	if err != nil {
		return "", fmt.Errorf("watch: resolve absolute common directory: %w", err)
	}
	return filepath.Clean(commonDir), nil
}

func (l *FileRecoveryLedger) Exact(ctx context.Context, scope string) ([]RecoveryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	lock, err := lockRecoveryLedger(l.path + ".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	return l.exactLocked(scope)
}

func (l *FileRecoveryLedger) exactLocked(scope string) ([]RecoveryEntry, error) {
	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("watch: read review recovery ledger: %w", err)
	}
	defer f.Close()
	var out []RecoveryEntry
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scan.Scan() {
		dec := json.NewDecoder(strings.NewReader(scan.Text()))
		dec.DisallowUnknownFields()
		var row RecoveryEntry
		if err := dec.Decode(&row); err != nil {
			return nil, fmt.Errorf("watch: malformed review recovery row: %w", err)
		}
		if err := validateRecoveryEntry(row); err != nil {
			return nil, err
		}
		if row.Scope == scope {
			out = append(out, row)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("watch: scan review recovery ledger: %w", err)
	}
	return out, nil
}

func (l *FileRecoveryLedger) Append(ctx context.Context, in RecoveryEntry) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	sealed, err := sealRecoveryEntry(in)
	if err != nil {
		return "", err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	lock, err := lockRecoveryLedger(l.path + ".lock")
	if err != nil {
		return "", err
	}
	defer lock.Close()
	rows, err := l.exactLocked(sealed.Scope)
	if err != nil {
		return "", err
	}
	key := recoveryIdempotenceKey(sealed)
	for _, existing := range rows {
		if recoveryIdempotenceKey(existing) != key {
			continue
		}
		if existing == sealed {
			return existing.EvidenceRef, nil
		}
		return "", fmt.Errorf("watch: conflicting review recovery replay for %s", sealed.ObligationID)
	}
	candidate := append(append([]RecoveryEntry(nil), rows...), sealed)
	if _, err := reduceRecovery(sealed.Scope, candidate); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return "", fmt.Errorf("watch: create review recovery directory: %w", err)
	}
	b, err := json.Marshal(sealed)
	if err != nil {
		return "", fmt.Errorf("watch: marshal review recovery row: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("watch: open review recovery ledger: %w", err)
	}
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", fmt.Errorf("watch: append review recovery ledger: %w", err)
	}
	if closeErr != nil {
		return "", fmt.Errorf("watch: close review recovery ledger: %w", closeErr)
	}
	return sealed.EvidenceRef, nil
}

func recoveryIdempotenceKey(e RecoveryEntry) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s", e.Scope, e.ObligationID, e.RecoveryEpoch, e.RecoveryCount, e.Action)
}
