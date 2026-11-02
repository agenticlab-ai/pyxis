// Package admission prepares one immutable published maintenance invocation
// without claiming or launching it.
package admission

import (
	"context"
	"errors"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const maxPublicationReads = 2

type ResultKind string

const (
	ResultPrepared ResultKind = "prepared"
	ResultSkipped  ResultKind = "skipped"
)

type SkipReason string

const (
	SkipDisabled SkipReason = "job_disabled"
	SkipRemoved  SkipReason = "job_removed"
)

type Control interface {
	RefStore(ref string) (*gitrefstore.Store, error)
	Close() error
}

// ExecutionSource is the non-secret storage identity needed to copy the
// captured commit into an independent execution repository.
type ExecutionSource struct {
	ControlRepository  string
	EnrolledRepository string
	EnrolledGitCommon  string
}

type executionControl interface {
	ExecutionSource() (ExecutionSource, error)
}

type Published struct {
	Commit  string
	Jobs    []byte
	Config  []byte
	Control Control
}

// RuntimeRoute is the admission-frozen native launch owner and its fidelity
// evidence. It is process-local and never serialized as policy.
type RuntimeRoute struct {
	Runtime port.AgentRuntime
	Launch  port.NativeLaunchSpec
	Capture port.LaunchCapture
}

type Publisher interface {
	Capture(context.Context, jobs.SourceBinding) (Published, error)
}

type Authority interface {
	Ready(context.Context) (installation.ReadyBinding, error)
	ValidateReady(context.Context, installation.ReadyBinding) error
	GuardReady(context.Context, installation.ReadyBinding, func() error) error
}

type Prepared struct {
	ConfigCommit  string
	DailyIdentity DailyIdentity
	Job           jobs.Job
	Catalog       nativerouting.ValidatedCatalog
	Selection     nativerouting.SelectedProfile
	Ready         installation.ReadyBinding
	StartedUTC    time.Time
	JobDate       string
	DeadlineUTC   time.Time
	Control       Control
	Route         RuntimeRoute
	authority     Authority
}

// ExecutionRoute returns the exact authenticated runtime and native launch
// selected during admission. Callers must not resolve mutable catalog state a
// second time.
func (p Prepared) ExecutionRoute() (port.AgentRuntime, port.NativeLaunchSpec, error) {
	if p.Route.Runtime == nil || p.Route.Launch.NativeModel == "" || p.Route.Launch.NativeEffort == "" ||
		!p.Route.Capture.MatchesSelected(p.Selection) {
		return nil, port.NativeLaunchSpec{}, errors.New("maintenance admission has no faithful execution route")
	}
	return p.Route.Runtime, p.Route.Launch, nil
}

// DailyIdentity is the comparable job/date authority within the remote-bound
// Control. Launch hour and mutable policy fields are deliberately absent.
type DailyIdentity struct {
	JobID   string
	JobDate string
}

func (p Prepared) ValidateReady(ctx context.Context) error {
	if p.authority == nil {
		return installation.ErrInstallationRequired
	}
	return p.authority.ValidateReady(ctx, p.Ready)
}

// GuardReady revalidates the captured installation authority and holds its
// transaction until the guarded remote operation has finished.
func (p Prepared) GuardReady(ctx context.Context, run func() error) error {
	if p.authority == nil {
		return installation.ErrInstallationRequired
	}
	return p.authority.GuardReady(ctx, p.Ready, run)
}

func (p Prepared) Close() error {
	if p.Control == nil {
		return nil
	}
	return p.Control.Close()
}

// ExecutionSource returns the admission-owned captured repository and the
// enrolled storage roots an execution attempt must remain outside.
func (p Prepared) ExecutionSource() (ExecutionSource, error) {
	control, ok := p.Control.(executionControl)
	if !ok {
		return ExecutionSource{}, errors.New("maintenance admission control does not expose a captured execution source")
	}
	return control.ExecutionSource()
}

type Result struct {
	Kind          ResultKind
	SkipReason    SkipReason
	JobID         string
	ConfigCommit  string
	Ready         installation.ReadyBinding
	DailyIdentity DailyIdentity
	JobDate       string
	Timezone      string
	Selection     *nativerouting.SelectedProfile
	Prepared      *Prepared
}

type Registry interface {
	ResolveConfigured(harness entity.Harness, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error)
}

type Service struct {
	Authority      Authority
	Publisher      Publisher
	Registry       Registry
	ProcessSupport func(context.Context) error
	Authenticate   func(context.Context, port.AgentRuntime) error
}

type Request struct {
	JobID      string
	StartedUTC time.Time
}
