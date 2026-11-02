// Package sessionlog identifies the retired session-log import class. Raw
// session history is optional operational data, not a durable-memory obligation.
package sessionlog

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// Path reports whether p is a canonical repository path in the optional log
// directory. Similar names and traversals must not exempt authored contracts.
func Path(p string) bool {
	return path.Clean(p) == p && strings.HasPrefix(p, "logs/")
}

// Document identifies literal log imports, never Decisions, Learnings, or
// requirements that happen to cite a log as their evidence.
func Document(r *record.Record) bool {
	if r == nil || r.Kind != record.Document || !r.LiteralBody || !Path(r.Provenance.Source) {
		return false
	}
	return strings.HasPrefix(r.Subject, "session-log/") ||
		strings.HasPrefix(r.Subject, "source/logs-") || r.Subject == "history/session-logs"
}

// Empty recognizes only the mandatory no-escalation skeleton. Any other
// content, including an appended decision after that sentence, is retained.
var wrappedTask = regexp.MustCompile(`^(\(?[a-z][a-z0-9-]+/[0-9]{3}/T[0-9]+\)?|(?:follow-up|resolution) on plan task [a-z][a-z0-9-]+/[0-9]{3}/T[0-9]+)$`)
var metadataCorrection = regexp.MustCompile(`^Correction: (CLI: [^\r\n]+|this log was created at [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}; the initial timestamp was entered incorrectly\.)$`)

func Empty(body []byte) bool {
	var lines []string
	for _, line := range strings.Split(string(body), "\n") {
		if line = strings.TrimSpace(line); line != "" && !metadataCorrection.MatchString(line) {
			lines = append(lines, line)
		}
	}
	if len(lines) == 4 && wrappedTask.MatchString(lines[2]) {
		lines = append(lines[:2], lines[3])
	}
	return len(lines) == 3 && strings.HasPrefix(lines[0], "# Session log") &&
		strings.HasPrefix(lines[1], "Started:") && lines[2] == "Nothing escalated this session."
}

// retiredJSON pins the exact empty imports approved for the one-time cleanup.
// It is not a user-configurable deletion allowance. New or modified records,
// substantive histories, and references to these records remain protected.
//
//go:embed retired.json
var retiredJSON []byte

// Retired reports whether an immutable envelope and body are in the closed
// migration set. The packet additionally requires both files to disappear
// together, an unchanged base, and no incoming or outgoing relations.
func Retired(r *record.Record, envelope, body []byte) bool {
	if !Document(r) || !Empty(body) || len(r.Relations) != 0 || len(r.Embeds) != 0 {
		return false
	}
	var approved map[string]string
	if err := json.Unmarshal(retiredJSON, &approved); err != nil {
		return false // Corrupt embedded migration data never grants deletion.
	}
	sum := sha256.Sum256(envelope)
	return approved[r.Path()] == hex.EncodeToString(sum[:]) && record.HashBody(body) == r.BodySHA256
}
