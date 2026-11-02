// Package adopt owns the adoption-plan/v1 artifact — the closed,
// schema-versioned record of what the `adopt` verb agreed to do to a
// non-empty repository and, once `apply` runs, what it did (design
// existing-repo-onboarding-v2 §3.2). It is the single durable statement of
// an adoption (§3.8 DINV-3): `adopt` writes it Draft; `apply` refuses a
// plan absent or stale relative to the tree, else applies it and rewrites
// it Applied with the epoch, the displaced prior hook path, and the
// added-path baselines undo's per-class "modified since adoption" test
// needs (§3.5, DINV-7); `undo` reads the Applied record. No second record
// of the same fact is kept anywhere else in the engine.
//
// This package owns exactly the artifact and its two faces — the closed
// machine face (Parse/Encode, unknown keys refused by name) and the
// deterministic human face (Plan.Render) the reviewer reads before
// approving apply and the operator reads from an applied receipt — never
// the logic that produces or consumes a plan's content. inspect, plan,
// apply, and undo are the adopt verb built on top of this artifact
// (prd:existing-repo-onboarding/FR-2 · FR-10;
// design:existing-repo-onboarding-v2#3.2, #DINV-3, #DINV-7).
package adopt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// SchemaVersion is the one adoption-plan schema this package speaks (design
// §3.2). A parsed plan declaring any other value is a named defect — the
// parse is closed, never a lenient guess at an unrecognized shape.
const SchemaVersion = "adoption-plan/v1"

// Status is the plan's lifecycle state (design §3.2, DINV-3): `adopt`
// writes a plan Draft; `apply` rewrites it Applied, stamping the epoch, the
// displaced hook path, and the added-path baselines. `undo` reads an
// Applied plan; it does not change Status — the plan is the record of what
// adoption agreed and did, not a live projection undo maintains.
type Status string

const (
	// StatusDraft is a plan `adopt` wrote that no `apply` has yet approved.
	StatusDraft Status = "draft"
	// StatusApplied is a plan `apply` has materialized: Epoch, Hooks, and
	// AddedPaths carry the resolved undo data.
	StatusApplied Status = "applied"
)

// Class is the resolution the plan assigns one path (design §3.2; refines
// prd:existing-repo-onboarding/FR-2's five-way classification into the six
// concrete resolutions the design settles on). ClassNeverTouch names a
// path's write-confinement — that adoption does not act on it — never a
// promise not to observe it (design §3.8 DINV-1).
type Class string

const (
	// ClassAdd is new content adoption writes that carries neither an
	// engine-verbatim manifest pin nor a generator (e.g. the repo-owned
	// config surface a non-empty tree does not yet have).
	ClassAdd Class = "add"
	// ClassChain is the hook-path collision: adoption chains its checks in
	// front of the hook path the repository already configures (design
	// §3.3).
	ClassChain Class = "chain"
	// ClassCollideManual is a collision with no honest mechanical
	// resolution: the engine writes nothing to the file, and Detail
	// carries the exact lines for the human to add (design §3.3, NG-2).
	ClassCollideManual Class = "collide-manual"
	// ClassNeverTouch names a path outside adoption's write confinement —
	// the release manifest's complement, the adopter's own content (design
	// §3.1, §3.8 DINV-1).
	ClassNeverTouch Class = "never-touch"
	// ClassGenerated is a path converge's generated-write primitive
	// renders (design §3.1).
	ClassGenerated Class = "generated"
	// ClassEngineVerbatim is a path converge's engine-verbatim write
	// primitive repairs byte-exact (design §3.1).
	ClassEngineVerbatim Class = "engine-verbatim"
)

// classes is the closed enum set Parse/Encode admit; anything else is a
// named defect.
var classes = map[Class]bool{
	ClassAdd:            true,
	ClassChain:          true,
	ClassCollideManual:  true,
	ClassNeverTouch:     true,
	ClassGenerated:      true,
	ClassEngineVerbatim: true,
}

