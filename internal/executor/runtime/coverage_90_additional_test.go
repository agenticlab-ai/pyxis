package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type runtimeErrorReader struct{ err error }

func (r runtimeErrorReader) Read([]byte) (int, error) { return 0, r.err }

func activateCoverageFence(t *testing.T, authority *FenceAuthority, fence *AdapterFence, attempt string) port.LaunchCapability {
	t.Helper()
	capability, err := fence.Activate(attempt, 1, resolution(attempt, fence.harness))
	if err != nil {
		t.Fatal(err)
	}
	if authority.attempts[attempt] == nil {
		t.Fatal("activation did not create state")
	}
	return capability
}

func TestCoverage90FenceLaunchRefusals(t *testing.T) {
	authority, fence := newTestFence(t, "coverage")
	capability := activateCoverageFence(t, authority, fence, "nil-spawn")
	if _, err := fence.Launch(t.Context(), capability, nil); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("nil spawn error = %v", err)
	}

	capability = activateCoverageFence(t, authority, fence, "cancelled")
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := fence.Launch(cancelled, capability, func() (port.Handle, error) { return &fenceHandle{}, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled launch error = %v", err)
	}

	capability = activateCoverageFence(t, authority, fence, "spawn-error")
	spawnErr := errors.New("spawn refused")
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) { return nil, spawnErr }); !errors.Is(err, spawnErr) {
		t.Fatalf("spawn error = %v", err)
	}
	capability = activateCoverageFence(t, authority, fence, "nil-handle")
	if _, err := fence.Launch(t.Context(), capability, func() (port.Handle, error) { return nil, nil }); !errors.Is(err, ErrFenceState) {
		t.Fatalf("nil handle error = %v", err)
	}

	var nilFence *AdapterFence
	if _, _, err := nilFence.FenceAndObserve(t.Context(), "attempt", 1); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("nil observation error = %v", err)
	}
	if _, err := authority.activate("attempt", "coverage", 1, port.OwnershipProof{}); !errors.Is(err, ErrFenceDenied) {
		t.Fatalf("unbound activation error = %v", err)
	}
	badFirst := resolution("wrong-kind", "coverage")
	badFirst.Kind = port.OwnershipProofTransfer
	if _, err := authority.activate("wrong-kind", "coverage", 1, badFirst); !errors.Is(err, ErrFenceState) {
		t.Fatalf("wrong first proof error = %v", err)
	}
}

func TestCoverage90FenceRandomAndStateRefusals(t *testing.T) {
	want := errors.New("random unavailable")
	authority := &FenceAuthority{random: runtimeErrorReader{err: want}, attempts: map[string]*attemptFence{}}
	if _, err := authority.nonce(); !errors.Is(err, want) {
		t.Fatalf("nonce error = %v", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := authority.fenceAndObserve(cancelled, "missing", "coverage", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled observation error = %v", err)
	}
	var nilAuthority *FenceAuthority
	nilAuthority.markTerminal(nil)
}

func TestCoverage90ReviewCredentialWriteFailure(t *testing.T) {
	originalHome := reviewUserHomeDir
	reviewUserHomeDir = func() (string, error) { return "", errors.New("home unavailable") }
	t.Cleanup(func() { reviewUserHomeDir = originalHome })
	if err := PrepareReviewAuthentication(t.Context(), t.TempDir(), "codex"); err == nil {
		t.Fatal("home lookup failure was ignored")
	}
	reviewUserHomeDir = originalHome

	source := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(source, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalOpen := openReviewCredential
	openReviewCredential = func(string) (*os.File, error) { return nil, errors.New("open refused") }
	t.Cleanup(func() { openReviewCredential = originalOpen })
	if err := copyReviewCredential(source, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("credential open failure was ignored")
	}
	openReviewCredential = originalOpen

	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeReviewCredential(filepath.Join(parent, "credential"), []byte("secret")); err == nil {
		t.Fatal("credential write through a file parent succeeded")
	}
	loop := filepath.Join(t.TempDir(), "loop")
	if err := os.Symlink("loop", loop); err == nil {
		if err := copyReviewCredential(loop, filepath.Join(t.TempDir(), "copy")); err == nil {
			t.Fatal("credential symlink loop copied")
		}
	}
}

func TestCoverage90ReviewEnvironmentDefaultsAllLoginDirectories(t *testing.T) {
	env := LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Workdir: "/review"}, "CLAUDE_CONFIG_DIR", "XDG_DATA_HOME")
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{
		"\nCLAUDE_CONFIG_DIR=/review-runtime/model-auth/claude\n",
		"\nXDG_DATA_HOME=/review-runtime/model-auth/xdg-data\n",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("review environment lacks %q: %s", want, joined)
		}
	}
}

type launchFailureRuntime struct{ *stubRuntime }

func (*launchFailureRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return nil, errors.New("launch refused")
}

func TestCoverage90ConformanceStopsAfterLaunchFailure(t *testing.T) {
	err := CheckConformance(t.Context(), &launchFailureRuntime{stubRuntime: &stubRuntime{}}, conformingCase())
	if err == nil {
		t.Fatal("launch failure passed conformance")
	}
}

func TestCoverage90NativeCatalogIdentityMustBeIndependent(t *testing.T) {
	same := nativerouting.ValidatedCatalog{Identity: "same-policy"}
	if err := requireIndependentCatalogs(same, same); err == nil {
		t.Fatal("identical implementation and review policies were accepted")
	}
}
