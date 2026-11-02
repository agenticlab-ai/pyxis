package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/github"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/trackeridentity"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/watch"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

type integrationReader struct {
	port.ProposalStateReader
	cadence time.Duration
}

func (r integrationReader) Cadence(context.Context) time.Duration { return r.cadence }

type proposalAssociation struct {
	Schema string `json:"schema"`
	Task   string `json:"task"`
	Source string `json:"source"`
	Number int    `json:"number"`
}

var integrationForgejoClient = forgejo.New
var integrationCommandOutput = func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() }

// configuredWatch independently binds task facts and proposal state. An absent
// tasks/code-host binding keeps the legacy watch path, including its identities.
func configuredWatch(repo, legacyRepo, legacyTracker, carinaRoot string, cfg repoconfig.Config, mapping planMap, complete bool) (integrationReader, watch.PlanSource, string, bool, error) {
	icfg, err := integrations.Load(repo)
	if err != nil {
		return integrationReader{}, nil, "", false, err
	}
	if icfg.Bindings.Tasks == (integrations.Binding{}) && icfg.Bindings.CodeHost == (integrations.Binding{}) {
		return integrationReader{}, nil, "", false, nil
	}
	tasks, err := icfg.Tasks(legacyRepo, legacyTracker, carinaRoot)
	if err != nil {
		return integrationReader{}, nil, "", true, err
	}
	code, err := icfg.CodeHost(legacyRepo)
	if err != nil {
		return integrationReader{}, nil, "", true, err
	}
	if code.Provider == integrations.LocalGit {
		probe, err := localCodeHostDiagnostics(repo, cfg)
		if err == nil {
			err = codehost.RequireWorkflow(context.Background(), probe)
		}
		return integrationReader{}, nil, integrations.LocalGit, true, err
	}
	var reader integrationReader
	if code.Provider == integrations.Forgejo {
		client, err := integrationForgejoClient(code.BaseURL, code.Token())
		if err != nil {
			return reader, nil, "", true, err
		}
		host := forgejocodehost.New(client, code.BaseURL, code.Repository, cfg.Branch)
		reader = integrationReader{ProposalStateReader: host, cadence: host.Cadence()}
	} else {
		host := ghcodehost.NewStateReaderWithTokenEnv(code.Repository, code.TokenEnv, "github.com")
		reader = integrationReader{ProposalStateReader: host, cadence: host.Cadence()}
	}
	source := integrationPlanSource(repo, cfg.Remote, cfg.Branch, tasks, code, mapping, complete)
	return reader, source, code.Provider, true, nil
}

var integrationClaimFacts = claimedIssuesScoped
var integrationAncestry = proposalOnMainline
var integrationTaskFacts = readIntegrationTasks
var integrationProposalFacts = readIntegrationProposals
var integrationCloseTask = closeIntegrationTask

