package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

func TestCoverage90GreenFactoryEmitsLocalAttempt(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  caps:\n    ci_self_heal_attempts: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	dir := codeGateWorkdir(t, "code-gate:\n  always-red:\n    command: false\n")
	emitter := &capturingEmitter{}
	driver, err := newGreenFactory(coverageGreenHost{}, cfg, emitter, "origin")(coverageAgentRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	_, driveErr := driver.DriveLocal(t.Context(), port.Task{ID: "T1"}, &healWorkdir{dir: dir, branch: "task"})
	if driveErr == nil {
		t.Fatal("permanently red local gate unexpectedly converged")
	}
	if len(emitter.events) == 0 {
		t.Fatal("local fix attempt emitted no event")
	}
}

type coverageGreenHost struct{}

func (coverageGreenHost) OpenProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (port.Proposal, error) {
	return port.Proposal{}, nil
}
func (coverageGreenHost) Checks(context.Context, port.Proposal) (port.CheckState, error) {
	return port.CheckRed, nil
}
func (coverageGreenHost) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return port.LandingOpen, "head", port.CheckRed, nil
}

func TestCoverage90ResolverAndReviewFactoryBoundaries(t *testing.T) {
	const simpleBackend = "coverage-implement-now-simple"
	executorruntime.Register(simpleBackend, func() (port.AgentRuntime, error) { return coverageAgentRuntime{}, nil })
	if rt, err := buildBackend(simpleBackend, config.Defaults(), fixtureHarnessConfig(t)); err != nil || rt == nil {
		t.Fatalf("custom backend = %T, %v", rt, err)
	}

	cfg := configuredReview(t)
	repo := implementReviewGitFixture(t)
	if _, err := newReviewHandoffRepairer(cfg, repo, "", "origin", "feature/001"); err == nil {
		t.Fatal("handoff repairer accepted invalid host identity after constructing its store")
	}

	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true})
	pairRepairer, err := newReviewPairRepairer(cfg, fixture.repo, e2eRepository, "origin")
	if err != nil {
		t.Fatal(err)
	}
	obligation := reviewobligation.Obligation{ID: "review-1", TaskID: "T1", TaskNumber: 1, Harness: "codex", ProposalRef: "github:owner/repository#77", Revision: "abc"}
	pair := reviewobligation.PublicationPair{ObligationID: obligation.ID, AttemptID: "attempt-1", PublicationAttemptID: "publication-1", ProposalRef: obligation.ProposalRef, Revision: obligation.Revision}
	repairer := reviewHandoffRepairer{store: &resolverStore{states: []reviewobligation.State{{Obligation: obligation, Status: "pair_verified", Pair: &pair}}}, repository: "wrong/repository", pairRepairer: pairRepairer}
	results, err := repairer.RepairPairs(t.Context())
	if err != nil || len(results) != 1 || results[0].Result.Err == nil {
		t.Fatalf("preflight repair results = %#v, %v", results, err)
	}

	logger := slog.New(slog.DiscardHandler)
	factory := newReviewFactory(cfg, fixtureHarnessConfig(t), reviewFactoryRepoDir(t), "owner/repo", "origin", "main", "feature/001", nil, logger, &activeRouting{})
	if _, err := factory(simpleBackend); err == nil || !strings.Contains(err.Error(), "routing admission") {
		t.Fatalf("non-native active reviewer error = %v", err)
	}

	const nativeBackend = "coverage-implement-now-native"
	catalog, qualifications := coverageActiveCatalogFixture(nativerouting.RoleReview)
	native := &coverageNativeRuntime{catalog: catalog, qualifications: qualifications}
	executorruntime.Register(nativeBackend, func() (port.AgentRuntime, error) { return native, nil })
	if _, err := factory(nativeBackend); err == nil || !strings.Contains(err.Error(), "routing outcome") {
		t.Fatalf("unfenced active reviewer error = %v", err)
	}
}

