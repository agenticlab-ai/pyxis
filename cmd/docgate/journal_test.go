package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliDamagedJournal = `# Widget — delivery ledger

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

func TestCorrectJournalOperatorPreviewApplyAndRefusals(t *testing.T) {
	root := docgateRoot(t)
	t.Chdir(root)
	file := filepath.Join(root, "docs", "problems", "widget", "delivery.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(cliDamagedJournal), 0o640); err != nil {
		t.Fatal(err)
	}
	// The preservation assertion needs this exact mode even under umask 077.
	if err := os.Chmod(file, 0o640); err != nil {
		t.Fatal(err)
	}
	args := []string{"correct-journal", "widget", "--correction", "E-1=reconciliation", "--correction", "E-2=reconciliation", "--date", "2026-10-08", "--principal", "test-operator", "--authority-version", "authority-contract/v1", "--reason", "Retain the original review decisions and correct only their classification."}
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "missing slug", args: []string{"correct-journal"}, want: 2},
		{name: "extra slug", args: []string{"correct-journal", "widget", "other"}, want: 2},
		{name: "malformed correction flag", args: []string{"correct-journal", "widget", "--correction", "E-1"}, want: 2},
		{name: "missing attribution", args: []string{"correct-journal", "widget", "--correction", "E-1=reconciliation"}, want: 1},
		{name: "invalid second request", args: append(append([]string(nil), args...), "--correction", "E-99=reconciliation"), want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, diagnostic := captureDocgateOutput(t, func() int { return run(tc.args) })
			if code != tc.want || diagnostic == "" {
				t.Fatalf("run returned %d with %q; want %d", code, diagnostic, tc.want)
			}
			current, err := os.ReadFile(file)
			if err != nil || string(current) != cliDamagedJournal {
				t.Fatalf("refused command changed the journal: %v", err)
			}
		})
	}
	code, preview, diagnostic := captureDocgateOutput(t, func() int { return run(append(append([]string(nil), args...), "--dry-run")) })
	if code != 0 || diagnostic != "" || !strings.HasPrefix(preview, cliDamagedJournal) || strings.Count(preview, "journal-type-correction/v1;") != 2 {
		t.Fatalf("preview lost history or ignored the interspersed flags: %d %q", code, diagnostic)
	}
	current, err := os.ReadFile(file)
	if err != nil || string(current) != cliDamagedJournal {
		t.Fatal("preview changed the journal")
	}
	code, receipt, diagnostic := captureDocgateOutput(t, func() int { return run(args) })
	if code != 0 || diagnostic != "" || !strings.Contains(receipt, "2 append-only entries") {
		t.Fatalf("apply failed: %d %q %q", code, receipt, diagnostic)
	}
	current, err = os.ReadFile(file)
	if err != nil || string(current) != preview {
		t.Fatalf("apply differed from the reviewed preview: %v", err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("apply changed file permissions: %v", err)
	}
	code, _, _ = captureDocgateOutput(t, func() int { return run(args) })
	if code != 1 {
		t.Fatal("replayed correction batch was applied twice")
	}
	current, err = os.ReadFile(file)
	if err != nil || string(current) != preview {
		t.Fatal("replayed batch changed retained history")
	}
}
