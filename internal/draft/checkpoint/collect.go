package checkpoint

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var planName = regexp.MustCompile(`^[0-9]{3}-plan\.md$`)

type firstLock struct {
	feature string
	order   int
}

// Run reads a pinned mainline, then writes exactly one checkpoint record once
// the first three locked features' plans have finished implementation.
func Run(ctx context.Context, root string, cfg config.Config, run lifecycle.RunCommand) (Result, error) {
	if run == nil || cfg.Remote == "" || cfg.Branch == "" {
		return Result{}, fmt.Errorf("checkpoint needs a process adapter and configured mainline")
	}
	if _, err := run(ctx, root, nil, "git", "fetch", "--no-tags", cfg.Remote, cfg.Branch); err != nil {
		return Result{}, fmt.Errorf("fetch checkpoint mainline: %w", err)
	}
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return Result{}, fmt.Errorf("open checkpoint repository: %w", err)
	}
	base, err := repo.ResolveCommit(ctx, cfg.MainlineRef())
	if err != nil {
		return Result{}, fmt.Errorf("resolve checkpoint mainline: %w", err)
	}
	entries, err := repo.ListTree(ctx, base, ".")
	if err != nil {
		return Result{}, fmt.Errorf("list checkpoint mainline: %w", err)
	}
	paths := map[string]bool{}
	var corpus []string
	for _, entry := range entries {
		paths[entry.Path] = entry.Regular()
		if entry.Regular() {
			corpus = append(corpus, entry.Path)
		}
	}
	locks, err := firstLocks(ctx, root, cfg.Remote, base, run)
	if err != nil {
		return Result{}, err
	}
	features := make([]Feature, 0, min(3, len(locks)))
	if len(locks) < 3 {
		return Result{Missing: []string{fmt.Sprintf("%d of the first 3 draft-born features have locked", len(locks))}}, nil
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: base, Prefix: store.Dir})
	if err != nil {
		return Result{}, fmt.Errorf("read checkpoint events: %v", err)
	}
	if len(set.Problems) > 0 || len(set.DocumentIssues) > 0 || len(set.Dangling) > 0 {
		return Result{}, fmt.Errorf("checkpoint event memory is invalid")
	}
	projection := resolve.New(set)
	if len(projection.Violations) > 0 || len(projection.Conflicts()) > 0 {
		return Result{}, fmt.Errorf("checkpoint event memory has unresolved conflicts")
	}
	for _, candidate := range locks[:3] {
		name := candidate.feature
		if paths[path.Join(cfg.PRDs, name, "000-base.md")] {
			return Result{}, fmt.Errorf("locked feature %s also has a PRD ledger", name)
		}
		feature := Feature{Name: name}
		planDir := path.Join(cfg.Plans, name)
		for file := range paths {
			feature.HasPlan = feature.HasPlan || path.Dir(file) == planDir && planName.MatchString(path.Base(file)) && paths[file]
		}
		if feature.HasPlan {
			feature.Unfinished, err = lifecycle.UnfinishedTasks(ctx, root, cfg, name, run)
			if err != nil {
				return Result{}, fmt.Errorf("inspect %s implementation: %w", name, err)
			}
		}
		feature.Events, err = retainedEvents(set, projection, name)
		if err != nil {
			return Result{}, err
		}
		features = append(features, feature)
	}
	result, err := Evaluate(base, features, corpus, func(file, feature string) bool { return ownsCorpusFile(file, feature, cfg) })
	if err != nil || !result.Due {
		return result, err
	}
	return Record(ctx, root, result)
}

