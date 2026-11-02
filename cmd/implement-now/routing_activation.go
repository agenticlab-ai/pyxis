package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/gitstore"
)

type activeRouting struct {
	store      *gitstore.Adapter
	policy     routingevidence.ProducerPolicy
	admission  *routingevidence.AdmissionVerifier
	outcomes   routingevidence.SigningCapability
	privateKey [][]byte
}

type activationStateReader interface {
	ReadHead(context.Context) (routingevidence.ExactRead, error)
}

// activeRoutingDependencies keeps the activation boundary testable without
// weakening its production defaults. Each dependency is still the exact
// fail-closed implementation used outside tests.
type activeRoutingDependencies struct {
	readMainline   func(context.Context, string, string, string, string) ([]byte, []byte, error)
	decodePolicy   func([]byte) (routingevidence.ProducerPolicy, error)
	newStore       func(string, string, routingevidence.ProducerPolicy) (*gitstore.Adapter, error)
	verify         func(context.Context, activationStateReader, routingevidence.ProducerPolicy, string) error
	newSecrets     func() credentialstore.Store
	loadCapability func(context.Context, credentialstore.Store, routingevidence.ProducerPolicy, routingevidence.ProducerClass, []routingevidence.EventKind, []string) (routingevidence.SigningCapability, error)
	newAdmission   func(routingevidence.Store, routingevidence.ResolutionTerminalWriter) (*routingevidence.AdmissionVerifier, error)
	randRead       func([]byte) (int, error)
}

var activeRoutingDeps = activeRoutingDependencies{
	readMainline:   readActivationMainline,
	decodePolicy:   routingevidence.DecodeProducerPolicy,
	newStore:       gitstore.New,
	verify:         verifyActivationCheckpoint,
	newSecrets:     func() credentialstore.Store { return credentialstore.NewOSStore() },
	loadCapability: credentialstore.LoadProducerCapability,
	newAdmission:   routingevidence.NewAdmissionVerifier,
	randRead:       rand.Read,
}

func loadActiveRouting(ctx context.Context, cfg config.Config, repoDir, repository, remote, mainline string) (*activeRouting, error) {
	activation, enabled := cfg.RoutingEvidenceActivation()
	if !enabled {
		return nil, nil
	}
	mainlineConfig, policyWire, err := activeRoutingDeps.readMainline(ctx, repoDir, remote, mainline, activation.ProducerPolicyRef)
	if err != nil {
		return nil, fmt.Errorf("routing activation: human-merged mainline: %w", err)
	}
	mainlineExecutor, err := config.Parse(mainlineConfig)
	if err != nil {
		return nil, fmt.Errorf("routing activation: exact mainline executor config: %w", err)
	}
	mainlineActivation, ok := mainlineExecutor.RoutingEvidenceActivation()
	if !ok || !reflect.DeepEqual(mainlineActivation, activation) {
		return nil, errors.New("routing activation: worktree activation differs from exact human-merged mainline")
	}
	policy, err := activeRoutingDeps.decodePolicy(trimCanonicalNewline(policyWire))
	if err != nil {
		return nil, fmt.Errorf("routing activation: decode producer policy: %w", err)
	}
	if policy.Repository != repository || policy.EvidenceRef != activation.EvidenceRef {
		return nil, errors.New("routing activation: config and producer policy cross repository or evidence ref")
	}
	store, err := activeRoutingDeps.newStore(repoDir, remote, policy)
	if err != nil {
		return nil, fmt.Errorf("routing activation: evidence store: %w", err)
	}
	if err := activeRoutingDeps.verify(ctx, store, policy, activation.CheckpointEvent); err != nil {
		return nil, err
	}
	secrets := activeRoutingDeps.newSecrets()
	resolver, err := activeRoutingDeps.loadCapability(ctx, secrets, policy, routingevidence.ProducerResolver,
		[]routingevidence.EventKind{routingevidence.RoutingResolved, routingevidence.RoutingResolutionFailed}, []string{"implementation", "review"})
	if err != nil {
		return nil, fmt.Errorf("routing activation: resolver producer: %w", err)
	}
	attemptOwner, err := loadActiveAttemptOwner(ctx, secrets, policy)
	if err != nil {
		clear(resolver.PrivateKey)
		return nil, fmt.Errorf("routing activation: attempt-outcome producer: %w", err)
	}
	writer := routingevidence.SignedResolutionTerminalWriter{Store: store, Capability: resolver}
	admission, err := activeRoutingDeps.newAdmission(store, writer)
	if err != nil {
		clear(resolver.PrivateKey)
		clear(attemptOwner.PrivateKey)
		return nil, err
	}
	return &activeRouting{store: store, policy: policy, admission: admission,
		outcomes:   attemptOwner,
		privateKey: [][]byte{resolver.PrivateKey, attemptOwner.PrivateKey}}, nil
}

