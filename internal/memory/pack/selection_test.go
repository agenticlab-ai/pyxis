package pack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func saveRecord(t *testing.T, tr *tree, r *record.Record) {
	t.Helper()
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	tr.files[r.Path()] = b
}

func TestPackPreservesDecisionEvidence(t *testing.T) {
	tr, ids := fixture(t)
	d := ids["d1"]
	d.Alternatives = []string{"Hosted queue rejected because the installation must work offline."}
	d.LearnedFrom = []string{"issue:42"}
	d.Expires = "2027-01-01T00:00:00Z"
	d.Provenance = record.Provenance{Source: "docs/decision.md", PR: 42, Commit: strings.Repeat("a", 40), Note: "pilot scope only"}
	saveRecord(t, tr, d)
	source := tr.add(record.Document, "design/x", "design", "Exact source")
	body := []byte("# D-1\nThe reason and the rejected alternative.\n")
	source.LiteralBody, source.BodySHA256 = true, record.HashBody(body)
	saveRecord(t, tr, source)
	tr.files[source.BodyPath()] = body
	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, 10000, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(pk)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{d.Alternatives[0], d.LearnedFrom[0], d.Expires, d.Provenance.Source, d.Provenance.Commit, d.Provenance.Note, "memory/" + source.BodyPath()} {
		if !strings.Contains(pk.Render(), want) || !strings.Contains(string(encoded), want) {
			t.Errorf("missing evidence %q", want)
		}
	}
	if len(pk.Narrative) != 1 || !strings.Contains(pk.Narrative[0].Note, "rejected alternative") {
		t.Fatalf("serving decision must select its exact explanation: %+v", pk.Narrative)
	}
}

func TestPackFindsActiveCodeAndRelatedKnowledge(t *testing.T) {
	tr, ids := fixture(t)
	ids["d1"].Tags = []string{"code:internal/widget"}
	saveRecord(t, tr, ids["d1"])
	old := tr.add(record.Learning, "learning/queue", "", "obsolete lesson")
	old.Tags = []string{"code:internal/widget"}
	saveRecord(t, tr, old)
	current := tr.add(record.Learning, "learning/queue", "", "offline lesson", rel(record.Supersedes, old.ID))
	current.Tags = old.Tags
	saveRecord(t, tr, current)
	linked := tr.add(record.Learning, "learning/linked", "", "related lesson", rel(record.About, ids["d1"].ID))
	other := tr.add(record.Learning, "learning/other", "", "unrelated lesson")
	other.Tags = []string{"code:internal/widgets"}
	saveRecord(t, tr, other)
	for _, scope := range []Scope{{Subjects: []string{"prd/x"}, Families: []string{"FR-1"}}, {Code: []string{"internal/widget/main.go"}}} {
		pk, err := Compile(tr.projection(), nil, scope, 10000, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{current.ID, linked.ID, ids["d1"].ID} {
			if !strings.Contains(pk.Render(), want) {
				t.Errorf("scope %+v missing %s", scope, want)
			}
		}
		for _, unwanted := range []string{old.ID, other.ID} {
			if strings.Contains(pk.Render(), unwanted) {
				t.Errorf("scope %+v leaked %s", scope, unwanted)
			}
		}
	}
	for _, bad := range []string{"", ".", "..", "../outside", "/absolute", "a/../b", "a//b", "a\\b", "a\nb"} {
		if _, err := Compile(tr.projection(), nil, Scope{Code: []string{bad}}, 10000, nil); err == nil {
			t.Errorf("accepted invalid code scope %q", bad)
		}
	}
}

func TestPackBudgetKeepsWhyAndPolicies(t *testing.T) {
	tr, _ := fixture(t)
	for n := range 30 {
		tr.add(record.Promise, "prd/x", string(rune('a'+n)), strings.Repeat("required behaviour ", 40))
	}
	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"prd/x"}}, 1600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Policies) != 1 || len(pk.Decisions) == 0 || len(pk.Learnings) == 0 || len(pk.Requirements) == 0 || len(pk.Omissions) == 0 || pk.Used > 1600 || pk.Used != record.Tokens(pk.Render()) {
		t.Fatalf("budget starved a semantic section: %+v", pk)
	}
	if _, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"prd/x"}}, 10, nil); err == nil || !strings.Contains(err.Error(), "policies") {
		t.Fatalf("must report that mandatory context cannot fit: %v", err)
	}
}

