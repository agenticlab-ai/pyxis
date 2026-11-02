package audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

const journalPath = ".pyxis/maintenance/logs.jsonl"

var authenticatedURL = regexp.MustCompile(`(?i)([a-z][a-z0-9+.\-]*:(?:/|\\/){2})[^/?#\s]+@`)
var apiCredential = regexp.MustCompile(`(?:(?:sk-|gsk_|xai-|pplx-|hf_|r8_|nvapi-|key_)[A-Za-z0-9_-]{20,}|ASIA[0-9A-Z]{16})`)

// Secret fields and private-key starts redact the remainder, including incomplete input.
const credentialField = `(?:[a-z0-9_.-]*(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|private[_-]?key|credential|authorization|cookie|session)[a-z0-9_.-]*|(?:[a-z0-9_.-]+[_.-])?(?:sig|signature)|[a-z0-9_.-]*(?:%[0-9a-f]{2}[a-z0-9_.-]*)+)`

var credentialTail = regexp.MustCompile(`(?is)(?:["']?` + credentialField + `\\*["']?\s*[:=]|--` + credentialField + `\s+|-----BEGIN[^-]*PRIVATE KEY[^-]*-----).*`)
var escapedASCII = regexp.MustCompile(`\\u00[0-7][0-9a-fA-F]`)

// Prefer complete known formats; compact JOSE is structural, independent of encoded JSON whitespace.
var rawCredential = regexp.MustCompile(`(?:ya29\.[A-Za-z0-9._-]{20,}|(?i:basic)\s+[A-Za-z0-9+/]+={0,2}|[A-Za-z0-9_-]{2,}\.(?:[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.)?[A-Za-z0-9_-]*\.[A-Za-z0-9_-]{20,})`)
var hexadecimal = regexp.MustCompile(`^[0-9A-Fa-f]+$`)
var redactor = drivetogreen.NewRedactor(64*1024, []*regexp.Regexp{authenticatedURL, apiCredential, credentialTail, rawCredential})
var levels = map[Level]bool{LevelInfo: true, LevelWarn: true, LevelError: true}
var events = map[EventName]bool{EventInstallation: true, EventClaim: true, EventRun: true, EventLifecycle: true, EventTail: true}
var phases = map[Phase]bool{PhaseAdmission: true, PhaseInstallation: true, PhaseConfiguration: true, PhaseRouting: true, PhaseClaim: true, PhaseWorkspace: true, PhaseAgent: true, PhaseVerification: true, PhasePublication: true, PhaseTerminal: true, PhaseCleanup: true, PhaseAudit: true}
var outcomes = map[Outcome]bool{OutcomeSucceeded: true, OutcomeNoChange: true, OutcomeFailed: true, OutcomeTimedOut: true, OutcomeIncomplete: true, OutcomeSkipped: true, OutcomeRefused: true, OutcomeWon: true, OutcomeLost: true, OutcomeUnproved: true}
var logLevels = map[Level]slog.Level{LevelWarn: slog.LevelWarn, LevelError: slog.LevelError}

type fileHooks struct {
	open     func(*os.Root, string, int, os.FileMode) (*os.File, error)
	write    func(*os.File, []byte) (int, error)
	sync     func(*os.File) error
	syncRoot func(*os.Root) error
}

var diskHooks = fileHooks{(*os.Root).OpenFile, (*os.File).Write, (*os.File).Sync, syncDirectory}

type Writer struct {
	root, hostID, repositoryID string
	logger                     *slog.Logger
	hooks                      fileHooks
	now                        func() time.Time
}

type UncertainError struct {
	Written int
	Cause   error
}

// Inspection is a read-only, best-effort view of the local journal. Valid
// records remain available when another complete record is malformed.
type Inspection struct {
	Events         []Event
	Missing        bool
	Malformed      int
	IncompleteTail bool
}

