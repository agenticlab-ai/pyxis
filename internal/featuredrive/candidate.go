package featuredrive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	ErrCandidateBlocked  = errors.New("feature drive: candidate blocked by incomplete task")
	ErrCandidateStale    = errors.New("feature drive: candidate inputs moved")
	ErrCandidateConflict = errors.New("feature drive: candidate merge conflict")
	ErrCandidateExists   = errors.New("feature drive: candidate publication does not match")
	ErrRepairCreation    = errors.New("feature drive: conflict repair creation failed")
)

type MergePolicy struct {
	Version string `json:"version"`
}

// CandidateIdentity is the complete, content-addressed identity of a candidate.
type CandidateIdentity struct {
	FeatureID          string `json:"feature_id"`
	FeatureHead        string `json:"feature_head"`
	MainHead           string `json:"main_head"`
	Commit             string `json:"commit"`
	Tree               string `json:"tree"`
	MergePolicyVersion string `json:"merge_policy_version"`
	BuilderVersion     string `json:"builder_version"`
}

func (c CandidateIdentity) Digest() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (c CandidateIdentity) Ref() string {
	return "refs/pyxis/candidates/" + c.FeatureID + "/" + c.Digest()
}

type CandidateBuilder interface {
	Build(context.Context, string, string, MergePolicy, string) (CandidateIdentity, error)
}
type CandidatePublisher interface {
	CreateOnly(context.Context, string, CandidateIdentity) error
}

// GitCandidateBuilder constructs the canonical candidate using Git's plumbing
// commands. It never updates a branch or the index in the supplied repository.
type GitCandidateBuilder struct{ Repository string }

func NewGitCandidateBuilder(repository string) GitCandidateBuilder {
	return GitCandidateBuilder{Repository: repository}
}

func (b GitCandidateBuilder) Build(ctx context.Context, featureHead, mainHead string, policy MergePolicy, builderVersion string) (CandidateIdentity, error) {
	if b.Repository == "" {
		return CandidateIdentity{}, fmt.Errorf("%w: repository is empty", ErrCandidateStale)
	}
	// Resolve both names before doing any work. This also makes the identity
	// independent of mutable branch spelling passed by an adapter.
	f, err := gitOutput(ctx, b.Repository, "rev-parse", "--verify", featureHead+"^{commit}")
	if err != nil {
		return CandidateIdentity{}, fmt.Errorf("resolve feature head: %w", err)
	}
	m, err := gitOutput(ctx, b.Repository, "rev-parse", "--verify", mainHead+"^{commit}")
	if err != nil {
		return CandidateIdentity{}, fmt.Errorf("resolve main head: %w", err)
	}
	tree, err := gitOutput(ctx, b.Repository, "merge-tree", "--write-tree", m, f)
	if err != nil {
		return CandidateIdentity{}, fmt.Errorf("%w: %s", ErrCandidateConflict, strings.TrimSpace(err.Error()))
	}
	// Fixed identity and timestamp make the synthetic commit reproducible.
	env := append(os.Environ(), "GIT_AUTHOR_NAME=pyxis-candidate", "GIT_AUTHOR_EMAIL=pyxis-candidate@invalid", "GIT_COMMITTER_NAME=pyxis-candidate", "GIT_COMMITTER_EMAIL=pyxis-candidate@invalid", "GIT_AUTHOR_DATE=1970-01-01T00:00:00Z", "GIT_COMMITTER_DATE=1970-01-01T00:00:00Z")
	commit, err := gitOutputEnv(ctx, b.Repository, env, "commit-tree", tree, "-p", m, "-p", f, "-m", "pyxis candidate\n\nmerge-policy: "+policy.Version+"\nbuilder: "+builderVersion)
	if err != nil {
		return CandidateIdentity{}, fmt.Errorf("create candidate commit: %w", err)
	}
	return CandidateIdentity{FeatureHead: f, MainHead: m, Commit: commit, Tree: tree, MergePolicyVersion: policy.Version, BuilderVersion: builderVersion}, nil
}

// GitCandidatePublisher publishes only a previously absent candidate ref.
type GitCandidatePublisher struct{ Repository string }

func NewGitCandidatePublisher(repository string) GitCandidatePublisher {
	return GitCandidatePublisher{Repository: repository}
}

