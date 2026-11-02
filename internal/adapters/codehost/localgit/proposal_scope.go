package localgit

import (
	"context"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

var _ codehost.ScopedProposals = (*ProposalStore)(nil)

func (s *ProposalStore) Scope(ctx context.Context) (codehost.ProposalScope, error) {
	r, err := s.inspect(ctx)
	if err != nil {
		return codehost.ProposalScope{}, err
	}
	if s.options.HeadRef != "" && (s.options.Kind != "implementation" || s.options.HeadRef == s.targetRef() || s.options.HeadRef == r.MainlineRef) {
		return codehost.ProposalScope{}, invalidProposal("workflow source must be a distinct implementation ref")
	}
	refs := []string{s.targetRef()}
	if s.options.HeadRef != "" {
		refs = append(refs, s.options.HeadRef)
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "refs/heads/") {
			return codehost.ProposalScope{}, invalidProposal("workflow destinations must be canonical head refs")
		}
		if err := checkRefFormat(ctx, s.options.Git.Root, ref); err != nil {
			return codehost.ProposalScope{}, invalidProposal("invalid workflow destination")
		}
	}
	return codehost.ProposalScope{
		SchemaVersion: codehost.ProposalScopeSchema, Origin: r.Origin, Kind: s.options.Kind,
		MainlineRef: r.MainlineRef, TargetRef: s.targetRef(), HeadRef: s.options.HeadRef,
	}, nil
}
