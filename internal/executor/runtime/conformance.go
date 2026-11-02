package runtime

import (
	"context"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// ConformanceCase is the fixture a backend adapter is driven against by
// CheckConformance. The caller (each adapter's test) constructs the runtime with
// a hermetic (fake) exec wired to report WantResult, then names the fixture spec.
type ConformanceCase struct {
	// Spec is the fixture LaunchSpec driven through Launch. Its Model and Effort
	// are already resolved.
	Spec port.LaunchSpec

	// WantResult is the RunResult the runtime's exec is wired to report, so Await
	// can be checked to parse the CLI's usage JSON into exactly these counts.
	WantResult port.RunResult
}

// FencingCase drives the generation/capability contract against one native
// adapter while its hermetic commander records the real start seam.
type FencingCase struct {
	Spec    port.LaunchSpec
	Harness entity.Harness
}

// CheckFencingConformance proves the common adapter predicate: altered,
// replayed, revoked, stale-generation, and post-terminal capabilities cannot
// reach native spawn, while an exact transfer after proven non-start can.
func CheckFencingConformance(ctx context.Context, rt port.FencedRuntime, c FencingCase) error {
	if rt == nil || c.Harness == "" {
		return errors.New("runtime/conformance: fenced runtime and harness are required")
	}
	resolution := func(attempt string) port.OwnershipProof {
		return port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "resolution:" + attempt,
			Digest: "digest:" + attempt, AttemptID: attempt, Harness: c.Harness, OwnerGeneration: 1}
	}
	capability, err := rt.Activate("fence-running", 1, resolution("fence-running"))
	if err != nil {
		return fmt.Errorf("Activate running fixture: %w", err)
	}
	altered := capability
	altered.AttemptID = "altered"
	if _, err := rt.LaunchFenced(ctx, c.Spec, altered); err == nil {
		return errors.New("altered capability reached native spawn")
	}
	altered = capability
	altered.CapabilityRef += "-altered"
	if _, err := rt.LaunchFenced(ctx, c.Spec, altered); err == nil {
		return errors.New("altered capability reference reached native spawn")
	}
	handle, err := rt.LaunchFenced(ctx, c.Spec, capability)
	if err != nil {
		return fmt.Errorf("LaunchFenced: %w", err)
	}
	if _, err := rt.LaunchFenced(ctx, c.Spec, capability); err == nil {
		return errors.New("replayed capability reached native spawn")
	}
	_, observation, err := rt.FenceAndObserve(ctx, "fence-running", 1)
	if err != nil || observation.Kind != port.RuntimeRunning || observation.Handle != handle {
		return fmt.Errorf("running observation = %+v, %v", observation, err)
	}
	if err := rt.Kill(ctx, handle); err != nil {
		return fmt.Errorf("kill running fixture: %w", err)
	}

	capability, err = rt.Activate("fence-transfer", 1, resolution("fence-transfer"))
	if err != nil {
		return fmt.Errorf("Activate transfer fixture: %w", err)
	}
	fenceProof, observation, err := rt.FenceAndObserve(ctx, "fence-transfer", 1)
	if err != nil || observation.Kind != port.RuntimeNotStarted {
		return fmt.Errorf("not-started observation = %+v, %v", observation, err)
	}
	if _, err := rt.LaunchFenced(ctx, c.Spec, capability); err == nil {
		return errors.New("revoked capability reached native spawn")
	}
	transfer := port.OwnershipProof{Kind: port.OwnershipProofTransfer, Reference: "transfer:fence-transfer",
		Digest: "transfer-digest", AttemptID: "fence-transfer", Harness: c.Harness, OwnerGeneration: 2, Fence: fenceProof}
	next, err := rt.Activate("fence-transfer", 2, transfer)
	if err != nil {
		return fmt.Errorf("activate transferred generation: %w", err)
	}
	if err := rt.AuthorizeTerminal(capability); err == nil {
		return errors.New("stale owner retained terminal authority")
	}
	if _, err := rt.LaunchFenced(ctx, c.Spec, next); err != nil {
		return fmt.Errorf("transferred LaunchFenced: %w", err)
	}
	return nil
}

// CheckConformance drives one backend adapter through the whole
// port.AgentRuntime contract against a fixture task and returns every contract
// violation joined into a single error (nil when the adapter conforms). It is
// the shared suite T7–T9 reuse verbatim; it talks only to the port interface, so
// it is backend-agnostic, and it is hermetic — the caller builds rt with a fake
// exec, so no real CLI or network is touched (docs/tech-design/implement-now.md
// §3.7, §3.10).
//
// It exercises, in order:
//   - AuthCheck preflights clean on an available backend (FR-16).
//   - Launch returns a usable handle and Await reports the parsed usage
//     (WantResult), i.e. tokens/turns are read from the CLI's JSON.
//   - A second task launches on a fresh handle (no session reuse — DINV-11) and
//     Kill terminates a live handle cleanly.
func CheckConformance(ctx context.Context, rt port.AgentRuntime, c ConformanceCase) error {
	if rt == nil {
		return errors.New("runtime/conformance: nil AgentRuntime")
	}

	var errs []error

	// AuthCheck — an available, authenticated backend preflights clean.
	if err := rt.AuthCheck(ctx); err != nil {
		errs = append(errs, fmt.Errorf("AuthCheck on an available backend: %w", err))
	}

	// Launch → Await: a fresh process runs and its usage is reported. Completion
	// is the process exit; success is decided elsewhere by verification (DINV-3),
	// so a nil Await error with the parsed RunResult is the whole contract here.
	h1, err := rt.Launch(ctx, c.Spec)
	if err != nil {
		errs = append(errs, fmt.Errorf("Launch: %w", err))
		return errors.Join(errs...)
	}
	if h1 == nil {
		errs = append(errs, errors.New("Launch returned a nil handle"))
		return errors.Join(errs...)
	}
	if got, err := rt.Await(ctx, h1); err != nil {
		errs = append(errs, fmt.Errorf("Await: %w", err))
	} else if got != c.WantResult {
		errs = append(errs, fmt.Errorf("Await RunResult = %+v; want %+v (tokens/turns must parse from the CLI JSON)", got, c.WantResult))
	}

	// A second task launches on its own fresh handle — no session is reused
	// across tasks (DINV-11) — and Kill terminates a live handle without error.
	h2, err := rt.Launch(ctx, c.Spec)
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("second Launch (a fresh process per task, DINV-11): %w", err))
	case h2 == nil:
		errs = append(errs, errors.New("second Launch returned a nil handle"))
	default:
		if err := rt.Kill(ctx, h2); err != nil {
			errs = append(errs, fmt.Errorf("Kill a live handle: %w", err))
		}
	}

	return errors.Join(errs...)
}
