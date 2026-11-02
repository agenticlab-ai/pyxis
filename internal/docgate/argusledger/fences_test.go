package argusledger

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func TestFencedArchivesPreserveLedgerEntries(t *testing.T) {
	for _, fence := range []struct{ name, open, enclosed, close string }{
		{"tilde archive", "~~~~markdown", "~~~\n", "~~~~"},
		{"backtick archive", "````" + "markdown", "```" + "\n", "````"},
		{"indented archive", "   ~~~markdown", "```" + "\n", "   ~~~~  "},
	} {
		for _, subject := range []struct{ name, file, body, anchor, region string }{
			{"delivery", "docs/problems/foo/delivery.md", goodDelivery, "E", "Journal"},
			{"problem", "docs/problems/foo/problem.md", goodProblem, "A", "Authority entries"},
		} {
			t.Run(fence.name+"/"+subject.name, func(t *testing.T) {
				archive := fence.open + "\n" + fence.enclosed +
					"## Archived section\n## " + subject.region + "\n" +
					"### " + subject.anchor + "-99 — archived example\n" +
					"literal archived content\n" + fence.close + "\n"
				needle := "### " + subject.anchor + "-2"
				body := strings.Replace(subject.body, needle, archive+"\n"+needle, 1)
				l := mustParse(t, subject.file, body)
				wantFirst, wantLast := subject.anchor+"-1", subject.anchor+"-2"
				if len(l.Order) != 2 || l.Order[0] != wantFirst || l.Order[1] != wantLast {
					t.Fatalf("entry order = %v, want [%s %s]", l.Order, wantFirst, wantLast)
				}
				if !strings.Contains(string(l.Entries[wantFirst]), string(schema.Normalize([]byte(archive)))) {
					t.Fatal("archived content was changed or truncated")
				}
				if !strings.Contains(string(l.Entries[wantLast]), "second") {
					t.Fatal("entry following archive was lost")
				}
			})
		}
	}
}

func TestFencedLedgerMetadataIsNotLive(t *testing.T) {
	body := "~~~markdown\n| **Template version** | not-a-ledger/v99 |\n~~~\n" + goodDelivery
	mustParse(t, "docs/problems/foo/delivery.md", body)

	for _, fence := range []string{"~~~", "```"} {
		body := strings.Replace(goodDelivery, "## Journal", fence+"\n## Journal\n"+fence, 1)
		l, defects := Parse(testProblems, verb, "docs/problems/foo/delivery.md", []byte(body))
		if l != nil || len(defects) == 0 {
			t.Fatal("fenced Journal heading established a live region")
		}
	}
}

func TestFencedArchiveRequiresMatchingClose(t *testing.T) {
	for _, extra := range []string{"~~~", "````", "~~~~ trailing text"} {
		archive := "~~~~markdown\n" + extra + "\n### E-99 — still archived\n"
		body := strings.Replace(goodDelivery, "### E-2", archive+"\n### E-2", 1)
		l := mustParse(t, "docs/problems/foo/delivery.md", body)
		if len(l.Order) != 1 || l.Order[0] != "E-1" {
			t.Fatalf("mismatched or unclosed archive exposed entries: %v", l.Order)
		}
	}
}
