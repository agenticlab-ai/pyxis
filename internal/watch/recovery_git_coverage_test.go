package watch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseRecoveryRowsRejectsMalformedAndFiltersScopes(t *testing.T) {
	if _, err := parseRecoveryRows([]byte(`{"unknown":true}`), "scope"); err == nil {
		t.Fatal("accepted an unknown recovery-row field")
	}
	if _, err := parseRecoveryRows([]byte(`{}`), "scope"); err == nil {
		t.Fatal("accepted an invalid recovery row")
	}
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	rows := []RecoveryEntry{
		{Scope: "wanted", ObligationID: "o-1", RecoveryEpoch: "e-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now},
		{Scope: "other", ObligationID: "o-2", RecoveryEpoch: "e-2", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now},
	}
	var data []byte
	for _, row := range rows {
		sealed, err := sealRecoveryEntry(row)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(sealed)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, append(encoded, '\n')...)
	}
	got, err := parseRecoveryRows(data, "wanted")
	if err != nil || len(got) != 1 || got[0].Scope != "wanted" {
		t.Fatalf("filtered rows = %+v, %v", got, err)
	}
	if _, err := parseRecoveryRows([]byte(strings.Repeat("x", (1<<20)+1)), "wanted"); err == nil {
		t.Fatal("accepted an oversized recovery row")
	}
}

func TestGitRecoveryLedgerReportsConstructionReadAndAppendFailures(t *testing.T) {
	if _, err := NewGitRecoveryLedger("", "", ""); err == nil {
		t.Fatal("NewGitRecoveryLedger accepted empty configuration")
	}
	ledger, err := NewGitRecoveryLedger(t.TempDir(), "origin", DefaultRecoveryEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Exact(context.Background(), "scope"); err == nil {
		t.Fatal("Exact accepted a non-repository")
	}
	if _, err := ledger.Append(context.Background(), RecoveryEntry{}); err == nil {
		t.Fatal("Append accepted an empty entry")
	}
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	entry := RecoveryEntry{Scope: "scope", ObligationID: "o-1", RecoveryEpoch: "e-1", Action: RecoveryInitialize, FirstObservedAtUTC: now, RecoveryBound: 2, RecordedAtUTC: now}
	if _, err := ledger.Append(context.Background(), entry); err == nil {
		t.Fatal("Append wrote through a non-repository")
	}
}
