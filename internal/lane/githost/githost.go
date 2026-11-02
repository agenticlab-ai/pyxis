// Package githost is the merge lane's production adapter set for a GitHub
// code host: the read-only Medium, the verdict-block Recorder, and the
// activation-gated machine-identity Machine (merge-lane-v1 §3.8's seams;
// plan T4's wiring). Every platform touch shells an authenticated `gh` behind
// an injectable run seam, the house ghclient pattern, so the argv construction
// and JSON decoding are unit-tested without a network. The Medium and Recorder
// carry no acting power: reads, plus one append-only comment the verdict
// history rides. Machine exists only in a deployment that explicitly
// constructs it with the dedicated machine identity's environment (DINV-5) —
// the shadow workflow never does.
package githost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/lane/acting"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// BlockMarker opens the one PR comment that carries the append-only lane
// verdict block (JSONL, verdict.AppendBlock's carrier). The marker is how the
// Recorder finds its own comment across evaluations; everything after the
// marker's line is the block.
const BlockMarker = "<!--lane-verdict-block/v1-->"

// Runner executes one `gh` invocation and returns its stdout — the injectable
// seam hermetic tests substitute.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// shellGH is the production Runner: it execs an authenticated `gh` (the repo's
// standing assumption — hooks and CI already require it) and surfaces gh's
// stderr on failure.
func shellGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Host binds the adapters to one repository slug (owner/name) and one local
// clone root (for the mainline-ancestry read, which is a git read, not an API
// read). It implements acting.Medium and acting.Recorder.
type Host struct {
	Forgejo  *forgejocodehost.Host
	Repo     string // owner/name
	Root     string // local clone whose origin is Repo (ancestry reads)
	Remote   string // the git remote the ancestry read fetches; blank → the config default (config-driven-portability-v1 §3.2, FR-2)
	Mainline string // the default branch the merged content must be an ancestor of; blank → the config default (FR-2)
	Run      Runner // nil → the production gh runner
	Git      GitRunner
}

// remote and mainline resolve the Host's configured remote/branch, defaulting
// to the config schema's single source (config.Defaults — "origin"/"main", the
// ONE place those literals live, DINV-1) so a Host built by a direct struct
// literal (the hermetic tests) behaves byte-identically to the pre-config
// engine (DINV-2). The production caller (cmd/lane) always supplies the values
// resolved from the repo config.
func (h *Host) remote() string {
	if h.Remote == "" {
		return config.Defaults().Remote
	}
	return h.Remote
}

func (h *Host) mainline() string {
	if h.Mainline == "" {
		return config.Defaults().Branch
	}
	return h.Mainline
}

// GitRunner executes one git read under a directory — the ancestry check's
// seam.
type GitRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

