package fold_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
)

// approvalDefect reports whether the fold flagged the merge-target approval bar.
func approvalDefect(res fold.Result) bool {
	for _, d := range res.Defects {
		if strings.Contains(d.Reason, "not Approved") {
			return true
		}
	}
	return false
}

// AllowUnapproved lifts the merge-target approval bar (FR-5 scopes it to
// validate-amendments "at merge-target") so a draft amendment can be folded for
// review before it is approved — and lifts nothing else.
func TestAllowUnapprovedLiftsOnlyTheApprovalBar(t *testing.T) {
	draft := amend(goodSupersede, "Status", "Draft")

	for _, tc := range []struct {
		name         string
		opts         []fold.Option
		wantApproval bool
		wantOutput   bool
	}{
		{name: "strict fold flags a draft", opts: nil, wantApproval: true, wantOutput: false},
		{name: "preview fold admits a draft", opts: []fold.Option{fold.AllowUnapproved()}, wantApproval: false, wantOutput: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := fold.Fold([]ledger.Entry{base(""), draft}, tc.opts...)
			if got := approvalDefect(res); got != tc.wantApproval {
				t.Fatalf("approval defect = %v, want %v (defects: %v)", got, tc.wantApproval, res.Defects)
			}
			if tc.wantOutput {
				if len(res.Defects) != 0 {
					t.Fatalf("preview of a well-formed draft flagged: %v", res.Defects)
				}
				if len(res.EffectiveMD) == 0 {
					t.Fatal("preview produced no projection")
				}
				// The superseding row must actually be in the projection —
				// a preview that returned the unamended base would be worse
				// than no preview.
				if !bytes.Contains(res.EffectiveMD, []byte("FR-2")) {
					t.Error("projection does not carry the amendment's FR-2 row")
				}
			}
		})
	}
}

// The preview must not become a way to smuggle a malformed amendment past
// review: every non-approval defect still fires under AllowUnapproved.
func TestAllowUnapprovedStillReportsRealDefects(t *testing.T) {
	for _, tc := range []struct {
		name, wantReason string
		entry            ledger.Entry
	}{
		{
			name:       "dangling target",
			wantReason: "dangling target",
			entry:      amend("### op: retire FR-9\n\nReason: r.\n", "Status", "Draft"),
		},
		{
			name:       "understated blast radius",
			wantReason: "understates computed",
			entry:      amend(goodSupersede, "Status", "Draft", "Blast radius", "tier: additive; touches: FR-1; reason: r"),
		},
		{
			name:       "unknown evidence class",
			wantReason: "unknown evidence class",
			entry:      amend(goodSupersede, "Status", "Draft", "Learned-from", "hearsay:x"),
		},
		{
			name:       "empty Learned-from",
			wantReason: "Learned-from is empty",
			entry:      amend(goodSupersede, "Status", "Draft", "Learned-from", ""),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := fold.Fold([]ledger.Entry{base(""), tc.entry}, fold.AllowUnapproved())
			var found bool
			for _, d := range res.Defects {
				if strings.Contains(d.Reason, tc.wantReason) {
					found = true
				}
			}
			if !found {
				t.Fatalf("preview did not report %q; defects: %v", tc.wantReason, res.Defects)
			}
			if approvalDefect(res) {
				t.Error("preview reported the approval bar it is meant to lift")
			}
		})
	}
}

// An approved amendment folds identically with and without the option — the
// preview and the persisting fold are one computation, so a review can never
// bless a projection that differs from the one that merges.
func TestPreviewMatchesStrictFoldWhenApproved(t *testing.T) {
	entries := []ledger.Entry{base(""), amend(goodSupersede)}
	strict := fold.Fold(entries)
	preview := fold.Fold(entries, fold.AllowUnapproved())

	if len(strict.Defects) != 0 {
		t.Fatalf("strict fold of an approved amendment flagged: %v", strict.Defects)
	}
	if !bytes.Equal(strict.EffectiveMD, preview.EffectiveMD) {
		t.Error("preview effective.md differs from the strict fold's")
	}
	if !bytes.Equal(strict.SourceJSON, preview.SourceJSON) {
		t.Error("preview effective.source.json differs from the strict fold's")
	}
	if strict.Fingerprint != preview.Fingerprint {
		t.Errorf("fingerprint differs: strict %s, preview %s", strict.Fingerprint, preview.Fingerprint)
	}
}
