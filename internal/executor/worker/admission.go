package worker

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

// ImplementationAdmission reconstructs and admits one task route. The Worker
// receives only the opaque permit and presents it at the existing lease seam.
type ImplementationAdmission func(context.Context, port.Task) (routingevidence.BoundaryPermit, error)

// ImplementationOutcome closes the same admitted attempt after the Worker has
// reached a terminal class. The launch capability is retained only inside the
// Worker and is required by the canonical outcome service.
type ImplementationOutcome func(context.Context, routingevidence.AttemptTerminal) error

// ImplementationRoutePermit is the active route's claim and process-start
// authority. A plain BoundaryPermit can guard a claim in focused tests, but it
// cannot activate production: native launch identity and runtime ownership must
// derive from the same exact-read resolution.
type ImplementationRoutePermit interface {
	routingevidence.BoundaryPermit
	RuntimeOwnershipProof() (port.OwnershipProof, error)
	NativeLaunch(nativerouting.Role) (port.NativeLaunchSpec, error)
}

func WithImplementationAdmission(admit ImplementationAdmission) Option {
	return func(w *Worker) {
		w.implementationAdmission = admit
	}
}

func WithImplementationOutcome(complete ImplementationOutcome) Option {
	return func(w *Worker) {
		w.implementationOutcome = complete
	}
}
