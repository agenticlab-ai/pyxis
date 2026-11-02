// Package gitstore implements the routing-evidence Store over one fetched Git
// ref. Canonical semantics remain owned by package routingevidence.
package gitstore

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

type Adapter struct {
	git     *gitrefstore.Store
	policy  routingevidence.ProducerPolicy
	reducer routingevidence.Reducer
}

var _ routingevidence.Store = (*Adapter)(nil)
var _ routingevidence.ActivationStateReader = (*Adapter)(nil)

func New(repo, remote string, policy routingevidence.ProducerPolicy) (*Adapter, error) {
	if err := routingevidence.ValidateProducerPolicy(policy); err != nil {
		return nil, fmt.Errorf("routing evidence git store: producer policy: %w", err)
	}
	store, err := gitrefstore.New(repo, remote, policy.EvidenceRef)
	if err != nil {
		return nil, err
	}
	return &Adapter{git: store, policy: policy, reducer: routingevidence.NewReducer(policy)}, nil
}

func (a *Adapter) Append(ctx context.Context, request routingevidence.AppendRequest) (routingevidence.EvidenceProof, error) {
	events, canonical, entries, err := a.prepare(request)
	if err != nil {
		return routingevidence.EvidenceProof{}, err
	}
	result, err := a.git.Apply(ctx, "", "append canonical routing evidence", func(snapshot gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		state, _, err := a.validateSnapshot(snapshot)
		if err != nil {
			return gitrefstore.Mutation{}, err
		}
		if err := validateLease(request.ObservedHead, snapshot); err != nil {
			return gitrefstore.Mutation{}, err
		}
		existing := 0
		for path, want := range canonical {
			got, ok := snapshot.Files[path]
			if !ok {
				continue
			}
			existing++
			if !bytes.Equal(got, want) {
				return gitrefstore.Mutation{}, routingevidence.NewStoreError(routingevidence.StoreIdentityConflict, "immutable path %s already contains different bytes", path)
			}
		}
		if existing != 0 && existing != len(canonical) {
			return gitrefstore.Mutation{}, routingevidence.NewStoreError(routingevidence.StorePartialExisting, "%d of %d composite paths already exist", existing, len(canonical))
		}
		if existing == len(canonical) {
			return gitrefstore.Mutation{Files: cloneFiles(canonical)}, nil
		}
		if _, err := a.reducer.ValidateTransition(state, routingevidence.EventOrComposite{Events: events}); err != nil {
			return gitrefstore.Mutation{}, err
		}
		return gitrefstore.Mutation{Files: cloneFiles(canonical), Changed: true}, nil
	})
	if err != nil {
		return routingevidence.EvidenceProof{}, err
	}
	proof := routingevidence.EvidenceProof{EvidenceRef: a.policy.EvidenceRef, Commit: result.Head, Entries: entries}
	if err := verifyProofBytes(proof, result.Files); err != nil {
		return routingevidence.EvidenceProof{}, err
	}
	return proof, nil
}

func (a *Adapter) ReadSubject(ctx context.Context, subject routingevidence.SubjectID) (routingevidence.ExactRead, error) {
	snapshot, err := a.git.Fetch(ctx, "")
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	state, events, err := a.validateSnapshot(snapshot)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	view, err := state.SubjectView(subject)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	proof, err := proofForEvents(snapshot.Head, a.policy.EvidenceRef, events)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	return routingevidence.ExactRead{
		ObservedHead: routingevidence.ObservedHead{EvidenceRef: a.policy.EvidenceRef, Commit: snapshot.Head},
		Proof:        proof, Events: events, Bytes: cloneFiles(snapshot.Files), Subject: &view,
	}, nil
}

// ReadHead returns the complete, freshly fetched canonical state for release
// gates. Activation cannot start from a subject slice because completeness and
// invalidation are predicates over the exact evidence-ref head.
func (a *Adapter) ReadHead(ctx context.Context) (routingevidence.ExactRead, error) {
	snapshot, err := a.git.Fetch(ctx, "")
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	_, events, err := a.validateSnapshot(snapshot)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	proof, err := proofForEvents(snapshot.Head, a.policy.EvidenceRef, events)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	return routingevidence.ExactRead{
		ObservedHead: routingevidence.ObservedHead{EvidenceRef: a.policy.EvidenceRef, Commit: snapshot.Head},
		Proof:        proof, Events: events, Bytes: cloneFiles(snapshot.Files),
	}, nil
}