func TestCoverage90ActivationMainlineAndFactoryErrors(t *testing.T) {
	if activeRoutingDeps.newSecrets() == nil {
		t.Fatal("default credential store is nil")
	}
	if _, _, err := readActivationMainline(t.Context(), t.TempDir(), "origin", "main", "policy.json"); err == nil {
		t.Fatal("non-git activation checkout succeeded")
	}

	noRemote := coverageActivationRepo(t, true, true, false)
	if _, _, err := readActivationMainline(t.Context(), noRemote, "upstream", "trunk", ".pyxis/routing/producer-policy.json"); err == nil {
		t.Fatal("activation without a remote succeeded")
	}
	valid := coverageActivationRepo(t, true, true, true)
	if _, _, err := readActivationMainline(t.Context(), valid, "upstream", "trunk", "bad\x00path"); err == nil {
		t.Fatal("activation with an invalid policy path succeeded")
	}
	missingConfig := coverageActivationRepo(t, false, false, true)
	if _, _, err := readActivationMainline(t.Context(), missingConfig, "upstream", "trunk", ".pyxis/routing/producer-policy.json"); err == nil {
		t.Fatal("activation without committed config succeeded")
	}
	missingPolicy := coverageActivationRepo(t, true, false, true)
	if _, _, err := readActivationMainline(t.Context(), missingPolicy, "upstream", "trunk", ".pyxis/routing/producer-policy.json"); err == nil {
		t.Fatal("activation without committed policy succeeded")
	}

	original := activeRoutingDeps
	t.Cleanup(func() { activeRoutingDeps = original })
	activeRoutingDeps.randRead = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	implCatalog, implQualifications := coverageActiveCatalogFixture(nativerouting.RoleImplementation)
	implNative := &coverageNativeRuntime{catalog: implCatalog, qualifications: implQualifications}
	active := &activeRouting{}
	admit, err := active.implementationFactory()(implNative)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admit(t.Context(), coverageActiveRoutingTask()); err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("implementation attempt error = %v", err)
	}
	reviewCatalog, reviewQualifications := coverageActiveCatalogFixture(nativerouting.RoleReview)
	reviewNative := &coverageNativeRuntime{catalog: reviewCatalog, qualifications: reviewQualifications}
	reviewAdmit, err := active.reviewAdmission("codex", reviewNative)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewAdmit(t.Context(), coverageActiveRoutingTask(), "12", "abc", reviewRouteFor(reviewCatalog)); err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("review attempt error = %v", err)
	}
	if _, err := active.reviewAdmission("codex", coverageAgentRuntime{}); err == nil {
		t.Fatal("active review accepted a non-native runtime")
	}

	activeRoutingDeps.randRead = func(p []byte) (int, error) {
		for i := range p {
			p[i] = byte(i)
		}
		return len(p), nil
	}
	reviewNative.buildErr = errors.New("build failed")
	reviewAdmit, err = active.reviewAdmission("codex", reviewNative)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewAdmit(t.Context(), coverageActiveRoutingTask(), "12", "abc", reviewRouteFor(reviewCatalog)); !errors.Is(err, reviewNative.buildErr) {
		t.Fatalf("review selection error = %v", err)
	}
}

func reviewRouteFor(catalog nativerouting.Catalog) reviewinvoke.Route {
	entry := catalog.Entries[0]
	return reviewinvoke.Route{NativeProfileID: entry.NativeProfileID, Model: entry.NativeModel, Effort: entry.NativeEffort}
}

