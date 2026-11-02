package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

type PrototypeResult struct {
	Worktree string
	Branch   string
	Base     string
	Route    PrototypeRoute
}

// CreatePrototype owns only the new prototype branch and worktree. It never
// checks out or writes files in the caller's checkout or the draft branch.
func CreatePrototype(ctx context.Context, root string, cfg config.Config, feature, idea string, resolve PrototypeRouteResolver, run RunCommand) (out PrototypeResult, err error) {
	if !validFeature(feature) {
		return out, fmt.Errorf("feature %q must be a lowercase hyphenated slug", feature)
	}
	body, err := os.ReadFile(idea)
	if err != nil {
		return out, fmt.Errorf("read idea file %q: %w", idea, err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return out, fmt.Errorf("idea file %q is empty", idea)
	}
	out, err = prototypeLocation(ctx, root, feature, run)
	if err != nil {
		return out, err
	}
	draft := cfg.Draft.BranchPrefix + feature
	exists, err := prototypeBranchExists(ctx, root, draft, run)
	if err != nil {
		return out, err
	}
	if !exists {
		return out, fmt.Errorf("local draft branch %s is missing; open the draft first", draft)
	}
	out.Route, err = resolve(ctx)
	if err != nil {
		return out, fmt.Errorf("resolve prototype build route: %w", err)
	}
	// Ignore remote ref mappings so fetching mainline cannot overwrite a draft ref.
	if _, err := run(ctx, root, nil, "git", "fetch", "--refmap=", cfg.Remote, cfg.Branch); err != nil {
		return out, fmt.Errorf("fetch mainline before prototyping %s: %w", feature, err)
	}
	base, err := run(ctx, root, nil, "git", "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return out, fmt.Errorf("resolve fetched mainline: %w", err)
	}
	out.Base = strings.TrimSpace(string(base))
	exists, err = prototypeBranchExists(ctx, root, out.Branch, run)
	if err != nil {
		return out, err
	}
	if exists {
		return out, fmt.Errorf("prototype branch %s already exists; use pyxis draft prototype %s --remove", out.Branch, feature)
	}
	if _, err := os.Lstat(out.Worktree); err == nil {
		return out, fmt.Errorf("prototype directory %s already exists; use pyxis draft prototype %s --remove", out.Worktree, feature)
	} else if !os.IsNotExist(err) {
		return out, fmt.Errorf("inspect prototype directory: %w", err)
	}
	registered, _, _, err := prototypeRegistration(ctx, root, out.Worktree, run)
	if err != nil {
		return out, err
	}
	if registered {
		return out, fmt.Errorf("prototype directory %s is already registered as a worktree; use pyxis draft prototype %s --remove", out.Worktree, feature)
	}
	if err := checkPrototypeCheckoutIsolation(ctx, root, run, out.Worktree); err != nil {
		return out, err
	}
	// Refuse static layouts that the matching removal would reject.
	if err := checkPrototypeRemovalWorktrees(ctx, root, out.Worktree, executorruntime.TaskStateRoot(out.Worktree), run); err != nil {
		return out, fmt.Errorf("prototype cannot be safely removed after creation: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out.Worktree), 0o755); err != nil {
		return out, fmt.Errorf("create prototype parent directory: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, cleanupPrototype(ctx, root, out, run))
		}
	}()
	if _, err = run(ctx, root, nil, "git", "worktree", "add", "--no-track", "-b", out.Branch, out.Worktree, out.Base); err != nil {
		return out, fmt.Errorf("create prototype worktree: %w", err)
	}
	registered, branch, head, err := prototypeRegistration(ctx, root, out.Worktree, run)
	if err != nil {
		return out, err
	}
	if !registered || branch != "refs/heads/"+out.Branch || head != out.Base {
		return out, fmt.Errorf("confirm prototype worktree %s: expected branch %s at %s; registered=%t branch=%q head=%q", out.Worktree, out.Branch, out.Base, registered, branch, head)
	}
	// --no-track does not clear tracking settings left behind without a ref.
	upstream, err := run(ctx, root, nil, "git", "for-each-ref", "--format=%(upstream)", "refs/heads/"+out.Branch)
	if err != nil {
		return out, fmt.Errorf("confirm prototype has no upstream: %w", err)
	}
	if value := strings.TrimSpace(string(upstream)); value != "" {
		return out, fmt.Errorf("confirm prototype branch %s: unexpected upstream %s", out.Branch, value)
	}
	return out, nil
}

// RemovePrototype refuses unregistered paths and other branches. Git owns the
// dirty-worktree and initialized-submodule refusals; neither is forced. Status
// supplies the filenames its dirty-worktree refusal omits.
func RemovePrototype(ctx context.Context, root, feature string, run RunCommand) (bool, error) {
	if !validFeature(feature) {
		return false, fmt.Errorf("feature %q must be a lowercase hyphenated slug", feature)
	}
	p, err := prototypeLocation(ctx, root, feature, run)
	if err != nil {
		return false, err
	}
	registered, branch, _, err := prototypeRegistration(ctx, root, p.Worktree, run)
	if err != nil {
		return false, err
	}
	if !registered {
		exists, err := prototypeBranchExists(ctx, root, p.Branch, run)
		if err != nil {
			return false, err
		}
		_, statErr := os.Lstat(p.Worktree)
		if !exists && os.IsNotExist(statErr) {
			return false, nil
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return false, fmt.Errorf("inspect prototype directory: %w", statErr)
		}
		return false, fmt.Errorf("refuse removal: %s is not a registered prototype worktree (branch %s)", p.Worktree, p.Branch)
	}
	if branch != "refs/heads/"+p.Branch {
		return false, fmt.Errorf("refuse removal: worktree %s is on %q, expected refs/heads/%s", p.Worktree, branch, p.Branch)
	}
	state := executorruntime.TaskStateRoot(p.Worktree)
	if err := checkPrototypeCheckoutIsolation(ctx, root, run, p.Worktree, state); err != nil {
		return false, err
	}
	if err := checkPrototypeRemovalWorktrees(ctx, root, p.Worktree, state, run); err != nil {
		return false, err
	}
	if _, err := run(ctx, root, nil, "git", "worktree", "remove", p.Worktree); err != nil {
		status, statusErr := run(ctx, p.Worktree, nil, "git", "status", "--short", "--untracked-files=all")
		return false, errors.Join(fmt.Errorf("remove prototype worktree: %w\n%s", err, status), statusErr)
	}
	if err := os.RemoveAll(state); err != nil {
		return false, fmt.Errorf("remove prototype runtime state: %w", err)
	}
	if _, err := run(ctx, root, nil, "git", "branch", "-D", p.Branch); err != nil {
		return false, fmt.Errorf("remove prototype branch: %w", err)
	}
	return true, nil
}

// Git can consider a worktree clean while an ignored nested checkout is dirty.
// Protect both deletion trees against nested and enclosing registered checkouts.
func checkPrototypeRemovalWorktrees(ctx context.Context, root, worktree, state string, run RunCommand) error {
	worktrees, err := run(ctx, root, nil, "git", "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return fmt.Errorf("inspect prototype removal worktree registrations: %w", err)
	}
	physicalPrototype, err := prototypePhysicalPath(ctx, worktree)
	if err != nil {
		return fmt.Errorf("resolve prototype worktree before removal: %w", err)
	}
	physicalState, err := prototypePhysicalPath(ctx, state)
	if err != nil {
		return fmt.Errorf("resolve prototype runtime state: %w", err)
	}
	for field := range strings.SplitSeq(string(worktrees), "\x00") {
		path, ok := strings.CutPrefix(field, "worktree ")
		if !ok {
			continue
		}
		physicalWorktree, err := prototypePhysicalPath(ctx, path)
		if err != nil {
			return fmt.Errorf("resolve registered worktree %s before prototype removal: %w", path, err)
		}
		for _, target := range []string{physicalState, physicalPrototype} {
			if target == physicalPrototype && physicalWorktree == physicalPrototype {
				continue // Only the exact prototype registration belongs to this removal.
			}
			if prototypePathContains(target, physicalWorktree) || prototypePathContains(physicalWorktree, target) {
				return fmt.Errorf("refuse removal: prototype path %s overlaps another registered worktree at %s", target, path)
			}
		}
	}
	return nil
}

// Check physical containment before creating directories or deleting trees.
// Both directions matter: an operation can change files inside the invoking
// checkout, or delete a directory containing that checkout.
func checkPrototypeCheckoutIsolation(ctx context.Context, root string, run RunCommand, paths ...string) error {
	top, err := run(ctx, root, nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("inspect invoking checkout before prototype operation: %w", err)
	}
	invoking, err := filepath.EvalSymlinks(strings.TrimSuffix(string(top), "\n"))
	if err != nil {
		return fmt.Errorf("resolve invoking checkout before prototype operation: %w", err)
	}
	for _, path := range paths {
		target, err := prototypePhysicalPath(ctx, path)
		if err != nil {
			return fmt.Errorf("resolve prototype path %s before protecting invoking checkout: %w", path, err)
		}
		if prototypePathContains(invoking, target) || prototypePathContains(target, invoking) {
			return fmt.Errorf("refuse prototype operation: %s overlaps invoking checkout %s; run from another checkout", path, invoking)
		}
	}
	return nil
}

func prototypePathContains(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func prototypeLocation(ctx context.Context, root, feature string, run RunCommand) (PrototypeResult, error) {
	if run == nil {
		return PrototypeResult{}, fmt.Errorf("draft process adapter is required")
	}
	raw, err := run(ctx, root, nil, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return PrototypeResult{}, fmt.Errorf("resolve Git common directory: %w", err)
	}
	common, err := filepath.EvalSymlinks(strings.TrimSuffix(string(raw), "\n"))
	if err != nil {
		return PrototypeResult{}, fmt.Errorf("resolve Git common directory: %w", err)
	}
	main := filepath.Dir(common)
	unsupported := func() (PrototypeResult, error) {
		return PrototypeResult{}, fmt.Errorf("unsupported repository layout: Git common directory %s is not the .git directory inside the main checkout Git reports (Git directories not named .git and bare repositories are unsupported)", common)
	}
	if filepath.Base(common) != ".git" {
		return unsupported()
	}
	// The .git parent Git recognizes is the main checkout, including when
	// the invoking checkout points to it through a separate-Git-directory file.
	top, err := run(ctx, main, nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return unsupported()
	}
	resolved, err := filepath.EvalSymlinks(strings.TrimSuffix(string(top), "\n"))
	if err != nil || resolved != main {
		return unsupported()
	}
	return PrototypeResult{
		Branch:   "prototype/" + feature,
		Worktree: filepath.Join(filepath.Dir(main), ".pyxis-worktrees", filepath.Base(main), "prototype-"+feature),
	}, nil
}

func prototypeBranchExists(ctx context.Context, root, branch string, run RunCommand) (bool, error) {
	ref := "refs/heads/" + branch
	out, err := run(ctx, root, nil, "git", "for-each-ref", "--format=%(refname)", ref)
	if err != nil {
		return false, fmt.Errorf("inspect branch %s: %w", branch, err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line == ref {
			return true, nil
		}
	}
	return false, nil
}

func prototypeRegistration(ctx context.Context, root, path string, run RunCommand) (registered bool, branch, head string, err error) {
	out, err := run(ctx, root, nil, "git", "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return false, "", "", fmt.Errorf("inspect prototype worktree registration: %w", err)
	}
	path, err = prototypePhysicalPath(ctx, path)
	if err != nil {
		return false, "", "", fmt.Errorf("resolve prototype registration path: %w", err)
	}
	for record := range strings.SplitSeq(string(out), "\x00\x00") {
		fields := strings.Split(record, "\x00")
		if len(fields) == 0 || fields[0] != "worktree "+path {
			continue
		}
		for _, field := range fields[1:] {
			if value, ok := strings.CutPrefix(field, "branch "); ok {
				branch = value
			}
			if value, ok := strings.CutPrefix(field, "HEAD "); ok {
				head = value
			}
		}
		return true, branch, head, nil
	}
	return false, "", "", nil
}

// Git registers physical paths. Resolve existing ancestors too, since the
// prototype directory can be absent during preflight or failed creation.
func prototypePhysicalPath(ctx context.Context, path string) (string, error) {
	suffix := ""
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		parent := filepath.Dir(path)
		if !os.IsNotExist(err) || parent == path {
			return "", err
		}
		suffix = filepath.Join(filepath.Base(path), suffix)
		path = parent
	}
}

func cleanupPrototype(ctx context.Context, root string, p PrototypeResult, run RunCommand) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	registered, _, _, err := prototypeRegistration(cleanupCtx, root, p.Worktree, run)
	if err != nil {
		return err
	}
	if registered {
		if _, err := run(cleanupCtx, root, nil, "git", "worktree", "remove", "--force", p.Worktree); err != nil {
			return fmt.Errorf("clean up failed prototype worktree: %w", err)
		}
	}
	if err := os.RemoveAll(p.Worktree); err != nil {
		return fmt.Errorf("clean up failed prototype directory: %w", err)
	}
	exists, err := prototypeBranchExists(cleanupCtx, root, p.Branch, run)
	if err != nil {
		return err
	}
	if exists {
		if _, err := run(cleanupCtx, root, nil, "git", "branch", "-D", p.Branch); err != nil {
			return fmt.Errorf("clean up failed prototype branch: %w", err)
		}
	}
	return nil
}
