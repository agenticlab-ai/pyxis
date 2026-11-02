package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	draftlock "github.com/agenticlab-ai/pyxis/internal/draft/lock"
	memorycli "github.com/agenticlab-ai/pyxis/internal/memory/cli"
)

var localDraftMemory = checkLocalDraftMemory

// The installed binary calls the same memory predicates as memory-gate. No Go
// toolchain or Pyxis source checkout is needed in the evaluated repository.
func checkLocalDraftMemory(ctx context.Context, root, base string) error {
	env := memorycli.OSEnv()
	var output bytes.Buffer
	env.Dir, env.Stdout, env.Stderr = root, &output, &output
	if code := memorycli.RunGate(ctx, []string{"--base", base}, env); code != 0 {
		return fmt.Errorf("memory gate refused (%d): %s", code, output.String())
	}
	return nil
}

func localBindingReader(options localcodehost.Options) lifecycle.LockedReader {
	return func(ctx context.Context, root, remote, feature string, version int, run lifecycle.RunCommand) (lifecycle.LockedContract, error) {
		// Existing hosted lock records keep their decoder. Only explicit local
		// selection composes the independent, fully verified local decoder.
		legacy, legacyErr := draftLocked(ctx, root, remote, feature, version, run)
		if legacyErr == nil {
			return legacy, nil
		}
		options.Root = root
		local, err := draftlock.LoadLocalContract(ctx, root, feature, version, localcodehost.NewDraftLandings(options))
		if err != nil {
			return local, errors.Join(legacyErr, err)
		}
		return local, nil
	}
}

func draftLocalLockVerb(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("draft "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	request := ""
	version := 0
	feature := ""
	if args[0] == "lock" {
		flags.StringVar(&request, "request", "", "saved exact prepared review request")
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || request == "" {
			terminalPrintln(stderr, "usage: pyxis draft lock --request <file>")
			return 2
		}
		if os.Getenv("ARGUS_DRIVE_SLUG") != "" {
			terminalPrintln(stderr, "pyxis: draft lock is an operator procedure; Argus presents the request and observes the result")
			return 1
		}
	} else {
		flags.IntVar(&version, "version", 0, "exact locked version")
		if len(args) < 2 {
			terminalPrintln(stderr, "usage: pyxis draft verify-lock <feature> --version <n>")
			return 2
		}
		feature = args[1]
		if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 || version < 1 {
			terminalPrintln(stderr, "usage: pyxis draft verify-lock <feature> --version <n>")
			return 2
		}
	}
	root, err := draftRepoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft lock: repository: %v\n", err)
		return 1
	}
	cfg, defects := config.Load(root)
	if len(defects) != 0 {
		terminalPrintf(stderr, "pyxis: draft lock: configuration: %v\n", defects)
		return 1
	}
	options, selected, err := localCodeHostOptions(root, cfg)
	if err != nil || !selected {
		terminalPrintf(stderr, "pyxis: draft lock requires explicit local-git selection: %v\n", err)
		return 1
	}
	ctx := context.Background()
	proposals := localcodehost.NewProposals(localcodehost.ProposalOptions{Git: options, Kind: "draft"})
	r, err := proposals.Mainline(ctx)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft lock: %v\n", err)
		return 1
	}
	options.Origin, cfg.Remote = r.Origin, r.Origin
	landings := localcodehost.NewDraftLandings(options)
	var result any
	if args[0] == "lock" {
		binding, readErr := readDraftReviewRequest(request)
		if readErr != nil {
			err = readErr
		} else {
			result, err = draftlock.LockLocal(ctx, root, cfg, binding, localDraftCommand,
				draftlock.LocalPreparation{Proposals: proposals, Locked: localBindingReader(options), Memory: localDraftMemory},
				localcodehost.NewReceipts(options), landings)
		}
	} else {
		if _, err = draftlock.LoadLocalContract(ctx, root, feature, version, landings); err == nil {
			result, err = landings.VerifyDraftLock(ctx, feature, version)
		}
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft %s: %v\n", args[0], err)
		return 1
	}
	terminalPrintln(stderr, "Local lock verification proves record consistency. Copying the entire origin preserves the same apparently consistent records. Shared-account access, unauthenticated reviewer/origin identity, and absent host-level mainline protection apply.")
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		terminalPrintf(stderr, "pyxis: draft lock: write result: %v\n", err)
		return 1
	}
	return 0
}
