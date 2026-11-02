package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness"
	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
)

var draftPrototypeRuntime = harness.New
var draftPrototypeWorkerBound = executorconfig.DefaultWorkerBudget

func draftPrototypeVerb(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("draft prototype", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	idea := flags.String("idea", "", "idea file")
	ambientBackend := flags.String("ambient-backend", "", "asking session's harness")
	ambientModel := flags.String("ambient-model", "", "asking session's model")
	remove := len(args) == 3 && args[2] == "--remove"
	if len(args) < 2 || strings.HasPrefix(args[1], "-") || (!remove && (flags.Parse(args[2:]) != nil || flags.NArg() != 0 || *idea == "")) {
		terminalPrintln(stderr, "usage: pyxis draft prototype <feature> --idea <file> [--ambient-backend <harness> --ambient-model <model>] | pyxis draft prototype <feature> --remove")
		return 2
	}
	root, err := draftRepoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prototype: find repository: %v\n", err)
		return 1
	}
	// All consumers parse one merged document, including the implementation
	// catalog passed to the runtime; no constructor may rediscover an overlay.
	effective, err := configlayer.Load(root)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prototype: configuration: %v\n", err)
		return 1
	}
	cfg, defects := config.Parse(effective)
	if len(defects) != 0 {
		terminalPrintf(stderr, "pyxis: draft prototype: configuration: %v\n", defects)
		return 1
	}
	// This local lifecycle operation needs no code-host or proposal admission.
	ctx := context.Background()
	if remove {
		removed, err := lifecycle.RemovePrototype(ctx, root, args[1], draftCommand)
		if err != nil {
			terminalPrintf(stderr, "pyxis: draft prototype: %v\n", err)
			return 1
		}
		if removed {
			terminalPrintln(stdout, "prototype removed")
		} else {
			terminalPrintln(stdout, "nothing to remove")
		}
		return 0
	}
	resolve := func(ctx context.Context) (lifecycle.PrototypeRoute, error) {
		hc, err := harnessconfig.Parse(effective)
		if err != nil {
			return lifecycle.PrototypeRoute{}, err
		}
		executorCfg, err := executorconfig.Parse(effective)
		if err != nil {
			return lifecycle.PrototypeRoute{}, err
		}
		return lifecycle.ResolvePrototypeRoute(ctx, cfg.Prototype, hc, *ambientBackend, *ambientModel, func(backend entity.Harness) (port.AgentRuntime, error) {
			return draftPrototypeRuntime(backend, executorCfg, hc)
		})
	}
	result, err := lifecycle.CreatePrototype(ctx, root, cfg, args[1], *idea, resolve, draftCommand)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prototype: %v\n", err)
		return 1
	}
	terminalPrintf(stdout, "worktree: %s\nbranch: %s\nbase: %s\nroute: %s\n", result.Worktree, result.Branch, result.Base, result.Route)
	build, err := lifecycle.BuildPrototype(ctx, result, *idea, draftPrototypeWorkerBound, draftCommand)
	outcome := "finished"
	if err != nil {
		outcome = "failed: " + err.Error()
	}
	terminalPrintf(stdout, "model: %s\neffort: %s\noutcome: %s\n", result.Route.Profile.Model, result.Route.Profile.Effort, outcome)
	if build.Head != "" {
		findings := build.Findings
		if findings == "" {
			findings = "missing"
		}
		terminalPrintf(stdout, "head: %s\nfindings: %s\nusage: tokens=%s turns=%s tool_calls=%s\n", build.Head, findings,
			prototypeUsage(build.Usage.Tokens, build.Usage.TokensObserved),
			prototypeUsage(build.Usage.Turns, build.Usage.TurnsObserved),
			prototypeUsage(build.Usage.ToolCalls, build.Usage.ToolCallsObserved))
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft prototype: %v\n", err)
		return 1
	}
	return 0
}

func prototypeUsage(value int, observed bool) string {
	if !observed {
		return "unavailable"
	}
	return fmt.Sprint(value)
}
