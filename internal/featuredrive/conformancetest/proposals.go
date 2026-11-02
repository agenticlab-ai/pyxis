// Package conformancetest owns the feature drive's proposal-port contract.
// Adapters supply their production port and medium-specific fixture operations.
package conformancetest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type ProposalFixture struct {
	Port                  featuredrive.LocalProposals
	Task, Final           codehost.ProposalChange
	UpdatedHead           string
	Integration, Mainline string
	Trust                 codehost.Trust
	Clone                 func(*testing.T) featuredrive.LocalProposals
	Targets               func(*testing.T) [2]string
	State, AuthorityState func(*testing.T) string
	SeedForeign           func(*testing.T, codehost.ProposalChange, bool)
}

// RunProposals fixes the assertions for every implementation of LocalProposals.
// The factory creates a fresh origin for each case. No backing-medium package
// or medium-specific ref, API, credential, or transport is used here.
func RunProposals(t *testing.T, factory func(*testing.T) ProposalFixture) {
	t.Helper()
	t.Run("request update and clone observation", func(t *testing.T) {
		f := factory(t)
		targets, authority := f.Targets(t), f.AuthorityState(t)
		task, err := f.Port.RequestTask(t.Context(), f.Task)
		expect(t, err == nil, "task request: %v", err)
		checkView(t, task, f.Task, f.Integration, "", featuredrive.LocalTaskMerge, f.Trust)
		retry, err := f.Port.RequestTask(t.Context(), f.Task)
		expect(t, err == nil && reflect.DeepEqual(retry, task), "retry changed the generation: %v", err)
		change := f.Task
		change.Head, change.ExpectedProposalOID, change.ExpectedBranchOID = f.UpdatedHead, task.Generation.OID, task.Generation.Proposal.Head
		updated, err := f.Port.RequestTask(t.Context(), change)
		expect(t, err == nil, "task update: %v", err)
		checkView(t, updated, change, f.Integration, "", featuredrive.LocalTaskMerge, f.Trust)
		expect(t, updated.Generation.Proposal.Sequence == task.Generation.Proposal.Sequence+1 && updated.Generation.Proposal.PreviousOID == task.Generation.OID, "update lost its monotonic predecessor")
		_, err = f.Port.RequestTask(t.Context(), f.Task)
		expect(t, errors.Is(err, codehost.ErrProposalConflict), "stale update accepted: %v", err)
		final, err := f.Port.RequestFinal(t.Context(), f.Final)
		expect(t, err == nil, "final request: %v", err)
		checkView(t, final, f.Final, f.Mainline, f.Integration, featuredrive.LocalFinalMerge, f.Trust)
		expect(t, f.Targets(t) == targets && f.AuthorityState(t) == authority, "proposal requests performed approval or landing effects")
		clone := f.Clone(t)
		before := f.State(t)
		seenTask, taskErr := clone.ObserveTask(t.Context(), f.Task.ID)
		seenFinal, finalErr := clone.ObserveFinal(t.Context(), f.Final.ID)
		expect(t, taskErr == nil && finalErr == nil && reflect.DeepEqual(seenTask, updated) && reflect.DeepEqual(seenFinal, final), "clone lost exact proposal or trust fields: %v / %v", taskErr, finalErr)
		expect(t, f.State(t) == before, "observation wrote shared state")
	})
	t.Run("scope and authority refusals", func(t *testing.T) {
		f := factory(t)
		for _, mutate := range []func(*codehost.ProposalChange){
			func(c *codehost.ProposalChange) { c.Feature += "-foreign" },
			func(c *codehost.ProposalChange) { c.Task = "" },
			func(c *codehost.ProposalChange) { c.Branch = f.Integration },
			func(c *codehost.ProposalChange) { c.Branch = f.Mainline },
			func(c *codehost.ProposalChange) { c.State = "prepared" },
			func(c *codehost.ProposalChange) { c.State = "abandoned" },
			func(c *codehost.ProposalChange) { c.ReviewRequest = &codehost.DraftReviewRequest{} },
		} {
			change := f.Task
			mutate(&change)
			before := f.State(t)
			_, err := f.Port.RequestTask(t.Context(), change)
			expect(t, errors.Is(err, codehost.ErrProposalInvalid) && f.State(t) == before, "unsafe task request mutated state: %v", err)
		}
		before := f.State(t)
		_, err := f.Port.RequestFinal(t.Context(), f.Task)
		expect(t, errors.Is(err, codehost.ErrProposalInvalid), "final request accepted a task: %v", err)
		for _, ref := range []string{f.Integration, f.Mainline} {
			change := f.Final
			change.Branch = ref
			_, err := f.Port.RequestFinal(t.Context(), change)
			expect(t, errors.Is(err, codehost.ErrProposalInvalid), "final request aliased a destination: %v", err)
		}
		expect(t, f.State(t) == before, "unsafe final request mutated state")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = f.Port.RequestTask(ctx, f.Task)
		expect(t, errors.Is(err, context.Canceled) && f.State(t) == before, "cancelled request wrote state: %v", err)
	})
	t.Run("foreign and missing observations", func(t *testing.T) {
		f := factory(t)
		for _, final := range []bool{false, true} {
			change, observe := f.Task, f.Port.ObserveTask
			if final {
				change, observe = f.Final, f.Port.ObserveFinal
			}
			change.Feature += "-foreign"
			f.SeedForeign(t, change, final)
			before := f.State(t)
			_, err := observe(t.Context(), change.ID)
			expect(t, errors.Is(err, codehost.ErrProposalInvalid), "foreign observation accepted: %v", err)
			_, err = observe(t.Context(), "missing-proposal")
			expect(t, errors.Is(err, codehost.ErrProposalNotFound), "missing proposal error lost: %v", err)
			expect(t, f.State(t) == before, "refused observation wrote state")
		}
	})
}

func checkView(t *testing.T, v featuredrive.LocalProposalView, c codehost.ProposalChange, target, source, operation string, trust codehost.Trust) {
	t.Helper()
	p := v.Generation.Proposal
	expect(t, v.SchemaVersion == featuredrive.LocalProposalSchema && v.Operation == operation && v.Trust == trust && v.Generation.OID != "" && p.SchemaVersion == codehost.ProposalSchema && p.Sequence != 0 && p.Kind == "implementation" && p.ID == c.ID && p.Feature == c.Feature && p.Task == c.Task && p.Head == c.Head && p.Branch == c.Branch && p.TargetRef == target && p.TargetOID == c.ExpectedTargetOID && p.SourceRef == source && p.State == "open" && p.ReviewRequest == nil, "proposal view lost its exact workflow or trust bindings: %+v", v)
}

func expect(t *testing.T, condition bool, format string, args ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, args...)
	}
}
