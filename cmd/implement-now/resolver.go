package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness"
	forgejopersist "github.com/agenticlab-ai/pyxis/internal/adapters/persist/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	githubpersist "github.com/agenticlab-ai/pyxis/internal/implreview/persist/github"
	"github.com/agenticlab-ai/pyxis/internal/implreview/process"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// ErrNoUsableBackend means no installed, authenticated provider can route the
// task. Provider login is the only setup the automatic path requires.
var ErrNoUsableBackend = errors.New("implement-now: no authenticated model backend is available")

var automaticHarnessPreference = []entity.Harness{entity.Codex, entity.Claude, entity.Cursor, entity.OpenCode}

func reviewHostAdapter(root, repository string, state publication.Store) (*githubpersist.HostAdapter, error) {
	cfg, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	binding, err := cfg.CodeHost(repository)
	if err != nil {
		return nil, err
	}
	if binding.Provider == integrations.Forgejo {
		return forgejopersist.NewHostAdapter(binding, state)
	}
	if binding.Provider == integrations.LocalGit {
		return nil, fmt.Errorf("%w: local Git review publication is not available in this build", codehost.ErrWorkflowUnavailable)
	}
	host := ""
	if cfg.Bindings.CodeHost != (integrations.Binding{}) {
		host = "github.com"
	}
	return githubpersist.NewHostAdapterWithTokenEnv(binding.Repository, binding.TokenEnv, state, host)
}

type backendSource struct {
	backends func() []entity.Harness
	build    func(entity.Harness) (port.AgentRuntime, error)
}

type backendProbe func(entity.Harness, port.AgentRuntime) error
type resumableBackendLookup func(context.Context, port.Task) (harness entity.Harness, ok bool, err error)

type taskBackendKey struct {
	source string
	number int
	id     string
}

type backendDecision struct {
	backend  entity.Harness
	resuming bool
}

type routedRuntime struct {
	port.AgentRuntime
	route port.ModelRoute
}

func (r *routedRuntime) ModelRoute() port.ModelRoute { return r.route }

type reviewHandoffRepairer struct {
	store        reviewobligation.Store
	scope        string
	repository   string
	pairRepairer *reviewinvoke.PairRepairer
}

func (r reviewHandoffRepairer) Pending(ctx context.Context) ([]dispatch.ReviewHandoff, error) {
	states, err := r.store.Open(ctx, r.scope)
	if err != nil {
		return nil, err
	}
	var out []dispatch.ReviewHandoff
	for _, state := range states {
		if state.Status != "handoff_pending" {
			continue
		}
		if state.Obligation.TaskNumber <= 0 {
			return nil, fmt.Errorf("review handoff %s lacks its tracker task number", state.Obligation.ID)
		}
		if err := verifyRequestedProfile(port.Task{ID: state.Obligation.TaskID, DemandID: state.Obligation.DemandID, DemandProfile: state.Obligation.DemandProfile, NativeRouting: state.Obligation.NativeRouting}); err != nil {
			return nil, fmt.Errorf("review handoff %s: %w", state.Obligation.ID, err)
		}
		out = append(out, dispatch.ReviewHandoff{ObligationID: state.Obligation.ID, Harness: state.Obligation.Harness, ProposalRef: state.Obligation.ProposalRef, Revision: state.Obligation.Revision, Task: port.Task{ID: state.Obligation.TaskID, Number: state.Obligation.TaskNumber, DemandID: state.Obligation.DemandID, DemandProfile: state.Obligation.DemandProfile, NativeRouting: state.Obligation.NativeRouting}})
	}
	return out, nil
}

func (r reviewHandoffRepairer) Complete(ctx context.Context, obligationID string) error {
	return r.store.CompleteHandoff(ctx, obligationID)
}

func (r reviewHandoffRepairer) RepairPairs(ctx context.Context) ([]dispatch.ReviewPairResult, error) {
	states, err := r.store.Open(ctx, r.scope)
	if err != nil {
		return nil, err
	}
	var out []dispatch.ReviewPairResult
	preflighted := false
	for _, state := range states {
		if state.Status != "pair_verified" {
			continue
		}
		pair := state.Pair
		result := worker.ReviewResult{ObligationID: state.Obligation.ID, Harness: state.Obligation.Harness,
			ProposalRef: state.Obligation.ProposalRef, Revision: state.Obligation.Revision,
			Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed,
			ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure}
		if pair == nil || r.pairRepairer == nil {
			result.Err = errors.New("pair-verified review obligation lacks its lifecycle repair evidence")
			out = append(out, dispatch.ReviewPairResult{TaskID: state.Obligation.TaskID, Result: result})
			continue
		}
		if !preflighted {
			if err := r.pairRepairer.Preflight(ctx, r.repository); err != nil {
				result.Err = err
				out = append(out, dispatch.ReviewPairResult{TaskID: state.Obligation.TaskID, Result: result})
				continue
			}
			preflighted = true
		}
		result, err = r.pairRepairer.Repair(ctx, state.Obligation, *pair, "")
		if err != nil {
			result.Err = err
		}
		out = append(out, dispatch.ReviewPairResult{TaskID: state.Obligation.TaskID, Result: result})
	}
	return out, nil
}

