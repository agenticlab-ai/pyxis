package trackeridentity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type legacyProposal struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	Head   struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

var migrationOutput = func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() }

// CheckLegacy performs read-only migration admission. A legacy numeric claim
// has no task-host identity, so it cannot safely coexist with a newly scoped
// claim. Existing work is retained and the operator gets a concrete remedy.
func CheckLegacy(ctx context.Context, repo, remote string, code integrations.ResolvedRepository) error {
	return checkLegacy(ctx, repo, remote, code, 0)
}

// CheckIssueMigration returns the source whose scoped claims must also reserve
// a numeric compatibility ref. Only an identical hosted task/code binding can
// interpret legacy numbers; ambiguous bindings retain the repository-wide guard.
// The caller must pass the returned key to lease.Config.LegacySourceKey: this
// read-only check alone cannot exclude a legacy worker racing after admission.
func CheckIssueMigration(ctx context.Context, repo, remote string, tasks, code integrations.ResolvedRepository, number int) (string, error) {
	if number <= 0 {
		return "", fmt.Errorf("migration admission requires a positive issue number")
	}
	if (code.Provider != integrations.GitHub && code.Provider != integrations.Forgejo) || tasks.Key() != code.Key() {
		return "", CheckLegacy(ctx, repo, remote, code)
	}
	if err := checkLegacy(ctx, repo, remote, code, number); err != nil {
		return "", err
	}
	return tasks.Key(), nil
}

func checkLegacy(ctx context.Context, repo, remote string, code integrations.ResolvedRepository, selected int) error {
	out, err := migrationOutput(exec.CommandContext(ctx, "git", "-C", repo, "ls-remote", remote, "refs/claim/*"))
	if err != nil {
		return fmt.Errorf("inspect legacy claims before scoped dispatch: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		_, ref, ok := strings.Cut(line, "\trefs/claim/")
		if !ok {
			continue
		}
		if number, err := strconv.Atoi(strings.TrimSpace(ref)); err == nil && number > 0 && (selected == 0 || number == selected) {
			return fmt.Errorf("scoped dispatch blocked by legacy claim refs/claim/%d; finish or reconcile its existing worker before changing integration bindings", number)
		}
	}
	rows, err := legacyProposals(ctx, repo, code)
	if err != nil {
		return err
	}
	for _, row := range rows {
		branch := strings.TrimPrefix(row.Head.Ref, "feat/")
		if branch == row.Head.Ref || strings.Contains(branch, "/") {
			continue
		}
		last := strings.LastIndex(branch, "-")
		if last < 0 {
			continue
		}
		if issue, err := strconv.Atoi(branch[last+1:]); err == nil && issue > 0 && (selected == 0 || issue == selected) {
			return fmt.Errorf("scoped dispatch blocked by legacy proposal #%d on %s; finish or explicitly reconcile its existing branch before changing integration bindings", row.Number, row.Head.Ref)
		}
	}
	return nil
}

func legacyProposals(ctx context.Context, root string, binding integrations.ResolvedRepository) ([]legacyProposal, error) {
	if binding.Provider == integrations.Forgejo {
		client, err := forgejo.New(binding.BaseURL, binding.Token())
		if err != nil {
			return nil, err
		}
		var rows []legacyProposal
		if err := client.List(ctx, forgejo.RepoPath(binding.Repository, "pulls")+"?state=open", &rows); err != nil {
			return nil, err
		}
		return rows, nil
	}
	if binding.Provider != integrations.GitHub {
		return nil, fmt.Errorf("unsupported code host for migration admission")
	}
	u, err := url.Parse(binding.BaseURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid GitHub origin for migration admission")
	}
	cmd := exec.CommandContext(ctx, "gh", "api", "--hostname", u.Host, "--paginate", "repos/"+binding.Repository+"/pulls?state=open&per_page=100")
	cmd.Dir = root
	cmd.Env, err = githubauth.EnvironmentForHost(binding.TokenEnv, "github.com")
	if err != nil {
		return nil, err
	}
	data, err := migrationOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("inspect legacy proposals before scoped dispatch: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var rows []legacyProposal
	for {
		var page []legacyProposal
		if err := dec.Decode(&page); err == io.EOF {
			return rows, nil
		} else if err != nil {
			return nil, fmt.Errorf("decode legacy proposals: %w", err)
		}
		rows = append(rows, page...)
	}
}
