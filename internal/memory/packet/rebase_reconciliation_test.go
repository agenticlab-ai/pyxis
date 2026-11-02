package packet

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestExplicitRebaseReconciliationRetainsOldAncestry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		relation string
		revoked  bool
		good     bool
	}{
		{"both heads explicitly reconciled", record.Supersedes, false, true},
		{"current head omitted", "", false, false},
		{"dependency is not replacement", record.DependsOn, false, false},
		{"withdrawn ancestor stays forbidden", record.Supersedes, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ancestor := newTree(t)
			old := ancestor.add(record.Need, "code/widget", "anchor", "original unit")
			base := ancestor.clone()
			base.n = 10
			var current *record.Record
			if tc.revoked {
				base.add(record.Revocation, old.Subject, "", "withdraw original", rel(record.Revokes, old.ID))
				current = base.add(record.Need, old.Subject, old.Family, "new unit")
			} else {
				current = base.add(record.Need, old.Subject, old.Family, "main unit", rel(record.Supersedes, old.ID))
			}
			head := base.clone()
			head.n = 30
			branch := head.add(record.Need, old.Subject, old.Family, "branch unit", rel(record.Supersedes, old.ID))
			links := []record.Relation{rel(record.Supersedes, branch.ID)}
			if tc.relation != "" {
				links = append(links, rel(tc.relation, current.ID))
			}
			merged := head.add(record.Need, old.Subject, old.Family, "exact integrated unit", links...)
			p := Compute(Input{Base: base.set(), Ancestor: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default(), Branch: "topic", BaseRef: "main"})
			if tc.good {
				if !p.OK() || len(p.Moved) != 0 || len(p.Superseded) != 1 || p.Superseded[0].Target.ID != current.ID || p.Superseded[0].By.ID != merged.ID {
					t.Fatalf("explicit reconciliation rejected: %s", p.Summary())
				}
			} else if p.OK() || !strings.Contains(codes(p.Blockers), "effective-set-moved") {
				t.Fatalf("unreconciled ancestry accepted: %s", p.Summary())
			}
		})
	}
}
