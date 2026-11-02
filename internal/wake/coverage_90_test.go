package wake

import (
	"errors"
	"testing"
)

func TestCoverage90PushDeliveryErrors(t *testing.T) {
	if _, err := Push(t.Context(), &recordDoor{}, stubLedgers{err: errors.New("ledger unavailable")}, testProblemsRoot, "wake:foo:merge"); err == nil {
		t.Fatal("Push ignored ledger listing failure")
	}
	if _, err := Push(t.Context(), &recordDoor{failSlugs: map[string]bool{"foo": true}}, stubLedgers{}, testProblemsRoot, "wake:foo:merge"); err == nil {
		t.Fatal("Push ignored front-door failure")
	}
}

func TestCoverage90TickMalformedAndOutOfScopeLedgers(t *testing.T) {
	docs := []Doc{
		{Path: testProblemsRoot + "/broken/delivery.md", Bytes: []byte("not a delivery ledger")},
		{Path: "docs/wiki/topic.md", Bytes: []byte("not a delivery ledger")},
	}
	rep, err := newBackstop(stubLedgers{docs: docs}, stubProber{}, &recordDoor{}).Tick(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Defects) == 0 {
		t.Fatal("malformed in-scope ledger produced no defect")
	}
}
