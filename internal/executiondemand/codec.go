// This file implements the canonical length-delimited codec for
// execution-demand/v2 origins (design §3.1, §3.3). Every field is framed by
// a fixed-width byte length, so concatenated fields can never be mistaken
// for one another's boundary; fields are always emitted and required in
// one strictly ascending tag order, so a duplicate, reordered, or unknown
// tag is a single structural defect rather than something a decoder must
// special-case per field.
package executiondemand

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
)

const lenPrefixSize = 8

// writeLD appends v as one length-delimited field: a fixed 8-byte
// big-endian length followed by the bytes themselves.
func writeLD(buf *bytes.Buffer, v []byte) {
	var lp [lenPrefixSize]byte
	binary.BigEndian.PutUint64(lp[:], uint64(len(v)))
	buf.Write(lp[:])
	buf.Write(v)
}

// readLD reads one length-delimited field from the front of b.
func readLD(b []byte) (value, rest []byte, err error) {
	if len(b) < lenPrefixSize {
		return nil, nil, fmt.Errorf("truncated length prefix")
	}
	n := binary.BigEndian.Uint64(b[:lenPrefixSize])
	b = b[lenPrefixSize:]
	if uint64(len(b)) < n {
		return nil, nil, fmt.Errorf("truncated field: want %d bytes, have %d", n, len(b))
	}
	return b[:n], b[n:], nil
}

// writeTag appends one tagged field: a 1-byte tag then a length-delimited
// value.
func writeTag(buf *bytes.Buffer, tag byte, v []byte) {
	buf.WriteByte(tag)
	writeLD(buf, v)
}

// readTag reads one tagged field from the front of b.
func readTag(b []byte) (tag byte, value, rest []byte, err error) {
	if len(b) < 1 {
		return 0, nil, nil, fmt.Errorf("truncated tag")
	}
	value, rest, err = readLD(b[1:])
	return b[0], value, rest, err
}

// writeFixedFields encodes values as tags 1..len(values), in that order.
func writeFixedFields(values [][]byte) []byte {
	var buf bytes.Buffer
	for i, v := range values {
		writeTag(&buf, byte(i+1), v)
	}
	return buf.Bytes()
}

// readFixedFields decodes exactly tags 1..n, strictly ascending with no
// gaps, duplicates, unknown tags, or trailing bytes. This one check is what
// makes duplicate, unknown, out-of-order, and missing fields all fail
// closed (design §3.1: "unknown keys, values, versions, duplicate fields
// ... are defects").
func readFixedFields(b []byte, n int) ([][]byte, error) {
	values := make([][]byte, n)
	want := 1
	for len(b) > 0 {
		if want > n {
			return nil, fmt.Errorf("trailing data after field %d", n)
		}
		tag, value, rest, err := readTag(b)
		if err != nil {
			return nil, err
		}
		if int(tag) != want {
			return nil, fmt.Errorf("expected field %d, got %d (duplicate, unknown, or out-of-order field)", want, tag)
		}
		values[tag-1] = value
		want++
		b = rest
	}
	if want != n+1 {
		return nil, fmt.Errorf("missing field %d", want)
	}
	return values, nil
}

func boolByte(b bool) []byte {
	if b {
		return []byte{1}
	}
	return []byte{0}
}

func decodeBool(b []byte) (bool, error) {
	switch {
	case len(b) == 1 && b[0] == 0:
		return false, nil
	case len(b) == 1 && b[0] == 1:
		return true, nil
	default:
		return false, fmt.Errorf("invalid boolean encoding")
	}
}

// encodeStringList canonically encodes xs as a sorted sequence of
// length-delimited items, so field reordering never changes the output
// (design DINV-4/DINV-5).
func encodeStringList(xs []string) []byte {
	sorted := append([]string(nil), xs...)
	sort.Strings(sorted)
	var buf bytes.Buffer
	for _, x := range sorted {
		writeLD(&buf, []byte(x))
	}
	return buf.Bytes()
}

func decodeStringList(b []byte) ([]string, error) {
	var out []string
	for len(b) > 0 {
		v, rest, err := readLD(b)
		if err != nil {
			return nil, err
		}
		out = append(out, string(v))
		b = rest
	}
	return out, nil
}

// citationIDs returns cs's citation IDs in normalized (sorted) order.
func citationIDs(cs []Citation) []string {
	sorted := NormalizeCitations(cs)
	ids := make([]string, len(sorted))
	for i, c := range sorted {
		ids[i] = c.ID
	}
	return ids
}

