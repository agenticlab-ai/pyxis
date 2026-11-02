package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type retirementItem struct {
	RecordID string                            `json:"record_id,omitempty"`
	Receipt  reconcile.SourceRetirementReceipt `json:"receipt"`
}
type retirementOutput struct {
	Schema  string           `json:"schema_version"`
	Status  string           `json:"status"`
	Sources []retirementItem `json:"sources"`
}
type retirementPlan struct {
	draft   reconcile.Draft
	receipt reconcile.SourceRetirementReceipt
	record  *record.Record
}

func runRetireSource(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("retire-source", env)
	var sources multi
	fs.Var(&sources, "source", "canonical standalone source path (repeatable)")
	reason := fs.String("reason", "", "explicit retirement reason")
	check := fs.Bool("check", false, "check the complete batch without writing")
	write := fs.Bool("write", false, "write the complete preflighted seal batch")
	actorFlag := fs.String("actor", "", "who issues the seals")
	asJSON := fs.Bool("json", false, "machine output")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 0 || len(sources) == 0 || *check == *write || strings.TrimSpace(*reason) == "" || *reason != strings.TrimSpace(*reason) || (*check && *actorFlag != "") {
		return exitUsage, usage("retire-source needs unique --source paths, --reason, and exactly one of --check or --write")
	}
	sort.Strings(sources)
	for i, source := range sources {
		if !reconcile.StandaloneSourcePath(source) || i > 0 && source == sources[i-1] {
			return exitUsage, usage("unsupported or repeated standalone source %q", source)
		}
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	snapshot, err := stableMainlineCloseoutSnapshot(ctx, m)
	if err != nil {
		return exitRefused, fmt.Errorf("retire-source: %w", err)
	}
	baseSet, err := m.baseSet(ctx, snapshot.Commit)
	if err != nil {
		return exitRefused, err
	}
	baseProj := resolve.New(baseSet)
	plans := make([]retirementPlan, 0, len(sources))
	for _, source := range sources {
		intro, err := m.repo.SourceIntroduction(ctx, snapshot.Commit, source)
		if err != nil {
			return exitRefused, err
		}
		draft, receipt, err := reconcile.BuildSourceRetirement(source, *reason, intro, snapshot, baseSet, baseProj)
		if err != nil {
			return exitRefused, err
		}
		evidence, err := sourceRetirementEvidence(ctx, m, snapshot, receipt, snapshot.Commit)
		if err != nil {
			return exitRefused, err
		}
		if err := reconcile.ValidateSourceRetirement(reconcile.ActiveSourceRetirement{Receipt: receipt}, evidence, snapshot, baseSet, baseProj, false); err != nil {
			return exitRefused, err
		}
		plan := retirementPlan{draft: draft, receipt: receipt}
		if draft.Kind == "" {
			plan.record = baseProj.Head(reconcile.SourceRetirementSubject(source), "seal")
		}
		plans = append(plans, plan)
	}
	dirty, err := m.repo.DirtyPathNames(ctx, []string{"."})
	if err != nil {
		return exitRefused, err
	}
	actor := ""
	if *write || len(dirty) > 0 {
		actor, err = m.actor(ctx, *actorFlag, env)
		if err != nil {
			return exitUsage, err
		}
	}
	if len(dirty) > 0 {
		if err := pendingRetirementBatch(ctx, m, baseSet, plans, dirty, actor); err != nil {
			return exitRefused, err
		}
		if err := assertHead(ctx, m, snapshot.Commit); err != nil {
			return exitRefused, err
		}
		return exitOK, writeRetirementOutput(env, *asJSON, "already-sealed", plans)
	}
	if *check {
		status := "already-sealed"
		for _, p := range plans {
			if p.draft.Kind != "" {
				status = "ready"
			}
		}
		return exitOK, writeRetirementOutput(env, *asJSON, status, plans)
	}
	trial := *m
	for i := range plans {
		if plans[i].draft.Kind == "" {
			continue
		}
		r, err := trial.recordFromDraft(plans[i].draft, actor, env)
		if err != nil {
			return exitRefused, err
		}
		if err := trial.admit(r, plans[i].draft.Body); err != nil {
			return exitRefused, err
		}
		if err := trial.captureEvidence(ctx, r); err != nil {
			return exitRefused, err
		}
		projection, err := trial.projectionWith(r, plans[i].draft.Body)
		if err != nil {
			return exitRefused, err
		}
		trial.set, trial.proj = projection.Set, projection
		plans[i].record = r
	}
	if err := assertHead(ctx, m, snapshot.Commit); err != nil {
		return exitRefused, err
	}
	quiet := env
	quiet.Stdout = io.Discard
	for _, p := range plans {
		if p.draft.Kind == "" {
			continue
		}
		if _, err := m.store(ctx, p.record, p.draft.Body, quiet); err != nil {
			return exitRefused, fmt.Errorf("source retirement batch incomplete at %s: %w", p.receipt.Path, err)
		}
		if err := m.reload(ctx); err != nil {
			return exitRefused, err
		}
	}
	return exitOK, writeRetirementOutput(env, *asJSON, "sealed", plans)
}

func pendingRetirementBatch(ctx context.Context, m *memory, base *store.Set, plans []retirementPlan, dirty []string, actor string) error {
	if _, err := reconcile.ActiveSourceRetirements(m.set, m.proj); err != nil {
		return err
	}
	expected := map[string]bool{}
	for i := range plans {
		p := &plans[i]
		if p.draft.Kind == "" {
			continue
		}
		r := m.proj.Head(p.draft.Subject, "seal")
		if r == nil || base.Get(r.ID) != nil {
			return fmt.Errorf("dirty or partial source retirement batch at %s", p.receipt.Path)
		}
		want := &record.Record{Schema: record.Schema, ID: r.ID, CreatedAt: r.CreatedAt, Issuer: actor,
			Kind: p.draft.Kind, Subject: p.draft.Subject, Family: p.draft.Family, Statement: p.draft.Statement,
			Intent: p.draft.Intent, Rationale: p.draft.Rationale, FitCriterion: p.draft.FitCriterion,
			Tags: p.draft.Tags, Relations: p.draft.Relations, LiteralBody: true, Provenance: p.draft.Provenance, BodySHA256: policyDigest(p.draft.Body)}
		if err := m.captureEvidence(ctx, want); err != nil {
			return err
		}
		body, err := record.Marshal(want)
		if err != nil {
			return err
		}
		if !bytes.Equal(body, rawRecord(m.set, r.ID)) || !bytes.Equal(p.draft.Body, m.set.Bodies[r.ID]) {
			return fmt.Errorf("altered pending source retirement at %s", p.receipt.Path)
		}
		expected[path.Join(store.Dir, r.Path())] = true
		expected[path.Join(store.Dir, r.BodyPath())] = true
		p.record = r
	}
	if len(expected) != len(dirty) {
		return fmt.Errorf("source retirement requires clean sources and memory; only the complete identical pending batch can be retried")
	}
	for _, name := range dirty {
		if !expected[name] {
			return fmt.Errorf("source retirement refuses unrelated dirty path %s", name)
		}
	}
	return nil
}

func writeRetirementOutput(env Env, asJSON bool, status string, plans []retirementPlan) error {
	out := retirementOutput{Schema: "memory-source-retirement-run/v1", Status: status, Sources: []retirementItem{}}
	for _, p := range plans {
		item := retirementItem{Receipt: p.receipt}
		if p.record != nil {
			item.RecordID = p.record.ID
		}
		out.Sources = append(out.Sources, item)
	}
	if asJSON {
		return json.NewEncoder(env.Stdout).Encode(out)
	}
	for _, item := range out.Sources {
		if _, err := fmt.Fprintf(env.Stdout, "source retirement %s: %s (%s)\n", item.Receipt.Path, status, item.RecordID); err != nil {
			return fmt.Errorf("write source retirement output: %w", err)
		}
	}
	return nil
}