func TestPackBudgetPrefersChoiceOverCoverageIndex(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	tr.add(record.Decision, "design/x", "D-SERVES", strings.Repeat("Declared requirement coverage. ", 60))
	choice := tr.add(record.Decision, "design/x", "D1", "Keep offline operation because the installation cannot reach a hosted queue.")
	pk, err := Compile(tr.projection(), nil, Scope{Subjects: []string{"design/x"}}, 400, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Decisions) != 1 || pk.Decisions[0].ID != choice.ID || len(pk.Omissions) == 0 {
		t.Fatalf("budget retained a generated index instead of the reason: %+v", pk)
	}
}

func TestLiteralSelectionKeepsOnlyScopedExplanation(t *testing.T) {
	tr := &tree{t: t, files: store.MapSource{}}
	tr.add(record.Decision, "design/x", "decision.3.2.1", "the selected decision")
	d := tr.add(record.Document, "design/x", "design", "design source")
	body := []byte("# Design\n## 0 Read to rule\noperator-only\n### Read to rule guide\nhuman-only subtree\n### Routing\nhuman-only sibling\n## 3 Layout\n### 3.2 Queue\nselected explanation\n#### Rejected alternatives\nselected rejected alternative\n### 3.20 Unrelated\nunrelated explanation\n")
	d.LiteralBody, d.BodySHA256 = true, record.HashBody(body)
	saveRecord(t, tr, d)
	tr.files[d.BodyPath()] = body
	for _, scope := range []Scope{{Subjects: []string{"design/x"}}, {Subjects: []string{"design/x"}, Families: []string{"decision.3.2.1"}}} {
		pk, err := Compile(tr.projection(), nil, scope, 10000, nil)
		if err != nil {
			t.Fatal(err)
		}
		text := pk.Render()
		if strings.Contains(text, "operator-only") || strings.Contains(text, "human-only") || !strings.Contains(text, "selected rejected alternative") {
			t.Fatalf("lost scoped source or leaked human routing: %s", text)
		}
		if len(scope.Families) > 0 && strings.Contains(text, "unrelated explanation") {
			t.Fatalf("loaded unrelated section: %s", text)
		}
	}
}

func TestHumanRoutingDoesNotHideTheGoverningTopic(t *testing.T) {
	for _, heading := range []string{"0. Read to rule", "§0 — Read to rule"} {
		sections := docSections("# Read to rule (§0) — technical design\nGoverning section contract.\n## " + heading + "\nHuman routing only.\n### Summary\nStill human routing.\n## 1 Requirements\nBinding body.\n")
		if len(sections) != 2 || !strings.Contains(sections[0].text, "Governing section contract") || !strings.Contains(sections[1].text, "Binding body") {
			t.Fatalf("routing filter must exclude only section zero, not the subject itself: %+v", sections)
		}
	}
}

func TestLiteralFamilyIdentityIsDelimited(t *testing.T) {
	d := &record.Record{Subject: "prd/x"}
	refs := map[string]bool{"prd/x#FR_1": true}
	for _, tc := range []struct {
		text string
		want bool
	}{{"The FR_1 constraint matters.", true}, {"The FR_10 constraint differs.", false}} {
		if got := literalSectionSelected(d, docSection{text: tc.text}, Scope{}, refs); got != tc.want {
			t.Fatalf("identity selection for %q = %v", tc.text, got)
		}
	}
}