// encodeRulePairs canonically encodes applied rules as (id, kind) pairs
// sorted by id, each pair itself length-delimited within the list.
func encodeRulePairs(rs []AppliedRule) []byte {
	sorted := append([]AppliedRule(nil), rs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	var buf bytes.Buffer
	for _, r := range sorted {
		var item bytes.Buffer
		writeLD(&item, []byte(r.ID))
		writeLD(&item, []byte(r.Kind))
		writeLD(&buf, item.Bytes())
	}
	return buf.Bytes()
}

func decodeRulePairs(b []byte) ([]AppliedRule, error) {
	var out []AppliedRule
	for len(b) > 0 {
		item, rest, err := readLD(b)
		if err != nil {
			return nil, err
		}
		id, r2, err := readLD(item)
		if err != nil {
			return nil, err
		}
		kind, r3, err := readLD(r2)
		if err != nil {
			return nil, err
		}
		if len(r3) != 0 {
			return nil, fmt.Errorf("trailing bytes in applied-rule item")
		}
		out = append(out, AppliedRule{ID: string(id), Kind: string(kind)})
		b = rest
	}
	return out, nil
}

const (
	planV2FieldCount       = 23
	planV1CompatFieldCount = 15
	originFieldCount       = 3
)

func encodePlanV2Body(p PlanV2Origin) []byte {
	raisePresent := p.Raise != nil
	var raiseValue Assurance
	var raiseCitations []Citation
	if raisePresent {
		raiseValue = p.Raise.Value
		raiseCitations = p.Raise.Citations
	}
	return writeFixedFields([][]byte{
		[]byte(p.Repository),
		[]byte(p.Plan.Feature),
		[]byte(p.Plan.Version),
		[]byte(p.Plan.Fingerprint),
		[]byte(p.Task),
		[]byte(p.Source),
		[]byte(p.Assessment.Reasoning),
		[]byte(p.Assessment.Coupling),
		[]byte(p.Assessment.Novelty),
		[]byte(p.Assessment.Verification),
		[]byte(p.Assessment.ImpactScope),
		[]byte(p.Assessment.ArchitecturalRole),
		encodeStringList(citationIDs(p.Assessment.Citations)),
		boolByte(raisePresent),
		[]byte(raiseValue),
		encodeStringList(citationIDs(raiseCitations)),
		[]byte(p.Profile.Capability),
		[]byte(p.Profile.Deliberation),
		[]byte(p.Profile.Context),
		[]byte(p.Profile.Assurance),
		[]byte(p.Profile.PolicyVersion),
		[]byte(p.Profile.PolicyDigest),
		encodeRulePairs(p.Profile.Applied),
	})
}

func decodePlanV2Body(b []byte) (PlanV2Origin, error) {
	f, err := readFixedFields(b, planV2FieldCount)
	if err != nil {
		return PlanV2Origin{}, err
	}
	citationIDList, err := decodeStringList(f[12])
	if err != nil {
		return PlanV2Origin{}, err
	}
	citations := make([]Citation, len(citationIDList))
	for i, id := range citationIDList {
		citations[i] = Citation{ID: id}
	}
	raisePresent, err := decodeBool(f[13])
	if err != nil {
		return PlanV2Origin{}, err
	}
	raiseCitationIDs, err := decodeStringList(f[15])
	if err != nil {
		return PlanV2Origin{}, err
	}
	var raise *AssuranceRaise
	if raisePresent {
		rc := make([]Citation, len(raiseCitationIDs))
		for i, id := range raiseCitationIDs {
			rc[i] = Citation{ID: id}
		}
		raise = &AssuranceRaise{Value: Assurance(f[14]), Citations: rc}
	} else if len(f[14]) != 0 || len(raiseCitationIDs) != 0 {
		return PlanV2Origin{}, fmt.Errorf("non-canonical encoding: raise fields set while raise_present is false")
	}
	applied, err := decodeRulePairs(f[22])
	if err != nil {
		return PlanV2Origin{}, err
	}
	return PlanV2Origin{
		Repository: RepositoryIdentity(f[0]),
		Plan:       PlanIdentity{Feature: string(f[1]), Version: string(f[2]), Fingerprint: string(f[3])},
		Task:       TaskIdentity(f[4]),
		Source:     SourceDigest(f[5]),
		Assessment: Assessment{
			Reasoning:         Reasoning(f[6]),
			Coupling:          Coupling(f[7]),
			Novelty:           Novelty(f[8]),
			Verification:      Verification(f[9]),
			ImpactScope:       ImpactScope(f[10]),
			ArchitecturalRole: ArchitecturalRole(f[11]),
			Citations:         citations,
		},
		Raise: raise,
		Profile: Profile{
			Capability:    Capability(f[16]),
			Deliberation:  Deliberation(f[17]),
			Context:       Context(f[18]),
			Assurance:     Assurance(f[19]),
			PolicyVersion: string(f[20]),
			PolicyDigest:  string(f[21]),
			Applied:       applied,
		},
	}, nil
}

// EncodeCanonical deterministically encodes o as canonical length-delimited
// bytes. It rejects o outright if o does not parse closed (design §3.1).
func EncodeCanonical(o Origin) ([]byte, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}
	var body []byte
	switch o.Kind {
	case OriginPlanV2:
		body = encodePlanV2Body(*o.PlanV2)

	}
	return writeFixedFields([][]byte{
		[]byte(o.Schema),
		[]byte(o.Kind),
		body,
	}), nil
}

