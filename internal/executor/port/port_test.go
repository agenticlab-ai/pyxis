package port_test

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCheckStateString(t *testing.T) {
	tests := []struct {
		state port.CheckState
		want  string
	}{
		{port.CheckPending, "pending"},
		{port.CheckGreen, "green"},
		{port.CheckRed, "red"},
		{port.CheckState(99), "CheckState(99)"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CheckState(%d).String() = %q, want %q", int(tt.state), got, tt.want)
		}
	}
}

// CheckPending is the zero value so an unset check reads as not-green
// (fail-closed): only CheckGreen may admit a succeeded outcome (DINV-3).
func TestCheckPendingIsZeroValue(t *testing.T) {
	var zero port.CheckState
	if zero != port.CheckPending {
		t.Fatalf("zero CheckState = %v, want pending (fail-closed)", zero)
	}
	if zero == port.CheckGreen {
		t.Fatal("the zero CheckState must never be green")
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	if port.ErrClaimLost == nil || port.ErrNoProposal == nil {
		t.Fatal("sentinels must be non-nil")
	}
	if errors.Is(port.ErrClaimLost, port.ErrNoProposal) || errors.Is(port.ErrNoProposal, port.ErrClaimLost) {
		t.Fatal("ErrClaimLost and ErrNoProposal must be distinct sentinels")
	}
}
