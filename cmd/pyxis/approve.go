package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strconv"
	"strings"

	approvalgithub "github.com/agenticlab-ai/pyxis/internal/adapters/approval/github"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

var publishDriverApproval = func(ctx context.Context, repository, tokenEnv string, decision approval.DriverDecision) (approval.Approval, error) {
	return approvalgithub.NewPolicyReader(repository, tokenEnv).Approve(ctx, decision)
}

func approveVerb(args []string, stdout, stderr io.Writer) int {
	const usage = "usage: pyxis approve <pull-number> --repo <owner/repo> --head <full-sha> --drive <slug> --evidence <review-summary-and-references> [--digest <prepared-contract-digest>] [--capture <text>]"
	if len(args) == 0 {
		terminalPrintln(stderr, usage)
		return 2
	}
	pull, err := strconv.Atoi(args[0])
	if err != nil || pull < 1 {
		terminalPrintln(stderr, usage)
		return 2
	}
	d := approval.DriverDecision{Schema: approval.DriverSchema, Principal: "argus", PullRequest: pull}
	flags := flag.NewFlagSet("approve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&d.Repository, "repo", "", "GitHub repository")
	flags.StringVar(&d.Head, "head", "", "exact reviewed full head SHA")
	flags.StringVar(&d.Drive, "drive", os.Getenv("ARGUS_DRIVE_SLUG"), "Argus problem slug")
	flags.StringVar(&d.Evidence, "evidence", "", "completed independent review evidence")
	flags.StringVar(&d.Digest, "digest", "", "reviewed prepared contract digest, for draft PRs")
	flags.StringVar(&d.CaptureLine, "capture", "", "optional approval capture")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || d.Repository == "" || d.Head == "" || d.Drive == "" || strings.TrimSpace(d.Evidence) == "" {
		terminalPrintln(stderr, usage)
		return 2
	}
	root, err := draftRepoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: approve: find repository: %v\n", err)
		return 1
	}
	cfg, defects := config.Load(root)
	if len(defects) != 0 {
		terminalPrintf(stderr, "pyxis: approve: configuration: %v\n", defects)
		return 1
	}
	// A local overlay may select a credential reference. Only host-read
	// pre-merge mainline configuration and authority can authorize the account.
	result, err := publishDriverApproval(context.Background(), d.Repository, cfg.Draft.AgentTokenEnv, d)
	if err != nil {
		terminalPrintf(stderr, "pyxis: approve: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		terminalPrintf(stderr, "pyxis: approve: report published approval: %v\n", err)
		return 1
	}
	return 0
}
