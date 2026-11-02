package localgit

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	featurelocal "github.com/agenticlab-ai/pyxis/internal/adapters/featuredrive/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func NewFeatureProposals(ctx context.Context, options Options, drive featuredrive.FeatureDrive) (featuredrive.LocalProposals, error) {
	return featurelocal.NewProposals(ctx, drive,
		NewProposals(ProposalOptions{Git: options, Kind: "implementation", IntegrationRef: drive.IntegrationRef}),
		NewProposals(ProposalOptions{Git: options, Kind: "implementation", IntegrationRef: drive.TargetMainRef, HeadRef: drive.IntegrationRef}),
	)
}

func featureProposalFixture(t *testing.T) (Options, featuredrive.FeatureDrive, featuredrive.LocalProposals) {
	t.Helper()
	o := gitFixture(t, true)
	transition := featuredrive.Transition{Kind: "create", NewVersion: 1}
	d := featuredrive.FeatureDrive{
		Schema: featuredrive.Schema, Repository: "local-fixture", FeatureID: "widget", PlanFingerprint: "approved-plan",
		TargetMainRef: "refs/heads/trunk", IntegrationRef: "refs/heads/feature/widget", Version: 1,
		Created: transition, Last: transition, Ownership: featuredrive.Ownership{State: featuredrive.Unowned}, Phase: featuredrive.Integrating,
	}
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", "upstream", base+":"+d.IntegrationRef)
	port, err := NewFeatureProposals(t.Context(), o, d)
	if err != nil {
		t.Fatal(err)
	}
	return o, d, port
}

func TestFeatureProposalRequestsAndCloneObservationsNeverLand(t *testing.T) {
	t.Parallel()
	o, d, port := featureProposalFixture(t)
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "task work")
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	task := codehost.ProposalChange{ID: "task-widget-t1", Feature: d.FeatureID, Task: "T1", Branch: "refs/heads/task/widget-t1", Head: head, State: "open", ExpectedTargetOID: base}
	opened, err := port.RequestTask(t.Context(), task)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := port.RequestTask(t.Context(), task)
	if err != nil || retry.Generation.OID != opened.Generation.OID {
		t.Fatalf("task request retry changed generation: %+v %v", retry, err)
	}
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "task feedback")
	task.Head = fixtureGit(t, o.Root, "rev-parse", "HEAD")
	task.ExpectedProposalOID, task.ExpectedBranchOID = opened.Generation.OID, head
	updated, err := port.RequestTask(t.Context(), task)
	if err != nil || updated.Generation.Proposal.Sequence != 2 || updated.Generation.Proposal.PreviousOID != opened.Generation.OID {
		t.Fatalf("task feedback did not retain its generation: %+v %v", updated, err)
	}
	final, err := port.RequestFinal(t.Context(), codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: base, State: "open", ExpectedTargetOID: base})
	if err != nil {
		t.Fatal(err)
	}
	if fixtureGit(t, o.Origin, "rev-parse", d.IntegrationRef) != base || fixtureGit(t, o.Origin, "rev-parse", d.TargetMainRef) != base {
		t.Fatal("drive proposal request changed an integration destination")
	}
	if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/receipts", "refs/pyxis/review-requests", "refs/pyxis/proposal-archives") != "" {
		t.Fatal("drive proposal request created approval or landing evidence")
	}
	clone := filepath.Join(t.TempDir(), "clone")
	fixtureGit(t, filepath.Dir(clone), "clone", "--origin", "upstream", o.Origin, clone)
	o.Root = clone
	reader, err := NewFeatureProposals(t.Context(), o, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		want    featuredrive.LocalProposalView
		observe func(context.Context, string) (featuredrive.LocalProposalView, error)
		target  string
	}{
		{updated, reader.ObserveTask, d.IntegrationRef},
		{final, reader.ObserveFinal, d.TargetMainRef},
	} {
		before := fixtureGit(t, o.Origin, "show-ref")
		seen, err := tc.observe(t.Context(), tc.want.Generation.Proposal.ID)
		if err != nil || seen != tc.want || seen.SchemaVersion != featuredrive.LocalProposalSchema || seen.Generation.Proposal.TargetRef != tc.target || seen.Trust != (codehost.Trust{SharedAccount: true}) {
			t.Fatalf("clone lost exact proposal or trust boundary: %+v %v", seen, err)
		}
		if fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("proposal observation changed origin refs")
		}
	}
}

