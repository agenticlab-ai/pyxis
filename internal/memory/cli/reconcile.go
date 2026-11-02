package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type reconciliationOutput struct {
	Schema  string           `json:"schema_version"`
	Written int              `json:"written"`
	Report  reconcile.Report `json:"report"`
}

type reconciliationRatchetOutput struct {
	Schema               string                           `json:"schema_version"`
	BaselinePath         string                           `json:"baseline_path"`
	BaselineSHA256       string                           `json:"baseline_sha256"`
	InputIdentitiesMatch bool                             `json:"input_identities_match"`
	BaselineRef          *reconciliationBaselineRefOutput `json:"baseline_ref,omitempty"`
	Result               reconcile.BaselineResult         `json:"result"`
	Report               reconcile.Report                 `json:"report"`
	Policy               policyEvidence                   `json:"policy"`
}

type reconciliationBaselineRefOutput struct {
	Ref            string                    `json:"ref"`
	Commit         string                    `json:"commit"`
	Status         string                    `json:"status"`
	BaselineSHA256 string                    `json:"baseline_sha256,omitempty"`
	ReplayCommit   string                    `json:"replay_commit,omitempty"`
	Result         *reconcile.BaselineResult `json:"result,omitempty"`
	Policy         *policyEvidence           `json:"policy,omitempty"`
}

const reconciliationBaselinePath = reconcile.PolicyPath

type closeoutOutput struct {
	Schema   string                    `json:"schema_version"`
	Status   string                    `json:"status"`
	RecordID string                    `json:"record_id,omitempty"`
	Receipt  reconcile.CloseoutReceipt `json:"receipt"`
}

type verificationOutput struct {
	Schema    string           `json:"schema_version"`
	Closeouts []string         `json:"active_closeouts"`
	Report    reconcile.Report `json:"report"`
}

