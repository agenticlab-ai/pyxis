package eval

import (
	"encoding/json"
	"strings"
)

// The harness's review-pack encoding. runtime.Review takes an opaque []byte pack
// and hands it to each lens's runner Run. The harness encodes a leg's (band,
// charter, diff) into that pack so BOTH a real runner (which reads the diff +
// charter to review) and the hermetic fake runner (which reads the encoded fields
// back to emit the fixture's seeded findings) drive off one deterministic pack.
//
// This is NOT the production internal/implreview/pack (that assembles a real
// context pack from repo files via contextpack). The harness's leg diffs are
// synthetic fixture strings, not repo files, so the harness builds a small
// self-describing byte pack here rather than writing fixture files to disk for
// contextpack to project — the runtime seam takes opaque bytes, and this is the
// eval's opaque bytes. A test asserts the round-trip (buildPack → parsePack).

// packEnvelope is the harness pack's on-wire shape — the leg's band, charter, and
// diff, JSON-encoded. Deterministic (a fixed field order via the struct) so the
// pack bytes are stable across runs (the runtime records a content-derived
// context_id; a stable pack keeps a run reproducible).
type packEnvelope struct {
	Band    string `json:"band"`
	Charter string `json:"charter"`
	Diff    string `json:"diff"`
}

// buildPack encodes a leg into the opaque review-pack bytes runtime.Review hands
// to the runner. It never returns an empty pack (runtime.Review rejects an empty
// pack) — even an empty-diff leg carries its band + charter.
func buildPack(band, charter, diff string) []byte {
	data, err := json.Marshal(packEnvelope{Band: band, Charter: charter, Diff: diff})
	if err != nil {
		// packEnvelope is three strings — json.Marshal cannot fail on it; this
		// branch is unreachable, but returning a non-empty sentinel keeps the pack
		// non-empty (runtime.Review's invariant) rather than panicking.
		return []byte(`{"band":"","charter":"","diff":""}`)
	}
	return data
}

// parsePack decodes the harness pack bytes back to its fields — the inverse of
// buildPack, used by the hermetic fake runner to read the leg it is reviewing and
// emit that leg's seeded findings. A malformed pack yields the zero envelope
// (empty fields), which the fake runner treats as "no seeded findings".
func parsePack(pack []byte) packEnvelope {
	var env packEnvelope
	_ = json.Unmarshal(pack, &env) // a malformed pack yields the zero envelope; the fake emits nothing.
	return env
}

// syntheticDiff returns a deterministic diff of exactly n bytes — the truncation
// family's oversize diff, synthesized so the fixture file stays tiny (it declares
// diff_bytes, not a giant literal). The content is a repeated legible line padded
// to n bytes; a size gate reads len(diff), so the content only needs to BE n
// bytes, deterministically.
func syntheticDiff(n int) []byte {
	if n <= 0 {
		return nil
	}
	const line = "+// oversize diff filler line — unread tail hides the defect (EVAL-8, UB-7)\n"
	var b strings.Builder
	b.Grow(n)
	for b.Len() < n {
		remaining := n - b.Len()
		if remaining >= len(line) {
			b.WriteString(line)
		} else {
			b.WriteString(line[:remaining])
		}
	}
	return []byte(b.String())
}
