package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type fenceHandle struct{ id int }

func (*fenceHandle) RuntimeHandle() {}

func newTestFence(t *testing.T, harness entity.Harness) (*FenceAuthority, *AdapterFence) {
	t.Helper()
	authority, err := NewFenceAuthority()
	if err != nil {
		t.Fatal(err)
	}
	fence, err := NewAdapterFence(authority, harness)
	if err != nil {
		t.Fatal(err)
	}
	return authority, fence
}

func TestDefaultFenceAuthorityIsShared(t *testing.T) {
	first, err := DefaultFenceAuthority()
	if err != nil {
		t.Fatal(err)
	}
	second, err := DefaultFenceAuthority()
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first != second {
		t.Fatalf("default authorities differ: %p != %p", first, second)
	}
}

func TestAdapterFenceRejectsMissingWiring(t *testing.T) {
	if _, err := NewAdapterFence(nil, "codex"); err == nil {
		t.Fatal("nil authority was accepted")
	}
	var fence *AdapterFence
	if _, err := fence.Activate("attempt", 1, port.OwnershipProof{}); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("nil fence activation = %v", err)
	}
}

func resolution(attempt string, harness entity.Harness) port.OwnershipProof {
	return port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "resolution:" + attempt,
		Digest: "digest:" + attempt, AttemptID: attempt, Harness: harness, OwnerGeneration: 1}
}

func transfer(attempt string, harness entity.Harness, generation port.OwnerGeneration, fence port.RuntimeFenceProof) port.OwnershipProof {
	return port.OwnershipProof{Kind: port.OwnershipProofTransfer, Reference: "transfer:" + attempt,
		Digest: "transfer-digest", AttemptID: attempt, Harness: harness, OwnerGeneration: generation, Fence: fence}
}

func TestSpawnBoundaryLinearizesAgainstConcurrentRevocation(t *testing.T) {
	_, fence := newTestFence(t, "codex")
	capability, err := fence.Activate("race", 1, resolution("race", "codex"))
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	launchDone := make(chan error, 1)
	go func() {
		_, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) {
			close(entered)
			<-release
			return &fenceHandle{id: 1}, nil
		})
		launchDone <- err
	}()
	<-entered
	type observed struct {
		proof port.RuntimeFenceProof
		view  port.RuntimeObservation
		err   error
	}
	observeDone := make(chan observed, 1)
	go func() {
		proof, view, err := fence.FenceAndObserve(t.Context(), "race", 1)
		observeDone <- observed{proof: proof, view: view, err: err}
	}()
	close(release)
	if err := <-launchDone; err != nil {
		t.Fatalf("spawn won boundary but failed: %v", err)
	}
	got := <-observeDone
	if got.err != nil || got.view.Kind != port.RuntimeRunning {
		t.Fatalf("fence after spawn = %+v, %v; want running", got.view, got.err)
	}
	if _, err := fence.Activate("race", 2, transfer("race", "codex", 2, got.proof)); err == nil {
		t.Fatal("running observation authorized takeover")
	}
}

func TestRevocationWinsBeforeSpawnAndProofReplaysAfterCoordinatorCrash(t *testing.T) {
	_, fence := newTestFence(t, "claude")
	capability, err := fence.Activate("revoke", 1, resolution("revoke", "claude"))
	if err != nil {
		t.Fatal(err)
	}
	proof, view, err := fence.FenceAndObserve(t.Context(), "revoke", 1)
	if err != nil || view.Kind != port.RuntimeNotStarted {
		t.Fatalf("fence = %+v, %v", view, err)
	}
	replayed, _, err := fence.FenceAndObserve(t.Context(), "revoke", 1)
	if err != nil || replayed != proof {
		t.Fatalf("crash recovery changed nonce-bound proof: %#v != %#v (%v)", replayed, proof, err)
	}
	var starts atomic.Int32
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) {
		starts.Add(1)
		return &fenceHandle{id: 1}, nil
	}); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("revoked launch error = %v, want ErrFenceDenied", err)
	}
	if starts.Load() != 0 {
		t.Fatalf("revoked capability crossed actual spawn %d times", starts.Load())
	}
	next, err := fence.Activate("revoke", 2, transfer("revoke", "claude", 2, proof))
	if err != nil {
		t.Fatalf("activate exact transferred generation: %v", err)
	}
	altered := proof
	altered.Nonce += "-altered"
	if _, err := fence.Activate("revoke", 3, transfer("revoke", "claude", 3, altered)); err == nil {
		t.Fatal("altered fence proof authorized a generation")
	}
	if _, err := fence.Launch(t.Context(), next, func() (port.Handle, error) {
		starts.Add(1)
		return &fenceHandle{id: 2}, nil
	}); err != nil {
		t.Fatalf("new current generation did not launch: %v", err)
	}
	if starts.Load() != 1 {
		t.Fatalf("model starts = %d, want exactly one", starts.Load())
	}
}