func runReconcile(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("reconcile", env)
	check := fs.Bool("check", false, "report drift without writing")
	write := fs.Bool("write", false, "append deterministic, lossless repairs")
	var actionFlags multi
	fs.Var(&actionFlags, "action", "write one exact code|scope action (repeatable)")
	generateBaseline := fs.Bool("baseline-generate", false, "emit the exact current baseline JSON to stdout")
	syncBaseline := fs.Bool("baseline-sync", false, "synchronize partitioned debt policy against a trusted committed base")
	migrateV2 := fs.Bool("migrate-v2", false, "explicit data-only migration from a trusted v1 baseline")
	baselineFlag := fs.String("baseline", "", "require an exact match with the checked-in debt inventory")
	baselineRefFlag := fs.String("baseline-ref", "", "trusted prior ref whose baseline the checked-in candidate may not widen")
	baselineReplayRefFlag := fs.String("baseline-replay-ref", "", "original snapshot ref for replaying a trusted baseline carried through a squash merge")
	asJSON := fs.Bool("json", false, "machine output")
	actorFlag := fs.String("actor", "", "who issues appended records")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	modes := 0
	for _, selected := range []bool{*check, *write, *generateBaseline, *syncBaseline} {
		if selected {
			modes++
		}
	}
	if len(pos) != 0 || modes != 1 || (*baselineFlag != "" && !*check) || (*baselineRefFlag != "" && !*syncBaseline && (*baselineFlag == "" || !*check)) || (*baselineReplayRefFlag != "" && *baselineRefFlag == "") || ((*check || *syncBaseline) && *actorFlag != "") || (*generateBaseline && (*asJSON || *actorFlag != "")) || (*migrateV2 && !*syncBaseline) || (*syncBaseline && *baselineRefFlag == "") {
		return exitUsage, reconcileUsage()
	}
	if len(actionFlags) > 0 && !*write {
		return exitUsage, usage("reconcile --action is accepted only with --write")
	}
	actionSelectors, err := parseReconciliationActionSelectors(actionFlags)
	if err != nil {
		return exitUsage, err
	}
	if *baselineFlag != "" && *baselineFlag != reconciliationBaselinePath {
		return exitUsage, usage("the reconciliation baseline has one canonical lineage at %s; got %s", reconciliationBaselinePath, *baselineFlag)
	}
	if *syncBaseline {
		return runBaselineSync(ctx, env, *baselineRefFlag, *baselineReplayRefFlag, *migrateV2, *asJSON)
	}
	if *generateBaseline {
		m, err := open(ctx, env)
		if err != nil {
			return exitRefused, err
		}
		head, err := m.repo.Head(ctx)
		if err != nil {
			return exitRefused, err
		}
		current, present, err := readPolicyAt(ctx, m, head)
		if err != nil {
			return exitRefused, err
		}
		if present && current.Policy.Schema == reconcile.PolicySchema {
			return exitRefused, fmt.Errorf("v2 policy requires reconcile --baseline-sync --baseline-ref <trusted-commit>; v1 generation would discard its lineage")
		}
		_, report, err := reconciliationPass(ctx, m)
		if err != nil {
			return exitRefused, err
		}
		if code, err := enforceEvidence(ctx, m, "", env); code != exitOK || err != nil {
			return code, err
		}
		body, err := reconcile.MarshalBaseline(reconcile.NewBaseline(report))
		if err != nil {
			return exitRefused, err
		}
		if _, err := env.Stdout.Write(body); err != nil {
			return exitRefused, fmt.Errorf("write reconciliation baseline: %w", err)
		}
		return exitOK, nil
	}

	if *check {
		m, err := open(ctx, env)
		if err != nil {
			return exitRefused, err
		}
		m.reconciliationBase, err = authoritativePolicyRef(ctx, m, *baselineRefFlag, env)
		if err != nil {
			return exitRefused, err
		}
		snapshot, report, err := reconciliationPass(ctx, m)
		if err != nil {
			return exitRefused, err
		}
		if code, err := enforceEvidence(ctx, m, *baselineRefFlag, env); code != exitOK || err != nil {
			return code, err
		}
		if *baselineFlag != "" {
			current, present, err := readPolicyAt(ctx, m, snapshot.Commit)
			if err != nil {
				return exitRefused, err
			}
			if !present {
				return exitRefused, fmt.Errorf("reconciliation baseline %s is absent at pinned commit %s", *baselineFlag, shortCommit(snapshot.Commit))
			}
			if current.Policy.Generator != reconcile.Generator {
				return exitRefused, fmt.Errorf("candidate policy evaluator %s is historical; candidate enforcement requires %s: synchronize the policy under the current evaluator", current.Policy.Generator, reconcile.Generator)
			}
			// v1 retains its existing working-tree generation/check semantics;
			// partitioned policy certifies only the pinned committed inputs.
			if current.Policy.Schema == reconcile.PolicySchema {
				snapshot, report, err = pinnedReconciliation(ctx, m, snapshot.Commit, reconcile.Generator)
				if err != nil {
					return exitRefused, err
				}
				_, m.set, err = reconciliationStateAt(ctx, m, snapshot.Commit)
				if err != nil {
					return exitRefused, err
				}
				m.proj = m.projection(m.set)
			}
			baseline := current.Policy.Normalized()
			baselineSHA256 := policyDigest(current.Files[reconcile.PolicyPath])
			result := reconcile.CompareBaseline(baseline, report)
			identitiesMatch := baseline.Schema == reconcile.PolicySchema || reconcile.BaselineIdentitiesMatch(baseline, report)
			ref, err := authoritativePolicyRef(ctx, m, *baselineRefFlag, env)
			if err != nil {
				return exitRefused, err
			}
			baselineRef, err := checkReconciliationBaselineRef(ctx, m, snapshot, report, *baselineFlag, ref, *baselineReplayRefFlag, baseline, result)
			if err != nil {
				return exitRefused, err
			}
			if err := writeReconciliationRatchetOutput(env, *asJSON, *baselineFlag, baselineSHA256, identitiesMatch, baselineRef, result, report, current.Evidence); err != nil {
				return exitRefused, err
			}
			if baselineRef != nil && baselineRef.Result != nil && baselineRef.Result.Blocking() {
				return exitRefused, fmt.Errorf("checked-in reconciliation baseline widens %s at %s by %d allowance regression(s)", baselineRef.Ref, shortCommit(baselineRef.Commit), len(baselineRef.Result.Regressions))
			}
			if result.Blocking() {
				return exitRefused, fmt.Errorf("offline reconciliation at %s has %d regression(s) beyond the checked-in baseline", shortCommit(snapshot.Commit), len(result.Regressions))
			}
			if len(result.Improvements) > 0 {
				return exitRefused, fmt.Errorf("offline reconciliation at %s resolved or reduced %d allowance(s); tighten the checked-in baseline in the same change", shortCommit(snapshot.Commit), len(result.Improvements))
			}
			if !identitiesMatch {
				return exitRefused, fmt.Errorf("offline reconciliation at %s does not match the checked-in baseline input identities; regenerate the baseline from this exact source, design, code, and memory inventory", shortCommit(snapshot.Commit))
			}
			return exitOK, nil
		}
		if err := writeReconciliationOutput(env, *asJSON, 0, report); err != nil {
			return exitRefused, err
		}
		if report.Blocking() {
			return exitRefused, fmt.Errorf("offline reconciliation at %s found %d blocker(s)", shortCommit(snapshot.Commit), report.Counts.Blocking)
		}
		return exitOK, nil
	}

	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	snapshot, report, err := reconciliationPass(ctx, m)
	if err != nil {
		return exitRefused, err
	}
	if err := safeToAppend(report); err != nil {
		if outputErr := writeReconciliationOutput(env, *asJSON, 0, report); outputErr != nil {
			return exitRefused, outputErr
		}
		return exitRefused, err
	}
	actions, err := selectReconciliationActions(report.Actions, actionSelectors)
	if err != nil {
		return exitRefused, err
	}
	written := 0
	if len(actions) > 0 {
		actor, err := m.actor(ctx, *actorFlag, env)
		if err != nil {
			return exitUsage, err
		}
		if err := assertHead(ctx, m, snapshot.Commit); err != nil {
			return exitRefused, err
		}
		quiet := env
		quiet.Stdout = io.Discard
		for _, action := range actions {
			r, err := m.recordFromDraft(action.Draft, actor, env)
			if err != nil {
				return exitRefused, fmt.Errorf("materialize %s for %s after %d write(s): %w", action.Code, action.Scope, written, err)
			}
			if _, err := m.store(ctx, r, action.Draft.Body, quiet); err != nil {
				return exitRefused, fmt.Errorf("append %s for %s after %d write(s): %w", action.Code, action.Scope, written, err)
			}
			written++
			if err := m.reload(ctx); err != nil {
				return exitRefused, fmt.Errorf("reload memory after %d write(s): %w", written, err)
			}
		}
		report = offlineReconciliationReport(ctx, m, snapshot)
	}
	if err := writeReconciliationOutput(env, *asJSON, written, report); err != nil {
		return exitRefused, err
	}
	if report.Blocking() {
		return exitRefused, fmt.Errorf("appended %d safe record(s); %d blocker(s) still require explicit reconciliation", written, report.Counts.Blocking)
	}
	return exitOK, nil
}

func reconcileUsage() error {
	return usage("reconcile needs exactly one of --check, --write, --baseline-generate, or --baseline-sync; --check accepts [--baseline .github/memory-reconciliation-baseline.json [--baseline-ref trusted-ref [--baseline-replay-ref original-snapshot]]] [--json], --write accepts [--action 'code|scope']... [--actor who] [--json], --baseline-sync requires --baseline-ref trusted-ref and accepts [--migrate-v2] [--baseline-replay-ref original-snapshot] [--json], and --baseline-generate has no other flags")
}

