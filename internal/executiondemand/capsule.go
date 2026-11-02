// This file defines the sealed execution-demand-capsule/v1 transport unit
// (design: execution-demand-contract-v1 §3.5, DINV-7): the complete
// canonical origin plus every identity a consumer must recheck before it
// trusts the origin at all, and the two-phase verification that
// distinguishes transport corruption (Capsule.selfConsistent, checked by
// Extract) from a stale, cross-task, or forged copy (VerifyCapsule, checked
// against an independently reconstructed AuthoritativeDemand). Marker
// embedding/extraction lives in capsule_marker.go; this file owns the
// capsule's own shape, digest, and both verification tiers.
package executiondemand

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// CapsuleCodec names one capsule wire-codec version (design §3.5): the
// value a Capsule's Codec field carries, and what Extract/VerifyCapsule
// check every capsule against. A future codec revision is a new
// CapsuleCodec value under a superseding design, never a silent behavior
// change under the same tag — mirrors OriginKind's version-as-value
// discipline (origin.go).
type CapsuleCodec string

// CapsuleSchema is the one capsule codec this package implements. It has no
// successor within this design; a semantic change to the capsule shape
// requires a superseding design and a new schema value (mirrors
// OriginSchema).
const CapsuleSchema CapsuleCodec = "execution-demand-capsule/v1"

// The typed boundary a CapsuleMismatch was raised at (design §3.5's
// two-phase readback, design F-32): transport failures never reach an
// origin at all; self failures are a capsule whose own fields disagree
// with each other; authority failures are an internally-consistent capsule
// that still disagrees with the demand reconstructed from the exact frozen
// plan.
const (
	BoundaryTransport = "transport"
	BoundarySelf      = "self"
	BoundaryAuthority = "authority"
)

// The closed set of refusal codes this file and capsule_marker.go raise.
// Named as constants so callers can match on a stable value rather than a
// message substring, and so every raise site in this package uses the same
// spelling.
const (
	CodeMissingCapsule       = "missing_capsule"
	CodeDuplicateCapsule     = "duplicate_capsule"
	CodeMalformedCapsule     = "malformed_capsule"
	CodeTruncatedCapsule     = "truncated_capsule"
	CodeOversizedCapsule     = "oversized_capsule"
	CodeOversizedBody        = "oversized_body"
	CodeInvalidCapsule       = "invalid_capsule"
	CodeInvalidOrigin        = "invalid_origin"
	CodeIdentityMismatch     = "identity_mismatch"
	CodeStaleCapsule         = "stale_capsule"
	CodeDemandIDMismatch     = "demand_id_mismatch"
	CodeCodecMismatch        = "codec_mismatch"
	CodeFieldMismatch        = "field_mismatch"
	CodeDigestMismatch       = "digest_mismatch"
	CodeAuthorityUnavailable = "authority_unavailable"
)

// CapsuleMismatch is the typed refusal every capsule boundary or
// authoritative-comparison failure carries (design §3.5, DINV-7; PRD UB-3:
// "name the boundary/field mismatch"): which boundary failed, which field,
// a stable code, and a human message — never a bare bool or a generic error
// a caller must pattern-match by string.
type CapsuleMismatch struct {
	Boundary   string             `json:"boundary"`
	Repository RepositoryIdentity `json:"repository,omitempty"`
	Task       TaskIdentity       `json:"task,omitempty"`
	DemandID   DemandID           `json:"demand_id,omitempty"`
	Schema     string             `json:"schema"`
	Version    string             `json:"version"`
	Field      string             `json:"field"`
	Rule       string             `json:"rule"`
	Code       string             `json:"code"`
	Message    string             `json:"message"`
}

func (m CapsuleMismatch) Error() string {
	return fmt.Sprintf("%s/%s (%s): %s", m.Boundary, m.Field, m.Code, m.Message)
}

// newCapsuleMismatch constructs every refusal with the safe, expected schema
// and codec version plus a stable rule id. It never derives those fields from
// tracker-controlled bytes. withTrustedCapsule adds only bounded
// identifier-shaped correlation values from a capsule that the caller has
// already established as authoritative; arbitrary tracker strings never
// reach diagnostics.
func newCapsuleMismatch(boundary, field, code, message string) CapsuleMismatch {
	return CapsuleMismatch{
		Boundary: boundary,
		Schema:   OriginSchema,
		Version:  string(CapsuleSchema),
		Field:    field,
		Rule:     code,
		Code:     code,
		Message:  message,
	}
}

