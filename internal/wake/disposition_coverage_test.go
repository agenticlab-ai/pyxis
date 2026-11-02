package wake

import "testing"

func TestDispositionStringCoversClosedEnum(t *testing.T) {
	for disposition, want := range map[Disposition]string{Open: "open", Parked: "parked", Closed: "closed", Disposition(99): "open"} {
		if got := disposition.String(); got != want {
			t.Fatalf("Disposition(%d).String = %q, want %q", disposition, got, want)
		}
	}
}
