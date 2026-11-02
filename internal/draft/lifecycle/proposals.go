package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

// ProposalLifecycle uses semantic proposal operations. Its process adapter
// supplies only local Git operations; it has no hosted identity fallback.
type ProposalLifecycle struct {
	Root    string
	Config  config.Config
	Store   codehost.DraftProposals
	Tracker worktracker.Lister
	Run     RunCommand
	Locked  LockedReader
}

type ProposalResult struct {
	Generation codehost.ProposalGeneration
	Version    int
}

func (l ProposalLifecycle) mainline(ctx context.Context) (codehost.Readiness, error) {
	if l.Root == "" || l.Store == nil || l.Run == nil {
		return codehost.Readiness{}, fmt.Errorf("draft requires repository, proposal store, and process adapter")
	}
	return l.Store.Mainline(ctx)
}

// Inspect reads ledgers from the pinned origin mainline, never working files.
func (l ProposalLifecycle) Inspect(ctx context.Context) (Snapshot, []codehost.ProposalGeneration, error) {
	r, err := l.mainline(ctx)
	if err != nil {
		return Snapshot{}, nil, err
	}
	return l.inspect(ctx, r.MainlineOID)
}

func (l ProposalLifecycle) inspect(ctx context.Context, base string) (Snapshot, []codehost.ProposalGeneration, error) {
	s := Snapshot{OpenDrafts: map[string]bool{}, LockedVersions: map[string]int{}, PRDLedgers: map[string]bool{}}
	ledgers, err := l.Run(ctx, l.Root, nil, "git", "ls-tree", "-r", "--name-only", base, "--", l.Config.PRDs)
	if err != nil {
		return s, nil, fmt.Errorf("read mainline ledgers: %w", err)
	}
	prefix := filepath.ToSlash(l.Config.PRDs) + "/"
	for line := range strings.SplitSeq(string(ledgers), "\n") {
		if feature, ok := strings.CutSuffix(strings.TrimPrefix(line, prefix), "/000-base.md"); ok && strings.HasPrefix(line, prefix) && validFeature(feature) {
			s.PRDLedgers[feature] = true
		}
	}
	tags, err := l.Run(ctx, l.Root, nil, "git", "ls-remote", "--tags", l.Config.Remote, "refs/tags/lock/*/v*")
	if err != nil {
		return s, nil, fmt.Errorf("read origin locks: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(tags)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return s, nil, fmt.Errorf("malformed origin lock advertisement")
		}
		feature, raw, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(fields[1], "refs/tags/lock/"), "^{}"), "/v")
		if version, err := strconv.Atoi(raw); ok && validFeature(feature) && err == nil && version > s.LockedVersions[feature] {
			s.LockedVersions[feature] = version
		}
	}
	proposals, err := l.Store.List(ctx)
	if err != nil {
		return s, nil, err
	}
	for _, g := range proposals {
		p := g.Proposal
		if p.Kind != "draft" || p.State == "abandoned" || p.State == "locked" {
			continue
		}
		version := s.LockedVersions[p.Feature] + 1
		if !validFeature(p.Feature) || p.ID != proposalID(p.Feature, version) || p.Branch != "refs/heads/"+draftBranch(l.Config, p.Feature, version) || p.TargetRef != "refs/heads/"+l.Config.Branch || s.OpenDrafts[p.Feature] {
			return s, nil, fmt.Errorf("origin has inconsistent or duplicate draft proposals for %s", p.Feature)
		}
		s.OpenDrafts[p.Feature] = true
	}
	return s, proposals, nil
}

func proposalID(feature string, version int) string {
	return fmt.Sprintf("draft-%s-v%d", feature, version)
}

func draftBranch(cfg config.Config, feature string, version int) string {
	branch := cfg.Draft.BranchPrefix + feature
	if version > 1 {
		branch += "-v" + strconv.Itoa(version)
	}
	return branch
}

// Open scaffolds from one fetched mainline and atomically publishes its pair.
func (l ProposalLifecycle) Open(ctx context.Context, feature string) (ProposalResult, error) {
	return l.start(ctx, feature, "", "")
}

