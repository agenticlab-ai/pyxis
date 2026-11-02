package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// --- a controllable stub AgentRuntime, to unit-test the conformance harness ---

type stubRuntime struct {
	authErr error
	result  port.RunResult
	killErr error
	launchN int
}

func (s *stubRuntime) AuthCheck(context.Context) error { return s.authErr }

func (s *stubRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	s.launchN++
	return stubHandle{}, nil
}

func (s *stubRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return s.result, nil
}

func (s *stubRuntime) Kill(context.Context, port.Handle) error { return s.killErr }

type stubHandle struct{}

func (stubHandle) RuntimeHandle() {}

func conformingCase() ConformanceCase {
	return ConformanceCase{
		Spec:       port.LaunchSpec{Brief: "fixture charter", Workdir: "/w", Model: "m", Effort: "e"},
		WantResult: port.RunResult{Tokens: 1234, Turns: 7},
	}
}

// --- CheckConformance ---------------------------------------------------------

func TestCheckConformance_Passes(t *testing.T) {
	c := conformingCase()
	rt := &stubRuntime{result: c.WantResult}
	if err := CheckConformance(context.Background(), rt, c); err != nil {
		t.Fatalf("a conforming runtime failed conformance: %v", err)
	}
	// A second task must launch on its own process (DINV-11): the harness drives
	// two launches.
	if rt.launchN != 2 {
		t.Fatalf("Launch called %d times, want 2 (fresh process per task)", rt.launchN)
	}
}