// Inspect reads the journal without opening audit state, locks, or identities.
func Inspect(ctx context.Context, root string) (Inspection, error) {
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(journalPath)), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return Inspection{Missing: true}, nil
	}
	if err != nil {
		return Inspection{}, fmt.Errorf("read maintenance journal: %w", err)
	}
	defer func() { _ = file.Close() }() // Read errors are returned by the loop.
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return Inspection{}, errors.Join(errors.New("maintenance journal must be a regular file"), err)
	}
	return inspectReader(ctx, root, file)
}

func inspectReader(ctx context.Context, root string, input io.Reader) (Inspection, error) {
	result := Inspection{}
	repositoryID := fmt.Sprintf("%x", sha256.Sum256([]byte(root)))
	reader := bufio.NewReaderSize(input, MaxRecordBytes)
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		line, readErr := reader.ReadSlice('\n')
		if errors.Is(readErr, bufio.ErrBufferFull) {
			result.Malformed++
			for errors.Is(readErr, bufio.ErrBufferFull) {
				_, readErr = reader.ReadSlice('\n')
			}
			result.IncompleteTail = errors.Is(readErr, io.EOF)
		} else if len(line) > 0 {
			if errors.Is(readErr, io.EOF) {
				result.IncompleteTail = true
			} else if event, decodeErr := Decode(line); decodeErr != nil || event.RepositoryID != repositoryID {
				result.Malformed++
			} else {
				result.Events = append(result.Events, event)
			}
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			return result, nil
		}
		return result, fmt.Errorf("read maintenance journal: %w", readErr)
	}
}

