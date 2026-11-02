package localgit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	approvaladapter "github.com/agenticlab-ai/pyxis/internal/adapters/approval/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func preparedReceiptFixture(t *testing.T) (Options, *ProposalStore, *ReceiptStore, codehost.ReviewBinding) {
	t.Helper()
	o, proposals, change := proposalFixture(t)
	first, err := proposals.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	request, err := codehost.NewDraftReviewRequest("sha256:" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	prepared := revision(first, first.Proposal.Head)
	prepared.State, prepared.ReviewRequest = "prepared", &request
	if _, err := proposals.Publish(t.Context(), prepared); err != nil {
		t.Fatal(err)
	}
	receipts := NewReceipts(o)
	binding, err := receipts.Binding(t.Context(), change.ID)
	if err != nil || binding.Sequence != 2 || binding.Nonce != request.Nonce || binding.Digest != request.Digest {
		t.Fatalf("stored prepared request: %+v, %v", binding, err)
	}
	return o, proposals, receipts, binding
}

func TestReceiptExactDeclarationAndCloneVerification(t *testing.T) {
	t.Parallel()
	o, _, store, binding := preparedReceiptFixture(t)
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := store.Verify(t.Context(), binding); !errors.Is(err, codehost.ErrReceiptNotFound) {
		t.Fatalf("missing receipt: %v", err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("verification wrote origin refs")
	}
	record, err := store.Record(t.Context(), binding, "declared-operator", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Trust != (codehost.Trust{SharedAccount: true}) || record.Receipt.Binding != binding || record.Receipt.CaptureLine != "" {
		t.Fatalf("declaration incorrectly claims authenticated authority: %+v", record)
	}
	if fixtureGit(t, o.Origin, "rev-parse", proposalPrefix+binding.ProposalID) != binding.ProposalOID || fixtureGit(t, o.Origin, "rev-parse", binding.Branch) != binding.Head || fixtureGit(t, o.Origin, "rev-parse", binding.TargetRef) != binding.TargetOID {
		t.Fatal("receipt publication advanced reviewed refs")
	}
	clone := NewReceipts(proposalClone(t, o))
	resumed, err := clone.Binding(t.Context(), binding.ProposalID)
	if err != nil || resumed != binding {
		t.Fatalf("clone minted another request: %+v, %v", resumed, err)
	}
	verified, err := clone.Verify(t.Context(), resumed)
	if err != nil || verified != record {
		t.Fatalf("clone verification: %+v, %v", verified, err)
	}
	approval, err := (approvaladapter.Reader{Receipts: clone}).DeclaredApproval(t.Context(), binding)
	if err != nil || approval.Operator != "declared-operator" || approval.Head != binding.Head || approval.Record != record {
		t.Fatalf("declared approval composition: %+v, %v", approval, err)
	}
	if retry, err := clone.Record(t.Context(), binding, "declared-operator", ""); err != nil || retry != record {
		t.Fatalf("identical retry did not recover once: %+v, %v", retry, err)
	}
	if _, err := clone.Record(t.Context(), binding, "another-operator", ""); !errors.Is(err, codehost.ErrReceiptConflict) {
		t.Fatalf("receipt overwrite was accepted: %v", err)
	}
	fixtureGit(t, o.Origin, "gc", "--prune=now")
	fixtureGit(t, o.Origin, "cat-file", "-e", record.OID)
	fixtureGit(t, o.Origin, "cat-file", "-e", binding.ProposalOID)
}

func TestReceiptRejectsEveryMovedBindingBeforeWriting(t *testing.T) {
	t.Parallel()
	o, _, store, binding := preparedReceiptFixture(t)
	before := fixtureGit(t, o.Origin, "show-ref")
	cases := []struct {
		name string
		edit func(*codehost.ReviewBinding)
	}{
		{"schema missing", func(b *codehost.ReviewBinding) { b.SchemaVersion = "" }},
		{"schema unsupported", func(b *codehost.ReviewBinding) { b.SchemaVersion = "local-review-binding/v2" }},
		{"proposal", func(b *codehost.ReviewBinding) { b.ProposalID = "another-proposal" }},
		{"kind", func(b *codehost.ReviewBinding) { b.Kind = "implementation" }},
		{"generation", func(b *codehost.ReviewBinding) { b.ProposalOID = b.Head }},
		{"sequence", func(b *codehost.ReviewBinding) { b.Sequence++ }},
		{"operation", func(b *codehost.ReviewBinding) { b.Operation = "task-merge" }},
		{"branch", func(b *codehost.ReviewBinding) { b.Branch = "refs/heads/other" }},
		{"head", func(b *codehost.ReviewBinding) { b.Head = b.TargetOID }},
		{"target", func(b *codehost.ReviewBinding) { b.TargetRef = "refs/heads/other" }},
		{"target oid", func(b *codehost.ReviewBinding) { b.TargetOID = b.Head }},
		{"digest", func(b *codehost.ReviewBinding) { b.Digest = "sha256:" + strings.Repeat("b", 64) }},
		{"nonce", func(b *codehost.ReviewBinding) { b.Nonce = strings.Repeat("0", 32) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrong := binding
			tc.edit(&wrong)
			if _, err := store.Record(t.Context(), wrong, "operator", ""); err == nil {
				t.Fatal("changed request was accepted")
			}
			if _, err := store.Verify(t.Context(), wrong); err == nil {
				t.Fatal("changed request verified")
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refusal wrote origin refs")
			}
		})
	}
	for _, reviewer := range []string{"", "\noperator", strings.Repeat("a", 129)} {
		if _, err := store.Record(t.Context(), binding, reviewer, ""); !errors.Is(err, codehost.ErrReviewInvalid) {
			t.Fatalf("invalid declared actor admitted: %v", err)
		}
	}
	for _, capture := range []string{"bad\x00line", strings.Repeat("a", 8193)} {
		if _, err := store.Record(t.Context(), binding, "operator", capture); !errors.Is(err, codehost.ErrReviewInvalid) {
			t.Fatalf("invalid capture admitted: %v", err)
		}
	}
}

func TestReceiptHeadRestorationStillRequiresNewReview(t *testing.T) {
	t.Parallel()
	o, proposals, store, binding := preparedReceiptFixture(t)
	if _, err := store.Record(t.Context(), binding, "operator", "first H1"); err != nil {
		t.Fatal(err)
	}
	old, err := proposals.Read(t.Context(), binding.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "H2")
	second, err := proposals.Publish(t.Context(), revision(old, fixtureGit(t, o.Root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Binding(t.Context(), binding.ProposalID); !errors.Is(err, codehost.ErrReviewInvalid) {
		t.Fatalf("open round had a prepared review: %v", err)
	}
	reused := revision(second, binding.Head)
	oldRequest := *old.Proposal.ReviewRequest
	reused.State, reused.ReviewRequest = "prepared", &oldRequest
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := proposals.Publish(t.Context(), reused); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("preparation reused a nonce across an open round: %v", err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("nonce refusal changed origin refs")
	}
	request, err := codehost.NewDraftReviewRequest(binding.Digest)
	if err != nil {
		t.Fatal(err)
	}
	restore := revision(second, binding.Head)
	restore.State, restore.ReviewRequest = "prepared", &request
	third, err := proposals.Publish(t.Context(), restore)
	if err != nil || third.OID == binding.ProposalOID || third.Proposal.Sequence <= binding.Sequence {
		t.Fatalf("restored head lost generation: %+v, %v", third, err)
	}
	if _, err := store.Verify(t.Context(), binding); !errors.Is(err, codehost.ErrReviewInvalid) {
		t.Fatalf("old H1 receipt authorized new H1 generation: %v", err)
	}
	next, err := store.Binding(t.Context(), binding.ProposalID)
	if err != nil || next.Head != binding.Head || next.Nonce == binding.Nonce {
		t.Fatalf("new preparation request: %+v, %v", next, err)
	}
	if _, err := store.Record(t.Context(), next, "operator", "new H1 review"); err != nil {
		t.Fatal(err)
	}
	repeat := revision(third, binding.Head)
	repeat.State, repeat.ReviewRequest = "prepared", &request
	if _, err := proposals.Publish(t.Context(), repeat); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("new preparation reused its immediate predecessor's nonce: %v", err)
	}
	fixtureGit(t, o.Origin, "update-ref", binding.TargetRef, third.OID)
	if _, err := store.Verify(t.Context(), next); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("mainline movement ignored: %v", err)
	}
}

func receiptTransaction(ctx context.Context, origin string, input []byte) error {
	_, err := proposalGit(ctx, origin, input, nil, "update-ref", "--stdin")
	return err
}

func TestPreparedProposalRejectsIncompleteOrForeignRequests(t *testing.T) {
	t.Parallel()
	o, proposals, _, binding := preparedReceiptFixture(t)
	previous, err := proposals.Read(t.Context(), binding.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, name := range []string{"missing", "schema", "operation", "digest", "nonce", "open", "abandoned"} {
		t.Run(name, func(t *testing.T) {
			request, err := codehost.NewDraftReviewRequest(binding.Digest)
			if err != nil {
				t.Fatal(err)
			}
			change := revision(previous, binding.Head)
			change.State, change.ReviewRequest = "prepared", &request
			switch name {
			case "missing":
				change.ReviewRequest = nil
			case "schema":
				request.SchemaVersion = "foreign"
			case "operation":
				request.Operation = "task-merge"
			case "digest":
				request.Digest = "missing exact digest"
			case "nonce":
				request.Nonce = "invalid nonce"
			case "open", "abandoned":
				change.State = name
			}
			if _, err := proposals.Publish(t.Context(), change); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("invalid preparation admitted: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("invalid preparation mutated origin")
			}
		})
	}
}

func TestReceiptCancellationPreservesErrorAndRefs(t *testing.T) {
	t.Parallel()
	o, _, store, binding := preparedReceiptFixture(t)
	before := fixtureGit(t, o.Origin, "show-ref")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Record(ctx, binding, "operator", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("admission cancellation lost: %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	store.commit = func(context.Context, string, []byte) error {
		cancel()
		return context.Canceled
	}
	if _, err := store.Record(ctx, binding, "operator", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("publication cancellation lost: %v", err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("cancelled request mutated origin refs")
	}
}

func TestReceiptAtomicLeaseRejectsMovementAtCommit(t *testing.T) {
	t.Parallel()
	for _, refKind := range []string{"proposal", "branch", "target"} {
		t.Run(refKind, func(t *testing.T) {
			o, _, store, binding := preparedReceiptFixture(t)
			store.commit = func(ctx context.Context, origin string, input []byte) error {
				ref := binding.TargetRef
				switch refKind {
				case "proposal":
					ref = proposalPrefix + binding.ProposalID
				case "branch":
					ref = binding.Branch
				}
				fixtureGit(t, o.Origin, "update-ref", ref, binding.ProposalOID)
				if refKind == "proposal" {
					fixtureGit(t, o.Origin, "update-ref", ref, binding.Head)
				}
				return receiptTransaction(ctx, origin, input)
			}
			if _, err := store.Record(t.Context(), binding, "operator", ""); !errors.Is(err, codehost.ErrReceiptConflict) {
				t.Fatalf("late movement accepted: %v", err)
			}
			if out := fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", receiptPrefix); out != "" {
				t.Fatalf("failed lease published a receipt: %s", out)
			}
		})
	}
}

func TestReceiptConcurrentDeclarationsAndInterruptedRecovery(t *testing.T) {
	t.Parallel()
	o, _, first, binding := preparedReceiptFixture(t)
	second := NewReceipts(proposalClone(t, o))
	stores := []*ReceiptStore{first, second}
	ready, release := make(chan struct{}, 2), make(chan struct{})
	for _, store := range stores {
		store.commit = func(ctx context.Context, origin string, input []byte) error {
			ready <- struct{}{}
			<-release
			return receiptTransaction(ctx, origin, input)
		}
	}
	var wg sync.WaitGroup
	errorsSeen := make([]error, 2)
	for i, store := range stores {
		wg.Go(func() { _, errorsSeen[i] = store.Record(t.Context(), binding, []string{"A", "B"}[i], "") })
	}
	<-ready
	<-ready
	close(release)
	wg.Wait()
	winners := 0
	for _, err := range errorsSeen {
		if err == nil {
			winners++
		} else if !errors.Is(err, codehost.ErrReceiptConflict) {
			t.Fatalf("unexpected racing declaration error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("got %d winners: %v", winners, errorsSeen)
	}
	for _, afterCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "before commit", true: "after commit"}[afterCommit], func(t *testing.T) {
			_, _, store, binding := preparedReceiptFixture(t)
			store.commit = func(ctx context.Context, origin string, input []byte) error {
				if afterCommit {
					if err := receiptTransaction(ctx, origin, input); err != nil {
						return err
					}
				}
				return errors.New("simulated interruption")
			}
			if _, err := store.Record(t.Context(), binding, "operator", "capture"); err == nil {
				t.Fatal("interruption reported success")
			}
			store.commit = nil
			_, err := store.Verify(t.Context(), binding)
			if afterCommit && err != nil {
				t.Fatalf("completed publication not recoverable: %v", err)
			}
			if !afterCommit && !errors.Is(err, codehost.ErrReceiptNotFound) {
				t.Fatalf("incomplete publication appeared durable: %v", err)
			}
			if _, err := store.Record(t.Context(), binding, "operator", "capture"); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
		})
	}
}

func TestReceiptClosedCanonicalRecordAndParent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(codehost.ReviewReceipt) []byte
	}{
		{"missing payload", nil},
		{"malformed JSON", func(codehost.ReviewReceipt) []byte { return []byte("{\n") }},
		{"unknown field", func(r codehost.ReviewReceipt) []byte {
			body, _ := json.Marshal(r)
			return append(body[:len(body)-1], []byte(",\"unknown\":true}\n")...)
		}},
		{"wrong schema", func(r codehost.ReviewReceipt) []byte {
			r.SchemaVersion = "foreign"
			body, _ := json.Marshal(r)
			return append(body, '\n')
		}},
		{"wrong binding", func(r codehost.ReviewReceipt) []byte {
			r.Binding.Sequence++
			body, _ := json.Marshal(r)
			return append(body, '\n')
		}},
		{"empty reviewer", func(r codehost.ReviewReceipt) []byte {
			r.Reviewer = ""
			body, _ := json.Marshal(r)
			return append(body, '\n')
		}},
		{"trailing", func(r codehost.ReviewReceipt) []byte {
			body, _ := json.Marshal(r)
			return append(body, []byte("\n{}\n")...)
		}},
		{"noncanonical", func(r codehost.ReviewReceipt) []byte {
			body, _ := json.MarshalIndent(r, "", "  ")
			return append(body, '\n')
		}},
		{"oversized", func(r codehost.ReviewReceipt) []byte { return []byte(strings.Repeat("x", 65<<10)) }},
		{"wrong parent", func(r codehost.ReviewReceipt) []byte { body, _ := json.Marshal(r); return append(body, '\n') }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, _, store, binding := preparedReceiptFixture(t)
			receipt := codehost.ReviewReceipt{SchemaVersion: codehost.ReceiptSchema, Binding: binding, Reviewer: "operator"}
			parent := binding.ProposalOID
			if tc.name == "wrong parent" {
				parent = binding.Head
			}
			oid := binding.ProposalOID
			if tc.edit != nil {
				var err error
				oid, err = prepareReceipt(t.Context(), o.Root, parent, tc.edit(receipt))
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := proposalGit(t.Context(), o.Origin, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, oid); err != nil {
				t.Fatal(err)
			}
			fixtureGit(t, o.Origin, "update-ref", receiptRef(binding), oid)
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.Verify(t.Context(), binding); !errors.Is(err, codehost.ErrReviewInvalid) {
				t.Fatalf("malformed receipt accepted: %v", err)
			}
			if _, err := store.Record(t.Context(), binding, "operator", ""); !errors.Is(err, codehost.ErrReviewInvalid) {
				t.Fatalf("malformed receipt overwritten: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("verification or refusal wrote origin refs")
			}
		})
	}
}