func TestHostLossUnknownNeverTransfersUntilAbsenceIsProven(t *testing.T) {
	authority, fence := newTestFence(t, "opencode")
	capability, err := fence.Activate("host-loss", 1, resolution("host-loss", "opencode"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) { return &fenceHandle{id: 1}, nil }); err != nil {
		t.Fatal(err)
	}
	if err := authority.ObserveHostLoss("host-loss", 1); err != nil {
		t.Fatal(err)
	}
	unknown, view, err := fence.FenceAndObserve(t.Context(), "host-loss", 1)
	if err != nil || view.Kind != port.RuntimeUnknown {
		t.Fatalf("host loss = %+v, %v; want unknown", view, err)
	}
	if _, err := fence.Activate("host-loss", 2, transfer("host-loss", "opencode", 2, unknown)); err == nil {
		t.Fatal("unknown liveness admitted a new owner")
	}
	if err := authority.ProveAbsent("host-loss", 1, "native-inventory:empty"); err != nil {
		t.Fatal(err)
	}
	absent, view, err := fence.FenceAndObserve(context.Background(), "host-loss", 1)
	if err != nil || view.Kind != port.RuntimeAbsentProven {
		t.Fatalf("positive absence = %+v, %v", view, err)
	}
	if _, err := fence.Activate("host-loss", 2, transfer("host-loss", "opencode", 2, absent)); err != nil {
		t.Fatalf("proven absence did not admit takeover: %v", err)
	}
}

func TestTerminalRecoveryTransfersAppendAuthorityButNeverRelaunches(t *testing.T) {
	_, fence := newTestFence(t, "cursor")
	capability, err := fence.Activate("terminal", 1, resolution("terminal", "cursor"))
	if err != nil {
		t.Fatal(err)
	}
	handle := &fenceHandle{id: 1}
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) { return handle, nil }); err != nil {
		t.Fatal(err)
	}
	fence.MarkTerminal(handle)
	proof, view, err := fence.FenceAndObserve(t.Context(), "terminal", 1)
	if err != nil || view.Kind != port.RuntimeTerminal {
		t.Fatalf("terminal observation = %+v, %v", view, err)
	}
	next, err := fence.Activate("terminal", 2, transfer("terminal", "cursor", 2, proof))
	if err != nil {
		t.Fatal(err)
	}
	var starts atomic.Int32
	if _, err := fence.Launch(t.Context(), next, func() (port.Handle, error) {
		starts.Add(1)
		return &fenceHandle{id: 2}, nil
	}); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("post-terminal launch error = %v, want denied", err)
	}
	if starts.Load() != 0 {
		t.Fatal("terminal recovery duplicated the model invocation")
	}
	if err := fence.AuthorizeTerminal(capability); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("old owner terminal authority = %v, want denied", err)
	}
	if err := fence.AuthorizeTerminal(next); err != nil {
		t.Fatalf("new owner cannot retry exact terminal append: %v", err)
	}
}

func TestOperatorReapOfUnknownCanTerminalButNeverReplacementLaunch(t *testing.T) {
	authority, fence := newTestFence(t, "claude")
	capability, err := fence.Activate("reap", 1, resolution("reap", "claude"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) { return &fenceHandle{id: 1}, nil }); err != nil {
		t.Fatal(err)
	}
	if err := authority.ObserveHostLoss("reap", 1); err != nil {
		t.Fatal(err)
	}
	if err := authority.ReapUnknown("reap", 1, ""); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("unsigned reap = %v, want denied", err)
	}
	if err := authority.ReapUnknown("reap", 1, "operator:incident-7"); err != nil {
		t.Fatal(err)
	}
	proof, view, err := fence.FenceAndObserve(t.Context(), "reap", 1)
	if err != nil || view.Kind != port.RuntimeTerminal {
		t.Fatalf("reaped observation = %+v, %v", view, err)
	}
	next, err := fence.Activate("reap", 2, transfer("reap", "claude", 2, proof))
	if err != nil {
		t.Fatal(err)
	}
	var starts atomic.Int32
	if _, err := fence.Launch(t.Context(), next, func() (port.Handle, error) {
		starts.Add(1)
		return &fenceHandle{id: 2}, nil
	}); !errors.Is(err, ErrFenceDenied) || starts.Load() != 0 {
		t.Fatalf("operator reap replacement launch = %v starts=%d", err, starts.Load())
	}
}