// integrationPlanSource uses the proposal's durable exact task association,
// not an issue-number guess, before completing a task on another service.
// status passes complete=false so its source remains read-only.
func integrationPlanSource(repo, remote, mainline string, tasks, code integrations.ResolvedRepository, mapping planMap, complete bool) watch.PlanSource {
	return func(ctx context.Context) (watch.Plan, error) {
		closed, err := integrationTaskFacts(ctx, repo, tasks, mapping)
		if err != nil {
			return watch.Plan{}, err
		}
		pulls, err := integrationProposalFacts(ctx, repo, code)
		if err != nil {
			return watch.Plan{}, err
		}
		claims, err := integrationClaimFacts(ctx, repo, remote, tasks.Key())
		if err != nil {
			return watch.Plan{}, err
		}
		byTask := map[string]forgejocodehost.Pull{}
		for _, pull := range pulls {
			association, found, err := readProposalAssociation(pull.Body)
			if err != nil {
				return watch.Plan{}, fmt.Errorf("proposal #%d task association: %w", pull.Number, err)
			}
			if !found || mapping.issueByTask[association.Task] != association.Number || association.Source != trackeridentity.SourceRef(tasks, association.Number) {
				continue
			}
			// The body is mutable host data. Also require the supervisor's scoped
			// head and the configured repository, preventing a numeric cross-host
			// lookalike from being treated as this task's proposal.
			scope := (port.Task{SourceKey: tasks.Key()}).ScopeToken()
			if !strings.HasPrefix(pull.Head.Ref, "feat/"+scope+"/") || headIssue(pull.Head.Ref) != association.Number || pull.Head.Repo.FullName != code.Repository {
				continue
			}
			if existing, ok := byTask[association.Task]; ok && existing.Number != pull.Number {
				return watch.Plan{}, fmt.Errorf("task %s has multiple associated proposals", association.Task)
			}
			byTask[association.Task] = pull
		}
		var ids []string
		for id := range mapping.issueByTask {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var plan watch.Plan
		for _, id := range ids {
			number := mapping.issueByTask[id]
			task := watch.Task{ID: id, BlockedBy: mapping.blockedBy[id], Closed: closed[number], Claimed: claims[number]}
			if pull, exists := byTask[id]; exists {
				if !task.Closed && pull.Merged && complete {
					if pull.Base.Ref != mainline || pull.MergeCommitSHA == "" {
						return watch.Plan{}, fmt.Errorf("proposal #%d merge does not certify configured mainline", pull.Number)
					}
					if err := integrationAncestry(ctx, repo, remote, mainline, pull.MergeCommitSHA); err != nil {
						return watch.Plan{}, err
					}
					if err := integrationCloseTask(ctx, repo, tasks, number); err != nil {
						return watch.Plan{}, err
					}
					task.Closed = true
				}
				if !task.Closed {
					task.Proposal = &port.ProposalRef{Number: pull.Number}
				}
			}
			plan.Tasks = append(plan.Tasks, task)
		}
		return plan, nil
	}
}

func readProposalAssociation(body string) (proposalAssociation, bool, error) {
	const marker = "<!-- pyxis-task-proposal:"
	_, rest, found := strings.Cut(body, marker)
	if !found {
		return proposalAssociation{}, false, nil
	}
	encoded, tail, closed := strings.Cut(rest, "-->")
	if !closed || strings.Contains(tail, marker) {
		return proposalAssociation{}, true, fmt.Errorf("unterminated or duplicate association")
	}
	wire, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return proposalAssociation{}, true, fmt.Errorf("decode association: %w", err)
	}
	var association proposalAssociation
	dec := json.NewDecoder(bytes.NewReader(wire))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&association); err != nil {
		return association, true, fmt.Errorf("parse association: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF || association.Schema != "pyxis-task-proposal/v1" || association.Task == "" || association.Source == "" || association.Number <= 0 {
		return association, true, fmt.Errorf("invalid task association")
	}
	return association, true, nil
}

func readIntegrationTasks(ctx context.Context, repo string, binding integrations.ResolvedRepository, mapping planMap) (map[int]bool, error) {
	closed := map[int]bool{}
	var forgejoTracker *forgejotracker.Forgejo
	if binding.Provider == integrations.Forgejo {
		client, err := integrationForgejoClient(binding.BaseURL, binding.Token())
		if err != nil {
			return nil, err
		}
		forgejoTracker = forgejotracker.NewForgejo(client, binding.Repository)
	}
	for _, number := range mapping.issueByTask {
		switch binding.Provider {
		case integrations.Forgejo:
			issue, err := forgejoTracker.Issue(ctx, number)
			if err != nil {
				return nil, err
			}
			closed[number] = issue.State == "closed"
		case integrations.GitHub:
			out, err := integrationGitHub(ctx, repo, binding, "--method", "GET", fmt.Sprintf("repos/%s/issues/%d", binding.Repository, number))
			if err != nil {
				return nil, err
			}
			var issue struct {
				Number int    `json:"number"`
				State  string `json:"state"`
			}
			if err := json.Unmarshal(out, &issue); err != nil || issue.Number != number || (issue.State != "open" && issue.State != "closed") {
				return nil, fmt.Errorf("invalid GitHub task state for #%d", number)
			}
			closed[number] = issue.State == "closed"
		case integrations.Carina:
			states, err := carinatracker.NewStateReader(binding.Root).States(ctx, []port.ProposalRef{{Number: number}}, port.Cursor{})
			if err != nil {
				return nil, err
			}
			if len(states) != 1 || states[0].Unknown != nil {
				return nil, fmt.Errorf("unknown Carina task state for item %d", number)
			}
			closed[number] = states[0].Landing != port.LandingOpen
		default:
			return nil, fmt.Errorf("unsupported task provider %q", binding.Provider)
		}
	}
	return closed, nil
}

