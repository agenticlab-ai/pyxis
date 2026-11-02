// impl-review-harvest exposes the shipped, read-only proposal-corpus harvest.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	forgejopersist "github.com/agenticlab-ai/pyxis/internal/adapters/persist/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("impl-review-harvest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", ".", "local repository containing integration bindings")
	repository := fs.String("repo-slug", "", "legacy GitHub repository as owner/name")
	operator := fs.String("operator", "", "operator identity (GitHub login or Forgejo server URL/login)")
	dispositionFloor := fs.Int("disposition-floor", 5, "minimum blocking dispositions required for a precision reading")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *operator == "" || *dispositionFloor < 1 || fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "impl-review-harvest: --repo-slug, --operator, and a positive --disposition-floor are required") // Best-effort diagnostic; exit status carries refusal.
		return 2
	}
	adapter, err := configuredHarvestAdapter(*root, *repository)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "impl-review-harvest: %v\n", err) // Best-effort diagnostic; exit status carries refusal.
		return 2
	}
	metrics, err := harvest.Harvest(context.Background(), adapter, adapter, harvest.NewGitHubResolver(*operator, adapter))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "impl-review-harvest: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	precision, precisionReady := metrics.Precision(*dispositionFloor)
	type missOutput struct {
		Proposal string `json:"proposal"`
		Kind     string `json:"kind"`
		Cause    string `json:"cause,omitempty"`
	}
	misses := make([]missOutput, 0, len(metrics.Misses))
	for _, miss := range metrics.Misses {
		misses = append(misses, missOutput{Proposal: miss.Proposal, Kind: string(miss.Kind), Cause: miss.Cause})
	}
	output := struct {
		Universe             int          `json:"universe"`
		Covered              int          `json:"covered"`
		Coverage             float64      `json:"coverage"`
		Reviews              int          `json:"reviews"`
		BlockingDisposed     int          `json:"blocking_disposed"`
		BlockingAccepted     int          `json:"blocking_accepted"`
		Precision            float64      `json:"precision"`
		PrecisionReady       bool         `json:"precision_ready"`
		OperatorRejected     int          `json:"operator_rejected"`
		NitRate              float64      `json:"nit_rate"`
		ExcludedDispositions int          `json:"excluded_dispositions"`
		Misses               []missOutput `json:"misses"`
	}{
		Universe: metrics.Universe, Covered: metrics.Covered, Coverage: metrics.Coverage(), Reviews: metrics.Reviews,
		BlockingDisposed: metrics.BlockingDisposed, BlockingAccepted: metrics.BlockingAccepted,
		Precision: precision, PrecisionReady: precisionReady, OperatorRejected: metrics.OperatorRejected,
		NitRate: metrics.NitRate(), ExcludedDispositions: metrics.ExcludedDispositions, Misses: misses,
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		_, _ = fmt.Fprintf(stderr, "impl-review-harvest: encode output: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	return 0
}

func configuredHarvestAdapter(root, repository string) (harvest.Adapter, error) {
	cfg, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	binding, err := cfg.CodeHost(repository)
	if err != nil {
		return nil, err
	}
	if binding.Provider == integrations.Forgejo {
		return forgejopersist.NewForgejoAdapter(binding)
	}
	return harvest.NewConfiguredAdapter(root, repository)
}