func (a *Adapter) ReadProof(ctx context.Context, proof routingevidence.EvidenceProof) (routingevidence.ExactRead, error) {
	if err := validateProofShape(proof, a.policy.EvidenceRef); err != nil {
		return routingevidence.ExactRead{}, err
	}
	current, err := a.git.Fetch(ctx, "")
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	if _, _, err := a.validateSnapshot(current); err != nil {
		return routingevidence.ExactRead{}, err
	}
	if !containsCommit(current.Commits, proof.Commit) {
		return routingevidence.ExactRead{}, routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "proof commit is not in the fetched authoritative history")
	}
	exact, err := a.git.SnapshotAt(ctx, proof.Commit, "")
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	_, allEvents, err := a.validateEventFiles(exact)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	paths := make([]string, len(proof.Entries))
	for i, entry := range proof.Entries {
		paths[i] = entry.Path
	}
	proofBytes, err := a.git.ReadExact(ctx, proof.Commit, paths...)
	if err != nil {
		return routingevidence.ExactRead{}, err
	}
	if err := verifyProofBytes(proof, proofBytes); err != nil {
		return routingevidence.ExactRead{}, err
	}
	selected := make([]routingevidence.Event, 0, len(proof.Entries))
	byID := make(map[routingevidence.EventID]routingevidence.Event, len(allEvents))
	for _, event := range allEvents {
		byID[event.EventID] = event
	}
	for _, entry := range proof.Entries {
		event, ok := byID[entry.EventID]
		if !ok {
			return routingevidence.ExactRead{}, routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "event %s is absent at proof commit", entry.EventID)
		}
		selected = append(selected, event)
	}
	return routingevidence.ExactRead{
		ObservedHead: routingevidence.ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit},
		Proof:        proof, Events: selected, Bytes: proofBytes,
	}, nil
}

func (a *Adapter) prepare(request routingevidence.AppendRequest) ([]routingevidence.Event, map[string][]byte, []routingevidence.EvidenceEntry, error) {
	if request.ObservedHead.EvidenceRef != a.policy.EvidenceRef || request.ObservedHead.Commit == "" {
		return nil, nil, nil, routingevidence.NewStoreError(routingevidence.StoreLeaseInvalid, "append requires an exact observed head for %s", a.policy.EvidenceRef)
	}
	if (request.Event == nil) == (request.Composite == nil) {
		return nil, nil, nil, routingevidence.NewStoreError(routingevidence.StorePartialExisting, "append requires exactly one event or one composite")
	}
	var events []routingevidence.Event
	if request.Event != nil {
		events = []routingevidence.Event{*request.Event}
	} else {
		events = append([]routingevidence.Event(nil), request.Composite.Events...)
		if len(events) < 2 {
			return nil, nil, nil, routingevidence.NewStoreError(routingevidence.StorePartialExisting, "atomic composite requires at least two events")
		}
	}
	canonical := make(map[string][]byte, len(events))
	entries := make([]routingevidence.EvidenceEntry, 0, len(events))
	decoded := make([]routingevidence.Event, 0, len(events))
	for _, event := range events {
		wire, err := routingevidence.EncodeCanonical(event)
		if err != nil {
			return nil, nil, nil, err
		}
		stored, err := routingevidence.DecodeCanonical(wire)
		if err != nil {
			return nil, nil, nil, err
		}
		path, err := routingevidence.EventPath(stored.EventID)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, duplicate := canonical[path]; duplicate {
			return nil, nil, nil, routingevidence.NewStoreError(routingevidence.StorePartialExisting, "append repeats immutable path %s", path)
		}
		canonical[path] = wire
		entries = append(entries, routingevidence.EvidenceEntry{Path: path, EventID: stored.EventID, ContentDigest: stored.ContentDigest})
		decoded = append(decoded, stored)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return decoded, canonical, entries, nil
}

func (a *Adapter) validateSnapshot(snapshot gitrefstore.Snapshot) (routingevidence.State, []routingevidence.Event, error) {
	if snapshot.Head == "" {
		return routingevidence.State{}, nil, routingevidence.NewStoreError(routingevidence.StoreRefUnavailable, "configured ref %s does not exist", a.policy.EvidenceRef)
	}
	commits := make([]routingevidence.RefCommit, len(snapshot.Commits))
	for i, commit := range snapshot.Commits {
		commits[i] = routingevidence.RefCommit{Object: commit.Object, Parents: append([]string(nil), commit.Parents...), Tree: commit.Tree}
	}
	view := routingevidence.RefView{
		Repository: a.policy.Repository, EvidenceRef: snapshot.Ref, Exists: true, Head: snapshot.Head,
		Commits: commits, DeletionObserved: snapshot.DeletionObserved,
		NonFastForwardObserved:      snapshot.NonFastForwardObserved,
		InitializationAuthorization: a.policy.Genesis.AuthorizationReference,
	}
	if err := routingevidence.VerifyContinuity(view, a.policy.Genesis); err != nil {
		return routingevidence.State{}, nil, err
	}
	return a.validateEventFiles(snapshot)
}

func (a *Adapter) validateEventFiles(snapshot gitrefstore.Snapshot) (routingevidence.State, []routingevidence.Event, error) {
	paths := sortedFilePaths(snapshot.Files)
	events := make([]routingevidence.Event, 0, len(paths))
	for _, path := range paths {
		if !strings.HasPrefix(path, "events/") {
			return routingevidence.State{}, nil, routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "unexpected path %s exists on evidence ref", path)
		}
		event, err := routingevidence.DecodeCanonical(snapshot.Files[path])
		if err != nil {
			return routingevidence.State{}, nil, err
		}
		want, err := routingevidence.EventPath(event.EventID)
		if err != nil || want != path {
			return routingevidence.State{}, nil, routingevidence.NewStoreError(routingevidence.StoreIdentityConflict, "path %s does not match event identity", path)
		}
		events = append(events, event)
	}
	state, err := a.reducer.Reduce(events)
	if err != nil {
		return routingevidence.State{}, nil, err
	}
	return state, events, nil
}

