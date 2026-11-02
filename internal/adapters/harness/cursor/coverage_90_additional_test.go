package cursor

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func TestCoverage90OptionDefaultsAndFenceFailure(t *testing.T) {
	if _, err := New(WithFenceAuthority(nil)); err == nil {
		t.Fatal("New must reject a nil fence authority")
	}

	a := mustAdapter(t, withGOOS(""))
	if a.goos == "" {
		t.Fatal("New must restore the host OS after an empty override")
	}
}

func TestCoverage90KillAndCaptureFailures(t *testing.T) {
	want := errors.New("kill failed")
	a := mustAdapter(t)

	if _, err := a.Await(context.Background(), &handle{proc: &fakeProcess{
		stdout:  []byte(resultJSON),
		killErr: want,
	}}); !errors.Is(err, want) {
		t.Fatalf("Await error = %v, want wrapped kill failure", err)
	}

	if err := a.Kill(context.Background(), &handle{proc: &fakeProcess{killErr: want}}); !errors.Is(err, want) {
		t.Fatalf("Kill error = %v, want wrapped kill failure", err)
	}

	if _, err := a.LaunchCapture(foreignHandle{}); err == nil {
		t.Fatal("LaunchCapture must reject a foreign handle")
	}
}

func TestCoverage90AuthCheckUsesDefaultProbe(t *testing.T) {
	t.Setenv("CURSOR_API_KEY", "")
	t.Setenv("CURSOR_AUTH_TOKEN", "")
	a := mustAdapter(t, withCommander(&fakeCommander{
		runStdout: []byte(`{"isAuthenticated":true}`),
	}))
	if err := a.AuthCheck(context.Background()); err != nil {
		t.Fatalf("AuthCheck with authenticated status: %v", err)
	}
}

func TestCoverage90UnknownNativeModel(t *testing.T) {
	a := mustAdapter(t)
	err := a.ValidateProfile(nativerouting.CatalogEntry{NativeModel: "unqualified-model"})
	if err == nil {
		t.Fatal("ValidateProfile must reject an unknown native model")
	}
}

func TestCoverage90CaptureNativeLaunchForBothRoles(t *testing.T) {
	a := mustAdapter(t)
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		capture, err := a.CaptureNativeLaunch(port.NativeLaunchSpec{
			Role:         role,
			NativeModel:  "cursor-grok-4.5-high",
			NativeEffort: "high",
		})
		if err != nil {
			t.Fatalf("CaptureNativeLaunch(%s): %v", role, err)
		}
		if capture.NativeModel != "cursor-grok-4.5-high" || capture.NativeEffort != "high" {
			t.Fatalf("CaptureNativeLaunch(%s) = %+v", role, capture)
		}
	}
}
