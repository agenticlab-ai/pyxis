// Package carina owns impl-review's local ProposalService review-record slice.
// It is intentionally separate from the WorkTracker decision namespaces: a
// review artifact is an immutable proposal record addressed by its own native
// receipt, never a latest-wins tracker decision.
package carina

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const recordSchema = "carina-impl-review-record/v1"

// invalidRecordError distinguishes deterministic receipt or record corruption
// from transient storage failures. Host verification must not retry or
// classify immutable evidence corruption as an infrastructure outage.
type invalidRecordError struct {
	cause error
}

func (e *invalidRecordError) Error() string { return e.cause.Error() }
func (e *invalidRecordError) Unwrap() error { return e.cause }

func invalidRecord(err error) error {
	if err == nil {
		return nil
	}
	var invalid *invalidRecordError
	if errors.As(err, &invalid) {
		return err
	}
	return &invalidRecordError{cause: err}
}

// ReviewRecord is the closed immutable object stored on the local proposal
// medium. CreatedAt and RecordDigest are store-owned fields.
type ReviewRecord struct {
	SchemaVersion        string    `json:"schema_version"`
	Repository           string    `json:"repository"`
	Proposal             int       `json:"proposal"`
	IdempotencyID        string    `json:"idempotency_id"`
	PublicationAttemptID string    `json:"publication_attempt_id"`
	ReviewObligationID   string    `json:"review_obligation_id"`
	ReviewProcessID      string    `json:"review_process_id"`
	ProposalID           string    `json:"proposal_id"`
	ReviewedRevision     string    `json:"reviewed_revision"`
	ArtifactDigest       string    `json:"artifact_digest"`
	ArtifactBody         string    `json:"artifact_body"`
	CreatedAt            time.Time `json:"created_at"`
	RecordDigest         string    `json:"record_digest"`
}

// ReviewRecordStore is the narrow design-owned ProposalService slice used by
// the host adapter. It deliberately exposes no proposal-wide scan operation.
type ReviewRecordStore interface {
	Preflight(context.Context, string) error
	CreateIfAbsent(context.Context, ReviewRecord) (string, error)
	ExactRead(context.Context, string) (ReviewRecord, error)
}

// FileRecordStore stores identity-derived immutable records beneath root.
type FileRecordStore struct {
	root string
	now  func() time.Time
}

func NewFileRecordStore(root string) (*FileRecordStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("carina review record: empty proposal-store root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("carina review record: resolve root: %w", err)
	}
	return &FileRecordStore{root: filepath.Clean(abs), now: time.Now}, nil
}

func (s *FileRecordStore) WithClock(now func() time.Time) *FileRecordStore {
	if now != nil {
		s.now = now
	}
	return s
}

// Preflight is side-effect-free. It proves the configured existing directory
// is a directory and has at least one write bit; publication still uses O_EXCL
// and treats any actual permission race as infrastructure failure.
func (s *FileRecordStore) Preflight(ctx context.Context, repository string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(repository) == "" {
		return errors.New("carina review record: empty repository")
	}
	info, err := os.Stat(s.root)
	if err != nil {
		return fmt.Errorf("carina review record: inspect proposal-store root: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o222 == 0 {
		return errors.New("carina review record: proposal-store root is not a writable directory")
	}
	return nil
}

func (s *FileRecordStore) CreateIfAbsent(ctx context.Context, in ReviewRecord) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateInput(in); err != nil {
		return "", err
	}
	receipt := receiptFor(in.Repository, in.Proposal, in.IdempotencyID)
	path, err := s.pathForReceipt(receipt)
	if err != nil {
		return "", err
	}
	if existing, readErr := s.readPath(path); readErr == nil {
		if sameInput(existing, in) {
			return receipt, nil
		}
		return "", errors.New("carina review record: conflicting create replay")
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", readErr
	}

	in.SchemaVersion = recordSchema
	in.CreatedAt = s.now().UTC()
	in.RecordDigest = recordDigest(in)
	encoded, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("carina review record: create proposal directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".review-record-*")
	if err != nil {
		return "", fmt.Errorf("carina review record: create temporary record: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(encoded)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return "", fmt.Errorf("carina review record: write temporary record: %w", err)
	}
	if closeErr != nil {
		return "", fmt.Errorf("carina review record: close temporary record: %w", closeErr)
	}
	if err := os.Link(tmpPath, path); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("carina review record: atomically create record: %w", err)
		}
		existing, readErr := s.readPath(path)
		if readErr != nil || !sameInput(existing, in) {
			return "", errors.New("carina review record: conflicting concurrent create")
		}
	}
	return receipt, nil
}

