package wire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// ForgejoRequiredCheckContext follows Forgejo Actions' workflow / job (event)
// status name for the installed docs-gate pull-request workflow.
const ForgejoRequiredCheckContext = "docs-gate / docs-gate (pull_request)"

// TableForIntegrations routes task operations and code-host administration
// independently, preserving Table's legacy derivation when no bindings exist.
func TableForIntegrations(cfg Config, bindings integrations.Config) ([]Op, error) {
	if bindings.SchemaVersion == "" {
		return Table(cfg), nil
	}
	tasks, err := bindings.Tasks(cfg.Slug, cfg.Repo.Knobs.Tracker, cfg.Repo.WorkItems)
	if err != nil {
		return nil, err
	}
	if tasks.Provider == integrations.Forgejo {
		cfg.Repo.Knobs.Tracker = integrations.GitHub
	} else {
		cfg.Repo.Knobs.Tracker = tasks.Provider
	}
	if bindings.CI.Mode != "" {
		cfg.Repo.Knobs.CIMode = bindings.CI.Mode
	}
	if bindings.Bindings.CodeHost.Provider == integrations.LocalGit {
		k := cfg.Repo.Knobs
		if k.Wiring.Repo == "create" || k.Wiring.Protection == "required-checks" || k.Wiring.Protection == "checks+review" || k.CIMode == "actions" || k.CIMode == "both" {
			return nil, fmt.Errorf("local-git requires an existing bare origin, local CI, and no hosted repository/protection wiring; set ci.mode=local, wiring.repo=use, and wiring.protection=none")
		}
	}
	ops := Table(cfg)
	var host integrations.ResolvedRepository
	for _, op := range ops {
		if op.Name == OpCreateRepo || op.Name == OpSetProtection || op.Name == OpPushWorkflow {
			host, err = bindings.CodeHost(cfg.Slug)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	for i := range ops {
		op := &ops[i]
		if op.Name == OpSetProtection {
			hostedContext := RequiredCheckContext
			if host.Provider == integrations.Forgejo {
				hostedContext = ForgejoRequiredCheckContext
			}
			op.Contexts = []string{hostedContext}
			switch cfg.Repo.Knobs.CIMode {
			case "local":
				op.Contexts = []string{"pyxis/local-gate"}
			case "both":
				op.Contexts = []string{hostedContext, "pyxis/local-gate"}
			}
		}
		if op.Transport == TransportConverge {
			op.Connection = host
			op.Slug = host.Repository
			continue
		}
		target := host
		if op.Name == OpEnsureLabels || op.Name == OpWelcomeItem || op.Name == OpOnboardingItems {
			target = tasks
		}
		op.Connection = target
		op.Slug = target.Repository
		if target.Provider == integrations.Forgejo {
			op.Transport = TransportForgejo
		}
		op.Condition += "; " + target.Provider + " " + target.Repository
	}
	return ops, nil
}

// WithForgejoClientFactory injects the one native API transport per server.
func WithForgejoClientFactory(f func(integrations.ResolvedRepository) (*forgejo.Client, error)) ExecOption {
	return func(x *executor) { x.forgejoClient = f }
}

func defaultForgejoClient(r integrations.ResolvedRepository) (*forgejo.Client, error) {
	if r.Token() == "" {
		return nil, fmt.Errorf("forgejo credential is missing: set %s for %s", r.TokenEnv, r.BaseURL)
	}
	return forgejo.New(r.BaseURL, r.Token())
}

type forgejoIssue struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	URL    string `json:"html_url"`
}

type forgejoLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type forgejoProtection struct {
	RuleName               string   `json:"rule_name"`
	BranchName             string   `json:"branch_name"`
	EnablePush             bool     `json:"enable_push"`
	EnableStatusCheck      bool     `json:"enable_status_check"`
	StatusCheckContexts    []string `json:"status_check_contexts"`
	RequiredApprovals      int      `json:"required_approvals"`
	BlockOnRejectedReviews bool     `json:"block_on_rejected_reviews"`
	DismissStaleApprovals  bool     `json:"dismiss_stale_approvals"`
}

func (x *executor) runForgejoOp(ctx context.Context, op Op, st *walkState) (OpResult, error) {
	res := OpResult{Name: op.Name, Transport: op.Transport, Connection: op.Connection}
	start := x.now()
	client, err := x.forgejoClient(op.Connection)
	if err != nil {
		return res, err
	}
	var failedCommand string
	var pendingCommands []string
	request := func(method, p string, body, out any) error {
		res.Commands = append(res.Commands, []string{method, op.Connection.BaseURL + "/api/v1/" + p})
		failedCommand = forgejoCommand(op.Connection, method, p, body)
		return client.Request(ctx, method, p, body, out)
	}
	list := func(p string, out any) error {
		res.Commands = append(res.Commands, []string{http.MethodGet, op.Connection.BaseURL + "/api/v1/" + p})
		failedCommand = forgejoCommand(op.Connection, http.MethodGet, p, nil)
		return client.List(ctx, p, out)
	}
	if st.repoDeferred {
		res.Outcome = OutcomeHandedOff
		res.Reason = "deferred: create-repo did not complete; complete repository creation before this operation"
		st.entries = append(st.entries, handoffEntry{op: op.Name, why: res.Reason, raw: strings.Join(forgejoEnsureCommands(op), "\n")})
		return res, nil
	}
	switch op.Name {
	case OpCreateRepo:
		var repo struct {
			ID int64 `json:"id"`
		}
		err = request(http.MethodGet, forgejo.RepoPath(op.Slug, ""), nil, &repo)
		if err == nil {
			res.Outcome = OutcomeSkipped
			res.Reason = "repository already exists"
			break
		}
		var he *forgejo.HTTPError
		if !errors.As(err, &he) || he.Status != http.StatusNotFound {
			break
		}
		var user forgejo.User
		err = request(http.MethodGet, "user", nil, &user)
		if err != nil {
			break
		}
		owner, name, _ := strings.Cut(op.Slug, "/")
		endpoint := "user/repos"
		if !strings.EqualFold(owner, user.Login) {
			endpoint = "orgs/" + url.PathEscape(owner) + "/repos"
		}
		err = request(http.MethodPost, endpoint, struct {
			Name    string `json:"name"`
			Private bool   `json:"private"`
		}{name, true}, &repo)
	case OpEnsureLabels:
		var labels []forgejoLabel
		err = list(forgejo.RepoPath(op.Slug, "labels"), &labels)
		if err != nil {
			break
		}
		exists := map[string]bool{}
		for _, label := range labels {
			exists[label.Name] = true
		}
		pending := op
		pending.Labels = nil
		for _, label := range op.Labels {
			if exists[label.Name] {
				continue
			}
			pending.Labels = append(pending.Labels, label)
		}
		for len(pending.Labels) > 0 {
			label := pending.Labels[0]
			err = request(http.MethodPost, forgejo.RepoPath(op.Slug, "labels"), forgejoLabel{Name: label.Name, Color: strings.TrimPrefix(label.Color, "#"), Description: label.Description}, nil)
			if err != nil {
				pendingCommands = forgejoEnsureCommands(pending)
				break
			}
			pending.Labels = pending.Labels[1:]
		}
	case OpSetProtection:
		p := forgejoProtection{RuleName: op.Branch, BranchName: op.Branch, EnablePush: false, EnableStatusCheck: true, StatusCheckContexts: op.Contexts, BlockOnRejectedReviews: true, DismissStaleApprovals: true}
		if op.RequireReview {
			p.RequiredApprovals = 1
		}
		var prior forgejoProtection
		endpoint := forgejo.RepoPath(op.Slug, "branch_protections/"+url.PathEscape(op.Branch))
		err = request(http.MethodGet, endpoint, nil, &prior)
		method := http.MethodPatch
		if err != nil {
			var he *forgejo.HTTPError
			if !errors.As(err, &he) || he.Status != http.StatusNotFound {
				break
			}
			method = http.MethodPost
			endpoint = forgejo.RepoPath(op.Slug, "branch_protections")
		}
		err = request(method, endpoint, p, nil)
		if err == nil {
			var actual forgejoProtection
			err = request(http.MethodGet, forgejo.RepoPath(op.Slug, "branch_protections/"+url.PathEscape(op.Branch)), nil, &actual)
			if err == nil && !protectionContains(actual, p) {
				err = fmt.Errorf("forgejo branch protection readback does not enforce the requested policy")
			}
		}
	case OpWelcomeItem, OpOnboardingItems:
		var issues []forgejoIssue
		err = list(forgejo.RepoPath(op.Slug, "issues?state=all&type=issues"), &issues)
		if err != nil {
			break
		}
		items := op.Items
		if op.Name == OpWelcomeItem {
			items = []WorkItem{{Title: op.Title, Body: op.Body}}
		} else if len(st.entries) > 0 {
			items = append(append([]WorkItem(nil), items...), wiringRemainderItem(st.entries))
		}
		pending := op
		pending.Name = OpOnboardingItems
		pending.Items = nil
		for _, item := range items {
			marker := WelcomeMarker
			if item.Slug != "" {
				marker = OnboardingMarker(item.Slug)
			}
			var matches []forgejoIssue
			for _, issue := range issues {
				if markerOutsideFences(issue.Body, marker) {
					matches = append(matches, issue)
				}
			}
			if len(matches) > 1 {
				err = fmt.Errorf("forgejo onboarding marker %q matches multiple issues; refusing an ambiguous update", marker)
				break
			}
			if len(matches) == 1 {
				res.Refs = append(res.Refs, matches[0].URL)
				continue
			}
			pending.Items = append(pending.Items, item)
		}
		if err != nil {
			break
		}
		for len(pending.Items) > 0 {
			item := pending.Items[0]
			var created forgejoIssue
			err = request(http.MethodPost, forgejo.RepoPath(op.Slug, "issues"), struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}{item.Title, item.Body}, &created)
			if err != nil {
				pendingCommands = forgejoEnsureCommands(pending)
				break
			}
			res.Refs = append(res.Refs, created.URL)
			pending.Items = pending.Items[1:]
		}
	default:
		err = fmt.Errorf("undeclared Forgejo operation %q", op.Name)
	}
	res.Duration = x.now().Sub(start)
	if op.Name == OpCreateRepo {
		st.repoDeferred = err != nil
		st.repoCreated = err == nil && res.Outcome != OutcomeSkipped
	}
	if err == nil {
		if res.Outcome == "" {
			res.Outcome = OutcomeApplied
			res.Reason = "Forgejo operation completed"
		}
		return res, nil
	}
	if ctx.Err() != nil {
		res.Outcome = OutcomeFailed
		res.Reason = "cancelled"
		return res, ctx.Err()
	}
	var he *forgejo.HTTPError
	if errors.As(err, &he) && (he.Status == 401 || he.Status == 403 || he.Status == 429 || he.Status >= 500) {
		res.Outcome = OutcomeHandedOff
		res.Reason = err.Error() + "; update the selected connection's access and retry the listed operation"
		if len(pendingCommands) > 0 {
			failedCommand = strings.Join(pendingCommands, "\n")
		}
		st.entries = append(st.entries, handoffEntry{op: op.Name, why: res.Reason, raw: failedCommand})
		return res, nil
	}
	x.fail(&res, st, err.Error())
	return res, nil
}

