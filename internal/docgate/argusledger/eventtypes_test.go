package argusledger

import (
	"strings"
	"testing"
)

func TestEntryType(t *testing.T) {
	cases := map[string]string{
		"### E-1 — stage-entered — began":                    "stage-entered",
		"### E-12 — item-materialized — a fleet item":        "item-materialized",
		"### E-3 — reconciliation — tag failed; routed back": "reconciliation",
		"### E-9 — delivered — done":                         "delivered",
		"## Journal":                                         "",
		"### E-1 malformed no separator":                     "",
		"- **Await**: pr:1 · merged · since 2026-07-11":      "",
	}
	for heading, want := range cases {
		if got := EntryType([]byte(heading + "\n\nbody")); got != want {
			t.Errorf("EntryType(%q) = %q, want %q", heading, got, want)
		}
	}
}

func TestKnownEntryType(t *testing.T) {
	for _, ok := range []string{TypeRulingRecorded, TypeItemMaterialized, TypeReconciliation, TypeDelivered, TypeBriefEmitted} {
		if !KnownEntryType(ok) {
			t.Errorf("KnownEntryType(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"invented", "materialize", "", "Delivered"} {
		if KnownEntryType(bad) {
			t.Errorf("KnownEntryType(%q) = true, want false", bad)
		}
	}
}

func TestMaterializationGist_RoundTrip(t *testing.T) {
	cases := []MaterializationGist{
		{Plan: "argus-unattended-drive/001", Trigger: TriggerWake},                                               // a trigger entry
		{Plan: "p/001", Items: []string{"item:5"}, Tag: TagVerified, Trigger: TriggerWake},                       // a confirmation
		{Plan: "p/001", Items: []string{"item:9"}, Tag: TagFailed, Trigger: TriggerWake},                         // a reconciliation
		{Plan: "p/001", Items: []string{"item:2", "item:5", "item:9"}, Tag: TagVerified, Trigger: TriggerPrompt}, // a directed multi-item
	}
	for _, want := range cases {
		rendered := want.Render()
		got, defects := ParseMaterializationGist("wake", "docs/problems/p/delivery.md", "E-3", rendered)
		if len(defects) != 0 {
			t.Fatalf("Render→Parse of %q raised defects: %v", rendered, defects)
		}
		if got.Plan != want.Plan || got.Tag != want.Tag || got.Trigger != want.Trigger || strings.Join(got.Items, ",") != strings.Join(want.Items, ",") {
			t.Fatalf("round trip of %q = %+v, want %+v", rendered, got, want)
		}
	}
}

func TestMaterializationGist_ClosedParse(t *testing.T) {
	cases := []struct {
		name string
		gist string
	}{
		{"unknown key", "plan=p; sudo=rm; trigger=wake"},
		{"bad tag", "plan=p; tag=maybe; trigger=wake"},
		{"bad trigger", "plan=p; trigger=cron"},
		{"missing plan", "items=item:1; trigger=wake"},
		{"missing trigger", "plan=p; items=item:1"},
		{"malformed segment", "plan=p; justtext; trigger=wake"},
		{"duplicate key", "plan=p; plan=q; trigger=wake"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, defects := ParseMaterializationGist("wake", "f", "E-1", c.gist); len(defects) == 0 {
				t.Fatalf("ParseMaterializationGist(%q) accepted a malformed gist; want a named defect", c.gist)
			}
		})
	}
}