func (s *FileRecordStore) ExactRead(ctx context.Context, receipt string) (ReviewRecord, error) {
	if err := ctx.Err(); err != nil {
		return ReviewRecord{}, err
	}
	path, err := s.pathForReceipt(receipt)
	if err != nil {
		return ReviewRecord{}, invalidRecord(err)
	}
	record, err := s.readPath(path)
	if err != nil {
		return ReviewRecord{}, err
	}
	wantReceipt := receiptFor(record.Repository, record.Proposal, record.IdempotencyID)
	if receipt != wantReceipt {
		return ReviewRecord{}, invalidRecord(errors.New("carina review record: receipt does not match stored identity"))
	}
	return record, nil
}

func (s *FileRecordStore) readPath(path string) (ReviewRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewRecord{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var record ReviewRecord
	if err := dec.Decode(&record); err != nil {
		return ReviewRecord{}, invalidRecord(fmt.Errorf("carina review record: decode: %w", err))
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ReviewRecord{}, invalidRecord(errors.New("carina review record: trailing data"))
	}
	if err := validateStored(record); err != nil {
		return ReviewRecord{}, invalidRecord(err)
	}
	return record, nil
}

func (s *FileRecordStore) pathForReceipt(receipt string) (string, error) {
	repository, proposal, idempotencyID, err := parseReceipt(receipt)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.root, "impl-review", repository, strconv.Itoa(proposal), idempotencyID+".json")
	clean := filepath.Clean(path)
	rel, err := filepath.Rel(s.root, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("carina review record: receipt escapes proposal-store root")
	}
	return clean, nil
}

func validateInput(in ReviewRecord) error {
	if in.Repository == "" || in.Proposal <= 0 || in.IdempotencyID == "" || in.PublicationAttemptID == "" || in.ReviewObligationID == "" || in.ReviewProcessID == "" || in.ProposalID == "" || in.ReviewedRevision == "" || in.ArtifactDigest == "" || in.ArtifactBody == "" {
		return errors.New("carina review record: incomplete record")
	}
	if in.SchemaVersion != "" && in.SchemaVersion != recordSchema {
		return errors.New("carina review record: unknown schema")
	}
	return nil
}

func validateStored(record ReviewRecord) error {
	if err := validateInput(record); err != nil || record.SchemaVersion != recordSchema || record.CreatedAt.IsZero() || record.RecordDigest == "" {
		return errors.New("carina review record: invalid stored record")
	}
	if record.RecordDigest != recordDigest(record) {
		return errors.New("carina review record: record digest mismatch")
	}
	return nil
}

func sameInput(existing, candidate ReviewRecord) bool {
	existing.SchemaVersion, existing.CreatedAt, existing.RecordDigest = "", time.Time{}, ""
	candidate.SchemaVersion, candidate.CreatedAt, candidate.RecordDigest = "", time.Time{}, ""
	return existing == candidate
}

func recordDigest(record ReviewRecord) string {
	copy := record
	copy.RecordDigest = ""
	b, _ := json.Marshal(copy)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func receiptFor(repository string, proposal int, idempotencyID string) string {
	repository = base64.RawURLEncoding.EncodeToString([]byte(repository))
	return fmt.Sprintf("carina-review-record:%s:%d:%s", repository, proposal, idempotencyID)
}

func parseReceipt(receipt string) (string, int, string, error) {
	parts := strings.Split(receipt, ":")
	if len(parts) != 4 || parts[0] != "carina-review-record" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return "", 0, "", errors.New("carina review record: invalid receipt grammar")
	}
	repositoryBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(repositoryBytes) == 0 {
		return "", 0, "", errors.New("carina review record: invalid repository identity")
	}
	proposal, err := strconv.Atoi(parts[2])
	if err != nil || proposal <= 0 {
		return "", 0, "", errors.New("carina review record: invalid proposal identity")
	}
	return parts[1], proposal, parts[3], nil
}
