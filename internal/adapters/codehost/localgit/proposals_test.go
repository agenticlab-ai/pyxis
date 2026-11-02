package localgit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func proposalFixture(t *testing.T) (Options, *ProposalStore, codehost.ProposalChange) {
	t.Helper()
	o := gitFixture(t, true)
	target := fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk")
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "proposal content")
	change := codehost.ProposalChange{ID: "draft-widget", Feature: "widget", Branch: "refs/heads/draft-widget", Head: fixtureGit(t, o.Root, "rev-parse", "HEAD"), State: "open", ExpectedTargetOID: target}
	return o, NewProposals(ProposalOptions{Git: o, Kind: "draft"}), change
}

func proposalClone(t *testing.T, o Options) Options {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fresh-clone")
	fixtureGit(t, filepath.Dir(root), "clone", "--origin", o.Remote, "--branch", o.Mainline, o.Origin, root)
	fixtureGit(t, root, "config", "user.name", "Proposal clone")
	fixtureGit(t, root, "config", "user.email", "clone@example.test")
	o.Root = root
	return o
}

func revision(previous codehost.ProposalGeneration, head string) codehost.ProposalChange {
	p := previous.Proposal
	return codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Task: p.Task, Branch: p.Branch, Head: head, State: "open", ExpectedProposalOID: previous.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID}
}

func TestProposalsPublishDiscoverReviseAndArchiveFromFreshClone(t *testing.T) {
	t.Parallel()
	o, store, change := proposalFixture(t)
	before := fixtureGit(t, o.Root, "show-ref")
	first, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	if first.Proposal.Sequence != 1 || first.Proposal.Head != change.Head || first.Proposal.TargetRef != "refs/heads/trunk" || first.Proposal.TargetOID != change.ExpectedTargetOID {
		t.Fatalf("incorrect generation: %+v", first)
	}
	if fixtureGit(t, o.Root, "show-ref") != before || fixtureGit(t, o.Root, "status", "--porcelain") != "" {
		t.Fatal("publication changed local refs or source")
	}
	clone := NewProposals(ProposalOptions{Git: proposalClone(t, o), Kind: "draft"})
	list, err := clone.List(t.Context())
	if err != nil || len(list) != 1 || list[0] != first {
		t.Fatalf("fresh clone discovery: %+v, %v", list, err)
	}
	if got, err := clone.Read(t.Context(), change.ID); err != nil || got != first {
		t.Fatalf("clone read: %+v, %v", got, err)
	}
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "second proposal head")
	second, err := store.Publish(t.Context(), revision(first, fixtureGit(t, o.Root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	third, err := store.Publish(t.Context(), revision(second, change.Head))
	if err != nil || third.OID == first.OID || third.Proposal.Sequence != 3 || third.Proposal.PreviousOID != second.OID {
		t.Fatalf("ABA lost generation: %+v, %v", third, err)
	}
	if _, err := store.Publish(t.Context(), revision(first, change.Head)); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("old generation admitted: %v", err)
	}
	abandon := revision(third, third.Proposal.Head)
	abandon.State = "abandoned"
	abandon.AbandonmentReason = "The operator no longer needs this draft."
	last, err := store.Publish(t.Context(), abandon)
	if err != nil || last.Proposal.ArchiveRef == "" || last.Proposal.Sequence != 4 {
		t.Fatalf("abandonment: %+v, %v", last, err)
	}
	if got, err := clone.Read(t.Context(), change.ID); err != nil || got != last {
		t.Fatalf("retained terminal generation: %+v, %v", got, err)
	}
	if _, err := store.Publish(t.Context(), revision(last, last.Proposal.Head)); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("terminal revision admitted: %v", err)
	}
	fixtureGit(t, o.Origin, "gc", "--prune=now")
	for _, oid := range []string{first.OID, second.OID, third.OID, first.Proposal.Head} {
		fixtureGit(t, o.Origin, "cat-file", "-e", oid)
	}
	fixtureGit(t, o.Origin, "update-ref", "-d", last.Proposal.ArchiveRef)
	if _, err := clone.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("missing archive admitted: %v", err)
	}
}