type reconciliationActionSelector struct {
	Code  string
	Scope string
}

var reconciliationActionCodes = map[string]bool{
	"add-code-anchor":     true,
	"add-design-coverage": true,
	"add-design-decision": true,
	"add-prd-claim":       true,
	"add-prd-question":    true,
	"capture-document":    true,
	"refresh-code-anchor": true,
}

func parseReconciliationActionSelectors(values []string) ([]reconciliationActionSelector, error) {
	selectors := make([]reconciliationActionSelector, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		code, scope, ok := strings.Cut(value, "|")
		if !ok || code == "" || scope == "" || strings.ContainsAny(code, "\x00\r\n") || strings.ContainsAny(scope, "\x00\r\n") {
			return nil, usage("reconcile --action %q must be an exact code|scope selector", value)
		}
		if !reconciliationActionCodes[code] {
			return nil, usage("reconcile --action %q uses unknown action code %q", value, code)
		}
		key := code + "\x00" + scope
		if seen[key] {
			return nil, usage("reconcile --action repeats selector %q", value)
		}
		seen[key] = true
		selectors = append(selectors, reconciliationActionSelector{Code: code, Scope: scope})
	}
	return selectors, nil
}

func selectReconciliationActions(actions []reconcile.Action, selectors []reconciliationActionSelector) ([]reconcile.Action, error) {
	if len(selectors) == 0 {
		return actions, nil
	}
	bySelector := make(map[string][]reconcile.Action, len(actions))
	for _, action := range actions {
		key := action.Code + "\x00" + action.Scope
		bySelector[key] = append(bySelector[key], action)
	}
	selected := make(map[string]bool, len(selectors))
	for _, selector := range selectors {
		key := selector.Code + "\x00" + selector.Scope
		matches := bySelector[key]
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("reconcile --action %q matches no current deterministic action", selector.Code+"|"+selector.Scope)
		case 1:
			selected[key] = true
		default:
			return nil, fmt.Errorf("reconcile --action %q is ambiguous across %d current deterministic actions", selector.Code+"|"+selector.Scope, len(matches))
		}
	}
	out := make([]reconcile.Action, 0, len(selectors))
	for _, action := range actions {
		if selected[action.Code+"\x00"+action.Scope] {
			out = append(out, action)
		}
	}
	return out, nil
}

func checkReconciliationBaselineRef(ctx context.Context, m *memory, snapshot reconcile.Snapshot, report reconcile.Report, name, ref, replayRef string, candidate reconcile.Baseline, live reconcile.BaselineResult) (*reconciliationBaselineRefOutput, error) {
	return compareReconciliationBaselineRef(ctx, m, snapshot, report, name, ref, replayRef, candidate, live, false)
}

