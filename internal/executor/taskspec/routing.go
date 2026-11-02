package taskspec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

type Routing struct {
	ID           executiondemand.DemandID
	Requirements *taskrequirements.Spec
}

// RoutingData preserves planned requirements without choosing an execution
// route. Identity binds the charter, both role assessments/profiles and source.
func (s Spec) RoutingData(identity DemandIdentity) (Routing, error) {
	if s.Requirements != nil {
		if strings.TrimSpace(identity.Repository) == "" {
			return Routing{}, fmt.Errorf("native task repository is required")
		}
		if identity.Task != "" && !strings.HasSuffix(s.PlanTaskID, "/"+identity.Task) {
			return Routing{}, fmt.Errorf("native task identity %q differs from tracker task %q", s.PlanTaskID, identity.Task)
		}
		body, err := RenderNative(s)
		if err != nil {
			return Routing{}, err
		}
		digest := sha256.Sum256([]byte("planned-task-identity/v1\x00" + identity.Repository + "\x00" + body))
		return Routing{ID: executiondemand.DemandID(hex.EncodeToString(digest[:])), Requirements: s.Requirements}, nil
	}
	return Routing{}, fmt.Errorf("task lacks current planning requirements; replan before execution")
}