func (p GitCandidatePublisher) CreateOnly(ctx context.Context, ref string, c CandidateIdentity) error {
	if p.Repository == "" || ref == "" {
		return fmt.Errorf("%w: incomplete publication", ErrCandidateExists)
	}
	if existing, err := gitOutput(ctx, p.Repository, "rev-parse", "--verify", ref+"^{commit}"); err == nil {
		if existing == c.Commit {
			return nil
		}
		return ErrCandidateExists
	}
	// update-ref's old-value argument is the null object, making publication
	// atomic and create-only even when two drives race.
	if _, err := gitOutput(ctx, p.Repository, "update-ref", ref, c.Commit, strings.Repeat("0", 40)); err != nil {
		if existing, lookupErr := gitOutput(ctx, p.Repository, "rev-parse", "--verify", ref+"^{commit}"); lookupErr == nil && existing == c.Commit {
			return nil
		}
		return fmt.Errorf("publish candidate: %w", err)
	}
	return nil
}

func gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	return gitOutputEnv(ctx, repo, nil, args...)
}
func gitOutputEnv(ctx context.Context, repo string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// BuildCandidate builds and publishes one candidate after re-reading both heads.
// The builder owns the medium-specific merge; callers provide the exact refs
// observed at the barrier, so a moving source cannot silently become evidence.
func BuildCandidate(ctx context.Context, featureID, featureHead, mainHead string, policy MergePolicy, builderVersion string, builder CandidateBuilder, publisher CandidatePublisher) (CandidateIdentity, error) {
	if featureID == "" || featureHead == "" || mainHead == "" || policy.Version == "" || builderVersion == "" || builder == nil || publisher == nil {
		return CandidateIdentity{}, fmt.Errorf("%w: incomplete candidate inputs", ErrCandidateStale)
	}
	c, err := builder.Build(ctx, featureHead, mainHead, policy, builderVersion)
	if err != nil {
		return CandidateIdentity{}, err
	}
	if c.FeatureID == "" {
		c.FeatureID = featureID
	}
	if c.FeatureID != featureID || c.FeatureHead != featureHead || c.MainHead != mainHead || c.MergePolicyVersion != policy.Version || c.BuilderVersion != builderVersion || c.Commit == "" || c.Tree == "" {
		return CandidateIdentity{}, fmt.Errorf("%w: builder returned an identity different from requested inputs", ErrCandidateStale)
	}
	if err := publisher.CreateOnly(ctx, c.Ref(), c); err != nil {
		return CandidateIdentity{}, err
	}
	return c, nil
}

type CandidateTask struct {
	ID     string
	Landed bool
}

func CandidateReady(tasks []CandidateTask) error {
	for _, t := range tasks {
		if t.ID == "" || !t.Landed {
			return fmt.Errorf("%w: task %q", ErrCandidateBlocked, t.ID)
		}
	}
	return nil
}

type ConflictRepair struct {
	FeatureID   string `json:"feature_id"`
	FeatureHead string `json:"feature_head"`
	MainHead    string `json:"main_head"`
	Report      string `json:"report"`
	ID          string `json:"id"`
}

func NewConflictRepair(featureID, featureHead, mainHead, report string) ConflictRepair {
	r := ConflictRepair{FeatureID: featureID, FeatureHead: featureHead, MainHead: mainHead, Report: report}
	b, _ := json.Marshal(r)
	h := sha256.Sum256(b)
	r.ID = hex.EncodeToString(h[:])
	return r
}

// CandidateMerge is the medium-neutral result of the merge attempt.
type CandidateMerge struct {
	Identity       CandidateIdentity
	ConflictReport string
}

func PublishCandidate(ctx context.Context, featureID, featureHead, mainHead string, policy MergePolicy, builderVersion string, builder CandidateBuilder, publisher CandidatePublisher, repairs func(context.Context, ConflictRepair) error) (CandidateIdentity, error) {
	c, err := BuildCandidate(ctx, featureID, featureHead, mainHead, policy, builderVersion, builder, publisher)
	if err == nil {
		return c, nil
	}
	if errors.Is(err, ErrCandidateConflict) && repairs != nil {
		// The merge failure remains authoritative: even when repair creation fails,
		// the caller must never mistake this attempt for a candidate.
		repairErr := repairs(ctx, NewConflictRepair(featureID, featureHead, mainHead, err.Error()))
		if repairErr != nil {
			return CandidateIdentity{}, errors.Join(err, fmt.Errorf("%w: %w", ErrRepairCreation, repairErr))
		}
	}
	return CandidateIdentity{}, err
}