func TestProposalsConcurrentWritersLeaseTheEntirePair(t *testing.T) {
	t.Parallel()
	o, store, change := proposalFixture(t)
	first, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	clone := proposalClone(t, o)
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "writer A")
	fixtureGit(t, clone.Root, "commit", "--allow-empty", "-m", "writer B")
	stores := []*ProposalStore{store, NewProposals(ProposalOptions{Git: clone, Kind: "draft"})}
	heads := []string{fixtureGit(t, o.Root, "rev-parse", "HEAD"), fixtureGit(t, clone.Root, "rev-parse", "HEAD")}
	ready, release, done := make(chan struct{}, 2), make(chan struct{}), make(chan struct{}, 2)
	for _, s := range stores {
		s.push = func(ctx context.Context, root string, args ...string) error {
			ready <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			_, err := proposalGit(ctx, root, nil, nil, args...)
			return err
		}
	}
	var wg sync.WaitGroup
	results := make([]codehost.ProposalGeneration, 2)
	errors := make([]error, 2)
	for i := range stores {
		wg.Go(func() {
			results[i], errors[i] = stores[i].Publish(t.Context(), revision(first, heads[i]))
			done <- struct{}{}
		})
	}
	<-ready
	// The prepared target lease allows one publisher at a time. The other
	// writer must refuse the busy target before it can enter the push.
	<-done
	close(release)
	wg.Wait()
	winner := -1
	for i, err := range errors {
		if err == nil {
			if winner != -1 {
				t.Fatal("both stale writers won")
			}
			winner = i
		}
	}
	if winner == -1 {
		t.Fatalf("no winner: %v", errors)
	}
	current, err := store.Read(t.Context(), change.ID)
	if err != nil || current != results[winner] || current.Proposal.Sequence != 2 {
		t.Fatalf("pair diverged: %+v, %v (%v)", current, err, errors)
	}
}