// DecodeCanonical strictly decodes b into an Origin, failing closed on
// unknown keys, duplicate fields, unknown tags/versions, truncation, and
// trailing (non-canonical) bytes.
func DecodeCanonical(b []byte) (Origin, error) {
	f, err := readFixedFields(b, originFieldCount)
	if err != nil {
		return Origin{}, err
	}
	o := Origin{Schema: string(f[0]), Kind: OriginKind(f[1])}
	switch o.Kind {
	case OriginPlanV2:
		p, err := decodePlanV2Body(f[2])
		if err != nil {
			return Origin{}, err
		}
		o.PlanV2 = &p

	default:
		return Origin{}, Diagnostic{"kind", "unknown_kind", fmt.Sprintf("unknown origin kind %q", o.Kind)}
	}
	if err := o.validate(); err != nil {
		return Origin{}, err
	}
	return o, nil
}

// --- identity-support encodings, shared by Identify (identity.go) ---

func identityPlanBytes(p PlanIdentity) []byte {
	var buf bytes.Buffer
	writeLD(&buf, []byte(p.Feature))
	writeLD(&buf, []byte(p.Version))
	writeLD(&buf, []byte(p.Fingerprint))
	return buf.Bytes()
}

func identityProfileMinimumBytes(m ProfileMinimum) []byte {
	var buf bytes.Buffer
	writeLD(&buf, []byte(m.Capability))
	writeLD(&buf, []byte(m.Deliberation))
	writeLD(&buf, []byte(m.Context))
	writeLD(&buf, []byte(m.Assurance))
	return buf.Bytes()
}

// identityPlanV2PayloadBytes frames the plan/v2 "origin payload": every raw
// assessment axis, the normalized citation set, and the assurance raise
// (design §3.3).
func identityPlanV2PayloadBytes(p PlanV2Origin) []byte {
	raisePresent := p.Raise != nil
	var raiseValue Assurance
	var raiseCitations []Citation
	if raisePresent {
		raiseValue = p.Raise.Value
		raiseCitations = p.Raise.Citations
	}
	var buf bytes.Buffer
	for _, v := range []string{
		string(p.Assessment.Reasoning), string(p.Assessment.Coupling), string(p.Assessment.Novelty),
		string(p.Assessment.Verification), string(p.Assessment.ImpactScope), string(p.Assessment.ArchitecturalRole),
	} {
		writeLD(&buf, []byte(v))
	}
	writeLD(&buf, encodeStringList(citationIDs(p.Assessment.Citations)))
	writeLD(&buf, boolByte(raisePresent))
	writeLD(&buf, []byte(raiseValue))
	writeLD(&buf, encodeStringList(citationIDs(raiseCitations)))
	return buf.Bytes()
}

// identityPlanV2PolicyBytes frames the plan/v2 policy identity: schema
// version, immutable policy digest, and every applied floor/promotion id.
func identityPlanV2PolicyBytes(p PlanV2Origin) []byte {
	var buf bytes.Buffer
	writeLD(&buf, []byte(p.Profile.PolicyVersion))
	writeLD(&buf, []byte(p.Profile.PolicyDigest))
	writeLD(&buf, encodeRulePairs(p.Profile.Applied))
	return buf.Bytes()
}