func compareReconciliationBaselineRef(ctx context.Context, m *memory, snapshot reconcile.Snapshot, report reconcile.Report, name, ref, replayRef string, candidate reconcile.Baseline, live reconcile.BaselineResult, allowSelfMigration bool) (*reconciliationBaselineRefOutput, error) {
	if ref == "" {
		return nil, nil
	}
	commit, err := m.repo.ResolveCommit(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve reconciliation baseline ref %q: %w", ref, err)
	}
	mergeBase, err := m.repo.MergeBase(ctx, commit, snapshot.Commit)
	if err != nil {
		return nil, fmt.Errorf("compare reconciliation baseline ref %q with pinned HEAD: %w", ref, err)
	}
	if mergeBase != commit {
		return nil, fmt.Errorf("reconciliation baseline ref %q at %s is not an ancestor of pinned HEAD %s", ref, shortCommit(commit), shortCommit(snapshot.Commit))
	}
	if commit == snapshot.Commit && !allowSelfMigration {
		return nil, fmt.Errorf("reconciliation baseline ref %q resolves to pinned HEAD %s; the trusted baseline must be a strict ancestor", ref, shortCommit(commit))
	}
	read, present, err := readPolicyAt(ctx, m, commit)
	if err != nil {
		return nil, err
	}
	prior := read.Policy.Normalized()
	digest := policyDigest(read.Files[name])
	check := &reconciliationBaselineRefOutput{Ref: ref, Commit: commit, Status: "compared", BaselineSHA256: digest}
	if present {
		check.Policy = &read.Evidence
	}
	priorOpts, priorSet, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return nil, err
	}
	// Reproduce a known legacy generator's exact trusted identities before
	// comparing the new policy; do not turn compiler changes into identity bypasses.
	if present {
		priorOpts = baselineReproductionOptions(priorOpts, prior.Generator)
	}
	priorSnapshot, err := m.snapshotAt(ctx, commit, priorOpts)
	if err != nil {
		return nil, fmt.Errorf("rebuild authored-source inventory at trusted reconciliation baseline %s: %w", shortCommit(commit), err)
	}
	if present {
		if prior.Schema == reconcile.PolicySchema && candidate.Schema != reconcile.PolicySchema {
			return nil, fmt.Errorf("v2 policy cannot downgrade to v1 or restart its lineage")
		}
		if prior.Schema == reconcile.BaselineSchema && candidate.Schema == reconcile.PolicySchema {
			if !reconcile.CompareBaselines(prior, candidate).Exact() {
				return nil, fmt.Errorf("v1 to v2 migration must preserve the entire trusted allowance inventory")
			}
			if err := migrationPathsOnly(ctx, m, commit, snapshot.Commit); err != nil {
				return nil, err
			}
		}
		result := reconcile.CompareBaselines(prior, candidate)
		priorSources := append([]string(nil), priorSnapshot.AuthoredSources...)
		if prior.Schema == reconcile.PolicySchema {
			_, priorReport, err := pinnedReconciliation(ctx, m, commit, prior.Generator)
			if err != nil {
				return nil, err
			}
			if !reconcile.CompareBaseline(prior, priorReport).Exact() {
				return nil, fmt.Errorf("trusted v2 policy at %s does not reproduce its exact allowances under %s", shortCommit(commit), prior.Generator)
			}
		} else if !baselineSnapshotMatches(prior, priorSnapshot, priorSet) {
			replayed, replaySet, replayErr := replayReconciliationBaseline(ctx, m, name, replayRef, prior, digest)
			if replayErr != nil {
				return nil, fmt.Errorf("trusted reconciliation baseline %s source, design, code, and memory identities cannot be reproduced by the candidate compiler: %w", shortCommit(commit), replayErr)
			}
			check.ReplayCommit = replayed.Commit
			// A replay proves old inputs, not permission to discard intervening
			// changes. Check transitions from both it and the actual trusted base.
			result.Regressions = append(result.Regressions, reconcile.CodeTransitionRegressions(replayed, snapshot, replaySet, m.projection(replaySet), m.set, m.proj)...)
			priorSources = unionBaselineSources(priorSources, replayed.AuthoredSources)
		}
		priorProjection := m.projection(priorSet)
		result.Regressions = append(result.Regressions, reconcile.CodeTransitionRegressions(priorSnapshot, snapshot, priorSet, priorProjection, m.set, m.proj)...)
		retirementRegressions := reconcile.SourceRetirementRegressions(priorSources, snapshot.AuthoredSources, nil)
		if len(retirementRegressions) > 0 {
			retired, retirementsErr := trustedSourceRetirements(ctx, m, commit, priorSnapshot)
			if retirementsErr != nil {
				return nil, retirementsErr
			}
			for source := range reconcile.TrustedStandaloneRetirements(snapshot, m.set, m.proj) {
				retired[source] = true
			}
			retirementRegressions = reconcile.SourceRetirementRegressions(priorSources, snapshot.AuthoredSources, retired)
		}
		result.Regressions = append(result.Regressions, retirementRegressions...)
		sort.Slice(result.Regressions, func(i, j int) bool {
			a, b := result.Regressions[i], result.Regressions[j]
			if a.Code != b.Code {
				return a.Code < b.Code
			}
			if a.Scope != b.Scope {
				return a.Scope < b.Scope
			}
			if a.Feature != b.Feature {
				return a.Feature < b.Feature
			}
			return a.Reason < b.Reason
		})
		check.Result = &result
		return check, nil
	}
	if candidate.Schema == reconcile.PolicySchema {
		return nil, fmt.Errorf("v2 policy cannot bootstrap without a trusted baseline; migrate the existing v1 lineage explicitly")
	}
	check.Status = "bootstrap"
	if !reconcile.BaselineIdentitiesMatch(candidate, report) || !live.Exact() {
		return nil, fmt.Errorf("reconciliation baseline bootstrap from %s is not an exact allowance set for pinned HEAD identities", shortCommit(commit))
	}
	bootstrap := reconcile.BaselineResult{
		Schema: reconcile.BaselineResultSchema, Regressions: []reconcile.BaselineDelta{}, Improvements: []reconcile.BaselineDelta{},
	}
	for priorPath := range priorSnapshot.ExistingPaths {
		if snapshot.ExistingPaths[priorPath] {
			continue
		}
		bootstrap.Regressions = append(bootstrap.Regressions, reconcile.BaselineDelta{
			Code: "bootstrap-path-deletion", Scope: priorPath,
			Reason:        "a first-baseline change may not erase a trusted-tree path before the ratchet has recorded its identity",
			BaselineCount: 1, CurrentCount: 0,
		})
	}
	sort.Slice(bootstrap.Regressions, func(i, j int) bool { return bootstrap.Regressions[i].Scope < bootstrap.Regressions[j].Scope })
	if bootstrap.Blocking() {
		check.Result = &bootstrap
	}
	return check, nil
}

func trustedSourceRetirements(ctx context.Context, m *memory, commit string, trustedSnapshot reconcile.Snapshot) (map[string]bool, error) {
	_, set, err := reconciliationStateAt(ctx, m, commit)
	if err != nil {
		return nil, fmt.Errorf("load memory at trusted reconciliation baseline %s: %w", shortCommit(commit), err)
	}
	proj := m.projection(set)
	closeouts, err := reconcile.ActiveCloseouts(set, proj)
	if err != nil {
		return nil, fmt.Errorf("validate source retirements at trusted reconciliation baseline %s: %w", shortCommit(commit), err)
	}
	owners, err := reconcile.ContextSourceOwners(trustedSnapshot)
	if err != nil {
		return nil, fmt.Errorf("derive feature context ownership at trusted reconciliation baseline %s: %w", shortCommit(commit), err)
	}
	ownerSets := map[string]map[string]bool{}
	mergeOwners := func(next map[string][]string) {
		for source, features := range next {
			if ownerSets[source] == nil {
				ownerSets[source] = map[string]bool{}
			}
			for _, feature := range features {
				ownerSets[source][feature] = true
			}
		}
	}
	mergeOwners(owners)
	replayed := make([]reconcile.ActiveCloseout, 0, len(closeouts))
	for _, closeout := range closeouts {
		snapshot, snapshotErr := closeoutSnapshotAt(ctx, m, closeout, commit)
		if snapshotErr != nil {
			return nil, fmt.Errorf("rebuild source retirement %s at %s: %w", closeout.RecordID, shortCommit(closeout.Receipt.SourceCommit), snapshotErr)
		}
		if validateErr := reconcile.ValidateCloseoutAtSnapshot(closeout, snapshot, set, proj); validateErr != nil {
			return nil, fmt.Errorf("validate source retirement %s at trusted reconciliation baseline %s: %w", closeout.RecordID, shortCommit(commit), validateErr)
		}
		historicalOwners, ownerErr := reconcile.ContextSourceOwners(snapshot)
		if ownerErr != nil {
			return nil, fmt.Errorf("derive historical feature context ownership for source retirement %s at %s: %w", closeout.RecordID, shortCommit(closeout.Receipt.SourceCommit), ownerErr)
		}
		mergeOwners(historicalOwners)
		replayed = append(replayed, closeout)
	}
	sealed := map[string]map[string]bool{}
	for _, closeout := range replayed {
		for _, source := range closeout.Receipt.SourceDocuments {
			file, present := trustedSnapshot.Files[source.Path]
			if !present || file.SHA256 != source.BodySHA256 || !ownerSets[source.Path][closeout.Receipt.Feature] {
				continue
			}
			if sealed[source.Path] == nil {
				sealed[source.Path] = map[string]bool{}
			}
			sealed[source.Path][closeout.Receipt.Feature] = true
		}
	}
	retired := map[string]bool{}
	for source, sourceOwners := range ownerSets {
		if len(sourceOwners) == 0 {
			continue
		}
		complete := true
		for owner := range sourceOwners {
			if !sealed[source][owner] {
				complete = false
				break
			}
		}
		if complete {
			retired[source] = true
		}
	}
	return retired, nil
}

