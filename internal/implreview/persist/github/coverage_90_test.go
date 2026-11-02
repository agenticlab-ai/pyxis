package github

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
)

func TestCoverage90ClientIdentityAndCapabilityFailures(t *testing.T) {
	c := &ghClient{repo: "o/r"}
	want := errors.New("transport")
	c.run = func(context.Context, []byte, ...string) ([]byte, error) { return nil, want }
	if _, err := c.currentLogin(t.Context()); !errors.Is(err, want) {
		t.Fatalf("currentLogin error = %v", err)
	}
	c.run = func(context.Context, []byte, ...string) ([]byte, error) { return []byte(" \n"), nil }
	if _, err := c.currentLogin(t.Context()); err == nil {
		t.Fatal("blank login was accepted")
	}
	if _, err := c.repositoryNodeID(t.Context()); err == nil {
		t.Fatal("blank repository node ID was accepted")
	}
}

func TestCoverage90HostRecoveryAndDigestBoundaries(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}}
	state := &memoryPublicationStore{}
	adapter := newHostTestAdapter(t, client, state)

	if _, err := adapter.Complete(t.Context(), implreview.VerifiedPublicationBinding{}, implreview.AggregateHandoff{}); err == nil {
		t.Fatal("incomplete verified binding was accepted")
	}
	binding := hostTestBinding()
	terminal, err := adapter.Complete(t.Context(), binding, implreview.AggregateHandoff{Status: implreview.HandoffMissing})
	if err != nil || terminal.Status != implreview.PublicationFailed {
		t.Fatalf("incomplete handoff terminal = %+v, %v", terminal, err)
	}

	state = &memoryPublicationStore{record: publication.Record{
		Admission:            publication.Admission{Binding: binding},
		PublicationAttemptID: "publication-1",
		IdempotencyID:        "idempotency-1",
		Stage:                publication.StageArmed,
		CreateArmed:          true,
	}}
	adapter = newHostTestAdapter(t, client, state)
	subject := implreview.ReviewSubject{
		ReviewObligationID: binding.ReviewObligationID,
		ProposalID:         binding.ProposalID,
		ReviewedRevision:   binding.ReviewedRevision,
	}
	inspection, found, err := adapter.InspectPublication(t.Context(), subject)
	if err != nil || !found || inspection.State != implreview.PublicationRecoveryArmed {
		t.Fatalf("inspection = %+v, found=%t, err=%v", inspection, found, err)
	}

	if _, err := adapter.recoverRecord(t.Context(), publication.Record{Stage: publication.StageTerminal}); err == nil {
		t.Fatal("empty terminal recovery was accepted")
	}
	failed := implreview.PublicationTerminal{Status: implreview.PublicationFailed}
	got, err := adapter.recoverRecord(t.Context(), publication.Record{Stage: publication.StageTerminal, Terminal: &failed})
	if err != nil || got.Status != implreview.PublicationFailed {
		t.Fatalf("failed terminal recovery = %+v, %v", got, err)
	}
	if _, err := adapter.recoverRecord(t.Context(), publication.Record{Stage: publication.StageAccepted}); err == nil {
		t.Fatal("unexpected recovered stage was accepted")
	}
}

func TestCoverage90HostReadsNativeReceiptLocator(t *testing.T) {
	handoff := hostTestHandoff(t)
	body, err := persist.ComposeArtifact(handoff.Aggregate.Report)
	if err != nil {
		t.Fatal(err)
	}
	client := &hostFakeClient{comments: map[string]string{"71": body}}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	adapter.repositoryNode = "R_test"
	if _, err := adapter.Read(t.Context(), persist.Ref{Medium: persist.MediumGitHub, Locator: "github-comment:R_test:71"}); err != nil {
		t.Fatal(err)
	}
	if body == "" {
		t.Fatal("empty canonical fixture")
	}
}

func TestCoverage90VerifiedTerminalFailsClosedWhenReceiptDisappears(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	record := publication.Record{Stage: publication.StageTerminal, NativeReceipt: "github-comment:R_test:404"}
	if _, err := adapter.verifyReceipt(t.Context(), record, ""); err == nil {
		t.Fatal("missing durable receipt was accepted")
	}
}
