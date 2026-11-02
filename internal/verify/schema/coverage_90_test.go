package schema

import "testing"

func TestCoverage90DurationRejectsMalformedJSON(t *testing.T) {
	var d DurationMS
	if err := d.UnmarshalJSON([]byte(`"not-a-number"`)); err == nil {
		t.Fatal("DurationMS accepted a string")
	}
}
