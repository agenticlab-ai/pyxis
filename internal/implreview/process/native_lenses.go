package process

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

const nativeLensPath = "impl-review/native-lens-evidence.jsonl"

// A separate path keeps the existing process journal readable by older hosts.
// Older completions have no native proof and cannot acquire it retrospectively.
type nativeLensRow struct {
	Process  implreview.AdmittedProcess    `json:"process"`
	Evidence implreview.NativeLensEvidence `json:"evidence"`
}

func (c *Coordinator) RecordNativeLensEvidence(ctx context.Context, admitted implreview.AdmittedProcess, evidence implreview.NativeLensEvidence) error {
	store, ok := c.store.(implreview.NativeLensEvidenceCoordinator)
	if !ok {
		return errors.New("impl-review process: native lens evidence store unavailable")
	}
	return store.RecordNativeLensEvidence(ctx, admitted, evidence)
}

func (c *Coordinator) VerifyNativeLensEvidence(ctx context.Context, admitted implreview.AdmittedProcess) error {
	store, ok := c.store.(implreview.NativeLensEvidenceCoordinator)
	if !ok {
		return errors.New("impl-review process: native lens evidence store unavailable")
	}
	return store.VerifyNativeLensEvidence(ctx, admitted)
}

func (s *GitStore) RecordNativeLensEvidence(ctx context.Context, admitted implreview.AdmittedProcess, evidence implreview.NativeLensEvidence) error {
	if err := validateNativeLensRow(nativeLensRow{admitted, evidence}); err != nil {
		return err
	}
	if _, err := s.nativeLensProcess(ctx, admitted, false); err != nil {
		return err
	}
	_, err := s.log.Update(ctx, nativeLensPath, "pyxis: verify native review lenses", func(current []byte) ([]byte, bool, error) {
		rows, err := parseNativeLensRows(current)
		if err != nil {
			return nil, false, err
		}
		next := nativeLensRow{admitted, evidence}
		for _, row := range rows {
			if row.Process.ReviewProcessID == admitted.ReviewProcessID {
				if row != next {
					return nil, false, errors.New("impl-review process: conflicting native lens evidence")
				}
				return current, false, nil
			}
		}
		wire, err := json.Marshal(next)
		if err != nil {
			return nil, false, err
		}
		return append(append(append([]byte(nil), current...), wire...), '\n'), true, nil
	})
	if err != nil {
		return fmt.Errorf("impl-review process: record native lenses: %w", err)
	}
	return nil
}

func (s *GitStore) VerifyNativeLensEvidence(ctx context.Context, admitted implreview.AdmittedProcess) error {
	digest, err := s.nativeLensProcess(ctx, admitted, true)
	if err != nil {
		return err
	}
	data, err := s.log.Read(ctx, nativeLensPath)
	if err != nil {
		return err
	}
	rows, err := parseNativeLensRows(data)
	if err != nil {
		return err
	}
	found := false
	for _, row := range rows {
		if row.Process.ReviewProcessID != admitted.ReviewProcessID {
			continue
		}
		if row.Process != admitted || row.Evidence.AggregateDigest != digest {
			return errors.New("impl-review process: native lens proof does not match completed process")
		}
		found = true
	}
	if !found {
		return errors.New("impl-review process: completed process has no host-verified native lens evidence")
	}
	return nil
}

func (s *GitStore) nativeLensProcess(ctx context.Context, admitted implreview.AdmittedProcess, completed bool) (string, error) {
	if err := validateAdmitted(admitted); err != nil {
		return "", err
	}
	data, err := s.log.Read(ctx, processStatePath)
	if err != nil {
		return "", err
	}
	rows, err := parseRows(data)
	if err != nil {
		return "", err
	}
	found, digest := false, ""
	for _, row := range rows {
		if row.ReviewProcessID != admitted.ReviewProcessID {
			continue
		}
		if row.Subject != admitted.Subject {
			return "", errors.New("impl-review process: native lens subject mismatch")
		}
		if row.Kind == processAdmission {
			found = true
		} else {
			if !completed || row.Status != implreview.HandoffCompleted || (digest != "" && digest != row.DiagnosticDigest) {
				return "", errors.New("impl-review process: native lens completion boundary mismatch")
			}
			digest = row.DiagnosticDigest
		}
	}
	if !found || (completed && digest == "") {
		return "", errors.New("impl-review process: native lens process evidence unavailable")
	}
	return digest, nil
}

func parseNativeLensRows(data []byte) ([]nativeLensRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var rows []nativeLensRow
	for {
		var row nativeLensRow
		if err := decoder.Decode(&row); errors.Is(err, io.EOF) {
			return rows, nil
		} else if err != nil {
			return nil, fmt.Errorf("impl-review process: decode native lenses: %w", err)
		}
		if err := validateNativeLensRow(row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
}

func validateNativeLensRow(row nativeLensRow) error {
	if err := validateAdmitted(row.Process); err != nil {
		return err
	}
	if row.Evidence.SchemaVersion != implreview.NativeLensEvidenceSchema {
		return errors.New("impl-review process: unknown native lens evidence schema")
	}
	for _, digest := range []string{row.Evidence.PackDigest, row.Evidence.EvidenceDigest, row.Evidence.AggregateDigest} {
		value, ok := strings.CutPrefix(digest, "sha256:")
		decoded, err := hex.DecodeString(value)
		if !ok || err != nil || len(decoded) != 32 {
			return errors.New("impl-review process: invalid native lens digest")
		}
	}
	return nil
}
