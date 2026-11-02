package attribution

import (
	"fmt"
	"testing"
)

func featureItem(id int, body, signals string) string {
	return fmt.Sprintf("---\nid: %d\ntype: feature\nstate: open\ntitle: Test feature\nsignals: %s\nassignees: []\nparent: —\nblocked_by: []\ncreated: 2026-09-28T00:00:00Z\nupdated: 2026-09-28T00:00:00Z\nclose_reason:\npriority: medium\n---\n\n%s\n\n## Log\n", id, signals, body)
}

func TestGit_CarinaItemScopeUsesOriginalAndCurrentOwner(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.write(".workitems/items/8.md", featureItem(8, "Argus problem acceptance-and-proposal. Initial item.", "[]"))
	r.write(".workitems/items/7.md", featureItem(7, "Argus problem go-no-go-review. Initial item.", "[]"))
	base := r.commit("chore: seed feature items")

	r.write(".workitems/items/8.md", featureItem(8, "Argus problem acceptance-and-proposal. Parked brief: delivery:acceptance-and-proposal#E-2.", "[hold]"))
	// Rewriting the other item's current pointer cannot change its original owner.
	r.write(".workitems/items/7.md", featureItem(7, "Argus problem acceptance-and-proposal. Spoofed current owner.", "[hold]"))
	r.write("internal/engine/core.go", "package engine\n")
	r.commitArgus("docs: reconcile parked feature", "acceptance-and-proposal")

	got := r.run(base)
	if _, refused := got[".workitems/items/8.md"]; refused {
		t.Fatalf("verified own feature item refused: %v", got)
	}
	if got[".workitems/items/7.md"] != "out-of-scope" {
		t.Fatalf("other feature item passed after current-body spoof: %v", got)
	}
	if got["internal/engine/core.go"] != "out-of-scope" {
		t.Fatalf("unrelated code path became in scope: %v", got)
	}
}

func TestGit_CarinaItemScopeRejectsOwnerChangeAtBase(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.write(".workitems/items/7.md", featureItem(7, "Argus problem go-no-go-review. Initial item.", "[]"))
	base := r.commit("chore: seed with other problem's item")
	r.write(".workitems/items/7.md", featureItem(7, "Argus problem acceptance-and-proposal. Spoofed owner.", "[hold]"))
	r.commitArgus("docs: rewrite other item", "acceptance-and-proposal")
	if got := r.run(base); got[".workitems/items/7.md"] != "out-of-scope" {
		t.Fatalf("base-owned other feature item passed: %v", got)
	}
}

func TestGit_CarinaItemScopeRejectsMalformedCurrentItem(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.write(".workitems/items/8.md", featureItem(8, "Argus problem acceptance-and-proposal. Initial item.", "[]"))
	base := r.commit("chore: seed own feature item")
	r.write(".workitems/items/8.md", "not a Carina item\n")
	r.commitArgus("docs: corrupt item", "acceptance-and-proposal")
	if got := r.run(base); got[".workitems/items/8.md"] != "out-of-scope" {
		t.Fatalf("malformed current item passed: %v", got)
	}
}

func TestGit_CarinaItemScopeRejectsSameProposalOwnershipClaim(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")
	r.write(".workitems/items/8.md", featureItem(8, "Argus problem acceptance-and-proposal. Initial item.", "[]"))
	r.commit("chore: create item inside proposal")
	r.write(".workitems/items/8.md", featureItem(8, "Argus problem acceptance-and-proposal. Parked brief.", "[hold]"))
	r.commitArgus("docs: update same-proposal item", "acceptance-and-proposal")
	if got := r.run(base); got[".workitems/items/8.md"] != "out-of-scope" {
		t.Fatalf("same-proposal owner assertion widened scope: %v", got)
	}
}

func TestCarinaItemOwnerRecognizesOriginalTrackerPointers(t *testing.T) {
	for _, tc := range []struct {
		name, target, body, want string
	}{
		{"company intake", ".workitems/items/5.md", "Argus intake: docs/problems/company-memory/problem.md. Parked brief follows.", "company-memory"},
		{"feature problem", ".workitems/items/8.md", "Argus problem acceptance-and-proposal. Dependencies follow.", "acceptance-and-proposal"},
		{"later log cannot claim", ".workitems/items/8.md", "Unrelated item.\nArgus problem acceptance-and-proposal.", ""},
		{"wrong numeric id", ".workitems/items/7.md", "Argus problem acceptance-and-proposal.", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := 8
			if tc.target == ".workitems/items/5.md" {
				id = 5
			}
			if got := carinaItemOwner(dcfg, tc.target, []byte(featureItem(id, tc.body, "[]"))); got != tc.want {
				t.Fatalf("owner = %q, want %q", got, tc.want)
			}
		})
	}
}
