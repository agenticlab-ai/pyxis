package compile

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// PRDRevision is one branch commit's exact PRD text, oldest first.
type PRDRevision struct {
	Commit string
	Body   []byte
}

// RequirementChange is one edit after a row's first appearance. A missing
// ruling is reported, not refused; readiness and lock approval decide it.
type RequirementChange struct {
	ID           string
	FromCommit   string
	ToCommit     string
	Before       []string
	After        []string
	Ruling       *Ruling
	RulingCommit string
	Unruled      bool
}

type revisionRow struct {
	Cells []string
}

// ChangeEvents emits one event per ruled requirement and ruling commit. Its
// stable key lets the event log ignore a repeated preparation or abandonment.
func ChangeEvents(result Result, at time.Time) ([]events.Event, error) {
	var emitted []events.Event
	for _, change := range result.Changes {
		if change.Ruling == nil {
			continue
		}
		if change.RulingCommit == "" {
			return nil, fmt.Errorf("ruled requirement %s has no ruling commit", change.ID)
		}
		event, err := events.New(events.RequirementChanged, result.Feature, result.Digest, at)
		if err != nil {
			return nil, err
		}
		event.Key = "ruling:" + change.RulingCommit + ":" + change.Ruling.Problem + ":" + change.ID
		event.Requirement = change.ID
		event.RulingAuthor = change.Ruling.Author
		event.ReasonPresent = strings.TrimSpace(change.Ruling.Reason) != ""
		event.Problem = change.Ruling.Problem
		emitted = append(emitted, event)
	}
	return emitted, nil
}

// CompileBranch pins the draft head and mainline, reads every working file
// from that head, and reads each PRD version from the branch's first-parent
// history. It never consults the mutable worktree or writes records.
func CompileBranch(ctx context.Context, repo *gitsrc.Repo, baseRef, headRef, draftsRoot, feature string) (Result, error) {
	if repo == nil || !record.ValidSubject("prd/"+feature) || feature == "" || strings.Contains(feature, "/") {
		return Result{}, fmt.Errorf("compile draft branch: repository and feature are required")
	}
	root := path.Clean(draftsRoot)
	if root == "." || root == ".." || strings.HasPrefix(root, "../") || strings.HasPrefix(root, "/") {
		return Result{}, fmt.Errorf("compile draft branch: invalid drafts root %q", draftsRoot)
	}
	base, err := repo.ResolveCommit(ctx, baseRef)
	if err != nil {
		return Result{}, err
	}
	head, err := repo.ResolveCommit(ctx, headRef)
	if err != nil {
		return Result{}, err
	}
	prefix := path.Join(root, feature) + "/"
	entries, err := repo.ListTree(ctx, head, prefix)
	if err != nil {
		return Result{}, fmt.Errorf("list draft at %s: %w", head, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Regular() {
			return Result{}, fmt.Errorf("draft part %s is not a regular file", entry.Path)
		}
		relative := strings.TrimPrefix(entry.Path, prefix)
		if relative == "brainstorm/.keep" || relative == "reviews/.keep" {
			continue // scaffold directory marker; it is never a contract part
		}
		if !strings.HasPrefix(entry.Path, prefix) || !validWorkingPath(relative) {
			return Result{}, fmt.Errorf("invalid draft part %s", entry.Path)
		}
		paths = append(paths, entry.Path)
	}
	blobs, err := repo.Files(ctx, head, paths)
	if err != nil {
		return Result{}, fmt.Errorf("read draft at %s: %w", head, err)
	}
	files := make(map[string][]byte, len(paths))
	for _, name := range paths {
		body, ok := blobs[name]
		if !ok {
			return Result{}, fmt.Errorf("draft part %s disappeared at pinned head %s", name, head)
		}
		files[strings.TrimPrefix(name, prefix)] = body
	}
	prdPath := prefix + "prd.md"
	commits, err := repo.PathCommits(ctx, base, head, prdPath)
	if err != nil {
		return Result{}, err
	}
	revisions := make([]PRDRevision, 0, len(commits))
	for _, commit := range commits {
		body, err := repo.Files(ctx, commit, []string{prdPath})
		if err != nil {
			return Result{}, fmt.Errorf("read PRD at %s: %w", commit, err)
		}
		prd, ok := body[prdPath]
		if !ok {
			// Lock preparation removes working files. A later round reverts
			// that commit, restoring the PRD. Compare its next actual text to
			// the previous one; absence itself is not a requirement-row edit.
			continue
		}
		revisions = append(revisions, PRDRevision{Commit: commit, Body: prd})
	}
	result, err := CompileWithHistory(feature, base, files, revisions)
	if err != nil {
		return Result{}, err
	}
	rulingCommits, err := readRulingCommits(ctx, repo, base, head, prefix+"problems.md", feature)
	if err != nil {
		return Result{}, err
	}
	for i := range result.Changes {
		if result.Changes[i].Ruling != nil {
			result.Changes[i].RulingCommit = rulingCommits[result.Changes[i].Ruling.Problem]
		}
	}
	return result, nil
}

// readRulingCommits names the commit that introduced the current ruling for
// each problem. The key survives a repeat preparation from another clone.
func readRulingCommits(ctx context.Context, repo *gitsrc.Repo, base, head, name, feature string) (map[string]string, error) {
	commits, err := repo.PathCommits(ctx, base, head, name)
	if err != nil {
		return nil, fmt.Errorf("read draft ruling history: %w", err)
	}
	seen := map[string]Ruling{}
	out := map[string]string{}
	for _, commit := range commits {
		files, err := repo.Files(ctx, commit, []string{name})
		if err != nil {
			return nil, fmt.Errorf("read problems at %s: %w", commit, err)
		}
		body, ok := files[name]
		if !ok {
			seen = map[string]Ruling{}
			continue
		}
		problems, _, err := compileProblems(feature, body)
		if err != nil {
			if !hasRulingLine(body) {
				// An invalid, unruled authoring round can be repaired later.
				// It conveys no ruling authority and interrupts continuity:
				// a reintroduced ruling belongs to the later valid commit.
				// CompileWithHistory already validated the current head.
				seen = map[string]Ruling{}
				continue
			}
			return nil, fmt.Errorf("compile problems at %s: %w", commit, err)
		}
		current := map[string]Ruling{}
		for _, problem := range problems {
			if problem.Ruling == nil {
				continue
			}
			if prior, ok := seen[problem.ID]; !ok || prior != *problem.Ruling {
				out[problem.ID] = commit
			}
			current[problem.ID] = *problem.Ruling
		}
		seen = current
	}
	return out, nil
}

func hasRulingLine(body []byte) bool {
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "**Ruling:**") {
			return true
		}
	}
	return false
}

