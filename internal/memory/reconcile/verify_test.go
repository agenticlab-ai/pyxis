package reconcile

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestVerifyOffline_AllowsFullyRetiredPRDAndDesignSources(t *testing.T) {
	f := newCloseoutFixture(t)
	addFixtureAnchor(t, f, 50)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	f.addDraft(t, draft, 90)
	set, proj = f.load(t)

	retired := f.snap
	retired.Files = map[string]File{}
	retired.AuthoredSources = nil
	retired.PRDs = nil
	retired.Designs = nil
	report := VerifyOffline(retired, set, proj, Options{})
	if report.Blocking() {
		t.Fatalf("fully retired development-context sources invalidated closeout: %#v", report.Findings)
	}
}

func TestVerifyOffline_CloseoutCodeIntegrity(t *testing.T) {
	tests := []struct {
		name      string
		wantCode  string
		wantScope string
		mutate    func(*testing.T, *closeoutFixture)
	}{
		{
			name:      "digest drift",
			wantCode:  "closeout-code-drift",
			wantScope: "internal/widget",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				f.snap.Code[0].SHA256 = strings.Repeat("9", 64)
			},
		},
		{
			name:      "deleted code",
			wantCode:  "closeout-code-missing",
			wantScope: "internal/widget",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				f.snap.Code = nil
			},
		},
		{
			name:      "new code beneath broad scope",
			wantCode:  "closeout-code-unsealed",
			wantScope: "internal/widget/subpackage",
			mutate: func(_ *testing.T, f *closeoutFixture) {
				f.snap.Code = append(f.snap.Code, codeinventory.Unit{
					Kind: codeinventory.GoPackage, Path: "internal/widget/subpackage",
					Files: []string{"internal/widget/subpackage/subpackage.go"}, SHA256: strings.Repeat("8", 64),
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCloseoutFixture(t)
			addFixtureAnchor(t, f, 50)
			set, proj := f.load(t)
			draft, _, err := BuildCloseout("widget", f.snap, set, proj)
			if err != nil {
				t.Fatalf("BuildCloseout: %v", err)
			}
			f.addDraft(t, draft, 90)
			set, proj = f.load(t)
			tt.mutate(t, f)

			report := VerifyOffline(f.snap, set, proj, Options{})
			if !hasFinding(report, tt.wantCode, tt.wantScope) {
				t.Fatalf("missing %s: %#v", tt.wantCode, report.Findings)
			}
		})
	}
}

func TestVerifyOffline_FailsWhenSealedCodeLosesSemanticAssociation(t *testing.T) {
	f := newCloseoutFixture(t)
	f.decision.Tags = nil
	anchor := addFixtureAnchor(t, f, 9)
	f.decision.Relations = append(f.decision.Relations, record.Relation{Type: record.About, Target: anchor.ID})
	f.replaceRecord(t, f.decision, nil)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout: %v", err)
	}
	f.addDraft(t, draft, 90)

	replacement := fixtureRecord(record.Need, anchor.Subject, anchor.Family, 91)
	replacement.Statement = anchor.Statement
	replacement.Tags = append([]string(nil), anchor.Tags...)
	replacement.Provenance = anchor.Provenance
	replacement.Relations = []record.Relation{{Type: record.Supersedes, Target: anchor.ID}}
	f.replaceRecord(t, replacement, nil)
	set, proj = f.load(t)
	if _, err := ActiveCloseouts(set, proj); err != nil {
		t.Fatalf("receipt itself should remain structurally active: %v", err)
	}

	report := VerifyOffline(f.snap, set, proj, Options{})
	if !hasFinding(report, "closeout-code-unassociated", "internal/widget") {
		t.Fatalf("lost semantic association was not detected: %#v", report.Findings)
	}
}

func TestVerifyOffline_PresentStaleSourceStillFails(t *testing.T) {
	f := newCloseoutFixture(t)
	addFixtureAnchor(t, f, 50)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)
	set, proj = f.load(t)

	changed := []byte("changed PRD source\n")
	f.snap.Files = cloneFiles(f.snap.Files)
	f.snap.Files[f.prdPath] = File{Path: f.prdPath, Mode: "100644", SHA256: sourceDigest(changed), Body: changed}
	report := VerifyOffline(f.snap, set, proj, Options{})
	if !hasFinding(report, "stale-document", f.prdPath) {
		t.Fatalf("present stale source was hidden by closeout: %#v", report.Findings)
	}
}

func TestVerifyOffline_InvalidActiveCloseoutFailsClosed(t *testing.T) {
	f := newCloseoutFixture(t)
	addFixtureAnchor(t, f, 50)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)

	// Superseding a bound native record makes the still-active seal stale.
	successor := fixtureRecord(record.Promise, f.promise.Subject, f.promise.Family, 91)
	*successor = *f.promise
	successor.ID = fixtureRecord(record.Promise, f.promise.Subject, f.promise.Family, 91).ID
	successor.CreatedAt = fixtureRecord(record.Promise, f.promise.Subject, f.promise.Family, 91).CreatedAt
	successor.Relations = []record.Relation{{Type: record.Supersedes, Target: f.promise.ID}}
	f.replaceRecord(t, successor, nil)
	set, proj = f.load(t)
	if _, err := ActiveCloseouts(set, proj); !errors.Is(err, ErrInvalidCloseout) {
		t.Fatalf("ActiveCloseouts error = %v", err)
	}
	report := VerifyOffline(f.snap, set, proj, Options{})
	if !hasFinding(report, "invalid-closeout", "closeout/") {
		t.Fatalf("stale bound native record did not invalidate closeout: %#v", report.Findings)
	}
}

