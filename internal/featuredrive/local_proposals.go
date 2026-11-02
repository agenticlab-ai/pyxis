package featuredrive

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

const (
	LocalProposalSchema = "local-feature-proposal-view/v1"
	LocalTaskMerge      = codehost.TaskMerge
	LocalFinalMerge     = "feature-final-merge"
)

// LocalProposalView presents the exact origin generation and workflow target
// to the drive and operator. It is a status observation, not review evidence
// or permission to land. Later preparation supplies the retained request.
type LocalProposalView struct {
	SchemaVersion string                      `json:"schema_version"`
	Operation     string                      `json:"operation"`
	Generation    codehost.ProposalGeneration `json:"generation"`
	Trust         codehost.Trust              `json:"trust"`
}

// LocalProposals is the drive's supported-call boundary. It can request open
// proposals and read origin state, but cannot declare approval or invoke a
// landing. Operator commands are composed separately. Under a shared OS
// account this separation is procedural, not a host security boundary.
// Ownership verification and persisted effect intents remain core predicates.
type LocalProposals interface {
	RequestTask(context.Context, codehost.ProposalChange) (LocalProposalView, error)
	RequestFinal(context.Context, codehost.ProposalChange) (LocalProposalView, error)
	ObserveTask(context.Context, string) (LocalProposalView, error)
	ObserveFinal(context.Context, string) (LocalProposalView, error)
}
