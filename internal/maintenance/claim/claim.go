// Package claim owns daily maintenance-attempt ownership and terminal records.
package claim

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const SchemaVersion = "pyxis-maintenance-occurrence/v1"
const maxRecordBytes = 64 << 10

type Outcome string

const (
	OutcomeClaimed   Outcome = "claimed"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeTimedOut  Outcome = "timed_out"
)

type Record struct {
	SchemaVersion string                        `json:"schema_version"`
	JobID         string                        `json:"job_id"`
	JobDate       string                        `json:"job_date"`
	Timezone      string                        `json:"timezone"`
	OwnerNonce    string                        `json:"owner_nonce"`
	ConfigCommit  string                        `json:"config_commit"`
	Route         nativerouting.SelectedProfile `json:"route"`
	ClaimedAt     time.Time                     `json:"claimed_at"`
	Outcome       Outcome                       `json:"outcome"`
	FinishedAt    *time.Time                    `json:"finished_at,omitempty"`
	DurationMS    *int64                        `json:"duration_ms,omitempty"`
	ExitCode      *int                          `json:"exit_code,omitempty"`
	Summary       string                        `json:"summary,omitempty"`
	Cause         string                        `json:"cause,omitempty"`
	DraftPRURL    string                        `json:"draft_pr_url,omitempty"`
	CodeHead      string                        `json:"code_head,omitempty"`
	Evidence      []string                      `json:"evidence,omitempty"`
}

type Disposition string

const (
	DispositionOwned    Disposition = "owned"
	DispositionConsumed Disposition = "consumed"
)

type ClaimResult struct {
	Disposition Disposition
	Record      Record
}

type Terminal struct {
	Outcome    Outcome
	FinishedAt time.Time
	DurationMS *int64
	ExitCode   *int
	Summary    string
	Cause      string
	DraftPRURL string
	CodeHead   string
	Evidence   []string
}

// ClaimStore is the maintenance consumer's positively-proved shared store.
type ClaimStore interface {
	Claim(context.Context, Record) (ClaimResult, error)
	Transition(context.Context, Record, Terminal) (Record, error)
}

type Service struct {
	Store  ClaimStore
	Now    func() time.Time
	Random io.Reader
}

func (s Service) Acquire(ctx context.Context, prepared admission.Prepared) (result ClaimResult, err error) {
	if s.Store == nil {
		return result, errors.New("maintenance claim store is required")
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	random := io.Reader(rand.Reader)
	if s.Random != nil {
		random = s.Random
	}
	var nonce [32]byte
	if _, err := io.ReadFull(random, nonce[:]); err != nil {
		return result, fmt.Errorf("mint maintenance owner nonce: %w", err)
	}
	candidate := Record{SchemaVersion: SchemaVersion, JobID: prepared.DailyIdentity.JobID, JobDate: prepared.DailyIdentity.JobDate,
		Timezone: prepared.Job.Schedule.Timezone, OwnerNonce: hex.EncodeToString(nonce[:]), ConfigCommit: prepared.ConfigCommit,
		Route: prepared.Selection, ClaimedAt: now().UTC(), Outcome: OutcomeClaimed}
	if err := candidate.Validate(); err != nil {
		return result, fmt.Errorf("construct maintenance claim: %w", err)
	}
	err = prepared.GuardReady(ctx, func() error {
		var claimErr error
		result, claimErr = s.Store.Claim(ctx, candidate)
		return claimErr
	})
	if err != nil {
		return ClaimResult{}, fmt.Errorf("claim maintenance occurrence: %w", err)
	}
	if err := result.Record.Validate(); err != nil || result.Record.JobID != candidate.JobID || result.Record.JobDate != candidate.JobDate {
		return ClaimResult{}, errors.Join(errors.New("maintenance claim store returned invalid daily identity"), err)
	}
	if result.Disposition == DispositionOwned && !SameOwner(candidate, result.Record) {
		return ClaimResult{}, errors.New("maintenance claim store returned unproved ownership")
	}
	if result.Disposition != DispositionOwned && result.Disposition != DispositionConsumed {
		return ClaimResult{}, errors.New("maintenance claim store returned an invalid disposition")
	}
	return result, nil
}

func (s Service) Transition(ctx context.Context, owned Record, terminal Terminal) (Record, error) {
	if s.Store == nil {
		return Record{}, errors.New("maintenance claim store is required")
	}
	return s.Store.Transition(ctx, owned, terminal)
}

func (r Record) Validate() error {
	if r.SchemaVersion != SchemaVersion || !jobs.ValidID(r.JobID) || !validDate(r.JobDate) || r.Timezone == "" || r.Timezone == "Local" {
		return errors.New("maintenance occurrence has invalid schema or daily identity")
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil || !validHex(r.OwnerNonce, 64) || !validHex(r.ConfigCommit, 40) && !validHex(r.ConfigCommit, 64) {
		return errors.New("maintenance occurrence has invalid timezone, nonce, or config commit")
	}
	if !validRoute(r.Route) || r.ClaimedAt.IsZero() || r.ClaimedAt.Location() != time.UTC {
		return errors.New("maintenance occurrence has invalid route or claimed timestamp")
	}
	if r.Outcome != OutcomeClaimed && r.Outcome != OutcomeSucceeded && r.Outcome != OutcomeFailed && r.Outcome != OutcomeTimedOut {
		return errors.New("maintenance occurrence has unknown outcome")
	}
	if r.Outcome == OutcomeClaimed && (r.FinishedAt != nil || r.DurationMS != nil || r.ExitCode != nil || r.Summary != "" || r.Cause != "" || r.DraftPRURL != "" || r.CodeHead != "" || len(r.Evidence) != 0) {
		return errors.New("claimed maintenance occurrence contains terminal metadata")
	}
	if r.Outcome != OutcomeClaimed && (r.FinishedAt == nil || r.FinishedAt.IsZero() || r.FinishedAt.Location() != time.UTC || r.FinishedAt.Before(r.ClaimedAt)) {
		return errors.New("terminal maintenance occurrence has invalid finish timestamp")
	}
	if r.DurationMS != nil && (*r.DurationMS < 0 || *r.DurationMS > int64((7*24*time.Hour)/time.Millisecond)) {
		return errors.New("maintenance occurrence duration is invalid")
	}
	if !boundedText(r.Summary, 1024) || !boundedText(r.Cause, 2048) || len(r.Evidence) > 16 || !safeURL(r.DraftPRURL) || r.CodeHead != "" && !validHex(r.CodeHead, 40) && !validHex(r.CodeHead, 64) {
		return errors.New("maintenance occurrence terminal metadata is invalid")
	}
	for _, evidence := range r.Evidence {
		if !boundedText(evidence, 512) || !safeReference(evidence) {
			return errors.New("maintenance occurrence evidence reference is invalid")
		}
	}
	return nil
}

func Encode(record Record) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) >= maxRecordBytes {
		return nil, fmt.Errorf("encode maintenance occurrence: %w", errors.Join(err, errors.New("record exceeds bound")))
	}
	return append(data, '\n'), nil
}

