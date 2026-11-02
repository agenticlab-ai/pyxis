package featuredrive

import (
	"context"
	"errors"
	"testing"
)

type candidateBuilderFunc func(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error)

func (f candidateBuilderFunc) Build(c context.Context, a, b string, p MergePolicy, v string) (CandidateIdentity, error) {
	return f(c, a, b, p, v)
}

type candidatePublisherFunc func(context.Context, string, CandidateIdentity) error

func (f candidatePublisherFunc) CreateOnly(ctx context.Context, ref string, c CandidateIdentity) error {
	return f(ctx, ref, c)
}

type candidatePublisher struct{ refs map[string]CandidateIdentity }

func (p *candidatePublisher) CreateOnly(_ context.Context, ref string, c CandidateIdentity) error {
	if p.refs == nil {
		p.refs = map[string]CandidateIdentity{}
	}
	if old, ok := p.refs[ref]; ok {
		if old != c {
			return ErrCandidateExists
		}
		return nil
	}
	p.refs[ref] = c
	return nil
}

func TestCandidateReadyRequiresEveryTaskLanded(t *testing.T) {
	if err := CandidateReady([]CandidateTask{{ID: "T1", Landed: true}, {ID: "T2"}}); !errors.Is(err, ErrCandidateBlocked) {
		t.Fatalf("err=%v", err)
	}
	if err := CandidateReady([]CandidateTask{{ID: "T1", Landed: true}, {ID: "T2", Landed: true}}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCandidateIsDeterministicAndCreateOnly(t *testing.T) {
	p := &candidatePublisher{}
	b := candidateBuilderFunc(func(_ context.Context, f, m string, policy MergePolicy, version string) (CandidateIdentity, error) {
		return CandidateIdentity{FeatureHead: f, MainHead: m, Commit: "C", Tree: "T", MergePolicyVersion: policy.Version, BuilderVersion: version}, nil
	})
	a, err := BuildCandidate(context.Background(), "feature/x", "F", "M", MergePolicy{Version: "merge/v1"}, "builder/v1", b, p)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := BuildCandidate(context.Background(), "feature/x", "F", "M", MergePolicy{Version: "merge/v1"}, "builder/v1", b, p)
	if err != nil {
		t.Fatal(err)
	}
	if a != b2 || len(p.refs) != 1 {
		t.Fatalf("candidate not deduplicated: %+v refs=%d", a, len(p.refs))
	}
	if _, err := BuildCandidate(context.Background(), "feature/x", "F2", "M", MergePolicy{Version: "merge/v1"}, "builder/v1", b, p); err != nil {
		t.Fatal(err)
	}
	if len(p.refs) != 2 {
		t.Fatalf("moved input did not create a new publication: %d", len(p.refs))
	}
}

func TestPublishCandidateCreatesTraceableRepairOnConflict(t *testing.T) {
	called := false
	b := candidateBuilderFunc(func(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error) {
		return CandidateIdentity{}, ErrCandidateConflict
	})
	_, err := PublishCandidate(context.Background(), "feature/x", "F", "M", MergePolicy{Version: "merge/v1"}, "builder/v1", b, &candidatePublisher{}, func(_ context.Context, r ConflictRepair) error {
		called = true
		if r.FeatureHead != "F" || r.MainHead != "M" || r.ID == "" || r.Report == "" {
			t.Fatalf("incomplete repair: %+v", r)
		}
		return nil
	})
	if !errors.Is(err, ErrCandidateConflict) || !called {
		t.Fatalf("err=%v repair=%v", err, called)
	}
}

func TestPublishCandidateKeepsConflictAndRepairFailureVisible(t *testing.T) {
	b := candidateBuilderFunc(func(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error) {
		return CandidateIdentity{}, ErrCandidateConflict
	})
	repairErr := errors.New("tracker unavailable")
	_, err := PublishCandidate(context.Background(), "feature/x", "F", "M", MergePolicy{Version: "merge/v1"}, "builder/v1", b, &candidatePublisher{}, func(context.Context, ConflictRepair) error {
		return repairErr
	})
	if !errors.Is(err, ErrCandidateConflict) || !errors.Is(err, ErrRepairCreation) || !errors.Is(err, repairErr) {
		t.Fatalf("conflict or repair failure was lost: %v", err)
	}
}

func TestCandidateBoundaryFailures(t *testing.T) {
	if _, err := (GitCandidateBuilder{}).Build(t.Context(), "feature", "main", MergePolicy{Version: "v1"}, "builder"); !errors.Is(err, ErrCandidateStale) {
		t.Fatalf("empty repository error = %v", err)
	}
	if _, err := NewGitCandidateBuilder(t.TempDir()).Build(t.Context(), "feature", "main", MergePolicy{Version: "v1"}, "builder"); err == nil {
		t.Fatal("GitCandidateBuilder hid a feature-head resolution failure")
	}
	if err := (GitCandidatePublisher{}).CreateOnly(t.Context(), "", CandidateIdentity{}); !errors.Is(err, ErrCandidateExists) {
		t.Fatalf("incomplete publication error = %v", err)
	}
	if _, err := BuildCandidate(t.Context(), "", "feature", "main", MergePolicy{Version: "v1"}, "builder", nil, nil); !errors.Is(err, ErrCandidateStale) {
		t.Fatalf("incomplete candidate error = %v", err)
	}

	valid := candidateBuilderFunc(func(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error) {
		return CandidateIdentity{FeatureID: "other", FeatureHead: "feature", MainHead: "main", Commit: "commit", Tree: "tree", MergePolicyVersion: "v1", BuilderVersion: "builder"}, nil
	})
	if _, err := BuildCandidate(t.Context(), "wanted", "feature", "main", MergePolicy{Version: "v1"}, "builder", valid, &candidatePublisher{}); !errors.Is(err, ErrCandidateStale) {
		t.Fatalf("mismatched builder identity error = %v", err)
	}

	valid = candidateBuilderFunc(func(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error) {
		return CandidateIdentity{FeatureHead: "feature", MainHead: "main", Commit: "commit", Tree: "tree", MergePolicyVersion: "v1", BuilderVersion: "builder"}, nil
	})
	publishErr := errors.New("publish failed")
	if _, err := BuildCandidate(t.Context(), "wanted", "feature", "main", MergePolicy{Version: "v1"}, "builder", valid, candidatePublisherFunc(func(context.Context, string, CandidateIdentity) error {
		return publishErr
	})); !errors.Is(err, publishErr) {
		t.Fatalf("publisher error = %v", err)
	}
}