func runCloseout(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("closeout", env)
	feature := fs.String("feature", "", "feature slug whose completed memory is sealed")
	check := fs.Bool("check", false, "validate closeout readiness without writing")
	write := fs.Bool("write", false, "append the immutable closeout receipt")
	asJSON := fs.Bool("json", false, "machine output")
	actorFlag := fs.String("actor", "", "who issues the closeout receipt")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 0 || *feature == "" || *check == *write || (*check && *actorFlag != "") {
		return exitUsage, usage("closeout --feature name needs exactly one of --check [--json] or --write [--actor who] [--json]")
	}

	if *check {
		m, err := open(ctx, env)
		if err != nil {
			return exitRefused, err
		}
		snapshot, err := stableMainlineCloseoutSnapshot(ctx, m)
		if err != nil {
			return exitRefused, err
		}
		draft, receipt, err := reconcile.BuildCloseout(*feature, snapshot, m.set, m.proj)
		if err != nil {
			return exitRefused, err
		}
		status := "ready"
		recordID := ""
		if draft.Kind == "" {
			status = "sealed"
			if head := m.proj.Head("closeout/"+*feature, "seal"); head != nil {
				recordID = head.ID
			}
		}
		if err := writeCloseoutOutput(env, *asJSON, status, recordID, receipt); err != nil {
			return exitRefused, err
		}
		return exitOK, nil
	}

	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	snapshot, err := stableMainlineCloseoutSnapshot(ctx, m)
	if err != nil {
		return exitRefused, err
	}
	draft, receipt, err := reconcile.BuildCloseout(*feature, snapshot, m.set, m.proj)
	if err != nil {
		return exitRefused, err
	}
	if draft.Kind == "" {
		recordID := ""
		if head := m.proj.Head("closeout/"+*feature, "seal"); head != nil {
			recordID = head.ID
		}
		if err := writeCloseoutOutput(env, *asJSON, "sealed", recordID, receipt); err != nil {
			return exitRefused, err
		}
		return exitOK, nil
	}
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	if err := assertHead(ctx, m, snapshot.Commit); err != nil {
		return exitRefused, err
	}
	r, err := m.recordFromDraft(draft, actor, env)
	if err != nil {
		return exitRefused, fmt.Errorf("materialize closeout receipt: %w", err)
	}
	quiet := env
	quiet.Stdout = io.Discard
	if _, err := m.store(ctx, r, draft.Body, quiet); err != nil {
		return exitRefused, fmt.Errorf("append closeout receipt: %w", err)
	}
	if err := m.reload(ctx); err != nil {
		return exitRefused, fmt.Errorf("reload memory after closeout: %w", err)
	}
	checkDraft, checkReceipt, err := reconcile.BuildCloseout(*feature, snapshot, m.set, m.proj)
	if err != nil {
		return exitRefused, fmt.Errorf("verify written closeout: %w", err)
	}
	if checkDraft.Kind != "" || checkReceipt.ClosureSHA256 != receipt.ClosureSHA256 {
		return exitRefused, fmt.Errorf("verify written closeout: the active receipt is not the deterministic fixed point")
	}
	if err := writeCloseoutOutput(env, *asJSON, "sealed", r.ID, receipt); err != nil {
		return exitRefused, err
	}
	return exitOK, nil
}

