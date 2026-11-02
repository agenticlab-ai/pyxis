// Command draft-lock-record writes annotated lock tags for mainline pushes.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	approvalgithub "github.com/agenticlab-ai/pyxis/internal/adapters/approval/github"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
)

var (
	processExit                  = os.Exit
	processRun                   = run
	processGetwd                 = os.Getwd
	processGetenv                = os.Getenv
	processRecordRange           = lock.RecordRange
	processOutput      io.Writer = os.Stdout
	processError       io.Writer = os.Stderr
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := processRun(ctx); err != nil {
		// The process is already failing; an unwritable stderr cannot be reported.
		_, _ = fmt.Fprintln(processError, err)
		processExit(1)
	}
}

func run(ctx context.Context) error {
	if processGetenv("PYXIS_LOCK_RECORD_RUN") != "1" {
		return nil
	}
	root, err := processGetwd()
	if err != nil {
		return fmt.Errorf("locate checkout: %w", err)
	}
	cfg, defects := config.Load(root)
	if len(defects) != 0 {
		return fmt.Errorf("invalid pyxis config: %v", defects)
	}
	repo := processGetenv("GITHUB_REPOSITORY")
	before, after := processGetenv("PYXIS_PUSH_BEFORE"), processGetenv("GITHUB_SHA")
	if processGetenv("GH_TOKEN") == "" {
		return fmt.Errorf("GH_TOKEN is required to verify the authorized review and publish a lock tag")
	}
	reader := approvalgithub.NewPolicyReader(repo, "GH_TOKEN")
	records, err := processRecordRange(ctx, root, cfg, repo, before, after, reader, draftshell.Run)
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, err := fmt.Fprintf(processOutput, "lock/%s/v%d on %s (PR #%d)\n", record.Feature, record.Version, record.MergeCommit, record.PullRequest); err != nil {
			return fmt.Errorf("report lock record: %w", err)
		}
	}
	return nil
}
