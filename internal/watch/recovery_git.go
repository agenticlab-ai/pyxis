package watch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

const DefaultRecoveryEvidenceRef = "refs/pyxis/review-recovery"

const gitRecoveryPath = "review-recovery/ledger.jsonl"

// GitRecoveryLedger is the production consumer ledger. Each append is a CAS
// commit on the configured fetched evidence ref; Exact always fetches first.
type GitRecoveryLedger struct{ log *gitrefstore.Store }

func NewGitRecoveryLedger(repoDir, remote, ref string) (*GitRecoveryLedger, error) {
	store, err := gitrefstore.New(repoDir, remote, ref)
	if err != nil {
		return nil, err
	}
	return &GitRecoveryLedger{log: store}, nil
}

func (l *GitRecoveryLedger) Exact(ctx context.Context, scope string) ([]RecoveryEntry, error) {
	data, err := l.log.Read(ctx, gitRecoveryPath)
	if err != nil {
		return nil, err
	}
	return parseRecoveryRows(data, scope)
}

func (l *GitRecoveryLedger) Append(ctx context.Context, in RecoveryEntry) (string, error) {
	sealed, err := sealRecoveryEntry(in)
	if err != nil {
		return "", err
	}
	_, err = l.log.Update(ctx, gitRecoveryPath, "pyxis: append review recovery decision", func(current []byte) ([]byte, bool, error) {
		rows, err := parseRecoveryRows(current, sealed.Scope)
		if err != nil {
			return nil, false, err
		}
		key := recoveryIdempotenceKey(sealed)
		for _, existing := range rows {
			if recoveryIdempotenceKey(existing) != key {
				continue
			}
			if existing == sealed {
				return current, false, nil
			}
			return nil, false, fmt.Errorf("watch: conflicting review recovery replay for %s", sealed.ObligationID)
		}
		candidate := append(append([]RecoveryEntry(nil), rows...), sealed)
		if _, err := reduceRecovery(sealed.Scope, candidate); err != nil {
			return nil, false, err
		}
		b, err := json.Marshal(sealed)
		if err != nil {
			return nil, false, err
		}
		return append(current, append(b, '\n')...), true, nil
	})
	if err != nil {
		return "", err
	}
	return sealed.EvidenceRef, nil
}

func parseRecoveryRows(data []byte, scope string) ([]RecoveryEntry, error) {
	var out []RecoveryEntry
	scan := bufio.NewScanner(bytes.NewReader(data))
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
		return nil, err
	}
	return out, nil
}