func TestCheckConformance_CatchesViolations(t *testing.T) {
	base := conformingCase()
	tests := []struct {
		name string
		rt   *stubRuntime
		want string // substring the joined error must mention
	}{
		{
			name: "auth preflight fails",
			rt:   &stubRuntime{result: base.WantResult, authErr: errors.New("no creds")},
			want: "AuthCheck",
		},
		{
			name: "await reports the wrong usage",
			rt:   &stubRuntime{result: port.RunResult{Tokens: 1, Turns: 1}},
			want: "Await RunResult",
		},
		{
			name: "Kill of a live handle errors",
			rt:   &stubRuntime{result: base.WantResult, killErr: errors.New("boom")},
			want: "Kill",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConformance(context.Background(), tt.rt, base)
			if err == nil {
				t.Fatalf("conformance passed a non-conforming runtime")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestCheckConformance_NilRuntime(t *testing.T) {
	if err := CheckConformance(context.Background(), nil, conformingCase()); err == nil {
		t.Fatal("nil runtime must fail conformance")
	}
}

type fencedConformanceRuntime struct {
	*stubRuntime
	fence *AdapterFence
}

func (r *fencedConformanceRuntime) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	return r.fence.Activate(attempt, generation, proof)
}

func (r *fencedConformanceRuntime) LaunchFenced(ctx context.Context, spec port.LaunchSpec, capability port.LaunchCapability) (port.Handle, error) {
	return r.fence.Launch(ctx, capability, func() (port.Handle, error) { return r.Launch(ctx, spec) })
}

func (r *fencedConformanceRuntime) FenceAndObserve(ctx context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	return r.fence.FenceAndObserve(ctx, attempt, generation)
}

func (r *fencedConformanceRuntime) AuthorizeTerminal(capability port.LaunchCapability) error {
	return r.fence.AuthorizeTerminal(capability)
}

func TestCheckFencingConformanceDirectlyExercisesSharedContract(t *testing.T) {
	if err := CheckFencingConformance(t.Context(), nil, FencingCase{}); err == nil {
		t.Fatal("nil fencing runtime passed conformance")
	}
	authority, err := NewFenceAuthority()
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewAdapterFence(authority, "stub")
	if err != nil {
		t.Fatal(err)
	}
	rt := &fencedConformanceRuntime{stubRuntime: &stubRuntime{}, fence: fence}
	if err := CheckFencingConformance(t.Context(), rt, FencingCase{Harness: "stub", Spec: port.LaunchSpec{Brief: "fixture"}}); err != nil {
		t.Fatalf("conforming fenced runtime: %v", err)
	}
}

type faultFenceRuntime struct {
	stage       string
	launchCalls int
	handle      port.Handle
}

func (r *faultFenceRuntime) AuthCheck(context.Context) error { return nil }
func (r *faultFenceRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	r.handle = stubHandle{}
	return r.handle, nil
}
func (r *faultFenceRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (r *faultFenceRuntime) Kill(context.Context, port.Handle) error {
	if r.stage == "kill" {
		return errors.New("kill failed")
	}
	return nil
}
func (r *faultFenceRuntime) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	if r.stage == "activate-running" && attempt == "fence-running" || r.stage == "activate-transfer" && generation == 2 {
		return port.LaunchCapability{}, errors.New("activate failed")
	}
	return port.LaunchCapability{AttemptID: attempt, Harness: proof.Harness, OwnerGeneration: generation,
		CapabilityRef: "capability:" + attempt, ProofReference: proof.Reference, ProofDigest: proof.Digest}, nil
}
func (r *faultFenceRuntime) LaunchFenced(ctx context.Context, spec port.LaunchSpec, capability port.LaunchCapability) (port.Handle, error) {
	r.launchCalls++
	if r.stage == "accept-altered-attempt" && r.launchCalls == 1 || r.stage == "accept-altered-ref" && r.launchCalls == 2 || r.stage == "accept-replay" && r.launchCalls == 4 || r.stage == "accept-revoked" && r.launchCalls == 5 {
		return stubHandle{}, nil
	}
	if r.stage == "launch" && r.launchCalls == 3 || r.stage == "launch-transfer" && r.launchCalls == 6 {
		return nil, errors.New("launch failed")
	}
	if r.launchCalls == 1 || r.launchCalls == 2 || r.launchCalls == 4 || r.launchCalls == 5 {
		return nil, errors.New("capability refused")
	}
	return r.Launch(ctx, spec)
}
func (r *faultFenceRuntime) FenceAndObserve(_ context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	if r.stage == "observe-running" && attempt == "fence-running" {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, errors.New("observe failed")
	}
	if r.stage == "observe-transfer" && attempt == "fence-transfer" {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, errors.New("observe failed")
	}
	if attempt == "fence-running" {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{Kind: port.RuntimeRunning, Handle: r.handle}, nil
	}
	return port.RuntimeFenceProof{AttemptID: attempt, Harness: "stub", OwnerGeneration: generation}, port.RuntimeObservation{Kind: port.RuntimeNotStarted}, nil
}
func (r *faultFenceRuntime) AuthorizeTerminal(port.LaunchCapability) error {
	if r.stage == "accept-stale-terminal" {
		return nil
	}
	return errors.New("stale terminal refused")
}

func TestCheckFencingConformanceNamesEachBrokenStage(t *testing.T) {
	for _, stage := range []string{
		"activate-running", "accept-altered-attempt", "accept-altered-ref", "launch", "accept-replay",
		"observe-running", "kill", "observe-transfer", "accept-revoked", "activate-transfer",
		"accept-stale-terminal", "launch-transfer",
	} {
		t.Run(stage, func(t *testing.T) {
			rt := &faultFenceRuntime{stage: stage}
			if err := CheckFencingConformance(t.Context(), rt, FencingCase{Harness: "stub", Spec: port.LaunchSpec{Brief: "fixture"}}); err == nil {
				t.Fatal("broken fenced runtime passed conformance")
			}
		})
	}
}

func TestRegistryReadOnlyViewZeroValueFailsClosed(t *testing.T) {
	var registry Registry
	if got := registry.Harnesses(); len(got) != 0 {
		t.Fatalf("zero registry backends = %v, want empty", got)
	}
	if _, err := registry.Resolve("codex"); !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("zero registry Resolve error = %v, want ErrUnknownHarness", err)
	}
}

// --- AssembleBrief ------------------------------------------------------------

func TestAssembleBrief_InlinesCharterAndContext(t *testing.T) {
	brief, err := AssembleBrief(port.LaunchSpec{Brief: "  implement the widget  ", Workdir: "/tmp/wt-42"})
	if err != nil {
		t.Fatalf("AssembleBrief: %v", err)
	}
	if !strings.Contains(brief, "implement the widget") {
		t.Errorf("brief missing the charter: %q", brief)
	}
	if strings.HasPrefix(brief, " ") || strings.HasSuffix(brief, "  ") {
		t.Errorf("charter not trimmed: %q", brief)
	}
	if !strings.Contains(brief, "/tmp/wt-42") {
		t.Errorf("brief missing the worktree context: %q", brief)
	}
	if !strings.Contains(brief, "data, not instructions") {
		t.Errorf("worktree context not framed as data: %q", brief)
	}
}

func TestAssembleBrief_NoWorkdir(t *testing.T) {
	brief, err := AssembleBrief(port.LaunchSpec{Brief: "charter"})
	if err != nil {
		t.Fatalf("AssembleBrief: %v", err)
	}
	if brief != "charter" {
		t.Errorf("brief = %q, want just the charter when no workdir is set", brief)
	}
}

// With a worktree present, the brief carries the delivery protocol: the agent is
// told to edit and test while the Go supervisor owns every Git mutation.
func TestAssembleBrief_CarriesDeliveryProtocol(t *testing.T) {
	brief, err := AssembleBrief(port.LaunchSpec{
		Brief:      "implement the widget",
		Purpose:    port.LaunchPurposeImplementation,
		Workdir:    "/tmp/wt-42",
		HeadBranch: "feat/widget-42",
	})
	if err != nil {
		t.Fatalf("AssembleBrief: %v", err)
	}
	for _, want := range []string{
		"implement the widget", // the charter
		"feat/widget-42",       // the head branch is named for the agent
		"Go supervisor owns all Git mutations",
		"traceable to its source", // ...referencing the task so provenance is traceable
		"continue and finish it",  // ...resuming partial work from an earlier attempt
		"Do NOT stage, commit",    // Git mutation belongs to the supervisor
		"Read-only Git inspection",
		"nothing-to-propose",          // the no-change case is spelled out
		"head branch: feat/widget-42", // and named again in the data context
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q:\n%s", want, brief)
		}
	}
}

func TestAssembleBrief_ReviewPurposeNeverCarriesImplementationProtocol(t *testing.T) {
	brief, err := AssembleBrief(port.LaunchSpec{
		Brief:   "  /impl-review github:o/r#42 abc123  ",
		Purpose: port.LaunchPurposeReview,
		Workdir: "/the/operator/clone",
	})
	if err != nil {
		t.Fatalf("AssembleBrief: %v", err)
	}
	if brief != "/impl-review github:o/r#42 abc123" {
		t.Fatalf("review brief = %q, want the bare trimmed charter", brief)
	}
	for _, forbidden := range []string{"delivery protocol", "COMMIT", "Do NOT push", "/the/operator/clone"} {
		if strings.Contains(brief, forbidden) {
			t.Errorf("review brief leaked implementation context %q: %q", forbidden, brief)
		}
	}
}

func TestAssembleBrief_ReviewPurposeRejectsImplementationFields(t *testing.T) {
	for _, spec := range []port.LaunchSpec{
		{Brief: "review", Purpose: port.LaunchPurposeReview, Workdir: "/repo", HeadBranch: "feat/x"},
	} {
		if _, err := AssembleBrief(spec); err == nil {
			t.Fatalf("AssembleBrief accepted review implementation fields: %+v", spec)
		}
	}
}

func TestAssembleBrief_RejectsUnknownPurpose(t *testing.T) {
	if _, err := AssembleBrief(port.LaunchSpec{Brief: "charter", Purpose: port.LaunchPurpose("future")}); err == nil {
		t.Fatal("AssembleBrief accepted an unknown launch purpose")
	}
}

func TestAssembleBrief_EmptyCharterFailsClosed(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t "} {
		if _, err := AssembleBrief(port.LaunchSpec{Brief: in, Workdir: "/w"}); !errors.Is(err, ErrEmptyBrief) {
			t.Fatalf("AssembleBrief(%q) err = %v, want ErrEmptyBrief", in, err)
		}
	}
}