func loadActiveAttemptOwner(ctx context.Context, secrets credentialstore.Store, policy routingevidence.ProducerPolicy) (routingevidence.SigningCapability, error) {
	// Outcome events are roleless canonical terminals. Their implementation or
	// review role is recovered from the parent resolution, not repeated as a
	// producer-authorization role on the terminal envelope.
	return activeRoutingDeps.loadCapability(ctx, secrets, policy, routingevidence.ProducerAttemptOwner,
		[]routingevidence.EventKind{routingevidence.RoutedAttemptCompleted, routingevidence.RoutedTaskOutcome}, []string{""})
}

func (a *activeRouting) Close() {
	if a == nil {
		return
	}
	for _, key := range a.privateKey {
		clear(key)
	}
}

func verifyActivationCheckpoint(ctx context.Context, store activationStateReader, policy routingevidence.ProducerPolicy, checkpoint string) error {
	read, err := store.ReadHead(ctx)
	if err != nil {
		return fmt.Errorf("routing activation: exact-read evidence head: %w", err)
	}
	state, err := routingevidence.NewReducer(policy).Reduce(read.Events)
	if err != nil {
		return fmt.Errorf("routing activation: reduce evidence: %w", err)
	}
	state, err = routingevidence.BindEvidenceHead(state, read.ObservedHead)
	if err != nil {
		return fmt.Errorf("routing activation: bind evidence head: %w", err)
	}
	for _, activation := range state.Activations {
		if string(activation.EventID) == checkpoint {
			if !activation.ActivationEligible {
				return errors.New("routing activation: configured checkpoint is invalidated or not ready")
			}
			return nil
		}
	}
	return errors.New("routing activation: configured checkpoint is absent from current canonical evidence")
}

func (a *activeRouting) implementationFactory() dispatch.ImplementationAdmissionFactory {
	if a == nil {
		return nil
	}
	return func(rt port.AgentRuntime) (worker.ImplementationAdmission, error) {
		native, catalog, err := validateModelCatalog(rt, nativerouting.RoleImplementation)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, task port.Task) (routingevidence.BoundaryPermit, error) {
			attemptID, err := freshRoutingAttempt("implementation")
			if err != nil {
				return nil, err
			}
			profile, err := task.ImplementationDemand()
			if err != nil {
				return nil, err
			}
			request := nativerouting.Request{SchemaVersion: nativerouting.RequestSchema, AttemptID: attemptID, DemandID: task.DemandID,
				Harness: task.Harness, Role: nativerouting.RoleImplementation, RequestedProfile: profile, Mode: nativerouting.ModeNormal}
			resolution := (nativerouting.Resolver{}).Resolve(request, catalog)
			if err := verifyModelSelection(native, resolution); err != nil {
				return nil, err
			}
			return a.admission.AdmitImplementation(ctx, routingevidence.RouteIntent{AttemptID: attemptID, DemandID: task.DemandID, TaskID: task.ID,
				Harness: task.Harness, RequestedProfile: profile, Catalog: catalog, Mode: nativerouting.ModeNormal})
		}, nil
	}
}

func (a *activeRouting) implementationOutcomeFactory() dispatch.ImplementationOutcomeFactory {
	if a == nil {
		return nil
	}
	return func(rt port.AgentRuntime) (worker.ImplementationOutcome, error) {
		fenced, ok := rt.(port.FencedRuntime)
		if !ok {
			return nil, errors.New("activated implementation runtime has no outcome fence")
		}
		service, err := routingevidence.NewOutcomeService(a.store,
			routingevidence.SignedOutcomeWriter{Store: a.store, Capability: a.outcomes},
			routingevidence.FencedOutcomeAuthorizer{Runtime: fenced})
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, terminal routingevidence.AttemptTerminal) error {
			_, err := service.CompleteAttempt(ctx, terminal)
			return err
		}, nil
	}
}

