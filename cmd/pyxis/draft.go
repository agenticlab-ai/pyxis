package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	draftlock "github.com/agenticlab-ai/pyxis/internal/draft/lock"
)

var (
	draftRepoRoot   = repoRoot
	draftInspect    = lifecycle.Inspect
	draftOpen       = lifecycle.Open
	draftReopen     = lifecycle.ReopenTracked
	draftLocked     = draftlock.LoadContract
	draftAbandon    = lifecycle.Abandon
	draftPrepare    = draftlock.PrepareTracked
	draftRound      = draftlock.Round
	draftCheckpoint = checkpoint.Run
	draftCommand    = lifecycle.RunCommand(draftshell.Run)
)

func draftVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		terminalPrintln(stderr, "usage: pyxis draft open <feature> | pyxis draft reopen <feature> [--cause <kind> --cause-ref <ref>] | pyxis draft abandon <feature> --reason <text> | pyxis draft prototype <feature> --idea <file> [--ambient-backend <harness> --ambient-model <model>] | pyxis draft prototype <feature> --remove | pyxis draft status [<feature>] | pyxis draft digest <feature> | pyxis draft notice <feature> | pyxis draft prepare-lock <feature> | pyxis draft round <feature> | pyxis draft checkpoint | pyxis draft review-request <proposal-id> --generation <displayed-object-id> | pyxis draft record-review --request <file> --reviewer <label> [--capture <text>] | pyxis draft verify-review --request <file> | pyxis draft lock --request <file> | pyxis draft verify-lock <feature> --version <n>")
		return 2
	}
	switch args[0] {
	case "prototype":
		return draftPrototypeVerb(args, stdout, stderr)
	case "lock", "verify-lock":
		return draftLocalLockVerb(args, stdout, stderr)
	case "review-request", "record-review", "verify-review":
		return draftReviewVerb(args, stdout, stderr)
	case "open":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft open <feature>")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftOpenVerb(args[1], stdout, stderr)
	case "reopen":
		if len(args) < 2 || strings.HasPrefix(args[1], "-") || len(args) != 2 && (len(args) != 6 || args[2] != "--cause" || args[4] != "--cause-ref" || args[3] == "" || args[5] == "") {
			terminalPrintln(stderr, "usage: pyxis draft reopen <feature> [--cause new-need|dependency-change|gap-lock-should-have-caught|other --cause-ref <ref>]")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		cause, causeRef := "other", ""
		if len(args) == 6 {
			cause, causeRef = args[3], args[5]
		}
		return draftReopenVerb(args[1], cause, causeRef, stdout, stderr)
	case "abandon":
		if len(args) != 4 || strings.HasPrefix(args[1], "-") || args[2] != "--reason" {
			terminalPrintln(stderr, "usage: pyxis draft abandon <feature> --reason <text>")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftAbandonVerb(args[1], args[3], stdout, stderr)
	case "status":
		if len(args) > 2 || len(args) == 2 && strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft status [<feature>]")
			return 2
		}
		feature := ""
		if len(args) == 2 {
			feature = args[1]
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftStatusVerb(feature, stdout, stderr)
	case "digest":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft digest <feature>")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftDigestVerb(args[1], stdout, stderr)
	case "notice":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft notice <feature>")
			return 2
		}
		return draftNoticeVerb(args[1], stdout, stderr)
	case "prepare-lock":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft prepare-lock <feature>")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftPrepareVerb(args[1], stdout, stderr)
	case "round":
		if len(args) != 2 || strings.HasPrefix(args[1], "-") {
			terminalPrintln(stderr, "usage: pyxis draft round <feature>")
			return 2
		}
		if exit, handled := localDraftVerb(args, stdout, stderr); handled {
			return exit
		}
		return draftRoundVerb(args[1], stdout, stderr)
	case "checkpoint":
		if len(args) != 1 {
			terminalPrintln(stderr, "usage: pyxis draft checkpoint")
			return 2
		}
		return draftCheckpointVerb(stdout, stderr)
	default:
		terminalPrintf(stderr, "pyxis: draft: unknown verb %q; use open, reopen, abandon, prototype, status, digest, notice, prepare-lock, round, checkpoint, review-request, record-review, verify-review, lock, or verify-lock\n", args[0])
		return 2
	}
}