func newReviewHandoffRepairer(cfg config.Config, repoDir, repoSlug, remote, scope string) (dispatch.ReviewHandoffRepairer, error) {
	if !cfg.ReviewConfigured() {
		return nil, nil
	}
	store, err := reviewobligation.NewGitStore(repoDir, remote, cfg.ReviewEvidenceRef())
	if err != nil {
		return nil, err
	}
	pairRepairer, err := newReviewPairRepairer(cfg, repoDir, repoSlug, remote)
	if err != nil {
		return nil, err
	}
	return reviewHandoffRepairer{store: store, scope: scope, repository: repoSlug, pairRepairer: pairRepairer}, nil
}

func newReviewPairRepairer(cfg config.Config, repoDir, repoSlug, remote string) (*reviewinvoke.PairRepairer, error) {
	store, err := reviewobligation.NewGitStore(repoDir, remote, cfg.ReviewEvidenceRef())
	if err != nil {
		return nil, fmt.Errorf("review obligation store: %w", err)
	}
	publicationStore, err := publication.NewGitStore(repoDir, remote, cfg.ReviewPublicationEvidenceRef())
	if err != nil {
		return nil, fmt.Errorf("impl-review publication store: %w", err)
	}
	host, err := reviewHostAdapter(repoDir, repoSlug, publicationStore)
	if err != nil {
		return nil, fmt.Errorf("impl-review host persistence: %w", err)
	}
	verifier, err := reviewinvoke.NewStoreTerminalVerifier(store, publicationStore)
	if err != nil {
		return nil, fmt.Errorf("review terminal verifier: %w", err)
	}
	return reviewinvoke.NewPairRepairer(store, host, verifier)
}

// newRuntimeResolver builds the Dispatcher's local per-task model router. A
// task route or repository default wins, then the invoking integration, then a
// task's resumable backend, then the first compatible authenticated built-in
// backend in stable preference order. Once selected, its shipped
// implementation catalog maps semantic demand to model and effort without any
// network evidence or setup.
func newRuntimeResolver(ctx context.Context, cfg config.Config, ambientBackend string, hc harnessconfig.Config, resumeBackend resumableBackendLookup) dispatch.RuntimeResolver {
	source := backendSource{
		backends: runtime.Harnesses,
		build: func(backend entity.Harness) (port.AgentRuntime, error) {
			return buildBackend(backend, cfg, hc)
		},
	}
	return newRuntimeResolverFrom(ctx, cfg, ambientBackend, source, resumeBackend)
}

func newRuntimeResolverFrom(ctx context.Context, cfg config.Config, ambientBackend string, source backendSource, resumeBackend resumableBackendLookup) dispatch.RuntimeResolver {
	var selected struct {
		sync.Mutex
		byTask map[taskBackendKey]backendDecision
	}
	selected.byTask = make(map[taskBackendKey]backendDecision)
	evidenceActivated := false
	if _, ok := cfg.RoutingEvidenceActivation(); ok {
		evidenceActivated = true
	}

	return func(task port.Task) (entity.Harness, port.AgentRuntime, error) {
		backend, err := cfg.ResolveHarness(task, ambientBackend)
		if task.NativeRouting != nil {
			if routeErr := task.NativeRouting.Validate(); routeErr != nil {
				return "", nil, routeErr
			}
			backend, err = task.NativeRouting.Implementation.Harness, nil
		}
		resuming := false
		if err != nil {
			if strings.TrimSpace(ambientBackend) != "" || cfg.DefaultHarness() != "" || !errors.Is(err, config.ErrNoDefaultHarness) {
				return "", nil, err
			}
			err = nil
			key := taskBackendKey{source: task.SourceRef, number: task.Number, id: task.ID}
			selected.Lock()
			decision, alreadySelected := selected.byTask[key]
			if alreadySelected {
				backend, resuming = decision.backend, decision.resuming
			} else {
				if resumeBackend != nil {
					backend, resuming, err = resumeBackend(ctx, task)
					if err != nil {
						selected.Unlock()
						return "", nil, fmt.Errorf("read resumable backend for task %s: %w", task.ID, err)
					}
					if resuming && strings.TrimSpace(backend.String()) == "" {
						selected.Unlock()
						return "", nil, fmt.Errorf("read resumable backend for task %s: empty backend", task.ID)
					}
				}
				if !resuming {
					var probe backendProbe
					if !evidenceActivated {
						probe = func(candidate entity.Harness, rt port.AgentRuntime) error {
							_, routeErr := resolveImplementationModel(task, candidate, rt)
							return routeErr
						}
					}
					backend, err = discoverBackend(ctx, source, probe)
				}
				if err == nil {
					selected.byTask[key] = backendDecision{backend: backend, resuming: resuming}
				}
			}
			selected.Unlock()
			if err != nil {
				return "", nil, err
			}
		}
		rt, err := source.build(backend)
		if err == nil && rt == nil {
			err = errors.New("runtime builder returned nil")
		}
		if err != nil {
			if resuming {
				return "", nil, fmt.Errorf("resume task %s on recorded backend %s: %w", task.ID, backend, err)
			}
			return "", nil, err
		}
		if resuming {
			if err := rt.AuthCheck(ctx); err != nil {
				return "", nil, fmt.Errorf("resume task %s on recorded backend %s: %w", task.ID, backend, err)
			}
		}
		// Keep the already-merged high-assurance mode compatible while it is
		// dormant. When explicitly activated, its admission path owns the native
		// route. Ordinary installations always use the local decision below.
		if evidenceActivated {
			return backend, rt, nil
		}
		route, err := resolveImplementationModel(task, backend, rt)
		if err != nil {
			return "", nil, err
		}
		return backend, &routedRuntime{AgentRuntime: rt, route: route}, nil
	}
}