func TestProposalsUnsupportedAtomicAndInterruptedPublication(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"unsupported atomic", "before publication", "after publication"} {
		t.Run(phase, func(t *testing.T) {
			o, store, change := proposalFixture(t)
			before := fixtureGit(t, o.Origin, "show-ref")
			calls := 0
			interrupted := errors.New("publication interrupted")
			if phase == "unsupported atomic" {
				fixtureGit(t, o.Origin, "config", "receive.advertiseAtomic", "false")
			} else {
				store.push = func(ctx context.Context, root string, args ...string) error {
					calls++
					if phase == "after publication" {
						if _, err := proposalGit(ctx, root, nil, nil, args...); err != nil {
							return err
						}
					}
					return interrupted
				}
			}
			if _, err := store.Publish(t.Context(), change); err == nil {
				t.Fatal("interruption reported success")
			} else if phase == "unsupported atomic" && !errors.Is(err, codehost.ErrAtomicUnavailable) {
				t.Fatalf("wrong capability failure: %v", err)
			}
			if phase != "after publication" && fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("failed push advanced origin refs")
			}
			if phase == "after publication" {
				clone := NewProposals(ProposalOptions{Git: proposalClone(t, o), Kind: "draft"})
				retained, err := clone.Read(t.Context(), change.ID)
				if err != nil {
					t.Fatal(err)
				}
				retry, err := store.Publish(t.Context(), change)
				if err != nil || retry != retained || calls != 1 || retry.Proposal.Sequence != 1 {
					t.Fatalf("retry duplicated publication: %+v, %v, calls=%d", retry, err, calls)
				}
			} else {
				fixtureGit(t, o.Origin, "config", "receive.advertiseAtomic", "true")
				store.push = nil
				if _, err := store.Publish(t.Context(), change); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestProposalsDeriveImplementationTargetAndRefuseBeforeOriginWrites(t *testing.T) {
	t.Parallel()
	o, store, change := proposalFixture(t)
	fixtureGit(t, o.Origin, "update-ref", "refs/heads/feature-widget", change.ExpectedTargetOID)
	store.options.Kind, store.options.IntegrationRef = "implementation", "refs/heads/feature-widget"
	change.ID, change.Task = "task-widget-5", "carina:5"
	got, err := store.Publish(t.Context(), change)
	if err != nil || got.Proposal.Kind != "implementation" || got.Proposal.TargetRef != store.options.IntegrationRef || got.Proposal.Task != "carina:5" {
		t.Fatalf("workflow destination: %+v, %v", got, err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, mutate := range []func(*codehost.ProposalChange){
		func(c *codehost.ProposalChange) { c.ID = "../wrong" },
		func(c *codehost.ProposalChange) { c.Head = c.Head[:12] },
		func(c *codehost.ProposalChange) { c.Feature = "" },
		func(c *codehost.ProposalChange) { c.State = "landed" },
		func(c *codehost.ProposalChange) { c.AbandonmentReason = "not terminal" },
		func(c *codehost.ProposalChange) { c.Branch = "refs/heads/trunk" },
		func(c *codehost.ProposalChange) { c.Branch = "refs/pyxis/evidence/chosen" },
		func(c *codehost.ProposalChange) { c.Branch = store.options.IntegrationRef },
		func(c *codehost.ProposalChange) { c.ExpectedTargetOID = c.Head },
		func(c *codehost.ProposalChange) { c.ExpectedBranchOID = c.Head },
		func(c *codehost.ProposalChange) { c.ExpectedProposalOID = strings.Repeat("a", 40) },
		func(c *codehost.ProposalChange) { c.State = "abandoned" },
	} {
		bad := change
		bad.ID = "different-proposal"
		bad.Branch = "refs/heads/different-proposal"
		mutate(&bad)
		if _, err := store.Publish(t.Context(), bad); err == nil {
			t.Fatalf("invalid change admitted: %+v", bad)
		}
		if fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("refusal changed origin")
		}
	}
	if _, err := store.Read(t.Context(), "missing"); !errors.Is(err, codehost.ErrProposalNotFound) {
		t.Fatalf("missing proposal: %v", err)
	}
	if _, err := store.Read(t.Context(), "invalid/id"); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("invalid id: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.Publish(ctx, change); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	fixtureGit(t, o.Origin, "update-ref", change.Branch, change.ExpectedTargetOID)
	if _, err := store.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("moved branch admitted: %v", err)
	}
	if _, err := store.List(t.Context()); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("list admitted moved branch: %v", err)
	}
}

func TestProposalsRejectWrongOriginsIncludingTransportRewrites(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"empty mainline", "hosted remote", "plain path rewrite"} {
		t.Run(state, func(t *testing.T) {
			o, store, change := proposalFixture(t)
			before := fixtureGit(t, o.Origin, "show-ref")
			switch state {
			case "empty mainline":
				fixtureGit(t, o.Origin, "update-ref", "-d", "refs/heads/trunk")
				before = ""
			case "hosted remote":
				fixtureGit(t, o.Root, "remote", "set-url", o.Remote, "https://invalid.example/no-transport")
			case "plain path rewrite":
				fixtureGit(t, o.Root, "remote", "set-url", o.Remote, "file://"+o.Origin)
				fixtureGit(t, o.Root, "config", "url.https://invalid.example/.insteadOf", o.Origin)
			}
			if _, err := store.Publish(t.Context(), change); err == nil {
				t.Fatal("wrong origin admitted")
			}
			if before != "" && fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("wrong origin changed refs")
			}
		})
	}
}

func TestProposalsClosedRecordsAndRetainedParents(t *testing.T) {
	t.Parallel()
	o, store, change := proposalFixture(t)
	first, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*codehost.Proposal){
		"schema":         func(p *codehost.Proposal) { p.SchemaVersion = "unknown" },
		"identity":       func(p *codehost.Proposal) { p.ID = "different" },
		"short object":   func(p *codehost.Proposal) { p.Head = p.Head[:12] },
		"state":          func(p *codehost.Proposal) { p.State = "approved" },
		"target":         func(p *codehost.Proposal) { p.TargetRef = "refs/pyxis/evidence/forged" },
		"wrong mainline": func(p *codehost.Proposal) { p.TargetRef = "refs/heads/wrong" },
		"sequence":       func(p *codehost.Proposal) { p.Sequence = 2 },
		"archive":        func(p *codehost.Proposal) { p.ArchiveRef = "refs/heads/forged" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := first.Proposal
			mutate(&bad)
			oid, err := prepareProposal(t.Context(), o.Root, bad)
			if err != nil {
				t.Fatal(err)
			}
			fixtureGit(t, o.Root, "push", "--force", o.Remote, oid+":"+proposalPrefix+change.ID)
			if _, err := store.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("closed record admitted: %v", err)
			}
		})
	}
	fixtureGit(t, o.Root, "push", "--force", o.Remote, first.OID+":"+proposalPrefix+change.ID)
	bytes, err := json.Marshal(first.Proposal)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{string(bytes[:len(bytes)-1]) + `,"unknown":true}` + "\n", string(bytes) + "\n{}", " " + string(bytes) + "\n", string(bytes) + "\n", strings.Repeat("x", 65<<10)} {
		blob, err := proposalGit(t.Context(), o.Root, []byte(body), nil, "hash-object", "-w", "--stdin")
		if err != nil {
			t.Fatal(err)
		}
		tree, err := proposalGit(t.Context(), o.Root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\tproposal.json\n"), nil, "mktree")
		if err != nil {
			t.Fatal(err)
		}
		oid, err := proposalGit(t.Context(), o.Root, []byte("malformed fixture"), nil, "commit-tree", strings.TrimSpace(string(tree)))
		if err != nil {
			t.Fatal(err)
		}
		fixtureGit(t, o.Root, "push", "--force", o.Remote, strings.TrimSpace(string(oid))+":"+proposalPrefix+change.ID)
		if _, err := store.List(t.Context()); !errors.Is(err, codehost.ErrProposalInvalid) {
			t.Fatalf("malformed body admitted: %v", err)
		}
	}
	if !slices.Equal(proposalParents(first.Proposal), []string{change.Head, change.ExpectedTargetOID}) {
		t.Fatal("head and target are not retained parents")
	}
	if _, err := os.Stat(filepath.Join(o.Root, "carina")); !os.IsNotExist(err) {
		t.Fatal("adapter created tracker state")
	}
}

