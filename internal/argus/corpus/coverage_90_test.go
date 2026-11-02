package corpus

import "testing"

func TestCoverage90LayerNamesAndMalformedContract(t *testing.T) {
	wants := map[Layer]string{
		WriteHook:       "write hook (write-time)",
		StructureArm:    "structure arm (merge boundary)",
		AttributionGate: "attribution gate (merge boundary)",
		Layer(99):       "unknown layer",
	}
	for layer, want := range wants {
		if got := layer.String(); got != want {
			t.Errorf("Layer(%d).String() = %q, want %q", layer, got, want)
		}
	}
	if refused, defects := StructureArmRefuses([]byte("not an authority contract")); !refused || len(defects) == 0 {
		t.Fatalf("malformed contract refused=%v defects=%v", refused, defects)
	}
}