func firstLocks(ctx context.Context, root, remote, base string, run lifecycle.RunCommand) ([]firstLock, error) {
	listed, err := command(ctx, run, root, "git", "ls-remote", "--tags", remote, "refs/tags/lock/*/v1")
	if err != nil {
		return nil, fmt.Errorf("list first lock tags: %w", err)
	}
	firstParent, err := command(ctx, run, root, "git", "rev-list", "--reverse", "--first-parent", base)
	if err != nil {
		return nil, fmt.Errorf("read mainline lock order: %w", err)
	}
	order := map[string]int{}
	for i, commit := range strings.Fields(firstParent) {
		order[commit] = i + 1
	}
	var found []firstLock
	seen := map[string]bool{}
	for line := range strings.SplitSeq(listed, "\n") {
		if line == "" {
			continue
		}
		oid, ref, ok := strings.Cut(line, "\t")
		if !ok || len(oid) != 40 {
			return nil, fmt.Errorf("malformed first lock tag line %q", line)
		}
		if _, err := hex.DecodeString(oid); err != nil {
			return nil, fmt.Errorf("invalid first lock tag object %q", oid)
		}
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		name, ok := strings.CutPrefix(ref, "refs/tags/lock/")
		if !ok || !strings.HasSuffix(name, "/v1") {
			return nil, fmt.Errorf("unexpected first lock ref %q", ref)
		}
		name = strings.TrimSuffix(name, "/v1")
		if !validFeature(name) || seen[name] {
			return nil, fmt.Errorf("invalid or duplicate first lock for %q", name)
		}
		seen[name] = true
		if _, err := run(ctx, root, nil, "git", "fetch", remote, ref+":"+ref); err != nil {
			return nil, fmt.Errorf("fetch first lock %s: %w", ref, err)
		}
		locked, err := lock.ReadLock(ctx, root, name, 1, run)
		if err != nil {
			return nil, fmt.Errorf("read first lock %s: %w", ref, err)
		}
		position := order[locked.MergeCommit]
		if position == 0 {
			return nil, fmt.Errorf("first lock %s is not on current first-parent mainline", ref)
		}
		found = append(found, firstLock{feature: name, order: position})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].order != found[j].order {
			return found[i].order < found[j].order
		}
		return found[i].feature < found[j].feature
	})
	return found, nil
}

func retainedEvents(set *store.Set, projection *resolve.Projection, feature string) ([]events.Event, error) {
	subject := "feature/" + feature
	var out []events.Event
	foundFirst := false
	for _, r := range set.BySubject[subject] {
		if r.Kind != record.Document || projection.Status(r.ID) != resolve.Active || !eventFamily(r.Family) {
			continue
		}
		foundFirst = foundFirst || r.Family == "events"
		batch, err := events.Parse(set.Bodies[r.ID])
		if err != nil {
			return nil, fmt.Errorf("read retained events for %s: %w", feature, err)
		}
		out = append(out, batch...)
	}
	if !foundFirst {
		return nil, fmt.Errorf("first lock for %s has no retained event log", feature)
	}
	return out, nil
}

func eventFamily(family string) bool {
	if family == "events" {
		return true
	}
	raw, ok := strings.CutPrefix(family, "events-v")
	version, err := strconv.Atoi(raw)
	return ok && err == nil && version >= 2 && strconv.Itoa(version) == raw
}

func ownsCorpusFile(file, feature string, cfg config.Config) bool {
	if strings.HasPrefix(file, path.Join(cfg.PRDs, feature)+"/") {
		return true
	}
	for _, root := range []string{cfg.TechDesigns, cfg.Brainstorms, cfg.Reviews} {
		if path.Dir(file) != root {
			continue
		}
		stem := strings.TrimSuffix(path.Base(file), path.Ext(file))
		if stem == feature || strings.HasPrefix(stem, feature+"-") {
			return true
		}
	}
	return false
}

func validFeature(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, c := range name[1:] {
		if c < 'a' || c > 'z' {
			if c < '0' || c > '9' {
				if c != '-' {
					return false
				}
			}
		}
	}
	return true
}

func command(ctx context.Context, run lifecycle.RunCommand, root, name string, args ...string) (string, error) {
	out, err := run(ctx, root, nil, name, args...)
	return strings.TrimSpace(string(out)), err
}