func draftContext(stderr io.Writer) (string, config.Config, bool) {
	root, err := draftRepoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft: find repository: %v\n", err)
		return "", config.Config{}, false
	}
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		terminalPrintf(stderr, "pyxis: draft: configuration: %v\n", defects)
		return "", config.Config{}, false
	}
	probe, err := localCodeHostDiagnostics(root, cfg)
	if err == nil && probe != nil {
		err = codehost.RequireWorkflow(context.Background(), probe)
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft: code host: %v\n", err)
		return "", config.Config{}, false
	}
	return root, cfg, true
}

func draftStatusVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	snapshot, err := draftInspect(context.Background(), root, cfg, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft status: %v\n", err)
		return 1
	}
	states := lifecycle.States(snapshot)
	shown := false
	for _, state := range states {
		if feature != "" && state.Feature != feature {
			continue
		}
		terminalPrintf(stdout, "%s: %s\n", state.Feature, renderDraftState(state))
		shown = true
	}
	if feature != "" && !shown {
		terminalPrintf(stderr, "pyxis: draft status: feature %q has no contract state\n", feature)
		return 1
	}
	if !shown {
		terminalPrintln(stdout, "no features")
	}
	return 0
}

func renderDraftState(state lifecycle.State) string {
	switch state.Kind {
	case lifecycle.DraftingFirst:
		return "drafting first version (v1)"
	case lifecycle.Locked:
		return "locked at version " + strconv.Itoa(state.Version)
	case lifecycle.DraftingNext:
		return fmt.Sprintf("drafting version %d while version %d binds", state.Version, state.Version-1)
	default:
		return "document pipeline"
	}
}

func draftOpenVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	result, err := draftOpen(context.Background(), root, cfg, feature, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft open: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s: %s (%s)\n", feature, result.PullRequest, result.Branch)
	return 0
}

func draftReopenVerb(feature, cause, causeRef string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	tasks, err := configuredTaskReader(root, cfg)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft reopen: task tracker: %v\n", err)
		return 1
	}
	result, err := draftReopen(context.Background(), root, cfg, feature, cause, causeRef, draftLocked, draftCommand, tasks)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft reopen: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s: drafting v%d from its binding lock on local branch %s; prepare-lock publishes it with its one pull request\n", feature, result.Version, result.Branch)
	if len(result.Unfinished) > 0 {
		terminalPrintf(stdout, "%s: open tasks keep running: %s — change their charters in the plan on this branch when the new version touches them\n", feature, strings.Join(result.Unfinished, ", "))
	}
	return 0
}

func draftAbandonVerb(feature, reason string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	result, err := draftAbandon(context.Background(), root, cfg, feature, reason, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft abandon: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s: PR #%d now proposes one abandonment Learning at %s; operator review required\n", feature, result.PullRequest, result.Head)
	return 0
}

func draftPrepareVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	tasks, err := configuredTaskReader(root, cfg)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prepare-lock: task tracker: %v\n", err)
		return 1
	}
	prepared, err := draftPrepare(context.Background(), root, cfg, feature, draftCommand, tasks)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prepare-lock: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s: prepared PR #%d at %s (%s)\n", feature, prepared.PullRequest, prepared.Head, prepared.Verdict.DraftDigest)
	return 0
}

func draftRoundVerb(feature string, stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	resumed, err := draftRound(context.Background(), root, cfg, feature, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft round: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "%s: working files restored at %s (%s)\n", feature, resumed.Head, resumed.Digest)
	return 0
}

func draftCheckpointVerb(stdout, stderr io.Writer) int {
	root, cfg, ok := draftContext(stderr)
	if !ok {
		return 1
	}
	result, err := draftCheckpoint(context.Background(), root, cfg, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft checkpoint: %v\n", err)
		return 1
	}
	if !result.Due {
		terminalPrintf(stdout, "checkpoint not yet due: %s\n", strings.Join(result.Missing, "; "))
		return 0
	}
	if result.AlreadyRecorded {
		terminalPrintf(stdout, "checkpoint inventory already recorded at %s\n", result.RecordPath)
		return 0
	}
	terminalPrintf(stdout, "checkpoint inventory written at %s for %s\n", result.RecordPath, strings.Join(result.Inventory.Features, ", "))
	return 0
}