func coverageActivationRepo(t *testing.T, withConfig, withPolicy, withRemote bool) string {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	if withRemote {
		runRoutingGit(t, root, "init", "--bare", "-q", remote)
	}
	runRoutingGit(t, root, "init", "-q", "-b", "trunk", repo)
	runRoutingGit(t, repo, "config", "user.name", "Pyxis Test")
	runRoutingGit(t, repo, "config", "user.email", "pyxis@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withConfig || withPolicy {
		if err := os.MkdirAll(filepath.Join(repo, ".pyxis", "routing"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if withConfig {
		if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("executor:\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if withPolicy {
		if err := os.WriteFile(filepath.Join(repo, ".pyxis", "routing", "producer-policy.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runRoutingGit(t, repo, "add", ".")
	runRoutingGit(t, repo, "commit", "-qm", "base")
	if withRemote {
		runRoutingGit(t, repo, "remote", "add", "upstream", remote)
		runRoutingGit(t, repo, "push", "-q", "upstream", "HEAD:refs/heads/trunk")
	}
	return repo
}

func TestCoverage90RunEarlyAndSelectedReviewBoundaries(t *testing.T) {
	t.Run("identity stale path and workspace", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		seedGitHubBudgetCutover(t)
		repo := t.TempDir()
		legacyDir := filepath.Join(repo, ".pyxis", "state")
		if err := os.MkdirAll(legacyDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacyDir, "github-content-budget.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", t.TempDir())
		var stdout, stderr bytes.Buffer
		if code := run([]string{"--repo", repo, "--repo-slug", "owner/repo", "--ambient-backend", "codex"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "prepare workspace") {
			t.Fatalf("run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	repo := implementReviewGitFixture(t)
	store, err := reviewobligation.NewGitStore(repo, "origin", reviewobligation.DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	appendObligation := func(id, backend string, pending bool) {
		t.Helper()
		o := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: id, Repository: "owner/repo", Scope: "feature/001", TaskID: "T-" + id, TaskNumber: 1, ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: entity.Harness(backend)}
		if err := store.Append(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		if pending {
			if err := store.CompleteHandoff(t.Context(), id); err != nil {
				t.Fatal(err)
			}
		}
	}
	appendObligation("not-pending", "codex", false)
	appendObligation("same-1", "codex", true)
	appendObligation("same-2", "codex", true)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runSelected := func(cfg config.Config, ids ...string) (int, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runSelectedReviews(t.Context(), &capturingEmitter{}, &stdout, &stderr, cfg, fixtureHarnessConfig(t), repo, "owner/repo", "origin", "main", "feature/001", nil, "round-1", ids, logger, nil)
		return code, stderr.String()
	}
	if code, stderr := runSelected(config.Defaults(), "not-pending"); code != 1 || !strings.Contains(stderr, "not pending") {
		t.Fatalf("not-pending = %d stderr=%q", code, stderr)
	}
	if code, _ := runSelected(config.Defaults(), "same-1", "same-2"); code != 1 {
		t.Fatalf("same-backend setup = %d", code)
	}

	t.Run("review-only entry", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		seedGitHubBudgetCutover(t)
		var stdout, stderr bytes.Buffer
		code := run([]string{"--repo", repo, "--repo-slug", "owner/repo", "--review-only", "--plan-ref", "feature/001", "--drive-round-id", "round-1", "--review-obligation", "missing", "--ambient-backend", "codex"}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "unavailable") {
			t.Fatalf("review-only = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("configured reviewer rejects unknown backend", func(t *testing.T) {
		fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true})
		cfg, err := config.Load(fixture.repo)
		if err != nil {
			t.Fatal(err)
		}
		reviewStore, err := reviewobligation.NewGitStore(fixture.repo, "origin", cfg.ReviewEvidenceRef())
		if err != nil {
			t.Fatal(err)
		}
		obligation := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: "unknown-backend", Repository: e2eRepository,
			Scope: "feature/001", TaskID: "T1", TaskNumber: 1, ProposalRef: "github:owner/repository#77", Revision: "abc", Harness: "not-a-backend"}
		if err := reviewStore.Append(t.Context(), obligation); err != nil {
			t.Fatal(err)
		}
		if err := reviewStore.CompleteHandoff(t.Context(), obligation.ID); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := runSelectedReviews(t.Context(), &capturingEmitter{}, &stdout, &stderr, cfg, fixtureHarnessConfig(t), fixture.repo,
			e2eRepository, "origin", "main", obligation.Scope, nil, "round-1", []string{obligation.ID}, logger, nil)
		if code != 1 || !strings.Contains(stdout.String(), "review-failed") {
			t.Fatalf("unknown backend = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("plan-scoped github tracker", func(t *testing.T) {
		fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
		var stdout, stderr bytes.Buffer
		code := run(append(fixture.args(), "--plan-ref", "feature/001"), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("plan-scoped run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("content-budget path failure", func(t *testing.T) {
		fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
		t.Setenv("XDG_STATE_HOME", "relative-state")
		var stdout, stderr bytes.Buffer
		code := run(fixture.args(), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "resolve the content-budget ledgers") {
			t.Fatalf("run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("routing activation wiring failure", func(t *testing.T) {
		fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement})
		configDir := filepath.Join(fixture.repo, ".pyxis")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), coverageActivationConfigWire(strings.Repeat("a", 64)), 0o644); err != nil {
			t.Fatal(err)
		}

		original := activeRoutingDeps
		activeRoutingDeps.readMainline = func(context.Context, string, string, string, string) ([]byte, []byte, error) {
			return nil, nil, errors.New("activation unavailable")
		}
		t.Cleanup(func() { activeRoutingDeps = original })

		var stdout, stderr bytes.Buffer
		code := run(fixture.args(), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "initialize routing-evidence activation") {
			t.Fatalf("run = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
}

func (coverageGreenHost) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