func runVerify(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("verify", env)
	offline := fs.Bool("offline", false, "require the local-only verifier")
	asJSON := fs.Bool("json", false, "machine output")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 0 || !*offline {
		return exitUsage, usage("verify requires --offline [--json]")
	}
	m, err := open(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	m.reconciliationBase, err = authoritativePolicyRef(ctx, m, "", env)
	if err != nil {
		return exitRefused, err
	}
	_, report, err := reconciliationPass(ctx, m)
	if err != nil {
		return exitRefused, err
	}
	if code, err := enforceEvidence(ctx, m, "", env); code != exitOK || err != nil {
		return code, err
	}
	active, activeErr := reconcile.ActiveCloseouts(m.set, m.proj)
	features := make([]string, 0, len(active))
	if activeErr == nil {
		for _, closeout := range active {
			features = append(features, closeout.Receipt.Feature)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(verificationOutput{Schema: "memory-offline-verification/v1", Closeouts: features, Report: report}); err != nil {
			return exitRefused, fmt.Errorf("write offline verification report: %w", err)
		}
	} else {
		if len(features) == 0 {
			if _, err := fmt.Fprintln(env.Stdout, "active closeouts: none"); err != nil {
				return exitRefused, fmt.Errorf("write command output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(env.Stdout, "active closeouts: %s\n", strings.Join(features, ", ")); err != nil {
				return exitRefused, fmt.Errorf("write command output: %w", err)
			}
		}
		if err := writeReconciliationOutput(env, false, 0, report); err != nil {
			return exitRefused, err
		}
	}
	if report.Blocking() {
		return exitRefused, fmt.Errorf("offline verification found %d blocker(s)", report.Counts.Blocking)
	}
	return exitOK, nil
}

func writeCloseoutOutput(env Env, asJSON bool, status, recordID string, receipt reconcile.CloseoutReceipt) error {
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(closeoutOutput{Schema: "memory-closeout-run/v1", Status: status, RecordID: recordID, Receipt: receipt}); err != nil {
			return fmt.Errorf("write closeout report: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "closeout %s: %s at %s (%d source document(s), %d native record(s), %d code unit(s))\n", receipt.Feature, status, shortCommit(receipt.SourceCommit), len(receipt.SourceDocuments), len(receipt.NativeRecords), len(receipt.CodeUnits)); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	if recordID != "" {
		if _, err := fmt.Fprintf(env.Stdout, "receipt: %s\n", recordID); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	if _, err := fmt.Fprintf(env.Stdout, "closure: %s\n", receipt.ClosureSHA256); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func reconciliationPass(ctx context.Context, m *memory) (reconcile.Snapshot, reconcile.Report, error) {
	snapshot, err := reconciliationSnapshot(ctx, m)
	if err != nil {
		return reconcile.Snapshot{}, reconcile.Report{}, err
	}
	return snapshot, offlineReconciliationReport(ctx, m, snapshot), nil
}

func offlineReconciliationReport(ctx context.Context, m *memory, snapshot reconcile.Snapshot) reconcile.Report {
	opts := reconciliationOptions(m)
	opts.LegacyNativeServes = snapshot.SourceOptions.LegacyNativeServes
	load := func(closeout reconcile.ActiveCloseout) (reconcile.Snapshot, error) {
		return closeoutSnapshotAt(ctx, m, closeout, snapshot.Commit)
	}
	return reconcile.VerifyOfflineWithHistory(snapshot, m.set, m.proj, opts, load)
}

func reconciliationSnapshot(ctx context.Context, m *memory) (reconcile.Snapshot, error) {
	head, err := m.repo.Head(ctx)
	if err != nil {
		return reconcile.Snapshot{}, fmt.Errorf("pin HEAD for offline reconciliation: %w", err)
	}
	opts := reconciliationOptions(m)
	snapshot, err := m.snapshotAt(ctx, head, opts)
	if err != nil {
		return reconcile.Snapshot{}, err
	}
	legacyNative, trustedFeatures, err := trustedNativeServes(ctx, m, snapshot.Commit)
	if err != nil {
		return reconcile.Snapshot{}, err
	}
	snapshot.SourceOptions.LegacyNativeServes = legacyNative
	snapshot.TrustedPRDFeatures = trustedFeatures
	snapshot, err = contractHistorySnapshot(ctx, m, snapshot)
	if err != nil {
		return reconcile.Snapshot{}, err
	}
	return sourceRetirementSnapshot(ctx, m, snapshot, m.set, m.reconciliationBase), nil
}

// stableMainlineCloseoutSnapshot pins closeout evidence to a commit that will
// survive a squash merge. A receipt created from a topic-only commit would
// become historically unverifiable once that topic branch is deleted, because
// its source commit is not an ancestor of the squash result.
func stableMainlineCloseoutSnapshot(ctx context.Context, m *memory) (reconcile.Snapshot, error) {
	mainline := m.repo.DefaultBase(ctx, m.remote, m.branch)
	if mainline == "" {
		return reconcile.Snapshot{}, fmt.Errorf("closeout requires a trusted mainline commit, but neither %s/%s nor %s exists", m.remote, m.branch, m.branch)
	}
	mainlineCommit, err := m.repo.ResolveCommit(ctx, mainline)
	if err != nil {
		return reconcile.Snapshot{}, fmt.Errorf("resolve closeout mainline %s: %w", mainline, err)
	}
	head, err := m.repo.Head(ctx)
	if err != nil {
		return reconcile.Snapshot{}, fmt.Errorf("pin HEAD for closeout: %w", err)
	}
	if head != mainlineCommit {
		return reconcile.Snapshot{}, fmt.Errorf("closeout source must be the trusted mainline %s at %s; HEAD is %s: land the completed feature and its memory, refresh the mainline ref, then create a closeout-only branch at that commit", mainline, shortCommit(mainlineCommit), shortCommit(head))
	}
	snapshot, err := m.snapshotAt(ctx, mainlineCommit, reconciliationOptions(m))
	if err != nil {
		return reconcile.Snapshot{}, err
	}
	return contractHistorySnapshot(ctx, m, snapshot)
}

func reconciliationOptions(m *memory) reconcile.Options {
	return reconciliationOptionsFor(m.cfg, m.set)
}

func reconciliationOptionsFor(cfg config.Config, set *store.Set) reconcile.Options {
	opts := reconcile.Options{
		ProductVision: cfg.ProductVision, ArchitecturePrinciples: cfg.ArchitecturePrinciples, Incidents: cfg.Incidents,
		PRDs: cfg.PRDs, TechDesigns: cfg.TechDesigns, Plans: cfg.Plans,
		Brainstorms: cfg.Brainstorms, Problems: cfg.Problems, Reviews: cfg.Reviews, Feedback: cfg.Feedback,
		Grandfathered:           append([]string(nil), cfg.Grandfathered...),
		PrecedenceGrandfathered: append([]string(nil), cfg.PrecedenceGrandfathered...),
	}
	for _, id := range set.IDs() {
		r := set.Get(id)
		if r.Kind == record.Document && repositoryPath(r.Provenance.Source) {
			opts.ExtraSources = append(opts.ExtraSources, r.Provenance.Source)
		}
	}
	sort.Strings(opts.ExtraSources)
	return opts
}

func reconciliationStateAt(ctx context.Context, m *memory, commit string) (reconcile.Options, *store.Set, error) {
	files, err := m.repo.Files(ctx, commit, []string{".pyxis/config.yaml"})
	if err != nil {
		return reconcile.Options{}, nil, fmt.Errorf("read configuration at %s: %w", shortCommit(commit), err)
	}
	cfg := config.Defaults()
	if body, ok := files[".pyxis/config.yaml"]; ok {
		parsed, defects := config.Parse(body)
		if len(defects) > 0 {
			messages := make([]string, 0, len(defects))
			for _, defect := range defects {
				messages = append(messages, defect.String())
			}
			return reconcile.Options{}, nil, fmt.Errorf("configuration at %s has %d defect(s): %s", shortCommit(commit), len(defects), strings.Join(messages, "; "))
		}
		cfg = parsed
	}
	set, err := m.baseSet(ctx, commit)
	if err != nil {
		return reconcile.Options{}, nil, fmt.Errorf("load memory at %s: %w", shortCommit(commit), err)
	}
	return reconciliationOptionsFor(cfg, set), set, nil
}

func closeoutSnapshotAt(ctx context.Context, m *memory, closeout reconcile.ActiveCloseout, descendant string) (reconcile.Snapshot, error) {
	mergeBase, err := m.repo.MergeBase(ctx, closeout.Receipt.SourceCommit, descendant)
	if err != nil || mergeBase != closeout.Receipt.SourceCommit {
		return reconcile.Snapshot{}, fmt.Errorf("source commit %s is not an ancestor of %s", closeout.Receipt.SourceCommit, shortCommit(descendant))
	}
	opts, _, err := reconciliationStateAt(ctx, m, closeout.Receipt.SourceCommit)
	if err != nil {
		return reconcile.Snapshot{}, err
	}
	for _, source := range closeout.Receipt.SourceDocuments {
		opts.ExtraSources = append(opts.ExtraSources, source.Path)
	}
	sort.Strings(opts.ExtraSources)
	unique := opts.ExtraSources[:0]
	for _, source := range opts.ExtraSources {
		if len(unique) == 0 || unique[len(unique)-1] != source {
			unique = append(unique, source)
		}
	}
	opts.ExtraSources = unique
	return m.snapshotAt(ctx, closeout.Receipt.SourceCommit, opts)
}

func repositoryPath(name string) bool {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, "\\") || strings.Contains(name, ":") || strings.ContainsAny(name, "\x00\r\n") || strings.HasPrefix(name, "../") {
		return false
	}
	return name != "."
}

func assertHead(ctx context.Context, m *memory, want string) error {
	got, err := m.repo.Head(ctx)
	if err != nil {
		return fmt.Errorf("re-read HEAD: %w", err)
	}
	if got != want {
		return fmt.Errorf("HEAD moved during reconciliation: pinned %s, now %s; rerun against one stable commit", shortCommit(want), shortCommit(got))
	}
	return nil
}

func (m *memory) reload(ctx context.Context) error {
	set, err := store.Load(ctx, store.DirSource{Root: m.dir})
	if err != nil {
		return err
	}
	m.set = set
	m.proj = resolve.New(set)
	return nil
}

func (m *memory) recordFromDraft(d reconcile.Draft, actor string, env Env) (*record.Record, error) {
	id, err := m.newID(d.Kind, env)
	if err != nil {
		return nil, err
	}
	r := &record.Record{
		Schema: record.Schema, ID: id, Kind: d.Kind, Subject: d.Subject, Family: d.Family,
		Statement: d.Statement, Intent: d.Intent, Rationale: d.Rationale, FitCriterion: d.FitCriterion,
		Tags: append([]string(nil), d.Tags...), Consequence: d.Consequence,
		Relations: append([]record.Relation(nil), d.Relations...), LiteralBody: d.LiteralBody,
		Issuer: actor, CreatedAt: env.Now().UTC().Format(time.RFC3339), Provenance: d.Provenance,
	}
	sort.Strings(r.Tags)
	sort.Slice(r.Relations, func(i, j int) bool {
		if r.Relations[i].Type != r.Relations[j].Type {
			return r.Relations[i].Type < r.Relations[j].Type
		}
		return r.Relations[i].Target < r.Relations[j].Target
	})
	for _, rel := range r.Relations {
		if m.set.Get(rel.Target) == nil {
			return nil, fmt.Errorf("draft relation %s targets absent record %s", rel.Type, rel.Target)
		}
	}
	if r.Kind == record.Document {
		if r.LiteralBody {
			sum := sha256.Sum256(d.Body)
			r.BodySHA256 = hex.EncodeToString(sum[:])
		} else {
			r.BodySHA256 = record.HashBody(d.Body)
			r.Embeds = record.ParseEmbeds(d.Body, r.Subject)
		}
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}

func safeToAppend(report reconcile.Report) error {
	fatal := map[string]bool{
		"invalid-memory-file": true, "dangling-memory-relation": true,
		"invalid-memory-projection": true, "memory-family-conflict": true,
		"duplicate-source-binding": true, "source-mode-conflict": true,
		"source-address-collision": true, "reconcile-action-collision": true,
		"code-anchor-collision": true, "duplicate-code-anchor": true,
		"unmappable-code-anchor": true,
	}
	for _, finding := range report.Findings {
		if fatal[finding.Code] {
			return fmt.Errorf("automatic writes refused while %s remains at %s: %s", finding.Code, finding.Scope, finding.Detail)
		}
	}
	return nil
}

func writeReconciliationOutput(env Env, asJSON bool, written int, report reconcile.Report) error {
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reconciliationOutput{Schema: "memory-reconciliation-run/v1", Written: written, Report: report}); err != nil {
			return fmt.Errorf("write reconciliation report: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "offline reconciliation %s: %d source(s), %d PRD unit(s), %d design(s), %d code unit(s)\n", shortCommit(report.SourceCommit), report.Counts.Sources, report.Counts.PRDUnits, report.Counts.Designs, report.Counts.Code); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	if written > 0 {
		if _, err := fmt.Fprintf(env.Stdout, "appended %d deterministic repair record(s)\n", written); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	for _, finding := range report.Findings {
		class := "advisory"
		if finding.Blocking {
			class = "blocking"
		}
		writable := ""
		if finding.Writable {
			writable = ", writable"
		}
		if _, err := fmt.Fprintf(env.Stdout, "- [%s%s] %s %s — %s\n", class, writable, finding.Code, finding.Scope, finding.Detail); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	if err := writeContractCompletion(env, report); err != nil {
		return err
	}
	if !report.Blocking() {
		if _, err := fmt.Fprintln(env.Stdout, "integrity: ok"); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "integrity: blocked by %d finding(s); %d safe action(s) available\n", report.Counts.Blocking, report.Counts.Actions); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func writeReconciliationRatchetOutput(env Env, asJSON bool, baselinePath, baselineSHA256 string, identitiesMatch bool, baselineRef *reconciliationBaselineRefOutput, result reconcile.BaselineResult, report reconcile.Report, policy policyEvidence) error {
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		out := reconciliationRatchetOutput{
			Schema: "memory-reconciliation-ratchet-run/v2", BaselinePath: baselinePath, Policy: policy,
			BaselineSHA256: baselineSHA256, InputIdentitiesMatch: identitiesMatch,
			BaselineRef: baselineRef, Result: result, Report: report,
		}
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("write reconciliation ratchet report: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "offline reconciliation ratchet %s: %d current finding(s), %d baseline finding(s)\n", shortCommit(report.SourceCommit), result.CurrentCount, result.BaselineCount); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	if _, err := fmt.Fprintf(env.Stdout, "baseline %s: %s\n", baselinePath, baselineSHA256); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	if _, err := fmt.Fprintf(env.Stdout, "policy: %s, evaluator %s, %d pinned file(s)\n", policy.Schema, policy.Generator, len(policy.Files)); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	if identitiesMatch {
		if _, err := fmt.Fprintln(env.Stdout, "baseline inputs: exact"); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(env.Stdout, "baseline inputs: stale"); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	if baselineRef != nil {
		if baselineRef.Status == "bootstrap" {
			if _, err := fmt.Fprintf(env.Stdout, "trusted baseline ref %s at %s: bootstrap (path absent; candidate exactly matches current findings)\n", baselineRef.Ref, shortCommit(baselineRef.Commit)); err != nil {
				return fmt.Errorf("write command output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(env.Stdout, "trusted baseline ref %s at %s: %s\n", baselineRef.Ref, shortCommit(baselineRef.Commit), baselineRef.BaselineSHA256); err != nil {
				return fmt.Errorf("write command output: %w", err)
			}
		}
		if baselineRef.ReplayCommit != "" {
			if _, err := fmt.Fprintf(env.Stdout, "trusted baseline inputs replayed exactly at %s\n", baselineRef.ReplayCommit); err != nil {
				return fmt.Errorf("write baseline replay evidence: %w", err)
			}
		}
		if baselineRef.Result != nil {
			for _, regression := range baselineRef.Result.Regressions {
				if _, err := fmt.Fprintf(env.Stdout, "- [baseline-regression] %s %s — %s (trusted %d, candidate %d)\n", regression.Code, regression.Scope, regression.Reason, regression.BaselineCount, regression.CurrentCount); err != nil {
					return fmt.Errorf("write command output: %w", err)
				}
			}
			if baselineRef.Result.Blocking() {
				if _, err := fmt.Fprintf(env.Stdout, "baseline monotonicity: blocked by %d widening(s)\n", len(baselineRef.Result.Regressions)); err != nil {
					return fmt.Errorf("write command output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(env.Stdout, "baseline monotonicity: ok; %d tightening(s)\n", len(baselineRef.Result.Improvements)); err != nil {
					return fmt.Errorf("write command output: %w", err)
				}
			}
		}
	}
	for _, regression := range result.Regressions {
		if _, err := fmt.Fprintf(env.Stdout, "- [regression] %s %s — %s (baseline %d, current %d)\n", regression.Code, regression.Scope, regression.Reason, regression.BaselineCount, regression.CurrentCount); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	for _, improvement := range result.Improvements {
		if _, err := fmt.Fprintf(env.Stdout, "- [baseline-stale] %s %s — %s (baseline %d, current %d)\n", improvement.Code, improvement.Scope, improvement.Reason, improvement.BaselineCount, improvement.CurrentCount); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	if result.Blocking() {
		if _, err := fmt.Fprintf(env.Stdout, "debt ratchet: blocked by %d regression(s); strict reconciliation reports %d blocker(s)\n", len(result.Regressions), report.Counts.Blocking); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	if len(result.Improvements) > 0 || !identitiesMatch {
		if _, err := fmt.Fprintf(env.Stdout, "debt ratchet: baseline update required (%d improvement(s), inputs exact: %t); strict reconciliation reports %d blocker(s)\n", len(result.Improvements), identitiesMatch, report.Counts.Blocking); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "debt ratchet: exact; strict reconciliation reports %d blocker(s)\n", report.Counts.Blocking); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return writeContractCompletion(env, report)
}

func writeContractCompletion(env Env, report reconcile.Report) error {
	if report.Completion == nil || len(report.Completion.Findings) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "contract completion: %d outstanding design item(s); planning and closeout still require completion\n", len(report.Completion.Findings)); err != nil {
		return fmt.Errorf("write completion output: %w", err)
	}
	for _, finding := range report.Completion.Findings {
		if _, err := fmt.Fprintf(env.Stdout, "- [%s] %s — %s\n", finding.State, finding.Scope, finding.Action); err != nil {
			return fmt.Errorf("write completion output: %w", err)
		}
	}
	return nil
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
