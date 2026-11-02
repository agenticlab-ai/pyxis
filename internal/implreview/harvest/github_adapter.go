package harvest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// GitHubAdapter is the shipped, read-only GitHub edge for the corpus harvest.
// It enumerates the full PR universe, locates the latest artifact-bearing issue
// comment, exact-reads that comment, and resolves disposition-comment authors.
type GitHubAdapter struct {
	repository string
	run        func(context.Context, ...string) ([]byte, error)
}

var _ Enumerator = (*GitHubAdapter)(nil)
var _ persist.Reader = (*GitHubAdapter)(nil)
var _ AuthorResolver = (*GitHubAdapter)(nil)

func NewGitHubAdapter(repository string) (*GitHubAdapter, error) {
	return newGitHubAdapter(repository, shellGitHubHarvest)
}

func newGitHubAdapter(repository string, run func(context.Context, ...string) ([]byte, error)) (*GitHubAdapter, error) {
	repository = strings.TrimSpace(repository)
	owner, name, ok := strings.Cut(repository, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") || run == nil {
		return nil, errors.New("harvest github: repository and command runner are required")
	}
	return &GitHubAdapter{repository: repository, run: run}, nil
}

func (a *GitHubAdapter) Enumerate(ctx context.Context) ([]Proposal, error) {
	data, err := a.run(ctx, "api", "--paginate", "--slurp", "repos/"+a.repository+"/pulls?state=all&per_page=100")
	if err != nil {
		return nil, fmt.Errorf("harvest github: enumerate pull requests: %w", err)
	}
	var pages [][]struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("harvest github: decode pull requests: %w", err)
	}
	var out []Proposal
	for _, page := range pages {
		for _, pr := range page {
			if pr.Number <= 0 {
				return nil, errors.New("harvest github: pull request carried no number")
			}
			proposalRef, err := persist.GitHubProposalRef(a.repository, pr.Number)
			if err != nil {
				return nil, err
			}
			comments, err := a.comments(ctx, pr.Number)
			if err != nil {
				return nil, err
			}
			proposal := Proposal{Ref: proposalRef}
			var invalidCandidate *persist.Ref
			for _, comment := range comments {
				body := strings.ReplaceAll(comment.Body, "\r\n", "\n")
				locator := strconv.FormatInt(comment.ID, 10)
				if parsed, parseErr := persist.ParseArtifact(body); parseErr == nil && parsed.ProposalRef == proposalRef {
					ref := persist.Ref{Medium: persist.MediumGitHub, Locator: locator}
					proposal.Review, proposal.Absent, proposal.AbsentReason = &ref, false, ""
					continue
				} else if hasExactFence(body, persist.FenceTag) {
					ref := persist.Ref{Medium: persist.MediumGitHub, Locator: locator}
					invalidCandidate = &ref
				}
				if reason, absent := parseAbsenceMarker(body, proposalRef); absent && proposal.Review == nil {
					proposal.Absent, proposal.AbsentReason = true, reason
				}
			}
			if proposal.Review == nil && !proposal.Absent && invalidCandidate != nil {
				proposal.Review = invalidCandidate
			}
			out = append(out, proposal)
		}
	}
	return out, nil
}

// HasExactFence reports whether body contains an exact fenced block for tag.
func HasExactFence(body, tag string) bool { return hasExactFence(body, tag) }

func hasExactFence(body, tag string) bool {
	want := "```" + tag
	for line := range strings.SplitSeq(body, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// ParseAbsenceMarker extracts a recorded review-absence from a comment body.
func ParseAbsenceMarker(body, proposalRef string) (string, bool) {
	return parseAbsenceMarker(body, proposalRef)
}

func parseAbsenceMarker(body, proposalRef string) (string, bool) {
	lines := strings.Split(body, "\n")
	open := -1
	for index, line := range lines {
		if strings.TrimSpace(line) != "```impl-review-absent" {
			continue
		}
		if open >= 0 {
			return "", false
		}
		open = index
	}
	if open < 0 {
		return "", false
	}
	close := -1
	for index := open + 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "```" {
			close = index
			break
		}
	}
	if close < 0 {
		return "", false
	}
	var marker struct {
		Absent      bool   `json:"absent"`
		ProposalRef string `json:"proposal_ref"`
	}
	decoder := json.NewDecoder(strings.NewReader(strings.Join(lines[open+1:close], "\n")))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil || !marker.Absent || marker.ProposalRef != proposalRef {
		return "", false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", false
	}
	reason := "review artifact publication was recorded absent"
	for _, line := range lines[:open] {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "cause:"); ok && strings.TrimSpace(value) != "" {
			reason = strings.TrimSpace(value)
		}
	}
	return reason, true
}

type githubHarvestComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (a *GitHubAdapter) comments(ctx context.Context, proposal int) ([]githubHarvestComment, error) {
	data, err := a.run(ctx, "api", "--paginate", "--slurp", fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", a.repository, proposal))
	if err != nil {
		return nil, fmt.Errorf("harvest github: list comments for #%d: %w", proposal, err)
	}
	var pages [][]githubHarvestComment
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("harvest github: decode comments for #%d: %w", proposal, err)
	}
	var out []githubHarvestComment
	for _, page := range pages {
		out = append(out, page...)
	}
	return out, nil
}

func (a *GitHubAdapter) Read(ctx context.Context, ref persist.Ref) (*implreport.Report, error) {
	if ref.Medium != persist.MediumGitHub {
		return nil, errors.New("harvest github: reference belongs to another medium")
	}
	comment, found, err := a.readComment(ctx, ref.Locator)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("harvest github: artifact comment not found")
	}
	return persist.ParseArtifact(strings.ReplaceAll(comment.Body, "\r\n", "\n"))
}

func (a *GitHubAdapter) AuthorOf(ctx context.Context, attribution string) (string, bool, error) {
	comment, found, err := a.readComment(ctx, attributionCommentID(attribution))
	if err != nil || !found {
		return "", found, err
	}
	if strings.TrimSpace(comment.User.Login) == "" {
		return "", false, nil
	}
	return comment.User.Login, true, nil
}

func (a *GitHubAdapter) readComment(ctx context.Context, locator string) (githubHarvestComment, bool, error) {
	id := attributionCommentID(locator)
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return githubHarvestComment{}, false, fmt.Errorf("harvest github: invalid comment reference %q", locator)
	}
	data, err := a.run(ctx, "api", "repos/"+a.repository+"/issues/comments/"+id, "--header", "Cache-Control: no-cache")
	if err != nil {
		return githubHarvestComment{}, false, fmt.Errorf("harvest github: read comment %s: %w", id, err)
	}
	var comment githubHarvestComment
	if err := json.Unmarshal(data, &comment); err != nil {
		return githubHarvestComment{}, false, fmt.Errorf("harvest github: decode comment %s: %w", id, err)
	}
	if comment.ID == 0 {
		return githubHarvestComment{}, false, nil
	}
	return comment, true, nil
}

func attributionCommentID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "github-comment:")
	if index := strings.LastIndexByte(value, ':'); index >= 0 {
		value = value[index+1:]
	}
	return value
}

func shellGitHubHarvest(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
