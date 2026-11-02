// Package cron renders and reconciles repository-owned maintenance entries.
// It performs no host inspection or scheduler writes.
package cron

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

const markerPrefix = "# pyxis-maintenance/"

var (
	ErrInvalidInput       = errors.New("invalid cron rendering input")
	ErrUnstableExecutable = errors.New("unstable maintenance executable binding")
	ErrMalformedOwnership = errors.New("malformed maintenance cron ownership")
	ErrDuplicateOwnership = errors.New("duplicate maintenance cron ownership")
	markerPattern         = regexp.MustCompile(`^# pyxis-maintenance/v1 repo=([0-9a-f]{64}) job=([a-z][a-z0-9-]{0,63})\n$`)
	dailyPrefixPattern    = regexp.MustCompile(`^((?:[0-9]|[1-5][0-9]) (?:[0-9]|1[0-9]|2[0-3]) \* \* \*) PATH=`)
)

// InstalledBinding is an installer-provided host binding. StableExecutable is
// an explicit attestation: the later installer sets it only after rejecting
// transient build-cache and repository-local executables.
type InstalledBinding struct {
	Executable       string
	ToolPath         string
	StableExecutable bool
}

// CronEntry is one complete owned marker/entry pair.
type CronEntry struct {
	RepositoryHash string
	JobID          string
	Text           []byte
}

// OwnedEntry identifies a validated repository-owned entry and its byte
// position in a complete user table.
type OwnedEntry struct {
	RepositoryHash string
	JobID          string
	Cron           string
	Offset         int
}

// Result contains bytes suitable for a scheduler write only when Changed is
// true. An error always returns the zero value, with no writable replacement.
type Result struct {
	Changed     bool
	Replacement []byte
}

// Render builds one direct core-CLI invocation for every enabled validated job.
func Render(canonicalRoot string, binding InstalledBinding, configured []jobs.Job) ([]CronEntry, error) {
	repositoryHash, err := RepositoryHash(canonicalRoot)
	if err != nil {
		return nil, err
	}
	if !binding.StableExecutable {
		return nil, ErrUnstableExecutable
	}
	if err := validatePath("installed executable", binding.Executable); err != nil {
		return nil, err
	}
	if err := validateValue("tool search path", binding.ToolPath); err != nil || binding.ToolPath == "" {
		return nil, fmt.Errorf("%w: tool search path is required", ErrInvalidInput)
	}
	entries := make([]CronEntry, 0, len(configured))
	seen := make(map[string]bool, len(configured))
	for _, job := range configured {
		if !jobs.ValidID(job.ID) || seen[job.ID] {
			return nil, fmt.Errorf("%w: invalid or duplicate job id %q", ErrInvalidInput, job.ID)
		}
		seen[job.ID] = true
		if !job.Enabled {
			continue
		}
		if !dailyPrefixPattern.MatchString(job.Schedule.Cron + " PATH=") {
			return nil, fmt.Errorf("%w: job %q has invalid daily schedule", ErrInvalidInput, job.ID)
		}
		logPath := filepath.Join(canonicalRoot, ".pyxis", "logs", "maintenance", "cron.log")
		command := job.Schedule.Cron + " PATH=" + shellQuote(binding.ToolPath) + " " + shellQuote(binding.Executable) +
			" maintenance run --repo " + shellQuote(canonicalRoot) + " --job " + shellQuote(job.ID) +
			" >> " + shellQuote(logPath) + " 2>&1"
		text := fmt.Sprintf("# pyxis-maintenance/v1 repo=%s job=%s\n%s\n", repositoryHash, job.ID, escapeCronPercents(command))
		entries = append(entries, CronEntry{RepositoryHash: repositoryHash, JobID: job.ID, Text: []byte(text)})
	}
	return entries, nil
}

// RepositoryHash returns the ownership identity for a caller-attested
// canonical root, including when an empty desired list yields no entries.
func RepositoryHash(canonicalRoot string) (string, error) {
	if err := validateValue("canonical repository root", canonicalRoot); err != nil || !filepath.IsAbs(canonicalRoot) || filepath.Clean(canonicalRoot) != canonicalRoot {
		return "", fmt.Errorf("%w: canonical repository root must be clean and absolute", ErrInvalidInput)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalRoot))), nil
}

