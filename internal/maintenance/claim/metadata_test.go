package claim

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTerminalMetadataRejectsEmbeddedHostPaths(t *testing.T) {
	for _, text := range []string{
		"path=/Users/operator/private/cache",
		"stat(/Users/operator/private/cache): permission denied",
		"path:/home/operator/private/cache",
		`{"path":"/Users/operator/private/cache"}`,
		`path="C:\Users\operator\private\cache"`,
		"path=~/private/cache",
		"file `/Users/operator/private/cache`",
		"file “/Users/operator/private/cache”",
		`path=\\server\private\cache`,
	} {
		for _, field := range []string{"summary", "cause"} {
			t.Run(field+"/"+text, func(t *testing.T) {
				record := fixtureRecord()
				terminal := Terminal{Outcome: OutcomeFailed, FinishedAt: record.ClaimedAt.Add(time.Minute)}
				if field == "summary" {
					terminal.Summary = text
				} else {
					terminal.Cause = text
				}
				if _, err := WithTerminal(record, terminal); err == nil {
					t.Fatalf("embedded private path accepted in %s: %q", field, text)
				}
			})
		}
	}
	for _, text := range []string{
		"changed internal/maintenance/claim.go",
		"1/2 checks passed",
		"fetch https://example.test/repo?attempt=1 failed",
		"evidence:https://example.test:443/report",
	} {
		t.Run("safe/"+text, func(t *testing.T) {
			record := fixtureRecord()
			terminal := Terminal{Outcome: OutcomeFailed, FinishedAt: record.ClaimedAt.Add(time.Minute), Summary: text, Cause: text}
			if _, err := WithTerminal(record, terminal); err != nil {
				t.Fatalf("safe shared diagnostic rejected: %v", err)
			}
		})
	}
}

func TestEncodedRecordFitsDecoderBound(t *testing.T) {
	for _, payloadSize := range []int{maxRecordBytes - 1, maxRecordBytes, maxRecordBytes + 1} {
		t.Run(fmt.Sprint(payloadSize), func(t *testing.T) {
			record := fixtureRecord()
			bare, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			record.Route.NativeModel += strings.Repeat("x", payloadSize-len(bare))
			bare, err = json.Marshal(record)
			if err != nil || len(bare) != payloadSize {
				t.Fatalf("fixture JSON size = %d, want %d: %v", len(bare), payloadSize, err)
			}
			encoded, err := Encode(record)
			if payloadSize >= maxRecordBytes {
				if err == nil {
					t.Fatalf("encoder accepted %d bytes plus framing, beyond decoder's %d-byte bound", payloadSize, maxRecordBytes)
				}
				return
			}
			if err != nil || len(encoded) != maxRecordBytes {
				t.Fatalf("largest encoded record = %d bytes: %v", len(encoded), err)
			}
			decoded, err := Decode(encoded)
			if err != nil || !SameOwner(record, decoded) {
				t.Fatalf("successful encoding did not round-trip: %v", err)
			}
		})
	}
}
