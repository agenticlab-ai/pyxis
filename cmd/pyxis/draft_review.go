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
	"strings"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// The submitted file pins what the operator reviewed. Looking up the current
// generation again at record time must not silently approve a newer request.
func draftReviewVerb(args []string, stdout, stderr io.Writer) int {
	var binding codehost.ReviewBinding
	var reviewer, capture string
	if args[0] == "review-request" {
		if len(args) != 4 || args[1] == "" || args[2] != "--generation" || (len(args[3]) != 40 && len(args[3]) != 64) || strings.Trim(args[3], "0123456789abcdef") != "" {
			terminalPrintln(stderr, "usage: pyxis draft review-request <proposal-id> --generation <displayed-object-id>")
			return 2
		}
	} else {
		flags := flag.NewFlagSet("draft "+args[0], flag.ContinueOnError)
		flags.SetOutput(stderr)
		request := flags.String("request", "", "saved exact review request JSON")
		usage := "usage: pyxis draft verify-review --request <file>"
		if args[0] == "record-review" {
			flags.StringVar(&reviewer, "reviewer", "", "declared operator label")
			flags.StringVar(&capture, "capture", "", "optional review capture")
			usage = "usage: pyxis draft record-review --request <file> --reviewer <label> [--capture <text>]"
		}
		if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *request == "" || args[0] == "record-review" && reviewer == "" {
			terminalPrintln(stderr, usage)
			return 2
		}
		if args[0] == "record-review" && os.Getenv("ARGUS_DRIVE_SLUG") != "" {
			terminalPrintln(stderr, "pyxis: draft record-review is an operator procedure; Argus presents the request and observes the result")
			return 1
		}
		var err error
		binding, err = readDraftReviewRequest(*request)
		if err != nil {
			terminalPrintf(stderr, "pyxis: draft %s: request: %v\n", args[0], err)
			return 1
		}
	}
	root, err := draftRepoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft review: find repository: %v\n", err)
		return 1
	}
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		terminalPrintf(stderr, "pyxis: draft review: configuration: %v\n", defects)
		return 1
	}
	options, selected, err := localCodeHostOptions(root, cfg)
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft review: code-host configuration: %v\n", err)
		return 1
	}
	if !selected {
		terminalPrintln(stderr, "pyxis: draft review requires explicit local-git selection")
		return 1
	}
	receipts := localcodehost.NewReceipts(options)
	var result any
	switch args[0] {
	case "review-request":
		binding, err = receipts.Binding(context.Background(), args[1])
		if err == nil && binding.ProposalOID != args[3] {
			err = fmt.Errorf("displayed proposal generation no longer matches: %w", codehost.ErrProposalConflict)
		}
		result = binding
	case "record-review":
		result, err = receipts.Record(context.Background(), binding, reviewer, capture)
	default:
		result, err = receipts.Verify(context.Background(), binding)
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: draft %s: %v\n", args[0], err)
		return 1
	}
	terminalPrintln(stderr, "Local review uses a shared OS account. Reviewer and independent origin identity are unauthenticated; mainline has no host-level protection. Copying the entire origin preserves the same apparently consistent records.")
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		terminalPrintf(stderr, "pyxis: draft review: write result: %v\n", err)
		return 1
	}
	return 0
}

func readDraftReviewRequest(path string) (codehost.ReviewBinding, error) {
	var binding codehost.ReviewBinding
	file, err := os.Open(path)
	if err != nil {
		return binding, err
	}
	defer func() {
		_ = file.Close() // Read-only file; no buffered writes need a close result.
	}()
	body, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil {
		return binding, err
	}
	if len(body) > 64<<10 {
		return binding, fmt.Errorf("review request exceeds 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return binding, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return binding, fmt.Errorf("review request contains trailing data")
	}
	if err := binding.Validate(); err != nil {
		return binding, fmt.Errorf("saved review request is invalid (schema %q): %w", binding.SchemaVersion, err)
	}
	return binding, nil
}