func validateLease(observed routingevidence.ObservedHead, snapshot gitrefstore.Snapshot) error {
	if observed.EvidenceRef != snapshot.Ref || !containsCommit(snapshot.Commits, observed.Commit) {
		return routingevidence.NewStoreError(routingevidence.StoreLeaseInvalid, "observed head is not in the fetched ref history")
	}
	return nil
}

func proofForEvents(commit, ref string, events []routingevidence.Event) (routingevidence.EvidenceProof, error) {
	entries := make([]routingevidence.EvidenceEntry, 0, len(events))
	for _, event := range events {
		path, err := routingevidence.EventPath(event.EventID)
		if err != nil {
			return routingevidence.EvidenceProof{}, err
		}
		entries = append(entries, routingevidence.EvidenceEntry{Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return routingevidence.EvidenceProof{EvidenceRef: ref, Commit: commit, Entries: entries}, nil
}

func validateProofShape(proof routingevidence.EvidenceProof, ref string) error {
	if proof.EvidenceRef != ref || proof.Commit == "" || len(proof.Entries) == 0 {
		return routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "proof identity is incomplete")
	}
	prior := ""
	for _, entry := range proof.Entries {
		want, err := routingevidence.EventPath(entry.EventID)
		if err != nil || want != entry.Path || entry.ContentDigest == "" || entry.Path <= prior {
			return routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "proof entry set is invalid")
		}
		prior = entry.Path
	}
	return nil
}

func verifyProofBytes(proof routingevidence.EvidenceProof, files map[string][]byte) error {
	if err := validateProofShape(proof, proof.EvidenceRef); err != nil {
		return err
	}
	if len(files) != len(proof.Entries) {
		return routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "exact-read path count differs from proof")
	}
	for _, entry := range proof.Entries {
		wire, ok := files[entry.Path]
		if !ok {
			return routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "proof path %s is absent", entry.Path)
		}
		event, err := routingevidence.DecodeCanonical(wire)
		if err != nil || event.EventID != entry.EventID || event.ContentDigest != entry.ContentDigest {
			return routingevidence.NewStoreError(routingevidence.StoreProofInvalid, "proof entry %s does not exact-read", entry.Path)
		}
	}
	return nil
}

func containsCommit(commits []gitrefstore.Commit, object string) bool {
	for _, commit := range commits {
		if commit.Object == object {
			return true
		}
	}
	return false
}

func cloneFiles(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for path, content := range files {
		out[path] = append([]byte(nil), content...)
	}
	return out
}

func sortedFilePaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
