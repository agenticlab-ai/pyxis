package payload_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func TestInitialAuthorityContractUsesTemplateLadderAndFloorBirth(t *testing.T) {
	template := read(t, payload.AuthorityContractTemplate)
	before := bytes.Clone(template)
	body, err := payload.InitialAuthorityContract(template, "2026-10-07")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, template) {
		t.Fatal("instantiation changed the template")
	}
	contract, defects := authoritycontract.Parse("test", payload.AuthorityContractTarget, body)
	if len(defects) != 0 {
		t.Fatalf("parse: %v", defects)
	}
	if contract.CurrentTier != "floor" || len(contract.History) != 1 || contract.History[0].Date != "2026-10-07" || contract.History[0].Move != "none" || contract.History[0].Tier != "floor" {
		t.Fatalf("unexpected birth: %+v", contract)
	}
	for _, row := range contract.Ladder {
		if row.Tier == "floor" && (len(row.Tokens) != 1 || row.Tokens[0] != "dispose-advisory-finding") {
			t.Fatalf("unexpected floor grant: %+v", row)
		}
	}
	for _, unexpected := range []string{"<repo>", "<initial tier>", "<YYYY", "Guidance paragraphs", "## Appendix", "*Ordered tiers", "*Append-only"} {
		if bytes.Contains(body, []byte(unexpected)) {
			t.Fatalf("uninstantiated guidance: %s", unexpected)
		}
	}
	// A reviewed template wording change remains the authority data source.
	changed := strings.Replace(string(template), "the kill switch", "the emergency stop", 1)
	custom, err := payload.InitialAuthorityContract([]byte(changed), "2026-10-08")
	if err != nil || !bytes.Contains(custom, []byte("the emergency stop")) {
		t.Fatalf("ladder did not come from template: %v", err)
	}
}

func TestInitialAuthorityContractRefusesMalformedInputs(t *testing.T) {
	template := string(read(t, payload.AuthorityContractTemplate))
	for _, tc := range []struct{ name, body, date string }{
		{"date", template, "2026-02-30"},
		{"missing repository placeholder", strings.Replace(template, "<repo>", "operand", 1), "2026-10-07"},
		{"duplicated pointer", template + "\n| **Current tier** | <a tier name declared in the ladder below> |\n", "2026-10-07"},
		{"missing birth", strings.Replace(template, "<YYYY-MM-DD>", "today", 1), "2026-10-07"},
		{"unknown version", strings.Replace(template, "authority-contract/v1", "authority-contract/v9", 1), "2026-10-07"},
		{"unknown action", strings.Replace(template, "dispose-advisory-finding", "override-human", 1), "2026-10-07"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if body, err := payload.InitialAuthorityContract([]byte(tc.body), tc.date); err == nil || body != nil {
				t.Fatalf("malformed seed admitted: %s, %v", body, err)
			}
		})
	}
}