func (m CapsuleMismatch) withTrustedCapsule(c Capsule) CapsuleMismatch {
	if safeDiagnosticIdentity(string(c.Repository)) {
		m.Repository = c.Repository
	}
	if safeDiagnosticIdentity(string(c.Task)) {
		m.Task = c.Task
	}
	if validDigest(string(c.DemandID)) {
		m.DemandID = c.DemandID
	}
	return m
}

func safeDiagnosticIdentity(s string) bool {
	if len(s) == 0 || len(s) > 256 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == '/' {
			continue
		}
		return false
	}
	return true
}

func validDigest(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && (s[i] < 'a' || s[i] > 'f') {
			return false
		}
	}
	return true
}

// Capsule is the complete self-contained execution-demand transport unit a
// tracker body carries (design §3.5): the canonical origin plus the
// identities and fingerprints a consumer checks before it decodes the
// origin at all, the demand id, the codec version, and a digest over every
// other field. Repository/Plan/Task/Source duplicate what Origin's own
// member already carries (Origin.coreIdentity) — Capsule.selfConsistent
// requires them to agree, so a capsule whose outer identity was edited
// independently of its inner origin is caught without a full origin decode.
type Capsule struct {
	Codec      CapsuleCodec
	DemandID   DemandID
	Repository RepositoryIdentity
	Plan       PlanIdentity
	Task       TaskIdentity
	Source     SourceDigest
	Origin     Origin
	Digest     string
}

// validate checks Capsule's own structural shape — mirrors every other
// validate() in this package (Diagnostic, not CapsuleMismatch): a bare
// shape defect, not yet a boundary or authority comparison.
func (c Capsule) validate() error {
	if c.Codec != CapsuleSchema {
		return Diagnostic{"codec", "unknown_codec", "capsule codec is unsupported"}
	}
	if c.DemandID == "" {
		return Diagnostic{"demand_id", "missing_identity", "capsule demand id is required"}
	}
	if c.Repository == "" {
		return Diagnostic{"repository", "missing_identity", "capsule repository identity is required"}
	}
	if err := c.Plan.validate(); err != nil {
		return err
	}
	if c.Task == "" {
		return Diagnostic{"task", "missing_identity", "capsule task identity is required"}
	}
	if c.Source == "" {
		return Diagnostic{"source_digest", "missing_identity", "capsule source digest is required"}
	}
	if err := c.Origin.validate(); err != nil {
		return err
	}
	if c.Digest == "" {
		return Diagnostic{"digest", "missing_digest", "capsule digest is required"}
	}
	return nil
}

// selfConsistent requires c's own fields to agree with each other: its
// demand id and outer identity must match what its embedded Origin itself
// carries, and its digest must match a fresh recomputation over its other
// fields. This catches transport corruption and naive tampering (an editor
// changing bytes without recomputing the digest) but NOT a forged capsule
// whose editor recomputed a consistent digest — only VerifyCapsule's
// comparison against an independently reconstructed AuthoritativeDemand
// catches that (design F-32).
func (c Capsule) selfConsistent() error {
	if err := c.validate(); err != nil {
		return newCapsuleMismatch(BoundarySelf, "shape", CodeInvalidCapsule, "capsule shape is invalid")
	}
	wantID, err := Identify(c.Origin)
	if err != nil {
		return newCapsuleMismatch(BoundarySelf, "origin", CodeInvalidOrigin, "capsule origin is invalid")
	}
	if wantID != c.DemandID {
		return newCapsuleMismatch(BoundarySelf, "demand_id", CodeDemandIDMismatch, "capsule demand id does not match its own embedded origin")
	}
	repo, plan, task, source := c.Origin.coreIdentity()
	switch {
	case repo != c.Repository:
		return newCapsuleMismatch(BoundarySelf, "repository", CodeIdentityMismatch, "capsule repository does not match its own embedded origin")
	case plan != c.Plan:
		return newCapsuleMismatch(BoundarySelf, "plan", CodeIdentityMismatch, "capsule plan identity does not match its own embedded origin")
	case task != c.Task:
		return newCapsuleMismatch(BoundarySelf, "task", CodeIdentityMismatch, "capsule task identity does not match its own embedded origin")
	case source != c.Source:
		return newCapsuleMismatch(BoundarySelf, "source_digest", CodeIdentityMismatch, "capsule source digest does not match its own embedded origin")
	}
	wantDigest, err := computeCapsuleDigest(c)
	if err != nil {
		return newCapsuleMismatch(BoundarySelf, "digest", CodeInvalidOrigin, "capsule digest could not be recomputed from its origin")
	}
	if wantDigest != c.Digest {
		return newCapsuleMismatch(BoundarySelf, "digest", CodeDigestMismatch, "capsule digest does not match its own content")
	}
	return nil
}

