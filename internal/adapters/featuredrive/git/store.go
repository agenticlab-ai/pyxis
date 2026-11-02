// Package gitdrive persists one feature's signed state on its dedicated Git
// ref. Trusted composition supplies the registered actors and genesis; state
// payloads cannot register keys or authorize their own ownership transitions.
package gitdrive

import (
	"context"
	"crypto/ed25519"
	"errors"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
)

const drivePath = "drive.json"

var ErrUnauthorized = errors.New("feature drive Git store: actor is not authorized")

type Principal struct {
	ID        string
	Owner     featuredrive.Owner
	Operator  bool
	PublicKey ed25519.PublicKey
}

type Binding struct {
	Repository string
	FeatureID  string
	Genesis    gitrefstore.Genesis
	Principals []Principal
}

// Signer holds no credential bytes. The composition root binds an existing
// registered credential's signing operation; private keys never enter records.
type Signer struct {
	ID   string
	Sign func(context.Context, []byte) ([]byte, error)
}

type Store struct {
	git        *gitrefstore.Store
	history    *gitrefstore.Store
	bound      Binding
	ref        string
	checkpoint string
	principals map[string]Principal
	owners     map[featuredrive.Owner]bool
	signer     Signer
}

var _ featuredrive.FeatureDriveStore = (*Store)(nil)

func NewStore(repo, remote string, binding Binding, signer Signer) (*Store, error) {
	if strings.TrimSpace(binding.Repository) == "" || strings.TrimSpace(binding.FeatureID) == "" || strings.ContainsAny(binding.Repository+binding.FeatureID+binding.Genesis.Authorization, "\x00\r\n") || strings.Contains(binding.FeatureID, "/") || binding.Genesis.Authorization == "" || !objectID(binding.Genesis.Commit) || !objectID(binding.Genesis.Tree) || len(binding.Genesis.Commit) != len(binding.Genesis.Tree) {
		return nil, featuredrive.ErrMalformed
	}
	ref := "refs/pyxis/feature-drives/" + binding.FeatureID
	checkpoint := "refs/pyxis/feature-drive-checkpoints/" + binding.FeatureID
	git, err := gitrefstore.NewWithCheckpoint(repo, remote, ref, checkpoint)
	if err != nil {
		return nil, err
	}
	// Apply holds the writer's lock while validating its fetched snapshot.
	// History reads must use a separate handle and the supplied exact OIDs.
	history, err := gitrefstore.New(repo, remote, ref)
	if err != nil {
		return nil, err
	}
	principals := make(map[string]Principal, len(binding.Principals))
	owners := make(map[featuredrive.Owner]bool, len(binding.Principals))
	for _, principal := range binding.Principals {
		ownerValid := principal.Operator && principal.Owner == (featuredrive.Owner{}) ||
			!principal.Operator && principal.Owner.ID != "" && (principal.Owner.Domain == "local" || principal.Owner.Domain == "remote")
		if principal.ID == "" || strings.ContainsAny(principal.ID+principal.Owner.Domain+principal.Owner.ID, "\x00\r\n") || len(principal.PublicKey) != ed25519.PublicKeySize || !ownerValid {
			return nil, ErrUnauthorized
		}
		if _, exists := principals[principal.ID]; exists {
			return nil, ErrUnauthorized
		}
		principal.PublicKey = append(ed25519.PublicKey(nil), principal.PublicKey...)
		principals[principal.ID] = principal
		owners[principal.Owner] = true
	}
	if len(principals) == 0 || (signer.ID == "") != (signer.Sign == nil) {
		return nil, ErrUnauthorized
	}
	if signer.ID != "" {
		if _, registered := principals[signer.ID]; !registered {
			return nil, ErrUnauthorized
		}
	}
	binding.Principals = nil // The verified private copy owns the registrations.
	return &Store{git: git, history: history, bound: binding, ref: ref, checkpoint: checkpoint, principals: principals, owners: owners, signer: signer}, nil
}

func (s *Store) Load(ctx context.Context, featureID string) (featuredrive.FeatureDrive, error) {
	if featureID != s.bound.FeatureID {
		return featuredrive.FeatureDrive{}, featuredrive.ErrNotFound
	}
	snapshot, err := s.git.Fetch(ctx, "")
	if err != nil {
		return featuredrive.FeatureDrive{}, err
	}
	drive, _, err := s.reduce(ctx, snapshot)
	return drive, err
}

func (s *Store) Create(ctx context.Context, drive featuredrive.FeatureDrive) error {
	_, err := s.git.Apply(ctx, "", "pyxis: create signed feature drive", func(snapshot gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		_, _, err := s.reduce(ctx, snapshot)
		if err == nil {
			return gitrefstore.Mutation{}, featuredrive.ErrConflict
		}
		if !errors.Is(err, featuredrive.ErrNotFound) {
			return gitrefstore.Mutation{}, err
		}
		if drive.Version != 1 || drive.Created != drive.Last || drive.Last.Kind != "create" || drive.Phase != featuredrive.Integrating || drive.Ownership != (featuredrive.Ownership{State: featuredrive.Unowned}) {
			return gitrefstore.Mutation{}, featuredrive.ErrMalformed
		}
		return s.mutation(ctx, snapshot.Head, "", nil, drive)
	})
	return err
}