func TestProposalsRejectAbsentRecordsMalformedRefsAndPredecessors(t *testing.T) {
	o, store, change := proposalFixture(t)
	if got, err := store.List(t.Context()); err != nil || len(got) != 0 {
		t.Fatalf("empty origin proposals: %+v, %v", got, err)
	}
	first, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	for _, predecessor := range []struct {
		oid      string
		sequence uint64
	}{{change.Head, 2}, {first.OID, 4}} {
		bad := first.Proposal
		bad.PreviousOID, bad.Sequence = predecessor.oid, predecessor.sequence
		oid, err := prepareProposal(t.Context(), o.Root, bad)
		if err != nil {
			t.Fatal(err)
		}
		fixtureGit(t, o.Root, "push", "--force", o.Remote, oid+":"+proposalPrefix+change.ID)
		if _, err := store.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalInvalid) {
			t.Fatalf("broken predecessor admitted: %v", err)
		}
	}
	fixtureGit(t, o.Origin, "update-ref", proposalPrefix+change.ID, change.Head)
	if _, err := store.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("absent record admitted: %v", err)
	}
	blob, err := proposalGit(t.Context(), o.Root, []byte("not a commit"), nil, "hash-object", "-w", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "push", "--force", o.Remote, strings.TrimSpace(string(blob))+":"+proposalPrefix+change.ID)
	if _, err := store.Read(t.Context(), change.ID); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("blob proposal admitted: %v", err)
	}
	if _, err := store.List(t.Context()); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("blob proposal listed: %v", err)
	}
	for _, id := range []string{"", "-invalid", "invalid-", strings.Repeat("a", 129), "INVALID"} {
		if _, err := store.Read(t.Context(), id); !errors.Is(err, codehost.ErrProposalInvalid) {
			t.Fatalf("identity %q: %v", id, err)
		}
	}
	fixtureGit(t, o.Root, "remote", "set-url", o.Remote, "https://invalid.example/unsupported")
	if _, err := store.Read(t.Context(), change.ID); err == nil {
		t.Fatal("read ignored invalid origin")
	}
	if _, err := store.List(t.Context()); err == nil {
		t.Fatal("list ignored invalid origin")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := proposalGit(ctx, o.Root, nil, nil, "status"); !errors.Is(err, context.Canceled) {
		t.Fatalf("process cancellation: %v", err)
	}
	if _, err := prepareProposal(ctx, o.Root, first.Proposal); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preparation: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := proposalGit(t.Context(), o.Root, nil, nil, "status"); err == nil {
		t.Fatal("missing Git executable reported success")
	}
}
