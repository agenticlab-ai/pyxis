// Package rewriteauth validates the operator-authorized direct rewrites of
// frozen repository sources: the rewrite that removed the retired review
// runtime from the active source, and the repair that restored the document
// names it changed. Each authorization is deliberately one-use: it is bound to
// one mainline base commit and the digest of its complete changed path set.
package rewriteauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	MarkerPath      = ".pyxis/direct-rewrite/review-runtime-retirement.json"
	SchemaVersion   = "direct-rewrite-authorization/v1"
	AuthorizationID = "review-runtime-retirement-2026-09-22"
	AuthorizedBase  = "5cc3bd8db2fe5ad61ae6ba58da965f72f936c855"

	RepairMarkerPath      = ".pyxis/direct-rewrite/review-runtime-repair.json"
	RepairAuthorizationID = "review-runtime-repair-2026-09-23"
	RepairAuthorizedBase  = "565565afa4dbff2e6089483fe15f809ed7758842"
)

var authorizedDocgateVerbs = map[string]bool{
	"check-immutable":       true,
	"check-ledger-history":  true,
	"check-standalone-docs": true,
	"solution-shape":        true,
}

type policy struct {
	markerPath      string
	schemaVersion   string
	authorizationID string
	baseCommit      string
}

// oneUsePolicies are the fixed authorizations. Each binds a different mainline
// base, so at most one can match a comparison, and each becomes inert once the
// mainline moves past its base.
var oneUsePolicies = []policy{
	{markerPath: MarkerPath, schemaVersion: SchemaVersion, authorizationID: AuthorizationID, baseCommit: AuthorizedBase},
	{markerPath: RepairMarkerPath, schemaVersion: SchemaVersion, authorizationID: RepairAuthorizationID, baseCommit: RepairAuthorizedBase},
}

// Result reports whether the current comparison is an authorized direct
// rewrite and, when it is, whether its exact changed path set is intact.
// Marker names the active authorization's marker file.
type Result struct {
	Active     bool
	Authorized bool
	Marker     string
	Err        error
}

type marker struct {
	SchemaVersion   string `json:"schema_version"`
	AuthorizationID string `json:"authorization_id"`
	BaseCommit      string `json:"base_commit"`
	ChangeSetSHA256 string `json:"change_set_sha256"`
}

// Verify checks the fixed authorizations against mergeBase..<head>. An empty
// head compares the base with the index and working tree, matching docgate's
// local/CI behavior. Only the authorization bound to mergeBase can activate; a
// different base makes every authorization inert.
func Verify(root, mergeBase, head string) Result {
	return verifyAny(root, mergeBase, head, oneUsePolicies)
}

// verifyAny returns the result of the first policy bound to mergeBase.
func verifyAny(root, mergeBase, head string, policies []policy) Result {
	for _, p := range policies {
		if result := verify(root, mergeBase, head, p); result.Active {
			return result
		}
	}
	return Result{}
}

func verify(root, mergeBase, head string, p policy) Result {
	base, err := resolveCommit(root, mergeBase)
	if err != nil || base != p.baseCommit {
		return Result{}
	}
	result := Result{Active: true, Marker: p.markerPath}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.markerPath)))
	if err != nil {
		result.Err = fmt.Errorf("read authorization marker: %w", err)
		return result
	}
	var m marker
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		result.Err = fmt.Errorf("decode authorization marker: %w", err)
		return result
	}
	if err := requireJSONEOF(dec); err != nil {
		result.Err = err
		return result
	}
	if m.SchemaVersion != p.schemaVersion || m.AuthorizationID != p.authorizationID || m.BaseCommit != p.baseCommit {
		result.Err = fmt.Errorf("authorization marker identity does not match the one-use authorization")
		return result
	}
	if !validDigest(m.ChangeSetSHA256) {
		result.Err = fmt.Errorf("authorization marker has an invalid change_set_sha256")
		return result
	}
	digest, err := changeSetDigest(root, base, head, p.markerPath)
	if err != nil {
		result.Err = err
		return result
	}
	if m.ChangeSetSHA256 != digest {
		result.Err = fmt.Errorf("authorized change-set digest mismatch: got %s, want %s", digest, m.ChangeSetSHA256)
		return result
	}
	result.Authorized = true
	return result
}

// AllowsDocgate reports whether a verified authorization covers a defect verb.
func (r Result) AllowsDocgate(verb string) bool {
	return r.Authorized && authorizedDocgateVerbs[verb]
}