// Decode accepts one complete closed journal record.
func Decode(data []byte) (Event, error) {
	if len(data) == 0 || len(data) > MaxRecordBytes || data[len(data)-1] != '\n' {
		return Event{}, errors.New("maintenance audit record is incomplete or exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var event Event
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("decode maintenance audit record: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Event{}, errors.New("decode maintenance audit record: trailing data")
	}
	encoded, err := encode(event)
	if err != nil || !bytes.Equal(encoded, data) {
		return Event{}, errors.Join(errors.New("invalid, noncanonical, or unsanitized maintenance audit record"), err)
	}
	return event, nil
}

func (e *UncertainError) Error() string {
	return fmt.Sprintf("maintenance audit append uncertain after %d bytes: %v", e.Written, e.Cause)
}
func (e *UncertainError) Unwrap() error { return e.Cause }

func NewIdentity() (Identity, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return Identity{}, fmt.Errorf("allocate maintenance identity: %w", err)
	}
	return Identity{TraceID: hex.EncodeToString(value[:16]), InvocationID: hex.EncodeToString(value[16:])}, nil
}

func Open(ctx context.Context, root string, logger *slog.Logger) (*Writer, error) {
	return open(ctx, root, logger, diskHooks)
}

func open(ctx context.Context, operand string, logger *slog.Logger, hooks fileHooks) (*Writer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(operand)
	if err != nil {
		return nil, fmt.Errorf("canonicalize maintenance repository: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, fmt.Errorf("resolve maintenance repository: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() || !ownedByProcess(info) {
		return nil, errors.New("maintenance repository must be an owned directory")
	}
	repo, err := gitsrc.Open(ctx, canonical)
	if err != nil {
		return nil, fmt.Errorf("open maintenance repository: %w", err)
	}
	repoRoot, err := filepath.EvalSymlinks(repo.Root)
	if err != nil || filepath.Clean(repoRoot) != filepath.Clean(canonical) {
		return nil, errors.New("maintenance root must name the canonical Git worktree root")
	}
	if err := repo.RequireIgnored(ctx, []string{journalPath}); err != nil {
		return nil, fmt.Errorf("admit maintenance journal (add /%s to .gitignore; keep it untracked): %w", journalPath, err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	w := &Writer{root: canonical, repositoryID: fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))), logger: logger, hooks: hooks, now: time.Now}
	if err := w.withLock(ctx, func(r *os.Root, _ *os.File) error { return w.prepare(ctx, repo, r) }); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Append(ctx context.Context, event Event, durability Durability) (Result, error) {
	if durability != Buffered && durability != Consequential {
		return Result{}, errors.New("invalid maintenance audit durability")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var result Result
	err := w.withLock(ctx, func(root *os.Root, lock *os.File) error {
		if err := w.rejectSymlinks(); err != nil {
			return err
		}
		file, _, err := w.openOwned(root, journalPath, os.O_APPEND)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }() // Write and Sync report append failures; Close releases the descriptor.
		if err := w.hooks.sync(file); err != nil {
			return fmt.Errorf("admit maintenance journal durability: %w", err)
		}
		event.SchemaVersion, event.Timestamp, event.HostID, event.RepositoryID = SchemaVersion, w.now().UTC(), w.hostID, w.repositoryID
		data, err := encode(event)
		if err != nil {
			return err
		}
		incomplete, size, err := tailState(file)
		if err != nil {
			return fmt.Errorf("inspect maintenance journal tail: %w", err)
		}
		// The lock is empty when clean; byte '1' is the v1 pending-tail marker.
		var pending [2]byte
		pendingSize, stateErr := lock.ReadAt(pending[:], 0)
		if pendingSize > 1 || pendingSize == 1 && pending[0] != '1' || stateErr != nil && !errors.Is(stateErr, io.EOF) {
			return fmt.Errorf("read maintenance tail marker: %w", errors.Join(errors.New("expected empty or v1 pending flag"), stateErr))
		}
		if incomplete || pendingSize == 1 {
			result.HistoricalTailIncomplete = true
			tail := Event{Level: LevelWarn, Event: EventTail, Phase: PhaseAudit, TraceID: event.TraceID, InvocationID: event.InvocationID, Outcome: OutcomeIncomplete, CauseCode: "historical_tail_incomplete", HistoricalTailBytes: &size}
			tail.SchemaVersion, tail.Timestamp, tail.HostID, tail.RepositoryID = SchemaVersion, event.Timestamp, w.hostID, w.repositoryID
			diagnostic, encodeErr := encode(tail)
			if encodeErr != nil {
				return encodeErr
			}
			_, stateErr = lock.WriteAt([]byte{'1'}, 0)
			if err := errors.Join(stateErr, w.hooks.sync(lock)); err != nil {
				return fmt.Errorf("persist maintenance tail marker: %w", err)
			}
			if incomplete {
				diagnostic = append([]byte{'\n'}, diagnostic...)
			}
			n, writeErr := w.writeFull(file, diagnostic)
			if writeErr != nil {
				return &UncertainError{Written: n, Cause: writeErr}
			}
			if err := w.hooks.sync(file); err != nil {
				return &UncertainError{Written: n, Cause: fmt.Errorf("sync maintenance tail diagnostic: %w", err)}
			}
			if err := errors.Join(lock.Truncate(0), w.hooks.sync(lock)); err != nil {
				return &UncertainError{Written: n, Cause: fmt.Errorf("clear maintenance tail marker: %w", err)}
			}
		}
		n, err := w.writeFull(file, data)
		if err != nil {
			return &UncertainError{Written: n, Cause: err}
		}
		if durability == Consequential || incomplete {
			if err := w.hooks.sync(file); err != nil {
				return &UncertainError{Written: n, Cause: fmt.Errorf("sync maintenance journal: %w", err)}
			}
		}
		return nil
	})
	if err == nil {
		w.logger.LogAttrs(ctx, logLevels[event.Level], string(event.Event), slog.String("trace_id", event.TraceID), slog.String("invocation_id", event.InvocationID), slog.String("phase", string(event.Phase)))
	}
	return result, err
}

func (w *Writer) prepare(ctx context.Context, repo *gitsrc.Repo, root *os.Root) error {
	if err := w.rejectSymlinks(); err != nil {
		return err
	}
	if err := repo.RequireIgnored(ctx, []string{journalPath}); err != nil {
		return fmt.Errorf("recheck maintenance journal admission (add /%s to .gitignore; keep it untracked): %w", journalPath, err)
	}
	if err := root.MkdirAll(".pyxis/maintenance", 0o700); err != nil {
		return fmt.Errorf("create maintenance journal directory: %w", err)
	}
	file, _, err := w.openOwned(root, journalPath, os.O_APPEND|os.O_CREATE)
	if err != nil {
		return err
	}
	if err := w.hooks.sync(file); err != nil {
		_ = file.Close() // Preserve the durability failure that refused admission.
		return fmt.Errorf("sync maintenance journal admission: %w", err)
	}
	if err := w.syncPath(root, ".pyxis/maintenance"); err != nil {
		_ = file.Close() // Preserve the directory-sync failure that refused admission.
		return fmt.Errorf("sync maintenance journal directory: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close maintenance journal admission: %w", err)
	}
	w.hostID, err = w.loadHostID(root)
	return err
}

func (w *Writer) openOwned(root *os.Root, name string, flags int) (file *os.File, created bool, err error) {
	flags |= os.O_RDWR | syscall.O_NOFOLLOW
	if flags&os.O_CREATE != 0 {
		file, err = w.hooks.open(root, name, flags|os.O_EXCL, 0o600)
		created = err == nil
		if errors.Is(err, os.ErrExist) {
			file, err = w.hooks.open(root, name, flags&^os.O_CREATE, 0)
		}
	} else {
		file, err = w.hooks.open(root, name, flags, 0)
	}
	if err != nil {
		return nil, false, fmt.Errorf("open maintenance file %s: %w", name, err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !ownedByProcess(info) {
		_ = file.Close() // Close the rejected descriptor; admission remains refused.
		return nil, false, fmt.Errorf("maintenance file %s must be an owned 0600 regular file", name)
	}
	return file, created, nil
}

func (w *Writer) loadHostID(root *os.Root) (string, error) {
	const name = ".pyxis/state/maintenance/host-id"
	file, created, err := w.openOwned(root, name, os.O_CREATE)
	if err != nil {
		return "", fmt.Errorf("open maintenance host identity: %w", err)
	}
	defer func() { _ = file.Close() }() // Host identity writes are synced before acknowledgement.
	if created {
		identity, randomErr := NewIdentity()
		if randomErr != nil {
			return "", fmt.Errorf("allocate maintenance host identity: %w", randomErr)
		}
		if n, writeErr := w.writeFull(file, []byte(identity.InvocationID+"\n")); writeErr != nil {
			return "", &UncertainError{Written: n, Cause: writeErr}
		}
		if syncErr := w.hooks.sync(file); syncErr != nil {
			return "", fmt.Errorf("sync maintenance host identity: %w", syncErr)
		}
		if syncErr := w.syncPath(root, ".pyxis/state/maintenance"); syncErr != nil {
			return "", fmt.Errorf("sync maintenance host identity directory: %w", syncErr)
		}
		return identity.InvocationID, nil
	}
	info, err := file.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > 128 {
		return "", errors.New("maintenance host identity is unreadable or invalid")
	}
	data := make([]byte, info.Size())
	if _, err := file.ReadAt(data, 0); err != nil || !validHexID(strings.TrimSpace(string(data)), 32) {
		return "", errors.New("maintenance host identity is unreadable or invalid")
	}
	return strings.TrimSpace(string(data)), nil
}

func (w *Writer) withLock(ctx context.Context, run func(*os.Root, *os.File) error) error {
	root, err := os.OpenRoot(w.root)
	if err != nil {
		return fmt.Errorf("open maintenance repository root: %w", err)
	}
	defer func() { _ = root.Close() }() // This root descriptor owns no pending writes.
	if err := w.rejectSymlinks(); err != nil {
		return err
	}
	if err := root.MkdirAll(".pyxis/state/maintenance", 0o700); err != nil {
		return fmt.Errorf("create maintenance audit lock directory: %w", err)
	}
	file, created, err := w.openOwned(root, ".pyxis/state/maintenance/audit.lock", os.O_CREATE)
	if err != nil {
		return fmt.Errorf("open maintenance audit lock: %w", err)
	}
	defer func() { _ = file.Close() }() // Lock state is synced explicitly; Close also releases the flock.
	for {
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return fmt.Errorf("lock maintenance audit: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }() // Closing the descriptor also releases the lock.
	if err := ctx.Err(); err != nil {
		return err
	}
	if created {
		if err := w.syncPath(root, ".pyxis/state/maintenance"); err != nil {
			return fmt.Errorf("sync maintenance audit lock directory: %w", err)
		}
	}
	return run(root, file)
}

func (w *Writer) syncPath(root *os.Root, name string) error {
	directory, err := root.OpenRoot(name)
	if err != nil {
		return fmt.Errorf("open maintenance directory %s: %w", name, err)
	}
	defer func() { _ = directory.Close() }() // No directory write remains after Sync.
	return w.hooks.syncRoot(directory)
}

func (w *Writer) rejectSymlinks() error {
	for _, relative := range []string{".pyxis", ".pyxis/maintenance", journalPath, ".pyxis/state", ".pyxis/state/maintenance", ".pyxis/state/maintenance/audit.lock", ".pyxis/state/maintenance/host-id"} {
		info, err := os.Lstat(filepath.Join(w.root, filepath.FromSlash(relative)))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("maintenance audit path is unreadable or symlinked: %s", relative)
		}
	}
	return nil
}

func (w *Writer) writeFull(file *os.File, data []byte) (int, error) {
	written := 0
	for written < len(data) {
		n, err := w.hooks.write(file, data[written:])
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, errors.New("zero-byte maintenance journal write")
		}
	}
	return written, nil
}

func encode(event Event) ([]byte, error) {
	event = sanitize(event)
	if err := validate(event); err != nil {
		return nil, err
	}
	for {
		data, err := json.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("encode maintenance audit event: %w", err)
		}
		if len(data)+1 <= MaxRecordBytes {
			return append(data, '\n'), nil
		}
		if len(event.References) > 0 {
			event.References = event.References[:len(event.References)-1]
			event.ReferencesTruncated = true
			continue
		}
		if len(event.Summary) > 0 {
			event.Summary, event.SummaryTruncated = truncateUTF8(event.Summary, len(event.Summary)/2), true
			continue
		}
		if len(event.Cause) > 0 {
			event.Cause, event.CauseTruncated = truncateUTF8(event.Cause, len(event.Cause)/2), true
			continue
		}
		return nil, errors.New("maintenance audit identity fields exceed 32 KiB")
	}
}

func sanitize(event Event) Event {
	event.Cause, event.CauseTruncated = bounded(event.Cause, MaxSummaryBytes, event.CauseTruncated)
	event.Summary, event.SummaryTruncated = bounded(event.Summary, MaxSummaryBytes, event.SummaryTruncated)
	if event.Delivery != nil {
		event.Delivery = &Delivery{Branch: event.Delivery.Branch, Head: event.Delivery.Head, PRURL: redact(event.Delivery.PRURL)}
	}
	event.ReferencesTruncated = event.ReferencesTruncated || len(event.References) > MaxReferences
	event.References = append([]Reference(nil), event.References[:min(len(event.References), MaxReferences)]...)
	for i, reference := range event.References {
		kind, kindTruncated := bounded(reference.Kind, 256, false)
		location, locationTruncated := bounded(reference.Location, 1024, false)
		event.References[i] = Reference{Kind: kind, Location: location, Digest: reference.Digest}
		event.ReferencesTruncated = event.ReferencesTruncated || kindTruncated || locationTruncated
	}
	return event
}

func identities(event Event) []string {
	values := []string{event.JobID, event.JobDate, event.Timezone, event.InstallationGeneration, event.ConfigCommit, event.CauseCode}
	for _, route := range []*Route{event.Route, event.Launch} {
		if route != nil {
			values = append(values, route.Harness, route.Profile, route.Model, route.Effort, route.Policy, route.Qualification)
		}
	}
	if event.Claim != nil {
		values = append(values, event.Claim.Owner, event.Claim.Ref, event.Claim.RecordOID)
	}
	if event.Delivery != nil {
		values = append(values, event.Delivery.Branch, event.Delivery.Head, event.Delivery.PRURL)
	}
	for _, reference := range event.References {
		values = append(values, reference.Digest)
	}
	return values
}

// Repair before bounding so JSON cannot expand invalid bytes past the limit.
func bounded(value string, limit int, truncated bool) (string, bool) {
	value, truncated = strings.ToValidUTF8(value, "\ufffd"), truncated || !utf8.ValidString(value)
	value = redact(value)
	return truncateUTF8(value, min(limit, len(value))), truncated || len(value) > limit
}

func redact(value string) string {
	value = escapedASCII.ReplaceAllStringFunc(value, func(escaped string) string {
		decoded, _ := strconv.Unquote(`"` + escaped + `"`) // The pattern admits only valid ASCII scalar escapes.
		return decoded
	})
	value = authenticatedURL.ReplaceAllString(value, "${1}")
	value = credentialTail.ReplaceAllString(value, "[REDACTED]")
	return redactor.Redact(rawCredential.ReplaceAllString(value, "[REDACTED]"))
}

func truncateUTF8(value string, limit int) string {
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func validate(event Event) error {
	if event.SchemaVersion != SchemaVersion || event.Timestamp.IsZero() || event.Timestamp.Location() != time.UTC || !validHexID(event.TraceID, 32) || !validHexID(event.InvocationID, 32) || !validHexID(event.HostID, 32) || !validHexID(event.RepositoryID, 64) {
		return errors.New("invalid maintenance audit schema, timestamp, or identity")
	}
	if !levels[event.Level] || !events[event.Event] || !phases[event.Phase] || event.Outcome != "" && !outcomes[event.Outcome] {
		return errors.New("invalid maintenance audit level, event, phase, or outcome")
	}
	for _, value := range identities(event) {
		if len(value) > MaxIdentityBytes || !utf8.ValidString(value) || redact(value) != value {
			return errors.New("maintenance audit identity exceeds its bound, is not UTF-8, or requires redaction")
		}
	}
	if event.TargetVerification != nil && event.TargetVerification.Status != TargetMeasured && event.TargetVerification.Status != TargetNotVerified {
		return errors.New("invalid maintenance target verification")
	}
	if event.Usage != nil && (event.Usage.Tokens != nil && *event.Usage.Tokens < 0 || event.Usage.Turns != nil && *event.Usage.Turns < 0 || event.Usage.ToolCalls != nil && *event.Usage.ToolCalls < 0) {
		return errors.New("invalid maintenance usage observation")
	}
	if event.CauseCode != "" {
		for _, character := range event.CauseCode {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '-' && character != '.' {
				return errors.New("invalid maintenance audit cause code")
			}
		}
	}
	return nil
}

func validHexID(value string, length int) bool {
	return len(value) == length && hexadecimal.MatchString(value)
}

func tailState(file *os.File) (bool, int64, error) {
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return false, 0, err
	}
	var last [1]byte
	if _, err := file.ReadAt(last[:], info.Size()-1); err != nil {
		return false, info.Size(), err
	}
	return last[0] != '\n', info.Size(), nil
}

func syncDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }() // Sync supplies the durability result; Close releases the descriptor.
	return directory.Sync()
}
