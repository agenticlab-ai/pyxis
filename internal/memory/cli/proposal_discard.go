package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type proposalDiscardFile struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
	Bytes  []byte `json:"-"`
}

type proposalDiscardPlan struct {
	Schema    string                `json:"schema_version"`
	Base      string                `json:"base"`
	Candidate string                `json:"candidate"`
	Reason    string                `json:"reason"`
	Records   []string              `json:"records"`
	Files     []proposalDiscardFile `json:"files"`
	Archive   string                `json:"archive"`
}

type proposalDiscardOutput struct {
	proposalDiscardPlan
	Applied bool `json:"applied"`
}

// Withdraw only lossless capture proposals. The original committed records
// remain addressable at Candidate; this operation never changes Git objects,
// refs, the index, a frozen record, or the evidence predicate.
func runDiscardProposal(ctx context.Context, args []string, env Env) (int, error) {
	flags := newFlags("discard-proposal", env)
	check := flags.Bool("check", false, "preview without changing files")
	write := flags.Bool("write", false, "archive and withdraw the selected source-capture proposals")
	asJSON := flags.Bool("json", false, "machine output")
	reason := flags.String("reason", "", "why these proposals are being withdrawn")
	var selected multi
	flags.Var(&selected, "record", "unlanded source Document or exact source-only Decision companion ID (repeatable)")
	pos, err := parse(flags, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 0 || *check == *write || len(selected) == 0 || strings.TrimSpace(*reason) == "" {
		return exitUsage, usage("discard-proposal --record ID… --reason text --check|--write [--json]")
	}
	var m *memory
	if *write {
		var release func()
		m, release, err = openLocked(ctx, env)
		if err == nil {
			defer release()
		}
	} else {
		m, err = open(ctx, env)
	}
	if err != nil {
		return exitRefused, err
	}
	s, err := newEvidenceSession(ctx, m, "")
	if err != nil {
		return exitRefused, err
	}
	out, err := prepareProposalDiscard(ctx, s, selected, *reason)
	if err != nil {
		return exitRefused, err
	}
	root, err := os.OpenRoot(m.root)
	if err != nil {
		return exitRefused, err
	}
	defer closePolicyRoot(root)
	if err := checkProposalState(ctx, s, root, &out); err != nil {
		return exitRefused, err
	}
	if *write {
		if err := applyProposalDiscard(ctx, s, root, &out, nativeProposalOps(out.Archive, s.m.repo.RequireIgnored)); err != nil {
			return exitRefused, err
		}
	}
	if *asJSON {
		return exitOK, json.NewEncoder(env.Stdout).Encode(out)
	}
	_, err = fmt.Fprintf(env.Stdout, "proposal discard: %d record(s); applied=%t; candidate=%s; archive=%s; rerun with --check --json to inspect exact paths, modes, and hashes\n", len(out.Records), out.Applied, out.Candidate, out.Archive)
	return exitOK, err
}

func prepareProposalDiscard(ctx context.Context, s *evidenceSession, selected []string, reason string) (proposalDiscardOutput, error) {
	out := proposalDiscardOutput{proposalDiscardPlan: proposalDiscardPlan{Schema: "memory-proposal-discard/v1", Base: s.base, Candidate: s.head, Reason: reason}}
	// Read committed candidate bytes even on retry after a process stopped
	// between removals. Missing working files alone never authorize a retry.
	set, err := s.m.baseSet(ctx, s.head)
	if err != nil {
		return out, err
	}
	if len(set.Problems) != 0 || len(set.Dangling) != 0 {
		return out, fmt.Errorf("invalid committed memory must be repaired before withdrawing proposals")
	}
	wanted := map[string]bool{}
	for _, id := range selected {
		if wanted[id] {
			continue
		}
		r := set.Get(id)
		if r == nil {
			return out, fmt.Errorf("proposal %q has no valid committed record", id)
		}
		if s.baseSet.Get(id) != nil || len(s.identities[id]) != 0 {
			return out, fmt.Errorf("%s is frozen or was previously present on mainline", id)
		}
		wanted[id] = true
		out.Records = append(out.Records, id)
	}
	for _, id := range out.Records {
		r := set.Get(id)
		if !proposalSourceDocument(r) {
			if r.Kind != record.Decision || len(r.Relations) != 1 || r.Relations[0].Type != record.DependsOn || !wanted[r.Relations[0].Target] {
				return out, fmt.Errorf("%s is not a lossless source-capture proposal or selected exact Decision companion", id)
			}
			parent := set.Get(r.Relations[0].Target)
			if err := reconcile.ValidateSourceDecisionProposal(r, parent, set.RawBytes[parent.BodyPath()]); err != nil {
				return out, fmt.Errorf("%s is not an exact source-only Decision companion: %w", id, err)
			}
		}
		paths := []string{r.Path()}
		if r.Kind == record.Document {
			paths = append(paths, r.BodyPath())
		}
		for _, p := range paths {
			body := set.RawBytes[p]
			out.Files = append(out.Files, proposalDiscardFile{Path: store.Dir + "/" + p, SHA256: fmt.Sprintf("%x", sha256.Sum256(body)), Bytes: body})
		}
	}
	// No surviving record may lose a relation target, including an unlanded
	// successor. Callers select the complete capture chain explicitly.
	for _, id := range set.IDs() {
		if wanted[id] {
			continue
		}
		for _, rel := range set.Get(id).Relations {
			if wanted[rel.Target] {
				return out, fmt.Errorf("%s still references selected proposal %s", id, rel.Target)
			}
		}
	}
	entries, err := s.m.repo.EvidenceTree(ctx, s.head)
	if err != nil {
		return out, err
	}
	for i := range out.Files {
		for _, entry := range entries {
			if entry.Path == out.Files[i].Path {
				out.Files[i].Mode = entry.Mode
				break
			}
		}
	}
	sort.Strings(out.Records)
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	selection := sha256.Sum256([]byte(strings.Join(out.Records, "\n")))
	out.Archive = fmt.Sprintf(".pyxis/logs/memory/proposal-discard/%s/%x", out.Candidate, selection)
	return out, nil
}

func proposalSourceDocument(r *record.Record) bool {
	return r.Kind == record.Document && r.LiteralBody && r.HasTag("reconcile:source") && r.Provenance.Source != ""
}
