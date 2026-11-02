package readiness

import (
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
)

func authorityFixture(t *testing.T, tier string) Sources {
	t.Helper()
	body, err := os.ReadFile("testdata/authority/tier-2.md")
	if err != nil {
		t.Fatal(err)
	}
	switch tier {
	case "tier-3":
		body = []byte(strings.Replace(string(body), "**Current tier** | tier-2", "**Current tier** | tier-3", 1) + "| 2026-01-03 | promotion | tier-3 | operator grant |\n")
	case "revoked":
		body = []byte(strings.Replace(string(body), "**Current tier** | tier-2", "**Current tier** | revoked", 1) + "| 2026-01-03 | revocation | revoked | operator revocation |\n")
	}
	return Sources{Documents: map[string][]byte{schema.AuthorityContractPath: body}}
}

func rulingResult(t *testing.T, author string) compile.Result {
	t.Helper()
	files := readyFiles(t)
	files["problems.md"] = []byte("### P-1 — FR-1 — Retry owner\n\n**Problem:** Who owns retries?\n**Ruling:** " + author + " · The owner must be explicit. · Assign a retry owner.\n")
	refreshReviews(t, files)
	result, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	result.Changes = []compile.RequirementChange{{ID: "FR-1", Ruling: result.Problems[0].Ruling}}
	return result
}

func TestRulingAuthorityAndApprovalChanges(t *testing.T) {
	cases := []struct {
		name, author, tier string
		accepted           bool
	}{
		{"operator role", "operator", "", true},
		{"operator login", "@Configured-Operator", "", true},
		{"another login", "@someone-else", "", false},
		{"Argus granted", "Argus", "tier-3", true},
		{"Argus no grant", "Argus", "tier-2", false},
		{"Argus revoked", "Argus", "revoked", false},
		{"Argus no contract", "Argus", "", false},
		{"other agent", "review-agent", "tier-3", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := rulingResult(t, tc.author)
			var sources Sources
			if tc.tier != "" {
				sources = authorityFixture(t, tc.tier)
			}
			sources.OperatorLogin = "configured-operator"
			verdict := Judge(result, sources)
			changes := AuthorizedChanges(result, sources)
			if tc.accepted {
				if !verdict.Ready || changes[0].Unruled || changes[0].Ruling == nil {
					t.Fatalf("accepted ruling: verdict=%+v changes=%+v", verdict.Unmet, changes)
				}
			} else {
				if verdict.Ready || len(verdict.Unmet) != 2 || verdict.Unmet[0].Condition != "FR-10" || verdict.Unmet[0].Item != "prd/widget#P-1" || !strings.Contains(verdict.Unmet[0].Remedy, tc.author) || verdict.Unmet[1].Condition != "FR-12" || !changes[0].Unruled || changes[0].Ruling != nil {
					t.Fatalf("refused ruling: verdict=%+v changes=%+v", verdict.Unmet, changes)
				}
			}
			if result.Changes[0].Ruling == nil || result.Changes[0].Unruled {
				t.Fatal("approval view mutated compiler evidence")
			}
		})
	}
}

func TestMalformedAuthorityContractCannotGrant(t *testing.T) {
	result := rulingResult(t, "Argus")
	sources := authorityFixture(t, "tier-3")
	sources.Documents[schema.AuthorityContractPath] = []byte(strings.Replace(string(sources.Documents[schema.AuthorityContractPath]), "authority-contract/v1", "authority-contract/v2", 1))
	if verdict := Judge(result, sources); verdict.Ready || verdict.Unmet[0].Condition != "FR-10" {
		t.Fatalf("malformed grant passed: %+v", verdict.Unmet)
	}
}

func TestBlockingDeclineNeedsDeclaredOperatorRuling(t *testing.T) {
	for _, tc := range []struct {
		disposition string
		want        bool
	}{
		{"applied", true},
		{"declined — operator · Risk accepted for this draft.", true},
		{"Declined — Operator · Risk accepted for this draft.", true},
		{"declined — @configured-operator · Risk accepted for this draft.", true},
		{"declined — @someone-else · Risk accepted for this draft.", false},
		{"declined — operator accepts the risk", false},
		{"declined — Argus · Risk accepted for this draft.", false},
		{"declined — operator · TBD", false},
		{"declined — operator · ", false},
	} {
		t.Run(tc.disposition, func(t *testing.T) {
			files := readyFiles(t)
			files["reviews/001.md"] = append(files["reviews/001.md"], []byte("\n### FND-1 — Blocking concern\n\n- **Severity**: blocking\n- **Disposition**: "+tc.disposition+"\n")...)
			result, err := compile.Compile("widget", "main-commit", files)
			if err != nil {
				t.Fatal(err)
			}
			verdict := Judge(result, Sources{OperatorLogin: "configured-operator"})
			if verdict.Ready != tc.want {
				t.Fatalf("ready=%t want %t: %+v", verdict.Ready, tc.want, verdict.Unmet)
			}
		})
	}
}

// With no configured login, a ruling naming an account is not the operator's:
// only the role counts.
func TestOperatorLoginRecognitionNeedsConfiguration(t *testing.T) {
	result := rulingResult(t, "@configured-operator")
	if verdict := Judge(result, Sources{}); verdict.Ready {
		t.Fatal("an account ruling was accepted with no operator login configured")
	}
	if operatorAuthor("@", "") {
		t.Fatal("a bare @ matched the empty login")
	}
}