func protectionContains(actual, requested forgejoProtection) bool {
	if actual.EnablePush || !actual.EnableStatusCheck || actual.RequiredApprovals < requested.RequiredApprovals || !actual.BlockOnRejectedReviews || !actual.DismissStaleApprovals {
		return false
	}
	contexts := make(map[string]bool, len(actual.StatusCheckContexts))
	for _, name := range actual.StatusCheckContexts {
		contexts[name] = true
	}
	for _, name := range requested.StatusCheckContexts {
		if !contexts[name] {
			return false
		}
	}
	return true
}

// forgejoEnsureCommands renders the declared remainder after repository
// creation was handed off. No credential values are embedded in the remedy.
func forgejoEnsureCommands(op Op) []string {
	var commands []string
	add := func(path string, body any) {
		commands = append(commands, forgejoCommand(op.Connection, http.MethodPost, forgejo.RepoPath(op.Slug, path), body))
	}
	switch op.Name {
	case OpEnsureLabels:
		for _, label := range op.Labels {
			add("labels", forgejoLabel{label.Name, strings.TrimPrefix(label.Color, "#"), label.Description})
		}
	case OpSetProtection:
		p := forgejoProtection{RuleName: op.Branch, BranchName: op.Branch, EnableStatusCheck: true, StatusCheckContexts: op.Contexts, BlockOnRejectedReviews: true, DismissStaleApprovals: true}
		if op.RequireReview {
			p.RequiredApprovals = 1
		}
		add("branch_protections", p)
	case OpWelcomeItem, OpOnboardingItems:
		items := op.Items
		if op.Name == OpWelcomeItem {
			items = []WorkItem{{Title: op.Title, Body: op.Body}}
		}
		for _, item := range items {
			add("issues", struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}{item.Title, item.Body})
		}
	}
	return commands
}

func forgejoCommand(r integrations.ResolvedRepository, method, p string, body any) string {
	command := "curl --fail --request " + shellQuote(method) + " --url " + shellQuote(r.BaseURL+"/api/v1/"+p) + " --header \"Authorization: token ${" + r.TokenEnv + "}\""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return "# request body could not be rendered; re-run the declared operation"
		}
		command += " --header 'Content-Type: application/json' --data " + shellQuote(string(b))
	}
	return command
}