func TestFeatureProposalRequestsRefuseForeignScopeAndApprovalWrites(t *testing.T) {
	t.Parallel()
	o, d, port := featureProposalFixture(t)
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	valid := codehost.ProposalChange{ID: "task-widget-t1", Feature: d.FeatureID, Task: "T1", Branch: "refs/heads/task/widget-t1", Head: base, State: "open", ExpectedTargetOID: base}
	for _, tc := range []struct {
		name   string
		mutate func(*codehost.ProposalChange)
		final  bool
	}{
		{"other feature", func(c *codehost.ProposalChange) { c.Feature = "other" }, false},
		{"missing task", func(c *codehost.ProposalChange) { c.Task = "" }, false},
		{"final has task", func(*codehost.ProposalChange) {}, true},
		{"feature alias", func(c *codehost.ProposalChange) { c.Branch = d.IntegrationRef }, false},
		{"final feature alias", func(c *codehost.ProposalChange) { c.Task, c.Branch = "", d.IntegrationRef }, true},
		{"main alias", func(c *codehost.ProposalChange) { c.Branch = d.TargetMainRef }, false},
		{"prepare", func(c *codehost.ProposalChange) { c.State = "prepared" }, false},
		{"approval request", func(c *codehost.ProposalChange) { c.ReviewRequest = &codehost.DraftReviewRequest{} }, false},
		{"abandonment", func(c *codehost.ProposalChange) { c.State = "abandoned" }, false},
		{"invalid head", func(c *codehost.ProposalChange) { c.Head = "unknown" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			change := valid
			tc.mutate(&change)
			before := fixtureGit(t, o.Origin, "show-ref")
			request := port.RequestTask
			if tc.final {
				request = port.RequestFinal
			}
			if _, err := request(t.Context(), change); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("unsafe request accepted: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refused request changed origin refs")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := port.RequestTask(ctx, valid); !errors.Is(err, context.Canceled) {
		t.Fatalf("request cancellation lost: %v", err)
	}
}

func TestFeatureProposalObservationsRefuseForeignFeatureAndPurpose(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, feature, task string
		final               bool
	}{
		{"foreign task", "other", "T1", false},
		{"task missing identity", "widget", "", false},
		{"foreign final", "other", "", true},
		{"final with task identity", "widget", "T1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, d, port := featureProposalFixture(t)
			target, observe := d.IntegrationRef, port.ObserveTask
			if tc.final {
				target, observe = d.TargetMainRef, port.ObserveFinal
			}
			store := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: target})
			base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			if _, err := store.Publish(t.Context(), codehost.ProposalChange{ID: "foreign", Feature: tc.feature, Task: tc.task, Branch: "refs/heads/proposal/foreign", Head: base, State: "open", ExpectedTargetOID: base}); err != nil {
				t.Fatal(err)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := observe(t.Context(), "foreign"); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("foreign proposal became drive state: %v", err)
			}
			if _, err := observe(t.Context(), "missing"); !errors.Is(err, codehost.ErrProposalNotFound) {
				t.Fatalf("missing proposal error lost: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refused observation changed origin refs")
			}
		})
	}
}

func TestFeatureProposalBindingRequiresApprovedDriveDestinations(t *testing.T) {
	t.Parallel()
	o, d, _ := featureProposalFixture(t)
	for _, mutate := range []func(*featuredrive.FeatureDrive){
		func(d *featuredrive.FeatureDrive) { d.Schema = "unknown" },
		func(d *featuredrive.FeatureDrive) { d.TargetMainRef = "refs/heads/other" },
		func(d *featuredrive.FeatureDrive) { d.IntegrationRef = d.TargetMainRef },
		func(d *featuredrive.FeatureDrive) { d.IntegrationRef = "feature/widget" },
		func(d *featuredrive.FeatureDrive) { d.IntegrationRef = "refs/heads/bad..ref" },
	} {
		bad := d
		mutate(&bad)
		if _, err := NewFeatureProposals(t.Context(), o, bad); err == nil {
			t.Fatal("invalid drive destination accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewFeatureProposals(ctx, o, d); !errors.Is(err, context.Canceled) {
		t.Fatalf("binding cancellation lost: %v", err)
	}
}

func TestFeatureFinalRequestRequiresCurrentIntegrationHead(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"stale", "unrelated", "missing feature"} {
		t.Run(failure, func(t *testing.T) {
			o, d, port := featureProposalFixture(t)
			base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "current feature result")
			current := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			fixtureGit(t, o.Root, "push", o.Remote, current+":"+d.IntegrationRef)
			head := base
			switch failure {
			case "unrelated":
				head = fixtureGit(t, o.Root, "commit-tree", base+"^{tree}", "-p", base, "-m", "unrelated work")
			case "missing feature":
				fixtureGit(t, o.Origin, "update-ref", "-d", d.IntegrationRef)
			}
			change := codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: head, State: "open", ExpectedTargetOID: base}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := port.RequestFinal(t.Context(), change); !errors.Is(err, codehost.ErrProposalConflict) {
				t.Fatalf("final request accepted an outdated feature head: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refused final request published a stale proposal")
			}
			if failure != "missing feature" {
				change.Head = current
				got, err := port.RequestFinal(t.Context(), change)
				if err != nil || got.Generation.Proposal.Head != current || got.Generation.Proposal.TargetOID != base {
					t.Fatalf("current final proposal refused: %+v %v", got, err)
				}
			}
		})
	}
}

