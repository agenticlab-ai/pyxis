package reviewgate

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCanonicalTaskAggregateRetainsCompleteEvidenceAndRejectsForeignBytes(t *testing.T) {
	b, s, p := completeFixture(t)
	data, err := Canonical(s)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := Parse(data)
	if err != nil || Validate(b, recovered, p) != nil {
		t.Fatalf("canonical complete proof cannot be recovered: %v", err)
	}
	for _, bad := range [][]byte{
		[]byte("{}\n{}\n"),
		[]byte("{"),
		append(append([]byte(nil), data...), []byte("{}")...),
		bytes.Replace(data, []byte(`"schema_version":`), []byte(`"unknown":true,"schema_version":`), 1),
		bytes.Replace(data, []byte(`"schema_version":`), []byte(`"schema_version":"duplicate","schema_version":`), 1),
		bytes.Replace(data, []byte(`"passed":true`), []byte(`"unknown_outcome_field":true,"passed":true`), 1),
		append([]byte(" "), data...),
		bytes.Repeat([]byte("x"), MaxSnapshotBytes+1),
	} {
		if bytes.Equal(bad, data) {
			t.Fatal("malformed fixture did not change the canonical bytes")
		}
		if _, err := Parse(bad); !errors.Is(err, ErrIncomplete) {
			t.Fatalf("foreign or noncanonical aggregate accepted: %v", err)
		}
	}
	s.Policy.Calibration = strings.Repeat("x", MaxSnapshotBytes)
	if _, err := Canonical(s); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("oversized aggregate was truncated or accepted: %v", err)
	}
}
