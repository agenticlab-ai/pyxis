package executiondemand

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestRetiredOriginAndCapsuleAreRefused(t *testing.T) {
	for _, tc := range []struct {
		path   string
		decode func([]byte) error
	}{
		{"testdata/golden_plan_v1_compat.json", func(b []byte) error { _, err := DecodeCanonical(b); return err }},
		{"conformancetest/testdata/golden_capsule_plan_v1_compat.json", func(b []byte) error { _, err := DecodeCapsuleCanonical(b); return err }},
	} {
		t.Run(tc.path, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				CanonicalHex string `json:"canonical_hex"`
			}
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			wire, err := hex.DecodeString(fixture.CanonicalHex)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.decode(wire); err == nil {
				t.Fatal("accepted retired compatibility origin")
			}
		})
	}
}