func TestFeatureFinalRequestHoldsIntegrationHeadDuringPublication(t *testing.T) {
	t.Parallel()
	o, d, _ := featureProposalFixture(t)
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	tasks := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.IntegrationRef})
	final := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef})
	port, err := featurelocal.NewProposals(t.Context(), d, tasks, final)
	if err != nil {
		t.Fatal(err)
	}
	clone := proposalClone(t, o)
	fixtureGit(t, clone.Root, "commit", "--allow-empty", "-m", "feature moves during final publication")
	next := fixtureGit(t, clone.Root, "rev-parse", "HEAD")
	entered := false
	final.push = func(ctx context.Context, root string, args ...string) error {
		entered = true
		if _, err := proposalGit(ctx, clone.Root, nil, nil, "push", o.Origin, next+":"+d.IntegrationRef); !refLockRejected(err) {
			t.Fatalf("final publication did not pin the feature head: %v", err)
		}
		_, err := proposalGit(ctx, root, nil, nil, args...)
		return err
	}
	change := codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: base, State: "open", ExpectedTargetOID: base}
	if got, err := port.RequestFinal(t.Context(), change); err != nil || !entered || got.Generation.Proposal.Head != base {
		t.Fatalf("final publication lost its source binding: %+v %v", got, err)
	}
	fixtureGit(t, clone.Root, "push", o.Origin, next+":"+d.IntegrationRef)
	if _, err := port.RequestFinal(t.Context(), change); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("old final request accepted after feature movement: %v", err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	change.ID, change.Branch, change.Head = "source-alias", d.IntegrationRef, next
	if _, err := final.Publish(t.Context(), change); !errors.Is(err, codehost.ErrProposalInvalid) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("proposal provider allowed writing its workflow source: %v", err)
	}
}

func TestFeatureProposalBindingRejectsMissingOrMisconfiguredPorts(t *testing.T) {
	t.Parallel()
	o, d, _ := featureProposalFixture(t)
	tasks := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.IntegrationRef})
	final := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef})
	for _, tc := range []struct {
		name         string
		tasks, final codehost.ScopedProposals
	}{
		{"missing tasks", nil, final},
		{"missing final", tasks, nil},
		{"wrong task kind", NewProposals(ProposalOptions{Git: o, Kind: "draft"}), final},
		{"missing final source", tasks, NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.TargetMainRef})},
		{"draft with source", tasks, NewProposals(ProposalOptions{Git: o, Kind: "draft", HeadRef: d.IntegrationRef})},
		{"unreadable final origin", tasks, NewProposals(ProposalOptions{Git: Options{Root: t.TempDir()}, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := featurelocal.NewProposals(t.Context(), d, tc.tasks, tc.final); err == nil {
				t.Fatal("incomplete proposal composition accepted")
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("failed composition wrote origin refs")
			}
		})
	}
}

func TestFeatureFinalObservationRefusesMovedOrMissingWorkflowHeads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ mainline, missing bool }{{false, false}, {false, true}, {true, false}, {true, true}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			o, d, port := featureProposalFixture(t)
			base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			change := codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: base, State: "open", ExpectedTargetOID: base}
			if _, err := port.RequestFinal(t.Context(), change); err != nil {
				t.Fatal(err)
			}
			if _, err := port.ObserveFinal(t.Context(), change.ID); err != nil {
				t.Fatal(err)
			}
			clone := proposalClone(t, o)
			cloned, err := NewFeatureProposals(t.Context(), clone, d)
			if err != nil {
				t.Fatal(err)
			}
			changedRef := d.IntegrationRef
			if tc.mainline {
				changedRef = d.TargetMainRef
			}
			if tc.missing {
				fixtureGit(t, o.Origin, "update-ref", "-d", changedRef)
			} else {
				fixtureGit(t, clone.Root, "commit", "--allow-empty", "-m", "workflow input advances after final proposal")
				fixtureGit(t, clone.Root, "push", o.Origin, "HEAD:"+changedRef)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			want := codehost.ErrProposalConflict
			if tc.mainline && tc.missing {
				want = codehost.ErrOrigin
			}
			for _, observer := range []featuredrive.LocalProposals{port, cloned} {
				if _, err := observer.ObserveFinal(t.Context(), change.ID); !errors.Is(err, want) {
					t.Fatalf("stale final generation remained eligible: %v", err)
				}
				if _, err := observer.RequestFinal(t.Context(), change); !errors.Is(err, want) {
					t.Fatalf("idempotent retry accepted stale workflow inputs: %v", err)
				}
			}
			for _, options := range []Options{o, clone} {
				finals := NewProposals(ProposalOptions{Git: options, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef})
				if _, err := finals.List(t.Context()); !errors.Is(err, want) {
					t.Fatalf("listing accepted stale workflow inputs: %v", err)
				}
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("final observation changed origin refs")
			}
		})
	}
}