func (l ProposalLifecycle) Reopen(ctx context.Context, feature, cause, causeRef string) (ProposalResult, error) {
	if cause != "new-need" && cause != "dependency-change" && cause != "gap-lock-should-have-caught" && cause != "other" || cause != "other" && strings.TrimSpace(causeRef) == "" {
		return ProposalResult{}, fmt.Errorf("reopen needs a valid cause and its evidence reference")
	}
	return l.start(ctx, feature, cause, causeRef)
}

func (l ProposalLifecycle) start(ctx context.Context, feature, cause, causeRef string) (out ProposalResult, err error) {
	if !validFeature(feature) {
		return out, fmt.Errorf("feature must be a lowercase hyphenated slug fitting the proposal identity")
	}
	r, err := l.mainline(ctx)
	if err != nil {
		return out, err
	}
	if err := l.clean(ctx); err != nil {
		return out, err
	}
	s, proposals, err := l.inspect(ctx, r.MainlineOID)
	if err != nil {
		return out, err
	}
	if s.PRDLedgers[feature] || s.OpenDrafts[feature] {
		return out, fmt.Errorf("%s already has a document ledger or open draft", feature)
	}
	previous := s.LockedVersions[feature]
	if (cause == "") != (previous == 0) {
		return out, fmt.Errorf("%s: use open for the first version and reopen for a binding lock", feature)
	}
	version := previous + 1
	if len(proposalID(feature, version)) > 128 {
		return out, fmt.Errorf("feature must be a lowercase hyphenated slug fitting the proposal identity")
	}
	for _, g := range proposals {
		if g.Proposal.ID == proposalID(feature, version) && g.Proposal.State == "abandoned" {
			return out, fmt.Errorf("%s has an abandoned draft v%d; choose a new feature slug", feature, version)
		}
	}
	step := "draft-open"
	if previous > 0 {
		step = "draft-reopen"
	}
	defer func() {
		if err != nil {
			err = recordStepFailure(ctx, l.Root, l.Config, feature, version, step, err)
		}
	}()
	files, err := scaffoldAt(ctx, l.Root, l.Config, feature, r.MainlineOID, l.Run)
	if err != nil {
		return out, err
	}
	if previous > 0 {
		if l.Locked == nil {
			return out, fmt.Errorf("reopen requires a verified locked-contract reader")
		}
		// Open tasks keep running: the reopened draft may change their
		// charters in the plan, and its lock carries contract and plan together.
		seed, err := l.Locked(ctx, l.Root, l.Config.Remote, feature, previous, l.Run)
		if err != nil {
			return out, err
		}
		if seed.Version != previous || len(seed.PRD) == 0 || len(seed.TechSpec) == 0 || compile.ContractDigest(seed.PRD, seed.TechSpec) != seed.Digest {
			return out, fmt.Errorf("binding lock has incomplete or inconsistent contract text")
		}
		for i := range files {
			switch filepath.Base(files[i].Path) {
			case "prd.md":
				files[i].Body = seed.PRD
			case "tech-spec.md":
				files[i].Body = seed.TechSpec
			}
		}
	}
	var prd, tech []byte
	for _, file := range files {
		if filepath.Base(file.Path) == "prd.md" {
			prd = file.Body
		}
		if filepath.Base(file.Path) == "tech-spec.md" {
			tech = file.Body
		}
	}
	queue, err := newPendingEvents(l.Root, l.Config.Remote, feature, version)
	if err != nil {
		return out, err
	}
	opened, err := events.New(events.DraftOpened, feature, compile.ContractDigest(prd, tech), time.Now())
	if err != nil {
		return out, err
	}
	opened.Origin = "new"
	if previous > 0 {
		opened.Origin, opened.BasisVersion = "reopen", &previous
	}
	files, err = scaffoldEvents(ctx, files, queue, opened)
	if err != nil {
		return out, err
	}
	if previous > 0 {
		if causeRef == "" {
			causeRef = fmt.Sprintf("lock/%s/v%d", feature, previous)
		}
		reopened, err := events.New(events.DraftReopened, feature, opened.VersionDigest, time.Now())
		if err != nil {
			return out, err
		}
		reopened.FromVersion, reopened.Cause, reopened.CauseRef = previous, cause, causeRef
		for i := range files {
			if filepath.Base(files[i].Path) == "events.md" {
				files[i].Body, _, err = events.Append(files[i].Body, reopened)
				if err != nil {
					return out, err
				}
			}
		}
	}
	branch := draftBranch(l.Config, feature, version)
	priorBranch, err := l.command(ctx, "branch", "--show-current")
	if err != nil {
		return out, err
	}
	priorHead, err := l.command(ctx, "rev-parse", "HEAD")
	if err != nil {
		return out, err
	}
	if _, err := l.command(ctx, "switch", "-c", branch, r.MainlineOID); err != nil {
		return out, err
	}
	var written []string
	committed, publishing := false, false
	defer func() {
		if err != nil && !publishing {
			err = errors.Join(err, rollbackDraftStart(ctx, l.Root, branch, priorBranch, priorHead, written, committed, l.Run))
		}
	}()
	for _, file := range files {
		full := filepath.Join(l.Root, file.Path)
		if _, statErr := os.Lstat(full); !os.IsNotExist(statErr) {
			return out, fmt.Errorf("scaffold path already exists or is unreadable: %s", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return out, err
		}
		written = append(written, file.Path)
		if err := os.WriteFile(full, file.Body, 0644); err != nil {
			return out, err
		}
	}
	if _, err := l.command(ctx, append([]string{"add", "--"}, written...)...); err != nil {
		return out, err
	}
	if _, err := l.command(ctx, "commit", "-m", fmt.Sprintf("docs(draft): open %s v%d", feature, version)); err != nil {
		return out, err
	}
	committed = true
	head, err := l.command(ctx, "rev-parse", "HEAD")
	if err != nil {
		return out, err
	}
	publishing = true
	out.Generation, err = l.Store.Publish(ctx, codehost.ProposalChange{ID: proposalID(feature, version), Feature: feature, Branch: "refs/heads/" + branch, Head: head, State: "open", ExpectedTargetOID: r.MainlineOID})
	out.Version = version
	if err != nil {
		err = fmt.Errorf("publish %s at %s: %w; keep this branch and retry pyxis draft round %s", branch, head, err, feature)
	}
	return out, err
}

func (l ProposalLifecycle) clean(ctx context.Context) error {
	status, err := l.command(ctx, "status", "--porcelain")
	if err == nil && status != "" {
		err = fmt.Errorf("draft requires a clean working tree; commit or move changes first")
	}
	return err
}

func (l ProposalLifecycle) command(ctx context.Context, args ...string) (string, error) {
	out, err := l.Run(ctx, l.Root, nil, "git", args...)
	return strings.TrimSpace(string(out)), err
}

// PublishRound publishes committed working-file revisions. Prepared rounds
// are restored by the lock package before this method is called.
func (l ProposalLifecycle) PublishRound(ctx context.Context, feature string) (out ProposalResult, err error) {
	if !validFeature(feature) {
		return out, fmt.Errorf("publish draft round requires a valid feature")
	}
	r, err := l.mainline(ctx)
	if err != nil {
		return ProposalResult{}, err
	}
	if err := l.clean(ctx); err != nil {
		return ProposalResult{}, err
	}
	s, _, err := l.inspect(ctx, r.MainlineOID)
	if err != nil {
		return ProposalResult{}, err
	}
	version := s.LockedVersions[feature] + 1
	defer func() {
		if err != nil {
			err = recordStepFailure(ctx, l.Root, l.Config, feature, version, "draft-round", err)
		}
	}()
	branch := draftBranch(l.Config, feature, version)
	current, err := l.command(ctx, "branch", "--show-current")
	if err != nil || current != branch || s.PRDLedgers[feature] {
		return ProposalResult{}, errors.Join(err, fmt.Errorf("publish draft round from %s", branch))
	}
	head, err := l.command(ctx, "rev-parse", "HEAD")
	if err != nil {
		return ProposalResult{}, err
	}
	change := codehost.ProposalChange{ID: proposalID(feature, version), Feature: feature, Branch: "refs/heads/" + branch, Head: head, State: "open", ExpectedTargetOID: r.MainlineOID}
	g, err := l.Store.Read(ctx, change.ID)
	if err != nil && !errors.Is(err, codehost.ErrProposalNotFound) {
		return ProposalResult{}, err
	}
	if err == nil {
		if g.Proposal.State != "open" {
			return ProposalResult{}, fmt.Errorf("draft round is %s; restore a prepared round before publication", g.Proposal.State)
		}
		if g.Proposal.Head == head && g.Proposal.TargetOID == r.MainlineOID {
			return ProposalResult{g, version}, nil
		}
		change.ExpectedProposalOID, change.ExpectedBranchOID = g.OID, g.Proposal.Head
	}
	// The working event log also distinguishes a recoverable opening from an
	// arbitrary branch with the same name, before first paired publication.
	body, err := l.Run(ctx, l.Root, nil, "git", "show", head+":"+filepath.ToSlash(filepath.Join(l.Config.Drafts, feature, "events.md")))
	if err != nil {
		return ProposalResult{}, fmt.Errorf("read round event log: %w", err)
	}
	log, err := events.Parse(body)
	if err != nil || len(log) == 0 {
		return ProposalResult{}, errors.Join(err, fmt.Errorf("round has no valid opening event"))
	}
	opened := false
	for _, event := range log {
		if event.Feature != feature {
			return ProposalResult{}, fmt.Errorf("round events name another feature")
		}
		if event.Name == events.DraftOpened {
			opened = true
		}
	}
	if !opened {
		return ProposalResult{}, fmt.Errorf("round has no opening event")
	}
	next, err := l.Store.Publish(ctx, change)
	return ProposalResult{next, version}, err
}

// Abandon retains the terminal reason, exact head, and previous generation.
// It creates no lock and never updates mainline or deletes evidence.
func (l ProposalLifecycle) Abandon(ctx context.Context, feature, reason string) (out ProposalResult, err error) {
	r, err := l.mainline(ctx)
	if err != nil {
		return ProposalResult{}, err
	}
	s, _, err := l.inspect(ctx, r.MainlineOID)
	if err != nil || !validFeature(feature) {
		return ProposalResult{}, errors.Join(err, fmt.Errorf("abandon requires a valid feature"))
	}
	version := s.LockedVersions[feature] + 1
	defer func() {
		if err != nil {
			err = recordStepFailure(ctx, l.Root, l.Config, feature, version, "draft-abandon", err)
		}
	}()
	g, err := l.Store.Read(ctx, proposalID(feature, version))
	if err != nil {
		return ProposalResult{}, err
	}
	if g.Proposal.State == "abandoned" {
		if g.Proposal.AbandonmentReason != strings.TrimSpace(reason) {
			return ProposalResult{}, fmt.Errorf("terminal abandonment has a different retained reason")
		}
		return ProposalResult{g, version}, nil
	}
	if err := l.clean(ctx); err != nil {
		return ProposalResult{}, err
	}
	head, err := l.command(ctx, "rev-parse", "HEAD")
	if err != nil || head != g.Proposal.Head {
		return ProposalResult{}, errors.Join(err, fmt.Errorf("publish or reconcile the current draft head before abandonment"))
	}
	next, err := l.Store.Publish(ctx, codehost.ProposalChange{ID: g.Proposal.ID, Feature: feature, Branch: g.Proposal.Branch, Head: head, State: "abandoned", ExpectedProposalOID: g.OID, ExpectedBranchOID: head, ExpectedTargetOID: r.MainlineOID, AbandonmentReason: strings.TrimSpace(reason)})
	return ProposalResult{next, version}, err
}