// classNames renders the closed class enum sorted, for a defect message a
// reader can act on without consulting source.
func classNames() string {
	names := make([]string, 0, len(classes))
	for c := range classes {
		names = append(names, string(c))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Entry is one path the plan names, the resolution Class it assigns, and
// the resolution Detail — the exact lines to add for a ClassCollideManual
// entry (required, design §3.2), or free-text context for any other class
// (optional).
type Entry struct {
	Path   string `json:"path"`
	Class  Class  `json:"class"`
	Detail string `json:"detail"`
}

// Hooks carries the hook-path collision's undo baseline (design §3.3): the
// core.hooksPath adoption displaced, captured by `apply` before it
// overwrites the setting, or "" when the repository configured none.
// Empty at Draft — `apply` populates it exactly once, and idempotently: a
// re-apply that finds core.hooksPath already resolved to the engine's own
// directory skips the capture, so it never overwrites the recorded prior
// path with the engine's own directory.
type Hooks struct {
	PriorHooksPath string `json:"prior_hooks_path"`
}

// Stack is the detected-stack block (design §3.6): every stack the marker
// registry identified and the `executor.gate.<command>` action each
// requires the adopter to supply. Detected empty means no marker matched —
// the degrade-to-unknown case (prd:existing-repo-onboarding/UB-3) — never
// an error and never a fabricated command.
type Stack struct {
	Detected    []string `json:"detected"`
	GateActions []string `json:"gate_actions"`
}

// AddedPath is one path adoption added, with the content hash `apply`
// captured — the baseline undo's per-class "modified since adoption" test
// compares an `add` path's current hash against (design §3.5, DINV-7 as
// reconciled 2026-07-26; a `chain` is git-config-only and reversed via
// hooks.prior_hooks_path, so it carries no AddedPath baseline).
// Empty at Draft; populated entry-for-entry when `apply` stamps the plan.
type AddedPath struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Plan is adoption-plan/v1 — the one durable, closed-schema record an
// adoption agreed to and, once applied, did (design §3.2, DINV-3). `adopt`
// writes it Draft; `apply` refuses a plan absent or stale relative to the
// tree, else applies it and rewrites it Applied with Epoch, Hooks, and
// AddedPaths populated; `undo` reads an Applied plan. Evolution is
// additive only (AP-5/AP-6): a later schema version adds a field, never
// repurposes one.
type Plan struct {
	SchemaVersion string      `json:"schema_version"`
	Status        Status      `json:"status"`
	Epoch         string      `json:"epoch"`
	Entries       []Entry     `json:"entries"`
	Hooks         Hooks       `json:"hooks"`
	Stack         Stack       `json:"stack"`
	AddedPaths    []AddedPath `json:"added_paths"`
}

// Parse parses adoption-plan/v1 bytes CLOSED: an unknown key at any
// nesting level, trailing content after the object, or a value
// inconsistent with the schema's invariants (an unrecognized status or
// class, a malformed sha256, a lifecycle-inconsistent field) is a named
// defect (design §3.2).
func Parse(b []byte) (Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("adoption-plan: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return Plan{}, errors.New("adoption-plan: trailing content after the plan object")
	}
	if err := p.validate(); err != nil {
		return Plan{}, fmt.Errorf("adoption-plan: %w", err)
	}
	return p, nil
}

// Encode renders the plan's one canonical byte form — two-space-indented
// JSON, struct-ordered fields, one trailing newline — so a plan is
// byte-stable across runs and a parse→encode round trip reproduces its
// input exactly. An invalid plan refuses to encode (fail closed).
func (p Plan) Encode() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("adoption-plan: encode: %w", err)
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("adoption-plan: encode: %w", err)
	}
	return append(b, '\n'), nil
}