func (a *activeRouting) reviewAdmission(backend entity.Harness, rt port.AgentRuntime) (reviewinvoke.ReviewAdmission, error) {
	if a == nil {
		return nil, nil
	}
	native, catalog, err := validateModelCatalog(rt, nativerouting.RoleReview)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, task port.Task, proposalRef, revision string, route reviewinvoke.Route) (routingevidence.BoundaryPermit, error) {
		attemptID, err := freshRoutingAttempt("review")
		if err != nil {
			return nil, err
		}
		profile, err := task.ReviewDemand()
		if err != nil {
			return nil, err
		}
		request := nativerouting.Request{SchemaVersion: nativerouting.RequestSchema, AttemptID: attemptID, DemandID: task.DemandID,
			Harness: backend, Role: nativerouting.RoleReview, RequestedProfile: profile, Mode: nativerouting.ModeNormal}
		resolution := (nativerouting.Resolver{}).Resolve(request, catalog)
		if err := verifyModelSelection(native, resolution); err != nil {
			return nil, err
		}
		selected := resolution.NativeProfile
		if selected == nil || selected.NativeProfileID != route.NativeProfileID || selected.NativeModel != route.Model || selected.NativeEffort != route.Effort {
			return nil, errors.New("activated review route differs from independently resolved native selection")
		}
		return a.admission.AdmitReview(ctx, routingevidence.RouteIntent{AttemptID: attemptID, DemandID: task.DemandID, TaskID: task.ID,
			Harness: backend, RequestedProfile: profile, Catalog: catalog, Mode: nativerouting.ModeNormal,
			ProposalID: proposalRef, ReviewAdmissionKey: task.ID + "\x00" + proposalRef + "\x00" + revision})
	}, nil
}

func (a *activeRouting) reviewOutcome(rt port.AgentRuntime) (reviewinvoke.ReviewOutcome, error) {
	if a == nil {
		return nil, nil
	}
	fenced, ok := rt.(port.FencedRuntime)
	if !ok {
		return nil, errors.New("activated review runtime has no outcome fence")
	}
	service, err := routingevidence.NewOutcomeService(a.store,
		routingevidence.SignedOutcomeWriter{Store: a.store, Capability: a.outcomes},
		routingevidence.FencedOutcomeAuthorizer{Runtime: fenced})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, terminal routingevidence.AttemptTerminal) error {
		_, err := service.CompleteAttempt(ctx, terminal)
		return err
	}, nil
}

func freshRoutingAttempt(role string) (string, error) {
	b := make([]byte, 16)
	if _, err := activeRoutingDeps.randRead(b); err != nil {
		return "", fmt.Errorf("routing %s attempt identity: %w", role, err)
	}
	return role + "-" + hex.EncodeToString(b), nil
}

func readActivationMainline(ctx context.Context, repoDir, remote, mainline, policyPath string) ([]byte, []byte, error) {
	repoDir, remote, mainline = strings.TrimSpace(repoDir), strings.TrimSpace(remote), strings.TrimSpace(mainline)
	if repoDir == "" || remote == "" || mainline == "" || strings.HasPrefix(remote, "-") || strings.HasPrefix(mainline, "-") ||
		strings.ContainsAny(remote+mainline, "\x00\r\n") {
		return nil, nil, errors.New("repository, remote, and mainline are required")
	}
	head, err := activationGit(ctx, repoDir, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, nil, err
	}
	remoteHead, err := activationGit(ctx, repoDir, "ls-remote", "--exit-code", remote, "refs/heads/"+mainline)
	if err != nil {
		return nil, nil, err
	}
	fields := strings.Fields(string(remoteHead))
	if len(fields) != 2 || fields[1] != "refs/heads/"+mainline || fields[0] != strings.TrimSpace(string(head)) {
		return nil, nil, errors.New("local HEAD is not the exact current remote mainline object")
	}
	status, err := activationGit(ctx, repoDir, "status", "--porcelain=v1", "--untracked-files=all", "--", ".pyxis/config.yaml", policyPath)
	if err != nil {
		return nil, nil, err
	}
	if len(status) != 0 {
		return nil, nil, errors.New("activation config or producer policy differs from committed mainline")
	}
	configWire, err := activationGit(ctx, repoDir, "show", "HEAD:.pyxis/config.yaml")
	if err != nil {
		return nil, nil, err
	}
	policyWire, err := activationGit(ctx, repoDir, "show", "HEAD:"+filepath.ToSlash(policyPath))
	if err != nil {
		return nil, nil, err
	}
	return configWire, policyWire, nil
}

func activationGit(ctx context.Context, repoDir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoDir}, args...)...)
	cmd.Env = activationGitEnvironment(os.Environ())
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return []byte(stdout.String()), nil
}

func activationGitEnvironment(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR":
			continue
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func trimCanonicalNewline(b []byte) []byte { return []byte(strings.TrimSuffix(string(b), "\n")) }
