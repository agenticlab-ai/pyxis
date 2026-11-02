package verdict

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

// RecordVersion is the one lane-verdict record shape this build writes and
// parses. The record is versioned, closed-schema data — additive evolution
// only (AP-6); a block line declaring any other version is a defect, never
// parsed leniently.
const RecordVersion = "lane-verdict/v1"

// Record is the structured, versioned lane-verdict record appended per
// evaluation (merge-lane-v1 §3.4's `lane_verdict` event row, §3.6's
// proposal-riding block). It names exactly the five properties the basis
// contracts — lane, floor hits, class, policy version, and head revision — so a
// verdict is cold-readable and its evaluation reconstructible years later
// (NFR-4). It is a projection of the classifier's verdict into the durable,
// append-only carrier; the classifier decides, this records.
type Record struct {
	RecordVersion string   `json:"record_version"`
	Lane          string   `json:"lane"`
	FloorHits     []string `json:"floor_hits,omitempty"`
	Class         string   `json:"class,omitempty"`
	PolicyVersion string   `json:"policy_version"`
	HeadRevision  string   `json:"head_revision"`
}

// NewRecord projects one classifier verdict into a lane-verdict record. Every
// named property is copied from the verdict's own fields — the lane, the floor
// hits it recorded, the AUTO class it matched (empty when the lane is HUMAN),
// and the pinned policy version and head revision — so the record and the
// verdict cannot disagree.
func NewRecord(v classifier.Verdict) Record {
	return Record{
		RecordVersion: RecordVersion,
		Lane:          string(v.Lane),
		FloorHits:     v.FloorHits,
		Class:         v.AutoClass,
		PolicyVersion: v.Pinned.PolicyVersion,
		HeadRevision:  v.Pinned.Head,
	}
}

// Marshal serializes the record deterministically — the struct's field order is
// fixed and FloorHits is recorded in the classifier's fixed order — so the same
// verdict yields byte-identical bytes (AP-6, DINV-4's determinism carried into
// the record). One line, no trailing newline; AppendBlock adds the separator.
func (r Record) Marshal() []byte {
	b, err := json.Marshal(r)
	if err != nil {
		// Record carries only strings and string slices, so Marshal cannot fail;
		// this guards a future field that breaks that (programmer error) rather
		// than emitting a partial, unreadable record.
		panic("verdict: record marshal: " + err.Error())
	}
	return b
}

// AppendBlock appends one evaluation's record to the append-only verdict block
// that rides the proposal and returns the new block. The block is newline-
// delimited records (JSONL): append-only across evaluations — earlier records
// are never rewritten, only followed — so the block is the evaluation history,
// not a latest-wins cell. A nil/empty block starts a fresh one.
func AppendBlock(block []byte, r Record) []byte {
	line := r.Marshal()
	if len(strings.TrimSpace(string(block))) == 0 {
		return append(line, '\n')
	}
	out := append([]byte{}, block...)
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, line...)
	return append(out, '\n')
}

// ParseBlock reads an append-only verdict block back into its records, in
// evaluation order. The parse is closed: a non-empty line that is not a
// well-formed record of the pinned version is a named defect, never a silent
// skip — a corrupted or version-drifted block is a fault, not partial data.
func ParseBlock(block []byte) ([]Record, error) {
	var recs []Record
	for i, line := range strings.Split(string(block), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("verdict block line %d is malformed: %w", i+1, err)
		}
		if r.RecordVersion != RecordVersion {
			return nil, fmt.Errorf("verdict block line %d declares version %q, not %q", i+1, r.RecordVersion, RecordVersion)
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// File is one path/content pair of the content under evaluation, as read from
// the trusted mainline context (the merge result), for computing its subject
// fingerprint.
type File struct {
	Path    string
	Content []byte
}

// Fingerprint computes the subject fingerprint of the evaluated content — a
// deterministic sha256 over the files sorted by path, each path and its
// normalized content framed with NUL separators (neither occurs in a path). It
// is the value a review report records as the content it reviewed; the arm
// recomputes it independently over the evaluated content and requires a match,
// so a report cannot claim to have reviewed content it never saw (DINV-3). It
// reuses schema.Normalize so the fingerprint is line-ending- and
// trailing-whitespace-stable, matching the rest of the doc system.
func Fingerprint(files []File) string {
	sorted := append([]File{}, files...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Path < sorted[b].Path })
	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write(schema.Normalize(f.Content))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// FoldAttribution folds the per-authoring-commit reviewer-flow attribution
// signals of a review report into one verdict, fail-closed (worst wins): any
// Underivable commit (a malformed or replayed reviewer stamp) makes the report
// Underivable; else any Unattributed commit (one with no reviewer-flow stamp —
// an appended-by-author edit) makes it Unattributed; only when every authoring
// commit carries the reviewer-flow stamp is the report Attested. An empty signal
// set is Unattributed — no attribution is not attribution. This is the seam the
// acting adapter (T4) fills from git trailers, following the shipped attribution
// arms' trailer discipline (ReviewerTrailerKey).
func FoldAttribution(perCommit []Attribution) Attribution {
	if len(perCommit) == 0 {
		return Unattributed
	}
	result := Attested
	for _, a := range perCommit {
		switch a {
		case Underivable:
			return Underivable
		case Unattributed:
			result = Unattributed
		}
	}
	return result
}

// ReviewerTrailerKey is the stable, documented commit-trailer key that carries a
// review report's reviewer-flow attribution — the reviewer-flow analogue of the
// shipped attribution arms' `Argus-Drive` (internal/argus/attribution). A report
// whose authoring commits carry a produced, content-bound stamp under this key
// is Attested; its absence is Unattributed. The git-backed derivation of the
// per-commit signals is the acting adapter's (T4); this package defines the key
// and the fold.
const ReviewerTrailerKey = "Review-Flow"