// validate enforces the invariants Parse and Encode share, so a plan that
// round-trips through either always satisfies the same contract; every
// defect names its field.
func (p Plan) validate() error {
	var errs []error

	if p.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version %q: this parser speaks exactly %q (closed parse)", p.SchemaVersion, SchemaVersion))
	}
	switch p.Status {
	case StatusDraft, StatusApplied:
	default:
		errs = append(errs, fmt.Errorf("status %q: want %q or %q", p.Status, StatusDraft, StatusApplied))
	}

	seenEntry := make(map[string]bool, len(p.Entries))
	for _, e := range p.Entries {
		if e.Path == "" {
			errs = append(errs, errors.New("entry with an empty path"))
			continue
		}
		if seenEntry[e.Path] {
			errs = append(errs, fmt.Errorf("duplicate entry path %q", e.Path))
		}
		seenEntry[e.Path] = true
		if reason, ok := cleanRelPath(e.Path); !ok {
			errs = append(errs, fmt.Errorf("entry path %q %s", e.Path, reason))
		}
		if !classes[e.Class] {
			errs = append(errs, fmt.Errorf("entry %q: unknown class %q (want one of %s)", e.Path, e.Class, classNames()))
		}
		if e.Class == ClassCollideManual && e.Detail == "" {
			errs = append(errs, fmt.Errorf("entry %q: class %s requires a non-empty detail (the exact lines for the human to add)", e.Path, ClassCollideManual))
		}
	}

	seenAdded := make(map[string]bool, len(p.AddedPaths))
	for _, a := range p.AddedPaths {
		if a.Path == "" {
			errs = append(errs, errors.New("added_paths entry with an empty path"))
			continue
		}
		if seenAdded[a.Path] {
			errs = append(errs, fmt.Errorf("duplicate added_paths path %q", a.Path))
		}
		seenAdded[a.Path] = true
		if reason, ok := cleanRelPath(a.Path); !ok {
			errs = append(errs, fmt.Errorf("added_paths path %q %s", a.Path, reason))
		}
		if err := checkHex(fmt.Sprintf("added_paths[%q].sha256", a.Path), a.SHA256); err != nil {
			errs = append(errs, err)
		}
	}

	// The fields the design names "set at apply" (§3.2) hold their zero
	// value until apply stamps them: a draft plan carrying a pre-populated
	// epoch, prior hook path, or added-path baseline did not come from
	// `adopt` alone, and an applied plan with no epoch never actually ran
	// apply's epoch recording (design §3.2, §3.4, DINV-4).
	switch p.Status {
	case StatusDraft:
		if p.Epoch != "" {
			errs = append(errs, errors.New("status draft carries a non-empty epoch — epoch is set at apply, never before (design §3.2, §3.4)"))
		}
		if p.Hooks.PriorHooksPath != "" {
			errs = append(errs, errors.New("status draft carries a non-empty hooks.prior_hooks_path — captured at apply, never before (design §3.3)"))
		}
		if len(p.AddedPaths) != 0 {
			errs = append(errs, errors.New("status draft carries added_paths entries — the baseline is stamped at apply, never before (design §3.2, DINV-7)"))
		}
	case StatusApplied:
		if p.Epoch == "" {
			errs = append(errs, errors.New("status applied carries no epoch — apply always records the repository's commit (design §3.4, DINV-4)"))
		}
		errs = append(errs, checkAddedPathsMatchEntries(p)...)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// checkAddedPathsMatchEntries enforces DINV-7's baseline split once a plan is
// Applied. An engine-verbatim path is checked against its manifest SHA and a
// generated path by regenerate-and-compare, so those carry no stored baseline;
// only an `add` entry — which pins neither a manifest entry nor a generator —
// carries a baseline apply stamps into AddedPaths, and undo deletes precisely
// that set.
//
// A `chain` entry carries NO baseline: DINV-7 originally paired `add`/`chain`
// (a "chained-hook wrapper" as its own added_paths example), but the built
// hook chain (T5) is git-config-only — it repoints core.hooksPath and
// forward-delegates through the engine shim, writing no wrapper file — so a
// chain entry has no file to hash. undo reverses it via hooks.prior_hooks_path,
// not by deleting a path (operator ruling 2026-07-26 reconciling DINV-7 to the
// built mechanism; the chain entry is disclosure only). The relation is exact
// both ways over the `add` class: every add entry has a stamped baseline, and
// every stamped baseline belongs to an add entry.
func checkAddedPathsMatchEntries(p Plan) []error {
	needsBaseline := make(map[string]bool, len(p.Entries))
	for _, e := range p.Entries {
		if e.Class == ClassAdd {
			needsBaseline[e.Path] = true
		}
	}
	stamped := make(map[string]bool, len(p.AddedPaths))
	for _, a := range p.AddedPaths {
		stamped[a.Path] = true
	}

	var errs []error
	for _, e := range p.Entries {
		if e.Class == ClassAdd && !stamped[e.Path] {
			errs = append(errs, fmt.Errorf("entry %q: class %s requires an added_paths baseline once applied, none stamped (design §3.5, DINV-7 as reconciled)", e.Path, e.Class))
		}
	}
	for _, a := range p.AddedPaths {
		if !needsBaseline[a.Path] {
			errs = append(errs, fmt.Errorf("added_paths %q: no add entry claims this path (design §3.5, DINV-7 as reconciled)", a.Path))
		}
	}
	return errs
}

// cleanRelPath reports whether v is a clean, slash-separated, repository-
// relative path — never absolute, never escaping with ".." — the same
// containment discipline the repo config surface enforces
// (config-driven-portability-v1 §3.3): every plan path is a location the
// engine joins under the repository root.
func cleanRelPath(v string) (reason string, ok bool) {
	if strings.HasPrefix(v, "/") {
		return "is absolute (escapes the repository root)", false
	}
	if v == ".." || strings.HasPrefix(v, "../") {
		return "escapes the repository root with a parent-directory reference", false
	}
	if path.Clean(v) != v {
		return "is not a clean, relative, slash-separated path", false
	}
	return "", true
}

// checkHex demands a 64-char lowercase sha256 hex string — the AddedPath
// baseline format (design §3.2, DINV-7).
func checkHex(field, v string) error {
	if len(v) != 64 {
		return fmt.Errorf("%s: %q is not a 64-char lowercase sha256 hex", field, v)
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("%s: %q is not a 64-char lowercase sha256 hex", field, v)
		}
	}
	return nil
}
