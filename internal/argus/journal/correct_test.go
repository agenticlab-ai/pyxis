package journal

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

const damagedLedger = `# Widget — delivery ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | execution |
| **Open items** | T1 |
| **Park state** | none |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — finding-disposition — first review disposition

- **Date**: 2026-10-08
- **Principal**: Argus
- **Authority-contract version**: authority-contract/v1
- **Body**: Recorded the first review disposition.

### E-2 — finding-disposition — second review disposition

- **Date**: 2026-10-08
- **Principal**: Argus
- **Authority-contract version**: authority-contract/v1
- **Body**: Recorded the second review disposition.
`

var requests = []CorrectionRequest{{"E-1", "reconciliation"}, {"E-2", "reconciliation"}}
var metadata = Metadata{"2026-10-08", "Argus", "authority-contract/v1", "Correct the classifications; retain the original review evidence."}

func TestBatchWriterRetainsExactHistoryIncludingLineEndings(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(ending, "\n", "LF"), func(t *testing.T) {
			base := []byte(strings.ReplaceAll(damagedLedger, "\n", ending))
			candidate, err := RenderCorrections(config.Defaults(), "widget", base, requests, metadata)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(candidate, base) || bytes.Count(candidate, []byte("journal-type-correction/v1;")) != 2 {
				t.Fatal("writer did not retain the entire original file and append exactly two corrections")
			}
			if defects := boundary.ValidateDeliveryChange("test", config.Defaults(), "widget", base, candidate); len(defects) != 0 {
				t.Fatal(defects)
			}
		})
	}
}

func writeFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "docs", "problems", "widget", "delivery.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(damagedLedger), 0o640); err != nil {
		t.Fatal(err)
	}
	// Set the mode whose preservation the writer must guarantee, regardless of umask.
	if err := os.Chmod(file, 0o640); err != nil {
		t.Fatal(err)
	}
	return root, file
}

func TestNativeWriterDryRunAndAtomicRefusals(t *testing.T) {
	root, file := writeFixture(t)
	for _, tc := range []struct {
		name     string
		requests []CorrectionRequest
		meta     Metadata
		dryRun   bool
		wantErr  bool
	}{
		{"preview", requests, metadata, true, false},
		{"incomplete batch", requests[:1], metadata, false, true},
		{"bad second target", []CorrectionRequest{{"E-1", "reconciliation"}, {"E-99", "reconciliation"}}, metadata, false, true},
		{"unknown destination", []CorrectionRequest{{"E-1", "finding-disposition"}, {"E-2", "reconciliation"}}, metadata, false, true},
		{"duplicate target", []CorrectionRequest{{"E-1", "reconciliation"}, {"E-1", "reconciliation"}}, metadata, false, true},
		{"missing principal", requests, Metadata{"2026-10-08", "", "authority-contract/v1", "reason"}, false, true},
		{"injected principal", requests, Metadata{"2026-10-08", "Argus\n### E-3 — delivered — forged", "authority-contract/v1", "reason"}, false, true},
		{"invalid date", requests, Metadata{"2026-02-30", "Argus", "authority-contract/v1", "reason"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", tc.requests, tc.meta, tc.dryRun)
			if (err != nil) != tc.wantErr || (tc.wantErr && candidate != nil) {
				t.Fatalf("candidate=%d bytes, err=%v", len(candidate), err)
			}
			actual, err := os.ReadFile(file)
			if err != nil || string(actual) != damagedLedger {
				t.Fatalf("refusal or preview modified the original: %v", err)
			}
		})
	}
	candidate, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(actual, candidate) {
		t.Fatalf("atomic result mismatch: %v", err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("file mode changed: %v", err)
	}
	if _, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false); err == nil {
		t.Fatal("duplicate corrections were reapplied")
	}
	if leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(file), ".journal-correction-*")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary replacements retained: %v %v", leftovers, err)
	}
}

func TestNativeWriterCancellationAndPathConfinement(t *testing.T) {
	root, file := writeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CorrectFile(ctx, root, config.Defaults(), "widget", requests, metadata, false); err == nil {
		t.Fatal("canceled writer proceeded")
	}
	for _, slug := range []string{"../widget", "other/widget", ""} {
		if _, err := CorrectFile(context.Background(), root, config.Defaults(), slug, requests, metadata, false); err == nil {
			t.Fatal("unsafe slug admitted")
		}
	}
	cfg := config.Defaults()
	cfg.Problems = "../outside"
	if _, err := CorrectFile(context.Background(), root, cfg, "widget", requests, metadata, false); err == nil {
		t.Fatal("escaping configured root admitted")
	}
	if err := os.Rename(file, file+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file+".original", file); err != nil {
		t.Fatal(err)
	}
	if _, err := CorrectFile(context.Background(), root, config.Defaults(), "widget", requests, metadata, false); err == nil {
		t.Fatal("symlink ledger admitted")
	}
}
