package compile

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

var numberedDocument = regexp.MustCompile(`^(brainstorm|reviews)/([0-9]{3})\.md$`)

// Link is a relation waiting for issuance-time IDs. The lock writer resolves
// the target's active record after compiling all parts.
type Link struct {
	Type   string
	Target string // subject#family
}

// Record carries a native memory draft and any issuance-time links.
type Record struct {
	Draft reconcile.Draft
	Links []Link
}

// Result is deterministic for one feature, mainline, and set of working files.
// Identity and issuance time are intentionally absent.
type Result struct {
	Feature      string
	Mainline     string
	Digest       string
	Records      []Record
	Problems     []Problem
	Reviews      []Review
	Changes      []RequirementChange
	Requirements []Requirement
}

// Compile parses one open draft in process. File names are relative to the
// feature's draft directory; callers read the files from one pinned tree.
func Compile(feature, mainline string, files map[string][]byte) (Result, error) {
	return CompileWithHistory(feature, mainline, files, nil)
}

// ContractDigest binds the two exact working-file paths and bytes. A locked
// version can be reopened even if a later compiler rejects its old grammar.
func ContractDigest(prd, techSpec []byte) string {
	return draftDigest(map[string][]byte{"prd.md": prd, "tech-spec.md": techSpec})
}

// CompileWithHistory also reports every changed requirement row from the
// branch's oldest-to-newest PRD revisions. A nil history leaves that list empty.
func CompileWithHistory(feature, mainline string, files map[string][]byte, revisions []PRDRevision) (Result, error) {
	if !record.ValidSubject("prd/"+feature) || feature == "" || strings.Contains(feature, "/") {
		return Result{}, fmt.Errorf("compile draft: invalid feature %q", feature)
	}
	if strings.TrimSpace(mainline) == "" {
		return Result{}, fmt.Errorf("compile draft %s: mainline is required", feature)
	}
	if _, ok := files["prd.md"]; !ok {
		return Result{}, fmt.Errorf("compile draft %s: prd.md is missing", feature)
	}
	if _, ok := files["tech-spec.md"]; !ok {
		return Result{}, fmt.Errorf("compile draft %s: tech-spec.md is missing", feature)
	}
	if body, ok := files["events.md"]; ok {
		parsed, err := events.Parse(body)
		if err != nil {
			return Result{}, fmt.Errorf("compile draft %s events.md: %w", feature, err)
		}
		for _, event := range parsed {
			if event.Feature != feature {
				return Result{}, fmt.Errorf("compile draft %s events.md: event %s belongs to %s", feature, event.ID, event.Feature)
			}
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if !validWorkingPath(name) {
			return Result{}, fmt.Errorf("compile draft %s: invalid working-file path %q", feature, name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := Result{Feature: feature, Mainline: mainline, Digest: draftDigest(files), Records: []Record{}}
	rows, requirements, err := compilePRD(feature, files["prd.md"])
	if err != nil {
		return Result{}, err
	}
	out.Records = append(out.Records, rows...)
	out.Requirements = requirements
	decisions, err := compileTechSpec(feature, files["tech-spec.md"])
	if err != nil {
		return Result{}, err
	}
	out.Records = append(out.Records, decisions...)
	questions, err := compileTechQuestions(feature, files["tech-spec.md"])
	if err != nil {
		return Result{}, err
	}
	out.Records = append(out.Records, questions...)
	if body, ok := files["problems.md"]; ok {
		problems, problemRecords, err := compileProblems(feature, body)
		if err != nil {
			return Result{}, err
		}
		out.Problems = problems
		out.Records = append(out.Records, problemRecords...)
	}
	for _, name := range names {
		subject, family := documentAddress(feature, name)
		draft := reconcile.Draft{
			Kind: record.Document, Subject: subject, Family: family,
			Statement: "Draft " + name, LiteralBody: true,
			Body: append([]byte(nil), files[name]...),
		}
		if strings.HasPrefix(name, "reviews/") {
			review, err := compileReview(name, files[name])
			if err != nil {
				return Result{}, err
			}
			out.Reviews = append(out.Reviews, review)
			draft.Tags = []string{"draft-digest:" + review.Digest}
		}
		out.Records = append(out.Records, Record{Draft: draft})
	}
	if len(revisions) > 0 {
		out.Changes, err = compileChanges(files["prd.md"], revisions, out.Problems)
		if err != nil {
			return Result{}, err
		}
	}
	return out, nil
}

// WithEvents updates the compiled event Document in memory. Lock preparation
// can retain pending events without touching the author's working files.
func WithEvents(result Result, additions ...events.Event) (Result, error) {
	for _, event := range additions {
		if event.Feature != result.Feature {
			return Result{}, fmt.Errorf("draft event feature %s differs from %s", event.Feature, result.Feature)
		}
	}
	result.Records = append([]Record(nil), result.Records...)
	for i := range result.Records {
		draft := &result.Records[i].Draft
		if draft.Subject != "feature/"+result.Feature || draft.Family != "events" {
			continue
		}
		body, _, err := events.Append(draft.Body, additions...)
		if err != nil {
			return Result{}, fmt.Errorf("append compiled draft events: %w", err)
		}
		draft.Body = body
		return result, nil
	}
	body, _, err := events.Append(nil, additions...)
	if err != nil {
		return Result{}, fmt.Errorf("append compiled draft events: %w", err)
	}
	result.Records = append(result.Records, Record{Draft: reconcile.Draft{
		Kind: record.Document, Subject: "feature/" + result.Feature, Family: "events",
		Statement: "Draft events.md", LiteralBody: true, Body: body,
	}})
	return result, nil
}

func validWorkingPath(name string) bool {
	if path.Clean(name) != name || strings.HasPrefix(name, "/") {
		return false
	}
	switch name {
	case "prd.md", "tech-spec.md", "problems.md", "events.md":
		return true
	}
	return numberedDocument.MatchString(name)
}

func documentAddress(feature, name string) (string, string) {
	switch name {
	case "prd.md":
		return "prd/" + feature, "prd"
	case "tech-spec.md":
		return "design/" + feature, "design"
	case "problems.md":
		return "prd/" + feature, "problems"
	case "events.md":
		return "feature/" + feature, "events"
	}
	match := numberedDocument.FindStringSubmatch(name)
	if match[1] == "brainstorm" {
		return "brainstorm/" + feature, "record-" + match[2]
	}
	return "review/" + feature, "report-" + match[2]
}

// draftDigest binds only the PRD and tech spec, using length-delimited paths
// and exact bytes. Reviews and other working files do not stale the version.
func draftDigest(files map[string][]byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte("pyxis-draft-contract/v1\x00"))
	for _, name := range []string{"prd.md", "tech-spec.md"} {
		writeDigestPart(h, []byte(name))
		writeDigestPart(h, files[name])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func writeDigestPart(h interface{ Write([]byte) (int, error) }, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = h.Write(length[:]) // hash.Hash.Write never returns an error
	_, _ = h.Write(value)     // hash.Hash.Write never returns an error
}
