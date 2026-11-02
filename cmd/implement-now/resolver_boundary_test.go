package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
)

func configuredReview(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte("executor:\n  review:\n    backend: codex\n"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestRuntimeResolverFailsClosedWithoutAnAvailableBackend(t *testing.T) {
	resolve := newRuntimeResolverFrom(context.Background(), config.Defaults(), "", backendSource{
		backends: func() []entity.Harness { return nil },
		build:    func(entity.Harness) (port.AgentRuntime, error) { return nil, errors.New("unexpected build") },
	}, nil)
	if _, _, err := resolve(port.Task{ID: "T1"}); !errors.Is(err, ErrNoUsableBackend) {
		t.Fatalf("runtime resolver error = %v, want ErrNoUsableBackend", err)
	}
}

func TestReviewRepairerConstructorsSurfaceInvalidRepositoryState(t *testing.T) {
	cfg := configuredReview(t)
	if _, err := newReviewHandoffRepairer(cfg, "", "owner/repo", "origin", "feature/001"); err == nil {
		t.Fatal("handoff repairer accepted an empty repository directory")
	}
	if _, err := newReviewPairRepairer(cfg, "", "owner/repo", "origin"); err == nil || !strings.Contains(err.Error(), "review obligation store") {
		t.Fatalf("pair repairer error = %v", err)
	}

	repo := implementReviewGitFixture(t)
	if _, err := newReviewPairRepairer(cfg, repo, "", "origin"); err == nil || !strings.Contains(err.Error(), "host persistence") {
		t.Fatalf("pair repairer repository identity error = %v", err)
	}
}

func TestReviewFactoryReportsDeferredConstructionFailures(t *testing.T) {
	cfg := configuredReview(t)
	logger := slog.New(slog.DiscardHandler)

	factory := newReviewFactory(cfg, fixtureHarnessConfig(t), "", "owner/repo", "origin", "main", "feature/001", nil, logger, nil)
	if _, err := factory("codex"); err == nil || !strings.Contains(err.Error(), "review obligation store") {
		t.Fatalf("invalid store error = %v", err)
	}

	repoWithoutCalibration := implementReviewGitFixture(t)
	factory = newReviewFactory(cfg, fixtureHarnessConfig(t), repoWithoutCalibration, "owner/repo", "origin", "main", "feature/001", nil, logger, nil)
	if _, err := factory("codex"); err == nil || !strings.Contains(err.Error(), "calibration") {
		t.Fatalf("missing calibration error = %v", err)
	}

	repo := reviewFactoryRepoDir(t)
	factory = newReviewFactory(cfg, fixtureHarnessConfig(t), repo, "owner/repo", "origin", "main", "feature/001", nil, logger, nil)
	if _, err := factory(""); err == nil || !strings.Contains(err.Error(), "harness selection") {
		t.Fatalf("empty harness error = %v", err)
	}
	if _, err := factory("not-a-backend"); err == nil || !strings.Contains(err.Error(), "review harness") {
		t.Fatalf("unknown harness error = %v", err)
	}

	factory = newReviewFactory(cfg, fixtureHarnessConfig(t), repo, "owner/repo", "origin", "main", "feature/001", nil, logger, &activeRouting{})
	if service, err := factory("codex"); err != nil || service == nil {
		t.Fatalf("active routing factory = %T, %v", service, err)
	}
}

func TestReviewHandoffRepairerRunsConfiguredPairRepairBoundary(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{HarnessMode: e2eHarnessImplement, Review: true})
	pairRepairer, err := newReviewPairRepairer(configuredReview(t), fixture.repo, e2eRepository, "origin")
	if err != nil {
		t.Fatal(err)
	}
	obligation := reviewobligation.Obligation{ID: "review-1", TaskID: "T1", TaskNumber: 1, Harness: "codex", ProposalRef: "github:owner/repository#77", Revision: "abc"}
	pair := reviewobligation.PublicationPair{ObligationID: obligation.ID, AttemptID: "attempt-1", PublicationAttemptID: "publication-1", ProposalRef: obligation.ProposalRef, Revision: obligation.Revision}
	store := &resolverStore{states: []reviewobligation.State{{Obligation: obligation, Status: "pair_verified", Pair: &pair}}}
	repairer := reviewHandoffRepairer{store: store, repository: e2eRepository, pairRepairer: pairRepairer}
	results, err := repairer.RepairPairs(t.Context())
	if err != nil || len(results) != 1 {
		t.Fatalf("RepairPairs = %#v, %v", results, err)
	}
}