func discoverBackend(ctx context.Context, source backendSource, probe backendProbe) (entity.Harness, error) {
	if source.backends == nil || source.build == nil {
		return "", fmt.Errorf("%w: runtime registry is unavailable", ErrNoUsableBackend)
	}
	var failures []string
	for _, backend := range preferredBackends(source.backends()) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		rt, err := source.build(backend)
		if err == nil && rt == nil {
			err = errors.New("runtime builder returned nil")
		}
		if err == nil {
			err = rt.AuthCheck(ctx)
		}
		if err == nil && probe != nil {
			err = probe(backend, rt)
		}
		if err == nil {
			return backend, nil
		}
		failures = append(failures, backend.String()+": "+err.Error())
	}
	detail := "no registered backends"
	if len(failures) != 0 {
		detail = strings.Join(failures, "; ")
	}
	return "", fmt.Errorf("%w (%s); log in to one supported provider or set --ambient-backend", ErrNoUsableBackend, detail)
}

func preferredBackends(available []entity.Harness) []entity.Harness {
	set := make(map[entity.Harness]bool, len(available))
	for _, backend := range available {
		backend = entity.Harness(strings.TrimSpace(strings.ToLower(backend.String())))
		if backend != "" {
			set[backend] = true
		}
	}
	ordered := make([]entity.Harness, 0, len(set))
	for _, backend := range automaticHarnessPreference {
		if set[backend] {
			ordered = append(ordered, backend)
			delete(set, backend)
		}
	}
	rest := make([]entity.Harness, 0, len(set))
	for backend := range set {
		rest = append(rest, backend)
	}
	slices.Sort(rest)
	return append(ordered, rest...)
}

func resolveImplementationModel(task port.Task, backend entity.Harness, rt port.AgentRuntime) (port.ModelRoute, error) {
	if task.NativeRouting != nil {
		if err := task.NativeRouting.Validate(); err != nil {
			return port.ModelRoute{}, err
		}
		route := task.NativeRouting.Implementation
		if route.Harness != backend {
			return port.ModelRoute{}, fmt.Errorf("task %s native harness %s differs from selected %s", task.ID, route.Harness, backend)
		}
		native, catalog, err := validateModelCatalog(rt, nativerouting.RoleImplementation)
		if err != nil {
			return port.ModelRoute{}, err
		}
		selected, err := catalog.SelectExact(route.Model, route.Effort)
		if err != nil {
			return port.ModelRoute{}, err
		}
		if err := verifyModelSelection(native, nativerouting.Resolution{NativeProfile: &selected}); err != nil {
			return port.ModelRoute{}, err
		}
		return port.ModelRoute{Model: route.Model, Effort: route.Effort}, nil
	}
	profile, err := task.ImplementationDemand()
	if err != nil {
		return port.ModelRoute{}, err
	}
	native, catalog, err := validateModelCatalog(rt, nativerouting.RoleImplementation)
	if err != nil {
		return port.ModelRoute{}, fmt.Errorf("route implementation task %s: %w", task.ID, err)
	}
	resolution := (nativerouting.Resolver{}).Resolve(nativerouting.Request{
		SchemaVersion:    nativerouting.RequestSchema,
		AttemptID:        "local-implementation-" + task.ID,
		DemandID:         task.DemandID,
		Harness:          backend,
		Role:             nativerouting.RoleImplementation,
		RequestedProfile: profile,
		Mode:             nativerouting.ModeNormal,
	}, catalog)
	if err := verifyModelSelection(native, resolution); err != nil {
		return port.ModelRoute{}, fmt.Errorf("route implementation task %s: %w", task.ID, err)
	}
	selected := resolution.NativeProfile
	return port.ModelRoute{
		Model:  selected.NativeModel,
		Effort: selected.NativeEffort,
	}, nil
}