// capsuleFieldValues returns c's nine canonical field values in fixed tag
// order 1..9 — every Capsule field except Digest itself, which is a
// function of these (computeCapsuleDigest) rather than an independent tag.
func capsuleFieldValues(c Capsule) ([][]byte, error) {
	originBytes, err := EncodeCanonical(c.Origin)
	if err != nil {
		return nil, err
	}
	return [][]byte{
		[]byte(c.Codec),
		[]byte(c.DemandID),
		[]byte(c.Repository),
		[]byte(c.Plan.Feature),
		[]byte(c.Plan.Version),
		[]byte(c.Plan.Fingerprint),
		[]byte(c.Task),
		[]byte(c.Source),
		originBytes,
	}, nil
}

// computeCapsuleDigest returns the lowercase hex sha256 over c's nine
// canonical fields, framed exactly as EncodeCapsuleCanonical frames them
// (minus the digest field, which cannot cover itself).
func computeCapsuleDigest(c Capsule) (string, error) {
	fields, err := capsuleFieldValues(c)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(writeFixedFields(fields))
	return hex.EncodeToString(sum[:]), nil
}

const capsuleFieldCount = 10

// EncodeCapsuleCanonical deterministically encodes c as canonical
// length-delimited bytes (the same tagged-field discipline as
// EncodeCanonical), rejecting c outright if it does not parse closed.
func EncodeCapsuleCanonical(c Capsule) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	fields, err := capsuleFieldValues(c)
	if err != nil {
		return nil, err
	}
	fields = append(fields, []byte(c.Digest))
	return writeFixedFields(fields), nil
}

// DecodeCapsuleCanonical strictly decodes b into a structurally valid
// Capsule, failing closed on unknown/duplicate/out-of-order tags,
// truncation, and trailing bytes — exactly DecodeCanonical's discipline,
// one level up. It checks shape only (validate); it does NOT check
// self-consistency (digest/demand-id/identity agreement) — call
// Capsule.selfConsistent (or go through Extract, which does) for that.
func DecodeCapsuleCanonical(b []byte) (Capsule, error) {
	f, err := readFixedFields(b, capsuleFieldCount)
	if err != nil {
		return Capsule{}, err
	}
	origin, err := DecodeCanonical(f[8])
	if err != nil {
		return Capsule{}, err
	}
	c := Capsule{
		Codec:      CapsuleCodec(f[0]),
		DemandID:   DemandID(f[1]),
		Repository: RepositoryIdentity(f[2]),
		Plan:       PlanIdentity{Feature: string(f[3]), Version: string(f[4]), Fingerprint: string(f[5])},
		Task:       TaskIdentity(f[6]),
		Source:     SourceDigest(f[7]),
		Origin:     origin,
		Digest:     string(f[9]),
	}
	if err := c.validate(); err != nil {
		return Capsule{}, err
	}
	return c, nil
}

// Seal derives o's one canonical Capsule (design §3.5): its demand id
// (Identify), its outer identity (Origin.coreIdentity, duplicating what o
// itself carries), and a digest over all of it. Sealing the same origin
// twice — including a fresh reconstruction of the same frozen source —
// always yields the same capsule; Seal has no clock, randomness, or
// harness/native input.
func Seal(o Origin) (Capsule, error) {
	if err := o.validate(); err != nil {
		return Capsule{}, err
	}
	id, err := Identify(o)
	if err != nil {
		return Capsule{}, err
	}
	repo, plan, task, source := o.coreIdentity()
	c := Capsule{
		Codec:      CapsuleSchema,
		DemandID:   id,
		Repository: repo,
		Plan:       plan,
		Task:       task,
		Source:     source,
		Origin:     o,
	}
	digest, err := computeCapsuleDigest(c)
	if err != nil {
		return Capsule{}, err
	}
	c.Digest = digest
	return c, nil
}

// AuthoritativeDemand is the trusted comparison point VerifyCapsule checks
// a tracker-read Capsule against (design §3.5, DINV-7): a fresh Seal of the
// Origin reconstructed from the exact frozen plan source, never from
// tracker state or from the capsule under test. Reconstruct is its only
// constructor — there is no way to build a "ready" one from a zero value.
type AuthoritativeDemand struct {
	capsule Capsule
}