// AllowsMemoryGate reports whether a verified authorization covers the memory
// gate. This direct rewrite intentionally changes historical source anchors.
func (r Result) AllowsMemoryGate() bool { return r.Authorized }

// ChangeSetDigest returns the SHA-256 of a stable manifest of every changed
// path except the review-runtime retirement marker itself. Each row binds
// status, path, modes, and old/new blob bytes.
func ChangeSetDigest(root, base, head string) (string, error) {
	return changeSetDigest(root, base, head, MarkerPath)
}

// changeSetDigest excludes exactly one marker: the active authorization's own
// file, which cannot contain its own digest. Every other marker change stays
// bound to the change set.
func changeSetDigest(root, base, head, markerPath string) (string, error) {
	args := []string{"-C", root, "diff", "--no-renames", "--name-status", "-z", base}
	if head != "" {
		args = append(args, head)
	}
	args = append(args, "--")
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("list direct-rewrite changes: %w", err)
	}
	tokens := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	if len(tokens) == 1 && len(tokens[0]) == 0 {
		tokens = nil
	}
	if len(tokens)%2 != 0 {
		return "", fmt.Errorf("parse direct-rewrite changes: odd name-status field count")
	}
	rows := make([]string, 0, len(tokens)/2)
	for i := 0; i < len(tokens); i += 2 {
		statusText, rel := string(tokens[i]), filepath.ToSlash(string(tokens[i+1]))
		if rel == markerPath {
			continue
		}
		if len(statusText) != 1 || !strings.Contains("AMDT", statusText) {
			return "", fmt.Errorf("unsupported direct-rewrite status %q for %q", statusText, rel)
		}
		if rel == "" || strings.ContainsAny(rel, "\t\r\n") {
			return "", fmt.Errorf("unsafe direct-rewrite path %q", rel)
		}
		status := statusText[0]
		oldMode, oldHash := "-", "-"
		newMode, newHash := "-", "-"
		if status != 'A' {
			oldMode, err = treeMode(root, base, rel)
			if err != nil {
				return "", err
			}
			oldHash, err = refBlobHash(root, base, rel)
			if err != nil {
				return "", err
			}
		}
		if status != 'D' {
			if head == "" {
				newMode, newHash, err = worktreeBlob(root, rel)
			} else {
				newMode, err = treeMode(root, head, rel)
				if err == nil {
					newHash, err = refBlobHash(root, head, rel)
				}
			}
			if err != nil {
				return "", err
			}
		}
		rows = append(rows, fmt.Sprintf("%c\t%s\t%s\t%s\t%s\t%s\n", status, rel, oldMode, oldHash, newMode, newHash))
	}
	sort.Strings(rows)
	h := sha256.New()
	for _, row := range rows {
		_, _ = io.WriteString(h, row)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func resolveCommit(root, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("empty authorization base")
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", ref+"^{commit}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func treeMode(root, ref, rel string) (string, error) {
	out, err := exec.Command("git", "-C", root, "ls-tree", "-z", ref, "--", filepath.FromSlash(rel)).Output()
	if err != nil {
		return "", fmt.Errorf("read mode for %s at %s: %w", rel, ref, err)
	}
	fields := strings.Fields(string(bytes.TrimSuffix(out, []byte{0})))
	if len(fields) < 3 {
		return "", fmt.Errorf("read mode for %s at %s: path is absent", rel, ref)
	}
	return fields[0], nil
}

func refBlobHash(root, ref, rel string) (string, error) {
	out, err := exec.Command("git", "-C", root, "show", ref+":"+rel).Output()
	if err != nil {
		return "", fmt.Errorf("read %s at %s: %w", rel, ref, err)
	}
	return sum(out), nil
}

func worktreeBlob(root, rel string) (string, string, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return "", "", fmt.Errorf("inspect %s: %w", rel, err)
	}
	mode := "100644"
	var body []byte
	if info.Mode()&os.ModeSymlink != 0 {
		mode = "120000"
		target, err := os.Readlink(path)
		if err != nil {
			return "", "", fmt.Errorf("read link %s: %w", rel, err)
		}
		body = []byte(target)
	} else {
		if !info.Mode().IsRegular() {
			return "", "", fmt.Errorf("changed path %s is not a regular file or symlink", rel)
		}
		if info.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		body, err = os.ReadFile(path)
		if err != nil {
			return "", "", fmt.Errorf("read %s: %w", rel, err)
		}
	}
	return mode, sum(body), nil
}

func sum(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func requireJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("authorization marker contains multiple JSON values")
		}
		return fmt.Errorf("decode authorization marker trailer: %w", err)
	}
	return nil
}