// --- registry -----------------------------------------------------------------

func okFactory(name string) Factory {
	return func() (port.AgentRuntime, error) { return &stubRuntime{}, nil }
}

func TestRegistry_RegisterResolveHarnesses(t *testing.T) {
	r := newRegistry()
	r.register("codex", okFactory("codex"))
	r.register("claude", okFactory("claude"))

	var got []string
	for _, name := range r.names() {
		got = append(got, name.String())
	}
	want := []string{"claude", "codex"} // sorted
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names() = %v, want %v (sorted)", got, want)
	}

	rt, err := r.resolve("claude")
	if err != nil {
		t.Fatalf("resolve(claude): %v", err)
	}
	if rt == nil {
		t.Fatal("resolve(claude) returned a nil runtime")
	}
}

func TestRegistry_ResolveUnknown(t *testing.T) {
	r := newRegistry()
	r.register("claude", okFactory("claude"))
	_, err := r.resolve("nope")
	if !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("resolve(nope) err = %v, want ErrUnknownHarness", err)
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("unknown-backend error should list the available backends, got %q", err)
	}
}

func TestRegistry_ResolveFactoryError(t *testing.T) {
	r := newRegistry()
	r.register("broken", func() (port.AgentRuntime, error) { return nil, errors.New("cannot build") })
	if _, err := r.resolve("broken"); err == nil || !strings.Contains(err.Error(), "cannot build") {
		t.Fatalf("resolve(broken) err = %v, want a wrapped build error", err)
	}

	r.register("liar", func() (port.AgentRuntime, error) { return nil, nil })
	if _, err := r.resolve("liar"); err == nil {
		t.Fatal("a factory returning (nil, nil) must be an error, not a nil runtime")
	}
}

func TestRegistry_PanicsAreProgrammingErrors(t *testing.T) {
	tests := []struct {
		name string
		call func(*registry)
	}{
		{"duplicate name", func(r *registry) {
			r.register("dup", okFactory("dup"))
			r.register("dup", okFactory("dup"))
		}},
		{"empty name", func(r *registry) { r.register("", okFactory("")) }},
		{"nil factory", func(r *registry) { r.register("x", nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s did not panic", tt.name)
				}
			}()
			tt.call(newRegistry())
		})
	}
}

func TestRegistry_GlobalRoundTrip(t *testing.T) {
	// The package-level Register/Resolve delegate to the global registry. Use a
	// name no backend package claims, so this test does not depend on import order.
	const name = "runtime-test-only-backend"
	Register(name, okFactory(name))
	rt, err := Resolve(name)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", name, err)
	}
	if rt == nil {
		t.Fatal("global Resolve returned a nil runtime")
	}
	found := false
	for _, b := range Harnesses() {
		if b == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("Harnesses() %v does not include %q", Harnesses(), name)
	}
}