// Reconstruct derives the AuthoritativeDemand a tracker capsule must match:
// it seals o exactly as the original projection would have. Callers build
// o fresh from the exact frozen plan each time — this design's own
// reconstruction path (EncodeCanonical's origin for plan_v2) — before claim and again before
// review (design §3.5), never by trusting tracker text. An error return
// means the authority itself could not be reconstructed; the caller must
// treat that as blocking (design §3.9: "authoritative plan unavailable:
// block, not default") rather than passing a zero-value AuthoritativeDemand
// to VerifyCapsule — which refuses one anyway (CodeAuthorityUnavailable).
func Reconstruct(o Origin) (AuthoritativeDemand, error) {
	c, err := Seal(o)
	if err != nil {
		return AuthoritativeDemand{}, err
	}
	return AuthoritativeDemand{capsule: c}, nil
}

// trustedCapsule returns the independently reconstructed capsule used for
// refusal correlation and comparison. The field is deliberately unexported:
// only Reconstruct can create a non-zero AuthoritativeDemand, so a caller
// cannot launder tracker bytes into supposedly trusted diagnostic context.
func (a AuthoritativeDemand) trustedCapsule() (Capsule, error) {
	if a.capsule.Codec == "" {
		return Capsule{}, newCapsuleMismatch(BoundaryAuthority, "capsule", CodeAuthorityUnavailable, "authoritative demand was not reconstructed from the frozen plan")
	}
	if err := a.capsule.selfConsistent(); err != nil {
		return Capsule{}, newCapsuleMismatch(BoundaryAuthority, "capsule", CodeAuthorityUnavailable, "authoritative demand is not internally consistent")
	}
	return a.capsule, nil
}

func withAuthority(err error, trusted Capsule) error {
	var mismatch CapsuleMismatch
	if errors.As(err, &mismatch) {
		return mismatch.withTrustedCapsule(trusted)
	}
	var mismatchPointer *CapsuleMismatch
	if errors.As(err, &mismatchPointer) && mismatchPointer != nil {
		return mismatchPointer.withTrustedCapsule(trusted)
	}
	return err
}

// VerifyCapsule requires c to be field-identical to want — the demand
// reconstructed from the exact frozen plan source (DINV-7). It is safe to
// call standalone: it first re-checks both c's and want's own
// self-consistency, so a caller need not have gone through Extract or
// trust that Reconstruct's result reached it unmodified (design: proposal
// state may persist only a capsule's id/digest and reconstruct the rest
// later).
//
// Once both sides are known self-consistent, a capsule's demand id, its
// digest, and its origin bytes are three names for the same content
// (DINV-5: every semantic field is identity-bearing) — comparing all three
// would only re-report one underlying difference three ways. This checks
// identity (catches a capsule copied from a different task's tracker
// item), source digest (catches a capsule sealed against a frozen source
// that has since changed — "stale"), codec version, and then the origin
// bytes directly — the ground-truth content comparison, giving the most
// specific diagnostic for a changed assessment, profile, or policy field.
func VerifyCapsule(c Capsule, want AuthoritativeDemand) error {
	trusted, err := want.trustedCapsule()
	if err != nil {
		return err
	}
	if err := c.selfConsistent(); err != nil {
		return withAuthority(err, trusted)
	}
	switch {
	case c.Repository != trusted.Repository, c.Plan != trusted.Plan, c.Task != trusted.Task:
		return newCapsuleMismatch(BoundaryAuthority, "identity", CodeIdentityMismatch, "capsule identity does not match the task it was found on").withTrustedCapsule(trusted)
	case c.Source != trusted.Source:
		return newCapsuleMismatch(BoundaryAuthority, "source_digest", CodeStaleCapsule, "capsule source digest does not match the exact frozen task source").withTrustedCapsule(trusted)
	case c.Codec != trusted.Codec:
		return newCapsuleMismatch(BoundaryAuthority, "codec", CodeCodecMismatch, "capsule codec version does not match the exact frozen source's codec").withTrustedCapsule(trusted)
	}
	gotOrigin, err := EncodeCanonical(c.Origin)
	if err != nil {
		return newCapsuleMismatch(BoundaryAuthority, "origin", CodeInvalidOrigin, "capsule origin is invalid").withTrustedCapsule(trusted)
	}
	wantOrigin, err := EncodeCanonical(trusted.Origin)
	if err != nil {
		return newCapsuleMismatch(BoundaryAuthority, "origin", CodeInvalidOrigin, "authoritative origin is invalid").withTrustedCapsule(trusted)
	}
	if !bytes.Equal(gotOrigin, wantOrigin) {
		return newCapsuleMismatch(BoundaryAuthority, "origin", CodeFieldMismatch, "capsule origin does not match the demand reconstructed from the exact frozen source").withTrustedCapsule(trusted)
	}
	return nil
}