// Reconcile replaces or removes only selectedRepositoryHash pairs and appends
// newly desired pairs in their supplied order. All other bytes retain their
// original values and ordering.
func Reconcile(current []byte, selectedRepositoryHash string, desired []CronEntry) (Result, error) {
	if !validHash(selectedRepositoryHash) {
		return Result{}, fmt.Errorf("%w: invalid selected repository hash", ErrInvalidInput)
	}
	records, err := parseRecords(current)
	if err != nil {
		return Result{}, err
	}
	wanted := make(map[string]CronEntry, len(desired))
	for _, entry := range desired {
		if entry.RepositoryHash != selectedRepositoryHash {
			return Result{}, fmt.Errorf("%w: desired entry belongs to another repository", ErrInvalidInput)
		}
		if wanted[entry.JobID].Text != nil {
			return Result{}, fmt.Errorf("%w: desired repository/job pair", ErrDuplicateOwnership)
		}
		parsed, parseErr := parseRecords(entry.Text)
		if parseErr != nil || len(parsed) != 1 || parsed[0].repositoryHash != entry.RepositoryHash || parsed[0].jobID != entry.JobID || parsed[0].start != 0 || parsed[0].end != len(entry.Text) {
			return Result{}, fmt.Errorf("%w: desired repository/job pair", ErrMalformedOwnership)
		}
		wanted[entry.JobID] = entry
	}
	var output bytes.Buffer
	cursor := 0
	used := make(map[string]bool, len(wanted))
	for _, record := range records {
		output.Write(current[cursor:record.start])
		if record.repositoryHash == selectedRepositoryHash {
			if entry, ok := wanted[record.jobID]; ok {
				output.Write(entry.Text)
				used[record.jobID] = true
			}
		} else {
			output.Write(current[record.start:record.end])
		}
		cursor = record.end
	}
	output.Write(current[cursor:])
	for _, entry := range desired {
		if used[entry.JobID] {
			continue
		}
		if output.Len() > 0 && output.Bytes()[output.Len()-1] != '\n' {
			output.WriteByte('\n')
		}
		output.Write(entry.Text)
	}
	if bytes.Equal(current, output.Bytes()) {
		return Result{}, nil
	}
	return Result{Changed: true, Replacement: bytes.Clone(output.Bytes())}, nil
}

// OwnedEntries exposes the renderer's ownership grammar without exposing its
// parser or duplicating it in installation admission.
func OwnedEntries(table []byte) ([]OwnedEntry, error) {
	records, err := parseRecords(table)
	if err != nil {
		return nil, err
	}
	entries := make([]OwnedEntry, len(records))
	for index, record := range records {
		// parseRecords has validated this entry with the same daily prefix.
		start := record.start + bytes.IndexByte(table[record.start:record.end], '\n') + 1
		schedule := dailyPrefixPattern.FindSubmatch(table[start:record.end])
		entries[index] = OwnedEntry{RepositoryHash: record.repositoryHash, JobID: record.jobID, Cron: string(schedule[1]), Offset: record.start}
	}
	return entries, nil
}

type ownedRecord struct {
	start, end     int
	repositoryHash string
	jobID          string
}

func parseRecords(table []byte) ([]ownedRecord, error) {
	var records []ownedRecord
	seen := map[string]bool{}
	for offset := 0; offset < len(table); {
		lineEnd := bytes.IndexByte(table[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(table)
		} else {
			lineEnd += offset + 1
		}
		line := table[offset:lineEnd]
		if !bytes.HasPrefix(line, []byte(markerPrefix)) {
			offset = lineEnd
			continue
		}
		match := markerPattern.FindSubmatch(line)
		if match == nil || lineEnd == len(table) {
			return nil, fmt.Errorf("%w: marker at byte %d", ErrMalformedOwnership, offset)
		}
		entryEnd := bytes.IndexByte(table[lineEnd:], '\n')
		if entryEnd < 0 {
			return nil, fmt.Errorf("%w: incomplete pair at byte %d", ErrMalformedOwnership, offset)
		}
		entryEnd += lineEnd + 1
		repositoryHash, jobID := string(match[1]), string(match[2])
		if err := validateCommand(table[lineEnd:entryEnd], repositoryHash, jobID); err != nil {
			return nil, fmt.Errorf("%w: pair at byte %d: %v", ErrMalformedOwnership, offset, err)
		}
		key := repositoryHash + "/" + jobID
		if seen[key] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateOwnership, key)
		}
		seen[key] = true
		records = append(records, ownedRecord{start: offset, end: entryEnd, repositoryHash: repositoryHash, jobID: jobID})
		offset = entryEnd
	}
	return records, nil
}