func TestVerifyOfflineWithHistory_ReplaysActiveCloseout(t *testing.T) {
	f := newCloseoutFixture(t)
	addFixtureAnchor(t, f, 50)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatal(err)
	}
	f.addDraft(t, draft, 90)
	set, proj = f.load(t)

	good := VerifyOfflineWithHistory(f.snap, set, proj, Options{}, func(closeout ActiveCloseout) (Snapshot, error) {
		commit := closeout.Receipt.SourceCommit
		if commit != f.snap.Commit {
			t.Fatalf("history loader commit = %s, want %s", commit, f.snap.Commit)
		}
		return f.snap, nil
	})
	if hasFinding(good, "closeout-history-invalid", "closeout/widget") || hasFinding(good, "closeout-history-unavailable", "closeout/widget") {
		t.Fatalf("valid historical closeout failed replay: %#v", good.Findings)
	}

	unavailable := VerifyOfflineWithHistory(f.snap, set, proj, Options{}, func(ActiveCloseout) (Snapshot, error) {
		return Snapshot{}, errors.New("commit missing")
	})
	if !hasFinding(unavailable, "closeout-history-unavailable", "closeout/widget") {
		t.Fatalf("missing historical commit was not reported: %#v", unavailable.Findings)
	}

	invalid := VerifyOfflineWithHistory(f.snap, set, proj, Options{}, func(ActiveCloseout) (Snapshot, error) {
		wrong := f.snap
		wrong.Commit = strings.Repeat("9", 40)
		return wrong, nil
	})
	if !hasFinding(invalid, "closeout-history-invalid", "closeout/widget") {
		t.Fatalf("mismatched historical closeout was not reported: %#v", invalid.Findings)
	}
}

func TestVerifyOfflineWithHistory_RequiresEveryOwnerBeforeSharedSourceRetirement(t *testing.T) {
	f := newCloseoutFixture(t)
	otherPath := "docs/prds/other/000-base.md"
	otherBody := []byte("# Other PRD\n")
	sharedPath := "docs/brainstorms/shared-tech-design.md"
	sharedBody := []byte(`# Shared design direction — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Shared direction. |
| **Relevant-when** | Revisiting the shared seam. |
| **Basis** | prd:widget · prd:other |
| **Authority** | @owner |
| **Date(s)** | 2026-09-08 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | — |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

Shared context.
`)
	other := f.snap.PRDs[0]
	other.Feature, other.Source = "other", otherPath
	f.snap.PRDs = append(f.snap.PRDs, other)
	f.snap.Files = cloneFiles(f.snap.Files)
	f.snap.Files[otherPath] = File{Path: otherPath, Mode: "100644", SHA256: sourceDigest(otherBody), Body: otherBody}
	f.snap.Files[sharedPath] = File{Path: sharedPath, Mode: "100644", SHA256: sourceDigest(sharedBody), Body: sharedBody}
	f.snap.AuthoredSources = append(f.snap.AuthoredSources, otherPath, sharedPath)
	sort.Strings(f.snap.AuthoredSources)

	f.replaceRecord(t, fixtureDocument("prd/other", "base", otherPath, otherBody, 70), otherBody)
	f.replaceRecord(t, fixtureDocument("brainstorm/shared-tech-design", "record", sharedPath, sharedBody, 71), sharedBody)
	set, proj := f.load(t)
	draft, _, err := BuildCloseout("widget", f.snap, set, proj)
	if err != nil {
		t.Fatalf("BuildCloseout(widget): %v", err)
	}
	f.addDraft(t, draft, 90)
	sealedSet, sealedProj := f.load(t)

	current := f.snap
	current.Files = cloneFiles(f.snap.Files)
	delete(current.Files, sharedPath)
	current.AuthoredSources = withoutString(current.AuthoredSources, sharedPath)
	report := VerifyOfflineWithHistory(current, sealedSet, sealedProj, Options{}, func(ActiveCloseout) (Snapshot, error) {
		return f.snap, nil
	})
	if !hasFinding(report, "closeout-shared-source-unsealed", sharedPath) {
		t.Fatalf("one owner retired shared source without the other owner: %#v", report.Findings)
	}
}

func withoutString(values []string, remove string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != remove {
			out = append(out, value)
		}
	}
	return out
}

func addFixtureAnchor(t *testing.T, f *closeoutFixture, seq byte) *record.Record {
	t.Helper()
	anchor := fixtureRecord(record.Need, "code/internal.widget", "anchor", seq)
	anchor.Statement = "Production code unit internal/widget exists."
	anchor.Tags = []string{"code:internal/widget", "reconcile:anchor"}
	anchor.Provenance = record.Provenance{
		Source: "code:internal/widget", Commit: f.snap.Commit,
		Note: "offline code inventory " + f.snap.Code[0].SHA256,
	}
	f.replaceRecord(t, anchor, nil)
	return anchor
}