func compileChanges(current []byte, revisions []PRDRevision, problems []Problem) ([]RequirementChange, error) {
	if len(revisions) == 0 {
		return nil, nil
	}
	if !bytes.Equal(revisions[len(revisions)-1].Body, current) {
		return nil, fmt.Errorf("compile draft changes: last PRD revision differs from working file")
	}
	rulings := map[string]*Ruling{}
	for _, problem := range problems {
		if problem.Ruling != nil {
			ruling := *problem.Ruling
			rulings[problem.Requirement] = &ruling
		}
	}
	seenCommits := map[string]bool{}
	introduced := map[string]bool{}
	previous := map[string]revisionRow{}
	lastCommit := map[string]string{}
	var changes []RequirementChange
	for _, revision := range revisions {
		if revision.Commit == "" || seenCommits[revision.Commit] {
			return nil, fmt.Errorf("compile draft changes: empty or repeated commit %q", revision.Commit)
		}
		seenCommits[revision.Commit] = true
		rows, err := requirementRows(revision)
		if err != nil {
			return nil, err
		}
		all := map[string]bool{}
		for id := range previous {
			all[id] = true
		}
		for id := range rows {
			all[id] = true
		}
		ids := make([]string, 0, len(all))
		for id := range all {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			before, hadBefore := previous[id]
			after, hasAfter := rows[id]
			if !introduced[id] {
				if hasAfter {
					introduced[id] = true
				}
				continue
			}
			if hadBefore == hasAfter && reflect.DeepEqual(before.Cells, after.Cells) {
				continue
			}
			change := RequirementChange{
				ID: id, FromCommit: lastCommit[id], ToCommit: revision.Commit,
				Before: append([]string(nil), before.Cells...),
				After:  append([]string(nil), after.Cells...),
				Ruling: rulings[id], Unruled: rulings[id] == nil,
			}
			changes = append(changes, change)
		}
		for id := range all {
			lastCommit[id] = revision.Commit
		}
		previous = rows
	}
	return changes, nil
}

func requirementRows(revision PRDRevision) (map[string]revisionRow, error) {
	doc, defects := ledger.ParseProjection("prd.md", schema.Normalize(revision.Body))
	if len(defects) > 0 {
		return nil, fmt.Errorf("compile draft changes at %s: %s", revision.Commit, defects[0].String())
	}
	rows := map[string]revisionRow{}
	for _, id := range doc.UnitOrder {
		unit := doc.Units[id]
		table := doc.Tables[unit.Table]
		if len(table.Cols) != len(unit.Cells) {
			return nil, fmt.Errorf("compile draft changes at %s: row %s has %d cells for %d columns", revision.Commit, id, len(unit.Cells), len(table.Cols))
		}
		if !reconcile.ProjectDraftRow(id, unit.Prefix, table.Cols, unit.Cells).Mapped {
			continue
		}
		rows[id] = revisionRow{Cells: append([]string(nil), unit.Cells...)}
	}
	return rows, nil
}