func readIntegrationProposals(ctx context.Context, repo string, binding integrations.ResolvedRepository) ([]forgejocodehost.Pull, error) {
	if binding.Provider == integrations.Forgejo {
		client, err := integrationForgejoClient(binding.BaseURL, binding.Token())
		if err != nil {
			return nil, err
		}
		return forgejocodehost.New(client, binding.BaseURL, binding.Repository, "").Pulls(ctx, "all")
	}
	data, err := integrationGitHub(ctx, repo, binding, "--paginate", "--method", "GET", "repos/"+binding.Repository+"/pulls?state=all&per_page=100")
	if err != nil {
		return nil, err
	}
	var pulls []forgejocodehost.Pull
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var page []forgejocodehost.Pull
		if err := dec.Decode(&page); err == io.EOF {
			return pulls, nil
		} else if err != nil {
			return nil, fmt.Errorf("parse GitHub proposals: %w", err)
		}
		for i := range page {
			// GitHub's list shape omits the definitive merged boolean. Exact
			// read closed proposals before considering task completion.
			if page[i].State == "closed" {
				detail, err := integrationGitHub(ctx, repo, binding, "--method", "GET", fmt.Sprintf("repos/%s/pulls/%d", binding.Repository, page[i].Number))
				if err != nil {
					return nil, err
				}
				var exact forgejocodehost.Pull
				if err := json.Unmarshal(detail, &exact); err != nil || exact.Number != page[i].Number {
					return nil, fmt.Errorf("invalid GitHub proposal detail for #%d", page[i].Number)
				}
				page[i] = exact
			}
			pulls = append(pulls, page[i])
		}
	}
}

func closeIntegrationTask(ctx context.Context, repo string, binding integrations.ResolvedRepository, number int) error {
	switch binding.Provider {
	case integrations.Forgejo:
		client, err := integrationForgejoClient(binding.BaseURL, binding.Token())
		if err != nil {
			return err
		}
		return forgejotracker.NewForgejo(client, binding.Repository).CloseContext(ctx, number)
	case integrations.GitHub:
		_, err := integrationGitHub(ctx, repo, binding, "--method", "PATCH", fmt.Sprintf("repos/%s/issues/%d", binding.Repository, number), "-f", "state=closed")
		if err != nil {
			return err
		}
	case integrations.Carina:
		if _, defects := localfs.New(binding.Root, time.Now).Transition(ctx, number, model.StateClosed, model.CloseCompleted, "pyxis-watch"); len(defects) != 0 {
			return fmt.Errorf("complete Carina task #%d: %v", number, defects)
		}
	default:
		return fmt.Errorf("unsupported task completion provider %q", binding.Provider)
	}
	states, err := readIntegrationTasks(ctx, repo, binding, planMap{issueByTask: map[string]int{"completion": number}})
	if err != nil {
		return err
	}
	if !states[number] {
		return fmt.Errorf("task #%d remains open after completion", number)
	}
	return nil
}

func integrationGitHub(ctx context.Context, repo string, binding integrations.ResolvedRepository, args ...string) ([]byte, error) {
	u, err := url.Parse(binding.BaseURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid GitHub server origin")
	}
	cmd := exec.CommandContext(ctx, "gh", append([]string{"api", "--hostname", u.Host}, args...)...)
	cmd.Dir = repo
	cmd.Env, err = githubauth.EnvironmentForHost(binding.TokenEnv, "github.com")
	if err != nil {
		return nil, err
	}
	out, err := integrationCommandOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("GitHub task/proposal request to %s failed: %w", u.Host, err)
	}
	return out, nil
}

func claimedIssuesScoped(ctx context.Context, repo, remote, source string) (map[int]bool, error) {
	scope := (port.Task{SourceKey: source}).ScopeToken()
	prefix := "refs/claim/" + scope + "/"
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "ls-remote", remote, prefix+"*")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list scoped claims: %w", err)
	}
	set := map[int]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		_, ref, ok := strings.Cut(line, "\t"+prefix)
		if !ok {
			continue
		}
		number, err := strconv.Atoi(strings.TrimSpace(ref))
		if err == nil && number > 0 {
			set[number] = true
		}
	}
	return set, nil
}

func proposalOnMainline(ctx context.Context, repo, remote, mainline, revision string) error {
	if revision == "" || strings.HasPrefix(revision, "-") {
		return fmt.Errorf("proposal has no safe merge revision")
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", repo, "fetch", "--no-tags", remote, mainline).CombinedOutput(); err != nil {
		return fmt.Errorf("fetch mainline for task completion: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := exec.CommandContext(ctx, "git", "-C", repo, "merge-base", "--is-ancestor", revision, "FETCH_HEAD").Run(); err != nil {
		return fmt.Errorf("proposal merge %s is not verified on configured mainline: %w", revision, err)
	}
	return nil
}