func validateCommand(encoded []byte, repositoryHash, jobID string) error {
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
		return errors.New("entry is not newline terminated")
	}
	command, err := unescapeCronPercents(string(encoded[:len(encoded)-1]))
	if err != nil {
		return err
	}
	match := dailyPrefixPattern.FindStringSubmatch(command)
	if match == nil {
		return errors.New("entry has invalid schedule or PATH assignment")
	}
	rest := command[len(match[0]):]
	toolPath, rest, ok := takeQuoted(rest, " ")
	if !ok {
		return errors.New("entry has invalid tool path")
	}
	executable, rest, ok := takeQuoted(rest, " maintenance run --repo ")
	if !ok {
		return errors.New("entry has invalid executable")
	}
	root, rest, ok := takeQuoted(rest, " --job ")
	if !ok {
		return errors.New("entry has invalid repository root")
	}
	parsedJob, rest, ok := takeQuoted(rest, " >> ")
	if !ok {
		return errors.New("entry has invalid job id")
	}
	logPath, rest, ok := takeQuoted(rest, " 2>&1")
	if !ok || rest != "" || parsedJob != jobID || !jobs.ValidID(parsedJob) {
		return errors.New("entry does not match its owned job")
	}
	if validatePath("installed executable", executable) != nil || validateValue("tool search path", toolPath) != nil || toolPath == "" || validateValue("repository root", root) != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("entry contains an invalid binding")
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(root))) != repositoryHash || logPath != filepath.Join(root, ".pyxis", "logs", "maintenance", "cron.log") {
		return errors.New("entry does not match its owned repository")
	}
	return nil
}

func shellQuote(value string) string {
	var output strings.Builder
	output.WriteByte('\'')
	for index := range len(value) {
		switch value[index] {
		case '\'':
			output.WriteString(`'\''`)
		case '\\':
			// Adjacent quoted segments keep literal slashes apart in cron's
			// command text while remaining one shell word.
			output.WriteString(`''\''`)
		default:
			output.WriteByte(value[index])
		}
	}
	output.WriteByte('\'')
	return output.String()
}

func takeQuoted(input, suffix string) (string, string, bool) {
	var value strings.Builder
	index := 0
	for index < len(input) && input[index] == '\'' {
		index++
		for index < len(input) && input[index] != '\'' {
			value.WriteByte(input[index])
			index++
		}
		if index == len(input) {
			return "", input, false
		}
		index++
		if strings.HasPrefix(input[index:], `\'`) {
			value.WriteByte('\'')
			index += 2
		}
	}
	if index == 0 || !strings.HasPrefix(input[index:], suffix) {
		return "", input, false
	}
	decoded := value.String()
	if input[:index] != shellQuote(decoded) {
		return "", input, false
	}
	return decoded, input[index+len(suffix):], true
}

func escapeCronPercents(command string) string { return strings.ReplaceAll(command, "%", `\%`) }

func unescapeCronPercents(command string) (string, error) {
	var output strings.Builder
	for index := 0; index < len(command); index++ {
		if command[index] == '%' {
			return "", errors.New("entry contains an unescaped cron percent")
		}
		if command[index] == '\\' && index+1 < len(command) && command[index+1] == '%' {
			output.WriteByte('%')
			index++
			continue
		}
		output.WriteByte(command[index])
	}
	return output.String(), nil
}

func validatePath(name, value string) error {
	if err := validateValue(name, value); err != nil || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return fmt.Errorf("%w: %s must be clean and absolute", ErrInvalidInput, name)
	}
	return nil
}

func validateValue(name, value string) error {
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%w: %s contains newline or NUL", ErrInvalidInput, name)
	}
	return nil
}

func validHash(value string) bool {
	return len(value) == 64 && strings.IndexFunc(value, func(character rune) bool {
		return character < '0' || character > '9' && character < 'a' || character > 'f'
	}) < 0
}
