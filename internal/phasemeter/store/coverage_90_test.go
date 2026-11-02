package store

import "testing"

func TestCoverage90EmptyRunID(t *testing.T) {
	if err := validRunID(""); err == nil {
		t.Fatal("empty run ID was accepted")
	}
}
