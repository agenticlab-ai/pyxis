package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/cliguard"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

const prDescriptionUsage = "pyxis pr-description lines [--base <rev>] [--head <rev>] [--files] | check --body <file|-> [--base <rev>] [--head <rev>] | gate"

// prDescriptionStdin is the body source for `check --body -`.
var prDescriptionStdin io.Reader = os.Stdin

func init() {
	mustRegisterRootCommand(rootCommand{key: "pr-description", usage: "  pr-description lines|check|gate …\n                           measure a change's reviewable lines of code, or check a\n                           pull request description against the template", handler: prDescriptionVerb})
}

// prDescriptionVerb is the pull request description template's command face:
// `lines` prints the Lines of code figure, `check` validates a description
// before it is published, and `gate` is the hosted check, which acts only in
// the hosted pull-request job (PYXIS_PR_DESCRIPTION_RUN=1) and passes
// everywhere else the shared code gate runs it.
func prDescriptionVerb(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return prDescriptionRefuse(stderr, "a subcommand is required")
	}
	verb, rest := args[0], args[1:]
	if verb != "lines" && verb != "check" && verb != "gate" {
		return prDescriptionRefuse(stderr, fmt.Sprintf("unknown subcommand %q", verb))
	}
	fs := flag.NewFlagSet("pr-description "+verb, flag.ContinueOnError)
	var base, head, body *string
	var listFiles *bool
	if verb != "gate" {
		base = fs.String("base", "", "base revision (default: the configured mainline, such as origin/main)")
		head = fs.String("head", "HEAD", "head revision")
	}
	if verb == "lines" {
		listFiles = fs.Bool("files", false, "list each counted file")
	}
	if verb == "check" {
		body = fs.String("body", "", "the description: a file, or - for standard input")
	}
	positionals, err := cliguard.Positionals(fs, rest)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			terminalPrintf(stderr, "usage: %s\n", prDescriptionUsage)
			return 2
		}
		var usageError *cliguard.UsageError
		if errors.As(err, &usageError) {
			return prDescriptionRefuse(stderr, usageError.Reason)
		}
		return prDescriptionRefuse(stderr, err.Error())
	}
	if len(positionals) != 0 {
		return prDescriptionRefuse(stderr, fmt.Sprintf("takes no positional arguments (got %q)", positionals[0]))
	}
	if verb == "check" && *body == "" {
		return prDescriptionRefuse(stderr, "--body is required")
	}
	if verb == "gate" && os.Getenv("PYXIS_PR_DESCRIPTION_RUN") != "1" {
		terminalPrintln(stdout, "pr-description: not a pull request description run; nothing to check")
		return 0
	}
	root, err := repoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: pr-description %s: %v\n", verb, err)
		return 1
	}
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		for _, d := range defects {
			terminalPrintln(stderr, d.String())
		}
		return 1
	}
	if verb == "gate" {
		return prDescriptionGate(ctx, root, cfg, stdout, stderr)
	}
	from := *base
	if from == "" {
		from = cfg.MainlineRef()
	}
	size, err := changesize.Measure(ctx, root, from, *head)
	if err != nil {
		terminalPrintf(stderr, "pyxis: pr-description %s: %v\n", verb, err)
		return 1
	}
	if verb == "lines" {
		terminalPrintln(stdout, size.String())
		if *listFiles {
			for _, f := range size.Files {
				terminalPrintf(stdout, "  %s  %s  +%d/-%d\n", f.Class, f.Path, f.Added, f.Removed)
			}
		}
		return 0
	}
	text, err := prDescriptionBody(*body)
	if err != nil {
		terminalPrintf(stderr, "pyxis: pr-description check: %v\n", err)
		return 1
	}
	return prDescriptionReport(prdescription.Check(text, size), size, stdout, stderr)
}

func prDescriptionGate(ctx context.Context, root string, cfg config.Config, stdout, stderr io.Writer) int {
	event, err := prdescription.ReadEvent(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		terminalPrintf(stderr, "pr-description: %v\n", err)
		return 1
	}
	result, err := prdescription.Gate(ctx, root, event, cfg.Remote, cfg.Draft.BranchPrefix)
	if err != nil {
		terminalPrintf(stderr, "pr-description: %v\n", err)
		return 1
	}
	if result.Exempt {
		terminalPrintf(stdout, "pr-description: #%d comes from the feature-draft branch %s; its description is the contract's approval view\n", event.Number, event.HeadRef)
		return 0
	}
	if code := prDescriptionReport(result.Problems, result.Size, stdout, stderr); code != 0 {
		terminalPrintf(stderr, "pr-description: edit the description of #%d to follow .pyxis/templates/pr-description-template.md; this check runs again when the description changes\n", event.Number)
		return code
	}
	return 0
}

func prDescriptionReport(problems []prdescription.Problem, size changesize.Size, stdout, stderr io.Writer) int {
	if len(problems) == 0 {
		terminalPrintf(stdout, "pr-description: the description follows the template (lines of code: %s)\n", size)
		return 0
	}
	for _, p := range problems {
		terminalPrintf(stderr, "pr-description: %s\n", p)
	}
	return 1
}

func prDescriptionBody(source string) (string, error) {
	var raw []byte
	var err error
	if source == "-" {
		raw, err = io.ReadAll(prDescriptionStdin)
	} else {
		raw, err = os.ReadFile(source)
	}
	if err != nil {
		return "", fmt.Errorf("read the description: %w", err)
	}
	return string(raw), nil
}

func prDescriptionRefuse(stderr io.Writer, reason string) int {
	terminalPrintf(stderr, "pyxis: pr-description: %s — nothing was run\nusage: %s\n", reason, prDescriptionUsage)
	return 2
}
