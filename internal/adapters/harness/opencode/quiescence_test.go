package opencode

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

func TestAwaitFailedKillWithholdsCheckpointPermission(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		authority, err := runtime.NewFenceAuthority()
		if err != nil {
			t.Fatal(err)
		}
		a := mustAdapter(t, WithFenceAuthority(authority))
		proof := port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "resolution:stop-test",
			Digest: "digest:stop-test", AttemptID: "stop-test", Harness: harnessID, OwnerGeneration: 1}
		capability, err := a.Activate("stop-test", 1, proof)
		if err != nil {
			t.Fatal(err)
		}
		proc := &fakeProcess{killErr: errors.New("signal denied")}
		h, err := a.fence.Launch(t.Context(), capability, func() (port.Handle, error) {
			return &handle{proc: proc}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		if cancelled {
			cancel()
		}
		_, err = a.Await(ctx, h)
		cancel()
		if !errors.Is(err, port.ErrNotQuiescent) {
			t.Fatalf("cancelled=%v: want process-tree failure, got %v", cancelled, err)
		}
		_, observed, err := a.FenceAndObserve(t.Context(), "stop-test", 1)
		if err != nil || observed.Kind != port.RuntimeRunning {
			t.Fatalf("failed teardown was recorded as terminal: observed=%+v err=%v", observed, err)
		}
	}
}
