package localgit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

const proposalPrefix = "refs/pyxis/proposals/"

// A finalized task embeds two bounded artifact bodies as JSON byte strings.
// Ordinary proposal records retain their existing 64 KiB bound.
const maxTaskProposalBytes = (64 << 10) + 2*((codehost.MaxReviewArtifactBytes+2)/3*4)

// ProposalOptions binds one workflow's kind and integration destination.
// Drafts derive their target solely from the configured mainline. The
// implementation composition root supplies its approved feature destination.
type ProposalOptions struct {
	Git            Options
	Kind           string
	IntegrationRef string
	HeadRef        string // Optional immutable workflow source for final proposals.
}

type ProposalStore struct {
	options ProposalOptions
	push    func(context.Context, string, ...string) error
}

func NewProposals(options ProposalOptions) *ProposalStore { return &ProposalStore{options: options} }

var _ codehost.Proposals = (*ProposalStore)(nil)

// Mainline imports the exact origin object without updating a tracking ref.
func (s *ProposalStore) Mainline(ctx context.Context) (codehost.Readiness, error) {
	r, err := s.inspect(ctx)
	if err == nil {
		_, err = proposalGit(ctx, s.options.Git.Root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", r.Origin, r.MainlineOID)
	}
	return r, err
}

func (s *ProposalStore) inspect(ctx context.Context) (codehost.Readiness, error) {
	r, err := NewDiagnostics(s.options.Git).Inspect(ctx)
	if err == nil && r.MainlineOID == "" {
		err = originError("%s is absent; %s", r.MainlineRef, r.MainlineRemedy)
	}
	if err == nil {
		// Resolve the exact transport destination without contacting it. A
		// file URL and a plain path can have different insteadOf rewrites.
		var destination []byte
		destination, err = proposalGit(ctx, s.options.Git.Root, nil, nil, "ls-remote", "--get-url", r.Origin)
		if err == nil {
			var canonical string
			canonical, err = localPath(s.options.Git.Root, strings.TrimSpace(string(destination)))
			if err == nil && canonical != r.Origin {
				err = originError("publication destination differs from configured origin")
			}
		}
	}
	return r, err
}

// List returns the proposals for the configured workflow. Other workflow kinds
// and integration destinations may coexist in the same bare origin.
func (s *ProposalStore) List(ctx context.Context) ([]codehost.ProposalGeneration, error) {
	r, err := s.inspect(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := proposalRefs(ctx, r.Origin, proposalPrefix)
	if err != nil {
		return nil, err
	}
	result := make([]codehost.ProposalGeneration, 0, len(refs))
	for _, ref := range slices.Sorted(maps.Keys(refs)) {
		id := strings.TrimPrefix(ref, proposalPrefix)
		generation, err := readProposal(ctx, r.Origin, id, refs[ref])
		if err != nil {
			return nil, err
		}
		if generation.Proposal.Kind != s.options.Kind || generation.Proposal.TargetRef != s.targetRef() || generation.Proposal.SourceRef != s.options.HeadRef {
			continue
		}
		if _, err := s.history(ctx, r.Origin, generation); err != nil {
			return nil, err
		}
		if err := verifyPair(ctx, r.Origin, generation, s.options.HeadRef); err != nil {
			return nil, err
		}
		result = append(result, generation)
	}
	return result, nil
}

func (s *ProposalStore) Read(ctx context.Context, id string) (codehost.ProposalGeneration, error) {
	if !proposalID(id) {
		return codehost.ProposalGeneration{}, invalidProposal("invalid identity")
	}
	r, err := s.inspect(ctx)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	refs, err := proposalRefs(ctx, r.Origin, proposalPrefix+id)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	oid := refs[proposalPrefix+id]
	if oid == "" {
		return codehost.ProposalGeneration{}, fmt.Errorf("%w: %s", codehost.ErrProposalNotFound, id)
	}
	generation, err := s.read(ctx, r.Origin, id, oid)
	if err == nil {
		err = verifyPair(ctx, r.Origin, generation, s.options.HeadRef)
	}
	return generation, err
}

func (s *ProposalStore) Publish(ctx context.Context, change codehost.ProposalChange) (codehost.ProposalGeneration, error) {
	r, err := s.inspect(ctx)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	target := s.targetRef()
	p := codehost.Proposal{SchemaVersion: codehost.ProposalSchema, ID: change.ID, Kind: s.options.Kind,
		Feature: change.Feature, Task: change.Task, Branch: change.Branch, Head: change.Head,
		TargetRef: target, TargetOID: change.ExpectedTargetOID, SourceRef: s.options.HeadRef, State: change.State, Sequence: 1,
		PreviousOID: change.ExpectedProposalOID, AbandonmentReason: change.AbandonmentReason}
	if p.State == "locked" {
		return codehost.ProposalGeneration{}, invalidProposal("lock finalization requires the operator landing operation")
	}
	if change.ReviewRequest != nil {
		request := *change.ReviewRequest
		p.ReviewRequest = &request
	}
	if change.TaskPreparation != nil {
		preparation := *change.TaskPreparation
		p.TaskPreparation = &preparation
	}
	if change.CandidatePreparation != nil {
		preparation := *change.CandidatePreparation
		p.CandidatePreparation = &preparation
	}
	if p.Branch == r.MainlineRef {
		return codehost.ProposalGeneration{}, invalidProposal("proposal publication cannot write configured mainline")
	}
	if s.options.HeadRef != "" {
		if _, err := s.Scope(ctx); err != nil {
			return codehost.ProposalGeneration{}, err
		}
		if p.Branch == s.options.HeadRef {
			return codehost.ProposalGeneration{}, invalidProposal("proposal publication cannot write its workflow source")
		}
	}
	if change.ExpectedProposalOID != "" {
		previous, err := readProposal(ctx, r.Origin, change.ID, change.ExpectedProposalOID)
		if err != nil {
			return codehost.ProposalGeneration{}, err
		}
		nonces, err := s.history(ctx, r.Origin, previous)
		if err != nil {
			return codehost.ProposalGeneration{}, err
		}
		old := previous.Proposal
		if (old.State != "open" && old.State != "prepared") || old.Sequence == math.MaxUint64 || old.Kind != p.Kind || old.Feature != p.Feature || old.Task != p.Task || old.Branch != p.Branch || old.TargetRef != target || old.Head != change.ExpectedBranchOID {
			return codehost.ProposalGeneration{}, invalidProposal("revision changes identity, targets a terminal proposal, or lacks its observed branch head")
		}
		if nonce := preparationNonce(p); nonce != "" {
			if _, used := nonces[nonce]; used {
				return codehost.ProposalGeneration{}, invalidProposal("new preparation requires a nonce absent from the retained lineage")
			}
		}
		p.Sequence = old.Sequence + 1
	} else if change.ExpectedBranchOID != "" {
		return codehost.ProposalGeneration{}, invalidProposal("first publication must create an absent proposal branch")
	}
	if p.State == "abandoned" {
		if p.PreviousOID == "" || p.Head != change.ExpectedBranchOID {
			return codehost.ProposalGeneration{}, invalidProposal("abandonment retains the existing reviewed head")
		}
		p.ArchiveRef = archiveRef(p)
	}
	if err := validateProposal(ctx, s.options.Git.Root, p); err != nil {
		return codehost.ProposalGeneration{}, err
	}
	if err := commitObject(ctx, s.options.Git.Root, p.Head); err != nil {
		return codehost.ProposalGeneration{}, err
	}
	// Parent links keep every reviewed head, target, and predecessor reachable
	// after branch cleanup or clone loss. Fetch only local objects, without refs.
	for _, oid := range []string{p.TargetOID, p.PreviousOID} {
		if oid == "" {
			continue
		}
		if err := commitObject(ctx, r.Origin, oid); err != nil {
			return codehost.ProposalGeneration{}, err
		}
		if _, err := proposalGit(ctx, s.options.Git.Root, nil, nil, "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", r.Origin, oid); err != nil {
			return codehost.ProposalGeneration{}, err
		}
	}
	oid, err := prepareProposal(ctx, s.options.Git.Root, p)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	result := codehost.ProposalGeneration{OID: oid, Proposal: p}
	refs, err := proposalRefs(ctx, r.Origin, p.Branch, proposalPrefix+p.ID, p.TargetRef, p.ArchiveRef, s.options.HeadRef)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	if s.options.HeadRef != "" && refs[s.options.HeadRef] != p.Head {
		return codehost.ProposalGeneration{}, fmt.Errorf("%w: final proposal must use the current workflow source head", codehost.ErrProposalConflict)
	}
	if refs[proposalPrefix+p.ID] == oid && refs[p.Branch] == p.Head {
		if err := verifyPair(ctx, r.Origin, result, s.options.HeadRef); err != nil {
			return codehost.ProposalGeneration{}, err
		}
		return result, nil
	}
	if refs[proposalPrefix+p.ID] != change.ExpectedProposalOID || refs[p.Branch] != change.ExpectedBranchOID || refs[p.TargetRef] != p.TargetOID || (p.ArchiveRef != "" && refs[p.ArchiveRef] != "") {
		return codehost.ProposalGeneration{}, fmt.Errorf("%w: refresh proposal, branch, and workflow target before publishing", codehost.ErrProposalConflict)
	}
	current, err := s.inspect(ctx)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	if current.Origin != r.Origin {
		return codehost.ProposalGeneration{}, originError("configured origin changed before publication")
	}
	args := []string{"push", "--atomic", "--no-recurse-submodules",
		"--force-with-lease=" + p.Branch + ":" + change.ExpectedBranchOID,
		"--force-with-lease=" + proposalPrefix + p.ID + ":" + change.ExpectedProposalOID}
	if p.ArchiveRef != "" {
		args = append(args, "--force-with-lease="+p.ArchiveRef+":")
	}
	args = append(args, r.Origin, p.Head+":"+p.Branch, oid+":"+proposalPrefix+p.ID)
	if p.ArchiveRef != "" {
		args = append(args, oid+":"+p.ArchiveRef)
	}
	push := s.push
	if push == nil {
		push = func(ctx context.Context, root string, args ...string) error {
			_, err := proposalGit(ctx, root, nil, nil, args...)
			return err
		}
	}
	unchanged := []unchangedRef{{Ref: p.TargetRef, OID: p.TargetOID}}
	if s.options.HeadRef != "" {
		unchanged = append(unchanged, unchangedRef{Ref: s.options.HeadRef, OID: p.Head})
	}
	if p.Head == change.ExpectedBranchOID {
		unchanged = append(unchanged, unchangedRef{Ref: p.Branch, OID: p.Head})
	}
	if err := withUnchangedRefs(ctx, r.Origin, unchanged, func() error {
		return push(ctx, s.options.Git.Root, args...)
	}); err != nil {
		if strings.Contains(err.Error(), "does not support --atomic push") {
			return codehost.ProposalGeneration{}, fmt.Errorf("%w: %v", codehost.ErrAtomicUnavailable, err)
		}
		if strings.Contains(err.Error(), "stale info") {
			return codehost.ProposalGeneration{}, fmt.Errorf("%w: %v", codehost.ErrProposalConflict, err)
		}
		return codehost.ProposalGeneration{}, err
	}
	if err := verifyPair(ctx, r.Origin, result, s.options.HeadRef); err != nil {
		return codehost.ProposalGeneration{}, err
	}
	return result, nil
}

func (s *ProposalStore) read(ctx context.Context, origin, id, oid string) (codehost.ProposalGeneration, error) {
	generation, err := readProposal(ctx, origin, id, oid)
	if err == nil {
		_, err = s.history(ctx, origin, generation)
	}
	return generation, err
}

func (s *ProposalStore) targetRef() string {
	if s.options.Kind == "implementation" {
		return s.options.IntegrationRef
	}
	return "refs/heads/" + s.options.Git.Mainline
}

// history validates every retained generation without relying on mutable old
// branch refs. Its nonce set also prevents reuse across intervening open rounds.
func (s *ProposalStore) history(ctx context.Context, origin string, generation codehost.ProposalGeneration) (map[string]struct{}, error) {
	p := generation.Proposal
	if p.Kind != s.options.Kind || p.TargetRef != s.targetRef() || p.SourceRef != s.options.HeadRef || p.Branch == "refs/heads/"+s.options.Git.Mainline {
		return nil, invalidProposal("record contradicts configured workflow kind or destination")
	}
	nonces := make(map[string]struct{})
	for {
		// A final lock retains the prepared request; it does not issue another
		// preparation. Its exact predecessor binding is checked below.
		if nonce := preparationNonce(p); nonce != "" && p.State != "locked" {
			if _, used := nonces[nonce]; used {
				return nil, invalidProposal("request nonce is reused in the retained lineage")
			}
			nonces[nonce] = struct{}{}
		}
		if p.PreviousOID == "" {
			return nonces, nil
		}
		prior, err := readProposal(ctx, origin, p.ID, p.PreviousOID)
		if err != nil {
			return nil, err
		}
		previous := prior.Proposal
		if previous.Kind != p.Kind || previous.Feature != p.Feature || previous.Task != p.Task || previous.Branch != p.Branch || previous.TargetRef != p.TargetRef || previous.SourceRef != p.SourceRef || (previous.State != "open" && previous.State != "prepared") || previous.Sequence == math.MaxUint64 || previous.Sequence+1 != p.Sequence || ((p.State == "abandoned" || p.State == "locked") && p.Head != previous.Head) {
			return nil, invalidProposal("invalid monotonic predecessor")
		}
		if p.State == "landed" {
			var binding codehost.ReviewBinding
			if p.TaskLanding != nil {
				binding = p.TaskLanding.Review.Receipt.Binding
			} else {
				binding = p.CandidateLanding.Review.Receipt.Binding
			}
			if previous.State != "prepared" || previous.Head != p.Head || previous.TargetOID != p.TargetOID || binding.Nonce != preparationNonce(previous) {
				return nil, invalidProposal("landing differs from its reviewed predecessor")
			}
			if p.CandidateLanding != nil && (previous.CandidatePreparation == nil || previous.CandidatePreparation.Candidate != binding.Candidate || previous.CandidatePreparation.ScenarioSetFingerprint != binding.ScenarioSetFingerprint) {
				return nil, invalidProposal("final landing differs from its prepared candidate")
			}
		}
		if p.State == "locked" && (previous.State != "prepared" || previous.ReviewRequest == nil || p.ReviewRequest == nil || *p.ReviewRequest != *previous.ReviewRequest || p.TargetOID != previous.TargetOID) {
			return nil, invalidProposal("locked generation must retain its exact prepared request and target")
		}
		p = previous
	}
}

// readProposal applies the same closed, canonical and structural checks to
// current and historical records. Workflow binding is checked by the store.
func readProposal(ctx context.Context, origin, id, oid string) (codehost.ProposalGeneration, error) {
	if !proposalID(id) || !objectID(oid) {
		return codehost.ProposalGeneration{}, invalidProposal("invalid identity or object id")
	}
	body, err := proposalRecord(ctx, origin, oid)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	n := len(body)
	var p codehost.Proposal
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return codehost.ProposalGeneration{}, invalidProposal("closed record: %v", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return codehost.ProposalGeneration{}, invalidProposal("record has trailing data")
	}
	if p.TaskLanding == nil && p.CandidateLanding == nil && n > 64<<10 {
		return codehost.ProposalGeneration{}, invalidProposal("ordinary proposal record is too large")
	}
	// Canonical bytes also reject duplicate keys and noncanonical encodings.
	canonical, _ := json.Marshal(p)
	if !bytes.Equal(body, append(canonical, '\n')) {
		return codehost.ProposalGeneration{}, invalidProposal("record is not canonical")
	}
	if p.ID != id {
		return codehost.ProposalGeneration{}, invalidProposal("record and ref identities differ")
	}
	if err := validateProposal(ctx, origin, p); err != nil {
		return codehost.ProposalGeneration{}, err
	}
	parents, err := proposalGit(ctx, origin, nil, nil, "rev-list", "--parents", "-n", "1", oid)
	if err != nil {
		return codehost.ProposalGeneration{}, err
	}
	if !slices.Equal(strings.Fields(string(parents))[1:], proposalParents(p)) {
		return codehost.ProposalGeneration{}, invalidProposal("record does not retain its head, target, and predecessor")
	}
	return codehost.ProposalGeneration{OID: oid, Proposal: p}, nil
}

func validateProposal(ctx context.Context, root string, p codehost.Proposal) error {
	if p.SchemaVersion != codehost.ProposalSchema || !proposalID(p.ID) || p.Feature == "" || strings.ContainsAny(p.Feature+p.Task, "\r\n\x00") || p.Sequence == 0 || !objectID(p.Head) || !objectID(p.TargetOID) || len(p.Head) != len(p.TargetOID) || (p.PreviousOID != "" && (!objectID(p.PreviousOID) || len(p.PreviousOID) != len(p.Head))) || (p.Sequence == 1) != (p.PreviousOID == "") {
		return invalidProposal("invalid schema, identity, generation, or full object id")
	}
	if (p.Kind != "draft" && p.Kind != "implementation") || (p.Kind == "draft" && p.Task != "") || (p.State != "open" && p.State != "prepared" && p.State != "abandoned" && p.State != "landed" && p.State != "locked") {
		return invalidProposal("invalid kind, task identity, or state")
	}
	if p.State == "landed" {
		if p.Kind != "implementation" {
			return invalidProposal("only implementation proposals can land")
		}
		var b codehost.ReviewBinding
		if p.Task != "" {
			if p.TaskLanding == nil || p.CandidateLanding != nil || p.TaskLanding.Validate() != nil || p.TaskLanding.Feature != p.Feature || p.TaskLanding.Task != p.Task {
				return invalidProposal("landed task requires complete retained review proof")
			}
			b = p.TaskLanding.Review.Receipt.Binding
		} else {
			if p.TaskLanding != nil || p.CandidateLanding == nil || p.SourceRef == "" || p.CandidateLanding.Validate() != nil || p.CandidateLanding.Candidate.FeatureID != p.Feature {
				return invalidProposal("landed final requires complete retained candidate proof")
			}
			b = p.CandidateLanding.Review.Receipt.Binding
			if err := verifyCandidateObjects(ctx, root, p.CandidateLanding.Candidate); err != nil {
				return err
			}
		}
		if b.ProposalID != p.ID || b.ProposalOID != p.PreviousOID || b.Sequence == math.MaxUint64 || b.Sequence+1 != p.Sequence || b.Head != p.Head || b.Branch != p.Branch || b.TargetRef != p.TargetRef || b.TargetOID != p.TargetOID {
			return invalidProposal("landing contradicts its proposal bindings")
		}
	} else if p.TaskLanding != nil || p.CandidateLanding != nil {
		return invalidProposal("only a landed proposal can embed landing proof")
	}
	if p.State == "prepared" || p.State == "locked" {
		if p.Kind == "draft" {
			if p.ReviewRequest == nil || p.ReviewRequest.Validate() != nil || p.TaskPreparation != nil || p.CandidatePreparation != nil {
				return invalidProposal("prepared draft requires its versioned draft-lock nonce and readiness digest")
			}
		} else if p.State == "locked" {
			return invalidProposal("only a draft can be locked")
		} else if p.Task == "" {
			if p.ReviewRequest != nil || p.TaskPreparation != nil || p.CandidatePreparation == nil || p.CandidatePreparation.Validate() != nil || p.SourceRef == "" {
				return invalidProposal("prepared final requires its own nonce and complete candidate identity")
			}
			c := p.CandidatePreparation.Candidate
			if c.FeatureID != p.Feature || c.FeatureHead != p.Head || c.MainHead != p.TargetOID {
				return invalidProposal("prepared candidate contradicts its feature or exact input heads")
			}
			if err := verifyCandidateObjects(ctx, root, c); err != nil {
				return err
			}
		} else if p.ReviewRequest != nil || p.TaskPreparation == nil || p.TaskPreparation.Validate() != nil || p.CandidatePreparation != nil {
			return invalidProposal("prepared task requires its own versioned nonce and task-merge operation")
		}
	} else if p.ReviewRequest != nil || p.TaskPreparation != nil || p.CandidatePreparation != nil {
		return invalidProposal("only a prepared proposal can carry preparation metadata")
	}
	if p.State == "locked" {
		if p.DraftLock == nil || p.DraftLock.Validate() != nil || p.DraftLock.Feature != p.Feature || p.Sequence < 2 || p.DraftLock.Review.Receipt.Binding.ProposalOID != p.PreviousOID || p.DraftLock.Review.Receipt.Binding.Sequence != p.Sequence-1 || p.DraftLock.Review.Receipt.Binding.ProposalID != p.ID || p.DraftLock.Review.Receipt.Binding.Head != p.Head || p.DraftLock.Review.Receipt.Binding.Branch != p.Branch || p.DraftLock.Review.Receipt.Binding.TargetRef != p.TargetRef || p.DraftLock.Review.Receipt.Binding.TargetOID != p.TargetOID || p.ReviewRequest.Nonce != p.DraftLock.Review.Receipt.Binding.Nonce || p.ReviewRequest.Digest != p.DraftLock.Review.Receipt.Binding.Digest {
			return invalidProposal("locked draft has no consistent retained lock and review")
		}
	} else if p.DraftLock != nil {
		return invalidProposal("only a locked draft can carry its landing")
	}
	if p.State == "abandoned" {
		if !utf8.ValidString(p.AbandonmentReason) || strings.TrimSpace(p.AbandonmentReason) == "" || len(p.AbandonmentReason) > 1000 || strings.ContainsAny(p.AbandonmentReason, "\r\n\x00") {
			return invalidProposal("abandonment requires a one-line reason of 1–1000 bytes")
		}
	} else if p.AbandonmentReason != "" {
		return invalidProposal("only a terminal abandonment can carry its reason")
	}
	for _, ref := range []string{p.Branch, p.TargetRef} {
		if !strings.HasPrefix(ref, "refs/heads/") {
			return invalidProposal("branch and workflow target must be canonical head refs")
		}
		if err := checkRefFormat(ctx, root, ref); err != nil {
			return invalidProposal("invalid branch or target ref")
		}
	}
	if p.SourceRef != "" {
		if p.Kind != "implementation" || p.Task != "" || !strings.HasPrefix(p.SourceRef, "refs/heads/") || p.SourceRef == p.Branch || p.SourceRef == p.TargetRef {
			return invalidProposal("invalid final workflow source ref")
		}
		if err := checkRefFormat(ctx, root, p.SourceRef); err != nil {
			return invalidProposal("invalid final workflow source ref")
		}
	}
	terminal := p.State == "abandoned" || p.State == "landed" || p.State == "locked"
	if p.Branch == p.TargetRef || (terminal && (p.PreviousOID == "" || p.ArchiveRef != archiveRef(p))) || (!terminal && p.ArchiveRef != "") {
		return invalidProposal("invalid workflow destination or archive")
	}
	return nil
}

func preparationNonce(p codehost.Proposal) string {
	if p.ReviewRequest != nil {
		return p.ReviewRequest.Nonce
	}
	if p.TaskPreparation != nil {
		return p.TaskPreparation.Nonce
	}
	if p.CandidatePreparation != nil {
		return p.CandidatePreparation.Nonce
	}
	return ""
}

func prepareProposal(ctx context.Context, root string, p codehost.Proposal) (string, error) {
	body, _ := json.Marshal(p)
	blob, err := proposalGit(ctx, root, append(body, '\n'), nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := proposalGit(ctx, root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\tproposal.json\n"), nil, "mktree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", strings.TrimSpace(string(tree))}
	for _, parent := range proposalParents(p) {
		args = append(args, "-p", parent)
	}
	env := []string{"GIT_AUTHOR_NAME=Pyxis local proposal", "GIT_COMMITTER_NAME=Pyxis local proposal", "GIT_AUTHOR_EMAIL=local-proposal@pyxis.invalid", "GIT_COMMITTER_EMAIL=local-proposal@pyxis.invalid", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z"}
	oid, err := proposalGit(ctx, root, []byte("Pyxis local proposal generation\n"), env, args...)
	return strings.TrimSpace(string(oid)), err
}

func proposalParents(p codehost.Proposal) []string {
	var parents []string
	oids := []string{p.PreviousOID, p.Head, p.TargetOID}
	if p.CandidatePreparation != nil {
		oids = append(oids, p.CandidatePreparation.Candidate.Commit)
	}
	if p.TaskLanding != nil {
		oids = append(oids, p.TaskLanding.MergeCommit)
	}
	if p.CandidateLanding != nil {
		oids = append(oids, p.CandidateLanding.Candidate.Commit, p.CandidateLanding.MergeCommit, p.CandidateLanding.FeatureMarker)
	}
	if p.DraftLock != nil {
		oids = append(oids, p.DraftLock.MergeCommit, p.DraftLock.Review.OID)
	}
	for _, oid := range oids {
		if oid != "" && !slices.Contains(parents, oid) {
			parents = append(parents, oid)
		}
	}
	return parents
}

func verifyPair(ctx context.Context, origin string, generation codehost.ProposalGeneration, sourceRef string) error {
	p := generation.Proposal
	required := []string{p.Branch, proposalPrefix + p.ID, p.ArchiveRef, sourceRef}
	if sourceRef != "" {
		required = append(required, p.TargetRef)
	}
	refs, err := proposalRefs(ctx, origin, required...)
	if err != nil {
		return err
	}
	expectedBranch := p.Head
	if p.State == "landed" || p.State == "locked" {
		expectedBranch = ""
	}
	if refs[p.Branch] != expectedBranch || refs[proposalPrefix+p.ID] != generation.OID || (p.ArchiveRef != "" && refs[p.ArchiveRef] != generation.OID) {
		return fmt.Errorf("%w: origin branch, proposal, or archive differs", codehost.ErrProposalConflict)
	}
	if p.State == "landed" && p.CandidateLanding != nil {
		// Final landing moves both workflow heads. Its dedicated reader checks
		// the exact retained marker, delivered tree, and current ancestry.
		if refs[sourceRef] == "" || refs[p.TargetRef] == "" {
			return codehost.ErrProposalConflict
		}
		return nil
	}
	if sourceRef != "" && refs[sourceRef] != p.Head {
		return fmt.Errorf("%w: feature source moved; rebuild final eligibility", codehost.ErrProposalConflict)
	}
	if sourceRef != "" && refs[p.TargetRef] != p.TargetOID {
		return fmt.Errorf("%w: mainline moved; rebuild final eligibility", codehost.ErrProposalConflict)
	}
	return nil
}

func proposalRefs(ctx context.Context, origin string, refs ...string) (map[string]string, error) {
	args := []string{"for-each-ref", "--format=%(refname) %(objectname) %(objecttype)"}
	for _, ref := range refs {
		if ref != "" {
			args = append(args, ref)
		}
	}
	body, err := proposalGit(ctx, origin, nil, nil, args...)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(body)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 || fields[2] != "commit" || !objectID(fields[1]) {
			return nil, invalidProposal("origin ref does not name a commit")
		}
		result[fields[0]] = fields[1]
	}
	return result, nil
}

func commitObject(ctx context.Context, root, oid string) error {
	if !objectID(oid) {
		return invalidProposal("expected full commit object id")
	}
	out, err := proposalGit(ctx, root, nil, nil, "cat-file", "-t", oid)
	if err != nil || strings.TrimSpace(string(out)) != "commit" {
		return invalidProposal("%s is not a readable commit", oid)
	}
	return nil
}

func archiveRef(p codehost.Proposal) string {
	return "refs/pyxis/proposal-archives/" + p.ID + "/" + strconv.FormatUint(p.Sequence, 10)
}

func proposalID(id string) bool {
	if id == "" || len(id) > 128 || id[0] == '-' || id[len(id)-1] == '-' {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func invalidProposal(format string, args ...any) error {
	return fmt.Errorf("%w: %s", codehost.ErrProposalInvalid, fmt.Sprintf(format, args...))
}

func proposalGit(ctx context.Context, root string, input []byte, overrides []string, args ...string) ([]byte, error) {
	cmd := proposalCommand(ctx, root, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(cmd.Env, overrides...)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("local Git %s: %w: %s", args[0], err, strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func proposalCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "protocol.allow=never", "-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = root
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(key, "GIT_") {
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	// Retained OIDs name the original objects, never a local replacement view.
	cmd.Env = append(cmd.Env, "GIT_NO_REPLACE_OBJECTS=1")
	return cmd
}