func Decode(data []byte) (Record, error) {
	if len(data) == 0 || len(data) > maxRecordBytes {
		return Record{}, errors.New("maintenance occurrence is absent or exceeds bound")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return Record{}, fmt.Errorf("decode maintenance occurrence: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("decode maintenance occurrence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Record{}, errors.New("decode maintenance occurrence: trailing data")
	}
	return record, record.Validate()
}

func SameOwner(a, b Record) bool {
	return a.SchemaVersion == b.SchemaVersion && a.JobID == b.JobID && a.JobDate == b.JobDate && a.Timezone == b.Timezone &&
		a.OwnerNonce == b.OwnerNonce && a.ConfigCommit == b.ConfigCommit && a.ClaimedAt.Equal(b.ClaimedAt) && reflect.DeepEqual(a.Route, b.Route)
}

func WithTerminal(record Record, terminal Terminal) (Record, error) {
	if err := record.Validate(); err != nil || record.Outcome != OutcomeClaimed {
		return Record{}, errors.Join(errors.New("maintenance occurrence is not an active valid claim"), err)
	}
	record.Outcome, record.FinishedAt, record.DurationMS, record.ExitCode = terminal.Outcome, &terminal.FinishedAt, terminal.DurationMS, terminal.ExitCode
	record.Summary, record.Cause, record.DraftPRURL, record.CodeHead = terminal.Summary, terminal.Cause, terminal.DraftPRURL, terminal.CodeHead
	record.Evidence = append([]string(nil), terminal.Evidence...)
	return record, record.Validate()
}

func validDate(value string) bool {
	parsed, err := time.Parse(time.DateOnly, value)
	return err == nil && parsed.Format(time.DateOnly) == value
}

func validHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validRoute(route nativerouting.SelectedProfile) bool {
	return route.CatalogID != "" && route.PolicyIdentity != "" && route.Harness != "" && route.Role != "" && route.NativeProfileID != "" && route.NativeModel != "" && route.QualificationID != "" && route.QualificationDigest != "" && route.AdapterGeneration != ""
}

func boundedText(value string, bound int) bool {
	if len(value) > bound || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r") {
		return false
	}
	for word := range strings.FieldsFuncSeq(value, func(char rune) bool {
		return unicode.IsSpace(char) || strings.ContainsRune("\"'`()[]{}=,;<>“”‘’", char)
	}) {
		word = strings.Trim(word, ":")
		for !safeURL(word) {
			windowsPath := len(word) > 2 && word[1] == ':' && (word[2] == '/' || word[2] == '\\')
			if filepath.IsAbs(word) || strings.HasPrefix(word, "~/") || strings.HasPrefix(word, `\\`) || windowsPath {
				return false
			}
			_, suffix, found := strings.Cut(word, ":")
			if !found {
				break
			}
			word = suffix
		}
	}
	return true
}

func safeURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func safeReference(value string) bool {
	if safeURL(value) {
		return true
	}
	if value == "" || filepath.IsAbs(value) || strings.ContainsAny(value, `/\`) {
		return false
	}
	for _, char := range value {
		letter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		digit := char >= '0' && char <= '9'
		if !letter && !digit && !strings.ContainsRune("._:-", char) {
			return false
		}
	}
	return true
}

func uniqueJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate or invalid JSON object key")
			}
			seen[name] = true
		}
		if err := uniqueJSON(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