func gitRun(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// New binds a Host to a repo slug and clone root with the production runners.
// remote and mainline are the configured git remote and default branch the
// mainline-ancestry read resolves against (config-driven-portability-v1 §3.2,
// FR-2); a blank value defaults to origin/main (DINV-2).
func New(repo, root, remote, mainline string) *Host {
	return &Host{Repo: repo, Root: root, Remote: remote, Mainline: mainline}
}

func (h *Host) run(ctx context.Context, args ...string) ([]byte, error) {
	r := h.Run
	if r == nil {
		r = shellGH
	}
	return r(ctx, args...)
}

func (h *Host) git(ctx context.Context, args ...string) ([]byte, error) {
	g := h.Git
	if g == nil {
		g = gitRun
	}
	return g(ctx, h.Root, args...)
}

// api runs one exact REST read with the freshness directive (AP-15: exact
// endpoint, never the search index, never a cacheable conditional read).
func (h *Host) api(ctx context.Context, method, path string, fields ...string) ([]byte, error) {
	args := []string{"api", "-X", method, "-H", "Cache-Control: no-cache", path}
	for _, f := range fields {
		args = append(args, "-f", f)
	}
	return h.run(ctx, args...)
}

// --- acting.Medium ---

// Head returns the proposal's current head revision, read fresh at act time so
// a post-evaluation push voids the verdict (DINV-3, AP-15).
func (h *Host) Head(ctx context.Context, proposal string) (string, error) {
	if h.Forgejo != nil {
		return h.forgejoHead(ctx, proposal)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/pulls/%s", h.Repo, proposal))
	if err != nil {
		return "", fmt.Errorf("reading head of PR %s: %w", proposal, err)
	}
	var pr struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return "", fmt.Errorf("decoding PR %s: %w", proposal, err)
	}
	if pr.Head.SHA == "" {
		return "", fmt.Errorf("PR %s carries no head sha", proposal)
	}
	return pr.Head.SHA, nil
}

// AllRequiredGreen reads the check runs and commit statuses of head from the
// medium's own set and reports whether every one concluded successfully
// (AP-11: the platform's required set, never a private copy). Zero reported
// checks reads false — a proposal with no mechanical verdict is never "all
// green" (fail-closed).
func (h *Host) AllRequiredGreen(ctx context.Context, proposal, head string) (bool, error) {
	if h.Forgejo != nil {
		return h.forgejoGreen(ctx, proposal, head)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/commits/%s/check-runs", h.Repo, head))
	if err != nil {
		return false, fmt.Errorf("reading check runs of %s: %w", head, err)
	}
	var page struct {
		Total     int `json:"total_count"`
		CheckRuns []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		return false, fmt.Errorf("decoding check runs of %s: %w", head, err)
	}
	if len(page.CheckRuns) == 0 {
		return false, nil
	}
	for _, cr := range page.CheckRuns {
		if cr.Status != "completed" {
			return false, nil
		}
		switch cr.Conclusion {
		case "success", "neutral", "skipped":
		default:
			return false, nil
		}
	}
	return true, nil
}

// ReviewVerdict returns the review-verdict arm's published check state for the
// proposal at head. The arm publishes under verdict.CheckContext as a check
// run whose output body carries the CheckState JSON; absence of that check is
// a non-approving state naming the absence — the precondition is unmet until
// the arm has published (DINV-3: the state is produced, never parsed from the
// proposal's tree).
func (h *Host) ReviewVerdict(ctx context.Context, proposal, head string) (verdict.CheckState, error) {
	if h.Forgejo != nil {
		return h.forgejoReviewVerdict(ctx, proposal, head)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/commits/%s/check-runs", h.Repo, head))
	if err != nil {
		return verdict.CheckState{}, fmt.Errorf("reading the review-verdict check of %s: %w", head, err)
	}
	var page struct {
		CheckRuns []struct {
			Name   string `json:"name"`
			Output struct {
				Summary string `json:"summary"`
			} `json:"output"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		return verdict.CheckState{}, fmt.Errorf("decoding the review-verdict check of %s: %w", head, err)
	}
	for _, cr := range page.CheckRuns {
		if cr.Name != verdict.CheckContext {
			continue
		}
		var st verdict.CheckState
		if err := json.Unmarshal([]byte(cr.Output.Summary), &st); err != nil {
			return verdict.CheckState{Context: verdict.CheckContext, Status: verdict.NonApproving,
				Reason: "the published review-verdict state is unreadable: " + err.Error()}, nil
		}
		return st, nil
	}
	return verdict.CheckState{Context: verdict.CheckContext, Status: verdict.NonApproving,
		Reason: "no review-verdict check state is published for this head"}, nil
}

// MergedIntoMainline reports whether the proposal's merge commit content is an
// ancestor of the mainline — the goal-object verification after an act
// (AP-26/I2: the content at the merge base, never the platform badge).
func (h *Host) MergedIntoMainline(ctx context.Context, proposal, head string) (bool, error) {
	if h.Forgejo != nil {
		return h.forgejoMerged(ctx, proposal, head)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/pulls/%s", h.Repo, proposal))
	if err != nil {
		return false, fmt.Errorf("reading merge state of PR %s: %w", proposal, err)
	}
	var pr struct {
		Merged         bool   `json:"merged"`
		MergeCommitSHA string `json:"merge_commit_sha"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return false, fmt.Errorf("decoding merge state of PR %s: %w", proposal, err)
	}
	if !pr.Merged || pr.MergeCommitSHA == "" {
		return false, nil
	}
	if _, err := h.git(ctx, "fetch", h.remote(), h.mainline(), "--quiet"); err != nil {
		return false, fmt.Errorf("fetching mainline for the ancestry read: %w", err)
	}
	if _, err := h.git(ctx, "merge-base", "--is-ancestor", pr.MergeCommitSHA, h.remote()+"/"+h.mainline()); err != nil {
		// git exits 1 for "not an ancestor" — the lying-badge case — and the
		// runner folds every non-zero exit into err. Either way the content is
		// not verified at the goal object; the caller halts the lane (DINV-7).
		return false, nil
	}
	return true, nil
}

// --- acting.Recorder ---

// RecordVerdict appends one evaluation's record to the proposal's append-only
// verdict block — a single marker-opened PR comment holding JSONL records
// (verdict.AppendBlock). The first evaluation creates the comment; later ones
// append, never rewrite earlier lines. Recording runs in every state including
// shadow; it is not an act (FR-8).
func (h *Host) RecordVerdict(ctx context.Context, proposal string, rec verdict.Record) error {
	if h.Forgejo != nil {
		return h.forgejoRecord(ctx, proposal, rec)
	}
	id, block, err := h.findBlockComment(ctx, proposal)
	if err != nil {
		return err
	}
	newBlock := verdict.AppendBlock(block, rec)
	body := BlockMarker + "\n" + "```json\n" + strings.TrimRight(string(newBlock), "\n") + "\n```"
	if id == 0 {
		_, err = h.api(ctx, "POST", fmt.Sprintf("repos/%s/issues/%s/comments", h.Repo, proposal), "body="+body)
		if err != nil {
			return fmt.Errorf("creating the verdict block on PR %s: %w", proposal, err)
		}
		return nil
	}
	_, err = h.api(ctx, "PATCH", fmt.Sprintf("repos/%s/issues/comments/%d", h.Repo, id), "body="+body)
	if err != nil {
		return fmt.Errorf("appending to the verdict block on PR %s: %w", proposal, err)
	}
	return nil
}

// LastRecordedHead returns the head revision of the proposal's most recent
// verdict record, or "" when no verdict block exists yet — the sweep's
// idempotence read: an unchanged head needs no re-evaluation (the workflow's
// per-synchronize semantics, replayed host-side).
func (h *Host) LastRecordedHead(ctx context.Context, proposal string) (string, error) {
	_, block, err := h.findBlockComment(ctx, proposal)
	if err != nil || len(block) == 0 {
		return "", err
	}
	recs, err := verdict.ParseBlock(block)
	if err != nil || len(recs) == 0 {
		// A malformed block is a fault surfaced at parse; the sweep treats it as
		// "re-evaluate" rather than trusting a corrupt history (fail-closed
		// toward a fresh record, never toward silence).
		return "", nil
	}
	return recs[len(recs)-1].HeadRevision, nil
}

// OpenProposals lists the repository's open pull requests (number, head sha)
// via the exact REST endpoint — the sweep's work list.
func (h *Host) OpenProposals(ctx context.Context) ([]Proposal, error) {
	if h.Forgejo != nil {
		return h.forgejoProposals(ctx)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/pulls?state=open&per_page=100", h.Repo))
	if err != nil {
		return nil, fmt.Errorf("listing open PRs: %w", err)
	}
	var pulls []struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(out, &pulls); err != nil {
		return nil, fmt.Errorf("decoding open PRs: %w", err)
	}
	ps := make([]Proposal, 0, len(pulls))
	for _, p := range pulls {
		ps = append(ps, Proposal{Number: p.Number, Head: p.Head.SHA})
	}
	return ps, nil
}

// Proposal is one open pull request the sweep evaluates.
type Proposal struct {
	Number int
	Head   string
}

// findBlockComment locates the marker-opened verdict comment and returns its
// id and current block bytes (0, nil when none exists yet).
func (h *Host) findBlockComment(ctx context.Context, proposal string) (int64, []byte, error) {
	if h.Forgejo != nil {
		return h.forgejoBlock(ctx, proposal)
	}
	out, err := h.api(ctx, "GET", fmt.Sprintf("repos/%s/issues/%s/comments?per_page=100", h.Repo, proposal))
	if err != nil {
		return 0, nil, fmt.Errorf("listing comments of PR %s: %w", proposal, err)
	}
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &comments); err != nil {
		return 0, nil, fmt.Errorf("decoding comments of PR %s: %w", proposal, err)
	}
	for _, c := range comments {
		if !strings.HasPrefix(c.Body, BlockMarker) {
			continue
		}
		return c.ID, extractBlock(c.Body), nil
	}
	return 0, nil, nil
}

// extractBlock recovers the JSONL block from the comment body — the lines
// between the fence markers after the block marker.
func extractBlock(body string) []byte {
	var lines []string
	in := false
	for line := range strings.SplitSeq(body, "\n") {
		switch {
		case strings.HasPrefix(line, "```"):
			if in {
				return []byte(strings.Join(lines, "\n") + "\n")
			}
			in = true
		case in:
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// --- acting.Machine (activation-gated) ---

// Machine is the credentialed acting adapter — approve + merge as the
// dedicated machine identity, bound to the exact evaluated head. It exists
// only where the deployment explicitly constructs it (the enforcing workflow
// holding the machine credential in its secret store); the shadow workflow and
// every drive session construct the Host WITHOUT one, so no session can act
// (DINV-5). The identity name is data; the credential never enters this
// package — the runner's `gh` resolves auth from the environment the workflow
// sealed.
type Machine struct {
	Host *Host
	Name string // the dedicated machine identity's login
	// The goal-object verification (the mainline-ancestry read) resolves the
	// configured remote/branch from Host (MergedIntoMainline), so the Machine
	// carries no mainline literal of its own (config-driven-portability-v1
	// §3.2, FR-2; DINV-1).
}

// Identity names the machine actor — structurally distinguishable from any
// human (A-8).
func (m *Machine) Identity() acting.Identity {
	return acting.Identity{Name: m.Name, Machine: true}
}

// Act approves and merges the proposal bound to the exact evaluated head: the
// approval cites the head it evaluated, and the merge passes the platform's
// head-match guard so a post-approval push fails the act rather than landing
// unevaluated content (DINV-3).
func (m *Machine) Act(ctx context.Context, proposal, head string) error {
	if m.Host.Forgejo != nil {
		return m.forgejoAct(ctx, proposal, head)
	}
	cur, err := m.Host.Head(ctx, proposal)
	if err != nil {
		return err
	}
	if cur != head {
		return fmt.Errorf("the head moved from %s to %s; the act is void (DINV-3)", head, cur)
	}
	if _, err := m.Host.api(ctx, "POST",
		fmt.Sprintf("repos/%s/pulls/%s/reviews", m.Host.Repo, proposal),
		"event=APPROVE", "body=lane AUTO act — evaluated head "+head, "commit_id="+head); err != nil {
		return fmt.Errorf("the machine approval of PR %s failed: %w", proposal, err)
	}
	if _, err := m.Host.run(ctx, "api", "-X", "PUT", "-H", "Cache-Control: no-cache",
		fmt.Sprintf("repos/%s/pulls/%s/merge", m.Host.Repo, proposal),
		"-f", "merge_method=squash", "-f", "sha="+head); err != nil {
		return fmt.Errorf("the machine merge of PR %s at %s failed: %w", proposal, head, err)
	}
	return nil
}