// buildBackend constructs the AgentRuntime for a resolved backend name. Claude
// receives WorkerTurns for its native --max-turns seam (§3.7). Every harness
// receives its model catalog from configuration.
func buildBackend(backend entity.Harness, cfg config.Config, hc harnessconfig.Config) (port.AgentRuntime, error) {
	return harness.New(backend, cfg, hc)
}

// buildReviewHarness constructs the independently configured review adapter.
// WorkerTurns is an implementation-session control and must not leak into the
// review catalog's qualified native launch.
func buildReviewHarness(backend entity.Harness, cfg config.Config, hc harnessconfig.Config) (port.AgentRuntime, error) {
	return harness.NewReview(backend, cfg, hc)
}

// newReviewFactory installs review only for an explicit executor.review block.
// Dispatcher supplies each already-selected explicit or inherited backend; the
// factory builds that backend independently through role=review.
func newReviewFactory(cfg config.Config, hc harnessconfig.Config, repoDir, repoSlug, remote, mainline, scope string, inspect reviewinvoke.ProposalInspector, log *slog.Logger, active *activeRouting) dispatch.ReviewInvokerFactory {
	if !cfg.ReviewConfigured() {
		return nil
	}
	store, storeErr := reviewobligation.NewGitStore(repoDir, remote, cfg.ReviewEvidenceRef())
	processStore, processStoreErr := process.NewGitStore(repoDir, remote, cfg.ReviewProcessEvidenceRef())
	publicationStore, publicationStoreErr := publication.NewGitStore(repoDir, remote, cfg.ReviewPublicationEvidenceRef())
	table, calibrationErr := calibration.LoadFile(filepath.Join(repoDir, ".pyxis", "skills", "impl-review", "calibration.md"))
	coordinator, coordinatorErr := process.New(processStore, table)
	host, hostErr := reviewHostAdapter(repoDir, repoSlug, publicationStore)
	terminalVerifier, terminalVerifierErr := reviewinvoke.NewStoreTerminalVerifier(store, publicationStore)
	return func(backend entity.Harness) (worker.ReviewService, error) {
		if storeErr != nil {
			return nil, fmt.Errorf("review obligation store: %w", storeErr)
		}
		if processStoreErr != nil {
			return nil, fmt.Errorf("impl-review process store: %w", processStoreErr)
		}
		if publicationStoreErr != nil {
			return nil, fmt.Errorf("impl-review publication store: %w", publicationStoreErr)
		}
		if calibrationErr != nil {
			return nil, fmt.Errorf("impl-review calibration: %w", calibrationErr)
		}
		if coordinatorErr != nil {
			return nil, fmt.Errorf("impl-review process coordinator: %w", coordinatorErr)
		}
		if hostErr != nil {
			return nil, fmt.Errorf("impl-review host persistence: %w", hostErr)
		}
		if terminalVerifierErr != nil {
			return nil, fmt.Errorf("review terminal verifier: %w", terminalVerifierErr)
		}
		if backend == "" {
			return nil, fmt.Errorf("review harness selection is empty")
		}
		rt, err := buildReviewHarness(backend, cfg, hc)
		if err != nil {
			return nil, fmt.Errorf("review harness %q: %w", backend, err)
		}
		inv, err := reviewinvoke.New(reviewinvoke.Dependencies{
			Runtime: rt, Harness: backend, RepositoryDir: repoDir, RepositorySlug: repoSlug,
			Remote: remote, Budget: cfg.ReviewBudget(), Processes: coordinator,
			Persistence: host, TerminalVerifier: terminalVerifier, Log: log,
		})
		if err != nil {
			return nil, err
		}
		if active != nil {
			admission, admissionErr := active.reviewAdmission(backend, rt)
			if admissionErr != nil {
				return nil, fmt.Errorf("review routing admission: %w", admissionErr)
			}
			outcome, outcomeErr := active.reviewOutcome(rt)
			if outcomeErr != nil {
				return nil, fmt.Errorf("review routing outcome: %w", outcomeErr)
			}
			inv.WithReviewAdmission(admission).WithReviewOutcome(outcome)
		}
		return inv.WithObligationStore(store).WithProposalInspector(inspect).WithScope(scope).WithMainline(mainline), nil
	}
}
