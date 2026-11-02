// Package persist is the impl-review persistence PORT — the consumer-side
// interface the reviewer writes a review through, plus the outcome types its
// contract needs (docs/tech-design/impl-review-v1.md §3.2, §3.10; DINV-9, UB-5).
//
// A review is persisted as ONE self-contained artifact on the proposal's own
// medium — the single source of truth, never mirrored to another medium and
// never split across containers (DINV-9). The port has two methods:
//
//	Persist(payload) → ref   — write the review, return an opaque medium ref
//	Read(ref)        → payload — recover the byte-identical payload from that ref
//
// The port is defined here on the CONSUMER side (the reviewer declares what it
// needs); each code-host adapter implements it in its own package — GitHub
// first (persist/github), Carina later — and every adapter passes the same
// conformance suite (conformance.go) so a proposal reviewed on either medium
// harvests identically (the port conformance idea, A-3, R1).
//
// The human rendering is ALWAYS report.Render(payload), never authored beside
// the payload (DINV-2, render-from-payload); on read the machine block is
// authoritative. Verbosity is a source defect, kept compact by prompt
// discipline (§3.5) so the artifact stays whole — an adapter NEVER splits a
// payload to fit a medium.
//
// Stdlib only, per the repo Go rules.
package persist

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// GitHubProposalRef constructs the canonical proposal-medium identity used by
// impl-review payloads and harvest joins. Keeping the grammar here prevents
// launchers, verifiers, and tests from hand-rolling subtly different prefixes.
func GitHubProposalRef(repo string, proposal int) (string, error) {
	repo = strings.TrimSpace(repo)
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || strings.TrimSpace(owner) == "" || strings.TrimSpace(name) == "" || strings.Contains(name, "/") || proposal <= 0 {
		return "", fmt.Errorf("impl-review persist: invalid GitHub proposal identity repo=%q proposal=%d", repo, proposal)
	}
	return fmt.Sprintf("%s:%s#%d", MediumGitHub, repo, proposal), nil
}

// ProposalRef accepts legacy GitHub coordinates or a canonical Forgejo
// repository key, including its server origin and optional installation path.
func ProposalRef(repository string, proposal int) (string, error) {
	if !strings.HasPrefix(repository, "forgejo:") {
		return GitHubProposalRef(repository, proposal)
	}
	value := strings.TrimPrefix(repository, "forgejo:")
	index := strings.LastIndexByte(value, ':')
	if index < 0 {
		return "", errors.New("impl-review persist: invalid Forgejo repository identity")
	}
	origin, repo := value[:index], value[index+1:]
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("impl-review persist: invalid Forgejo origin")
	}
	if _, err := GitHubProposalRef(repo, proposal); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s#%d", repository, proposal), nil
}

// Medium names the code-host an adapter persists on — the one medium a given
// proposal lives on (DINV-9: never a second medium for the same review).
type Medium string

const (
	// MediumGitHub is the GitHub adapter's medium (the v1 path — the dogfood
	// proposals are GitHub PRs).
	MediumGitHub  Medium = "github"
	MediumForgejo Medium = "forgejo"
	// MediumCarina is the Carina adapter's medium (a later phase, §3.2).
	MediumCarina Medium = "carina"
)

// Ref is an OPAQUE reference to a persisted review on its medium — what Persist
// returns and Read consumes. It carries the Medium (so a ref is never applied
// to the wrong adapter) and a medium-specific Locator the adapter interprets
// (for GitHub, the bot comment id). A consumer treats Locator as opaque; only
// the issuing adapter parses it.
type Ref struct {
	// Medium is the code-host this ref addresses.
	Medium Medium
	// Locator is the medium-specific address of the one artifact (opaque to
	// consumers; the GitHub adapter stores its comment id here).
	Locator string
}

// String renders a Ref for logs/errors — never parsed back, purely diagnostic.
func (r Ref) String() string { return fmt.Sprintf("%s:%s", r.Medium, r.Locator) }

// Reader is the harvest-side persistence port. Publication ownership moved to
// implreview.HostPersistenceAdapter; harvest needs only exact artifact reads and
// cannot acquire a proposal-write capability through this seam.
type Reader interface {
	Read(ctx context.Context, ref Ref) (*report.Report, error)
}

// LegacyPort is retained only for the frozen impl-review-v1 conformance and
// lifecycle fixtures. Production publication is exclusively performed by the
// authenticated host adapter and must never be wired through this interface.
//
// Persist writes the payload as ONE artifact — the human rendering
// (report.Render) plus the machine payload (report.Marshal) — and returns its
// Ref. On a write failure it retries to a bounded cap and then records a
// review-absence via the medium's alternate signal, returning an
// *AbsenceRecordedError (never a silent loss, never an unbounded wait — UB-5).
//
// Read recovers the byte-identical *report.Report from the artifact the Ref
// addresses (report.Unmarshal of the machine block, which is authoritative).
type LegacyPort interface {
	Persist(ctx context.Context, r *report.Report) (Ref, error)
	Reader
}

// ErrRecordedAbsence is the legacy lifecycle sentinel reused by RecordAbsence.
// The authenticated host publisher reports typed publication terminals instead
// of retrying through LegacyPort.
var ErrRecordedAbsence = errors.New("impl-review persist: write failed past retry — review-absence recorded")

// Verification sentinels let the executor distinguish absence/malformed
// evidence from evidence that exists but fails reviewer attribution.
var (
	ErrArtifactUnverified = errors.New("impl-review persist: artifact unverified")
	ErrArtifactUntrusted  = errors.New("impl-review persist: artifact untrusted")
)

// AbsenceRecordedError is the UB-5 outcome: a write that failed past the bounded
// retry cap, whose absence was recorded on the proposal via an alternate signal
// rather than lost or hung. It carries the Ref of the alternate signal (the
// absence marker the adapter wrote so the absence is visible and later
// harvestable) and the underlying write Cause. It IS an error — a Persist
// returning it did not persist the review — but a distinguishable one: the
// absence is recorded, not silently dropped.
type AbsenceRecordedError struct {
	// Proposal is the proposal reference the absence was recorded against.
	Proposal string
	// Attempts is how many write attempts were made before the cap tripped.
	Attempts int
	// AlternateSignal is the Ref of the recorded absence marker (the alternate
	// signal), or the zero Ref when even the alternate signal could not be
	// written (still never a silent loss: Cause is returned and Recorded is
	// false).
	AlternateSignal Ref
	// Recorded is true when the alternate absence signal was written; false when
	// the fallback itself failed (Cause then carries both failures).
	Recorded bool
	// Cause is the underlying write failure that exhausted the retry cap.
	Cause error
}

// Error renders the recorded-absence outcome.
func (e *AbsenceRecordedError) Error() string {
	if e.Recorded {
		return fmt.Sprintf("impl-review persist: write to %s failed after %d attempts; "+
			"review-absence recorded via alternate signal %s: %v",
			e.Proposal, e.Attempts, e.AlternateSignal, e.Cause)
	}
	return fmt.Sprintf("impl-review persist: write to %s failed after %d attempts and the "+
		"alternate absence signal could not be written: %v", e.Proposal, e.Attempts, e.Cause)
}

// Is reports that an *AbsenceRecordedError matches the ErrRecordedAbsence
// sentinel, so a caller can errors.Is(err, ErrRecordedAbsence) without knowing
// the concrete type.
func (e *AbsenceRecordedError) Is(target error) bool { return target == ErrRecordedAbsence }

// Unwrap exposes the underlying write cause for errors.Is/As chaining.
func (e *AbsenceRecordedError) Unwrap() error { return e.Cause }