func (s *Store) CAS(ctx context.Context, featureID string, expected uint64, next featuredrive.FeatureDrive) error {
	if featureID != s.bound.FeatureID || expected == 0 {
		return featuredrive.ErrMalformed
	}
	_, err := s.git.Apply(ctx, "", "pyxis: advance signed feature drive", func(snapshot gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		prior, digest, err := s.reduce(ctx, snapshot)
		if err != nil {
			return gitrefstore.Mutation{}, err
		}
		if prior.Version != expected {
			return gitrefstore.Mutation{}, featuredrive.ErrConflict
		}
		return s.mutation(ctx, snapshot.Head, digest, &prior, next)
	})
	return err
}

func (s *Store) mutation(ctx context.Context, parent, previous string, prior *featuredrive.FeatureDrive, next featuredrive.FeatureDrive) (gitrefstore.Mutation, error) {
	principal, registered := s.principals[s.signer.ID]
	if !registered || s.signer.Sign == nil {
		return gitrefstore.Mutation{}, ErrUnauthorized
	}
	if err := s.authorize(principal, prior, next); err != nil {
		return gitrefstore.Mutation{}, err
	}
	transition, err := s.signTransition(ctx, next.Last)
	if err != nil {
		return gitrefstore.Mutation{}, err
	}
	next.Last = transition
	if prior == nil {
		next.Created = transition
	}
	record := Record{SchemaVersion: RecordSchema, Ref: s.ref, Authorization: s.bound.Genesis.Authorization, Parent: parent, PreviousDigest: previous, Actor: principal.ID, Drive: next}
	record.AggregateDigest, err = aggregateDigest(next)
	if err != nil {
		return gitrefstore.Mutation{}, err
	}
	unsigned, err := record.signingBytes()
	if err != nil {
		return gitrefstore.Mutation{}, err
	}
	record.Signature, err = s.sign(ctx, unsigned)
	if err != nil {
		return gitrefstore.Mutation{}, err
	}
	body, err := record.canonical()
	return gitrefstore.Mutation{Files: map[string][]byte{drivePath: body}, Changed: true}, err
}

func (s *Store) reduce(ctx context.Context, snapshot gitrefstore.Snapshot) (featuredrive.FeatureDrive, string, error) {
	if snapshot.Ref != s.ref || snapshot.Head == "" || snapshot.DeletionObserved || snapshot.NonFastForwardObserved || len(snapshot.Commits) == 0 {
		return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
	}
	// Creation may start from the exact pristine genesis. Every published
	// state thereafter must agree with the retained origin checkpoint.
	// A cold process must never interpret a primary-ref rollback as absence.
	if snapshot.CheckpointHead != snapshot.Head && (snapshot.CheckpointHead != "" || snapshot.Head != s.bound.Genesis.Commit) {
		return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
	}
	root := snapshot.Commits[0]
	if root.Object != s.bound.Genesis.Commit || root.Tree != s.bound.Genesis.Tree || len(root.Parents) != 0 || snapshot.Head != snapshot.Commits[len(snapshot.Commits)-1].Object {
		return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
	}
	// Exact-read every retained generation at once. A failed read still
	// surfaces only when the replay below reaches that generation.
	reads := []string{root.Object}
	for _, commit := range snapshot.Commits[1:] {
		if commit.Object != snapshot.Head {
			reads = append(reads, commit.Object)
		}
	}
	views, readErrs := s.history.SnapshotsAt(ctx, reads, "")
	genesis, err := views[0], readErrs[0]
	if err != nil {
		return featuredrive.FeatureDrive{}, "", err
	}
	if len(genesis.Files) != 0 {
		return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
	}
	if len(snapshot.Commits) == 1 {
		return featuredrive.FeatureDrive{}, "", featuredrive.ErrNotFound
	}
	var prior *featuredrive.FeatureDrive
	previous := ""
	read := 1
	for i, commit := range snapshot.Commits[1:] {
		parent := snapshot.Commits[i].Object
		if len(commit.Parents) != 1 || commit.Parents[0] != parent {
			return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
		}
		view := snapshot
		if commit.Object != snapshot.Head {
			view, err = views[read], readErrs[read]
			read++
			if err != nil {
				return featuredrive.FeatureDrive{}, "", err
			}
		}
		if len(view.Files) != 1 || view.Files[drivePath] == nil {
			return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
		}
		record, err := decodeRecord(view.Files[drivePath])
		if err != nil {
			return featuredrive.FeatureDrive{}, "", err
		}
		principal, registered := s.principals[record.Actor]
		if !registered || record.Ref != s.ref || record.Authorization != s.bound.Genesis.Authorization || record.Parent != parent || record.PreviousDigest != previous || record.Drive.Last.Actor != record.Actor {
			return featuredrive.FeatureDrive{}, "", ErrUnauthorized
		}
		unsigned, err := record.signingBytes()
		if err != nil || !verifySignature(principal.PublicKey, unsigned, record.Signature) || !featuredrive.VerifyTransition(record.Drive.Last, principal.PublicKey) {
			return featuredrive.FeatureDrive{}, "", ErrUnauthorized
		}
		digest, err := aggregateDigest(record.Drive)
		if err != nil || record.AggregateDigest != digest {
			return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
		}
		if err := s.authorize(principal, prior, record.Drive); err != nil {
			return featuredrive.FeatureDrive{}, "", err
		}
		if prior == nil && !reflect.DeepEqual(record.Drive.Created, record.Drive.Last) {
			return featuredrive.FeatureDrive{}, "", featuredrive.ErrMalformed
		}
		previous = contentDigest(view.Files[drivePath])
		drive := record.Drive
		prior = &drive
	}
	return *prior, previous, nil
}
