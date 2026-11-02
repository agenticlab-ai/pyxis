package reconcile

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// ValidateSourceDecisionProposal recognizes only an exact source-only native
// Decision projection of the selected captured design. It uses the captured
// bytes, because the editable source may have changed since the proposal was
// generated. Code-associated Decisions and added semantic fields are excluded.
// The caller separately proves that both identities are unlanded and that the
// complete selected graph has no surviving incoming relations.
func ValidateSourceDecisionProposal(r, document *record.Record, body []byte) error {
	if r == nil || document == nil || document.Kind != record.Document || !document.LiteralBody || !document.HasTag("reconcile:source") || document.Family != "design" || !strings.HasPrefix(document.Subject, "design/") {
		return fmt.Errorf("companion lacks a selected literal design source Document")
	}
	file := File{Path: document.Provenance.Source, Body: body, SHA256: document.BodySHA256}
	design := Design{Slug: strings.TrimPrefix(document.Subject, "design/"), Source: file.Path, SHA256: file.SHA256, Status: "live"}
	blocks, err := ExtractLiveDesignDecisions(design, file)
	if err != nil {
		return err
	}
	for _, block := range blocks {
		if block.Subject == r.Subject && block.Family == r.Family {
			return validateDesignDecisionMemory(r, block, document, nil)
		}
	}
	return fmt.Errorf("companion has no exact captured Decision block")
}
