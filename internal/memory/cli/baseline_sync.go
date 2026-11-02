package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

type policyChange struct {
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256,omitempty"`
	AfterSHA256  string `json:"after_sha256,omitempty"`
}

type policySyncOutput struct {
	Schema    string                           `json:"schema_version"`
	Head      string                           `json:"head"`
	Base      *reconciliationBaselineRefOutput `json:"baseline_ref"`
	Policy    policyEvidence                   `json:"policy"`
	Report    reconcile.Report                 `json:"report"`
	Planned   []policyChange                   `json:"planned"`
	Completed []string                         `json:"completed"`
	Written   []string                         `json:"written"`
	Status    string                           `json:"status"`
}

func runBaselineSync(ctx context.Context, env Env, ref, replayRef string, migrate, asJSON bool) (int, error) {
	return baselineSync(ctx, env, ref, replayRef, migrate, asJSON, nil)
}

// beforeFile is an internal test seam for deterministic interruption and HEAD
// movement. It is never exposed through flags, configuration, or environment.
func baselineSync(ctx context.Context, env Env, ref, replayRef string, migrate, asJSON bool, beforeFile func(int, string) error) (int, error) {
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	head, err := m.repo.Head(ctx)
	if err != nil {
		return exitRefused, err
	}
	ref, err = authoritativePolicyRef(ctx, m, ref, env)
	if err != nil {
		return exitRefused, err
	}
	if ref == "" {
		return exitRefused, fmt.Errorf("baseline synchronization requires a trusted base")
	}
	m.reconciliationBase = ref
	base, err := m.repo.ResolveCommit(ctx, ref)
	if err != nil {
		return exitRefused, err
	}
	allowSelf := false
	if migrate && base == head {
		mainline := m.repo.DefaultBase(ctx, m.remote, m.branch)
		if mainline == "" {
			return exitRefused, fmt.Errorf("migration requires a locally retained trusted mainline")
		}
		mainlineCommit, err := m.repo.ResolveCommit(ctx, mainline)
		if err != nil {
			return exitRefused, err
		}
		if mainlineCommit != head {
			return exitRefused, fmt.Errorf("self-based migration is allowed only at the trusted mainline")
		}
		allowSelf = true
	}
	current, present, err := readPolicyAt(ctx, m, head)
	if err != nil {
		return exitRefused, err
	}
	if !present {
		return exitRefused, fmt.Errorf("baseline synchronization cannot bootstrap missing policy")
	}
	if migrate != (current.Policy.Schema == reconcile.BaselineSchema) {
		return exitRefused, fmt.Errorf("v1 requires explicit --migrate-v2; an adopted v2 policy uses --baseline-sync without migration")
	}
	snapshot, report, err := pinnedReconciliation(ctx, m, head, reconcile.Generator)
	if err != nil {
		return exitRefused, err
	}
	_, m.set, err = reconciliationStateAt(ctx, m, head)
	if err != nil {
		return exitRefused, err
	}
	m.proj = m.projection(m.set)
	m.pol, err = policy.Parse(m.set.Policy)
	if err != nil {
		return exitRefused, err
	}
	next := reconcile.NewPolicySet(report)
	if migrate && !reconcile.CompareBaselines(current.Policy.Normalized(), next.Normalized()).Exact() {
		return exitRefused, fmt.Errorf("migration must preserve the exact current v1 allowance inventory")
	}
	compared, err := compareReconciliationBaselineRef(ctx, m, snapshot, report, reconcile.PolicyPath, base, replayRef, next.Normalized(), reconcile.CompareBaseline(next.Normalized(), report), allowSelf)
	if err != nil {
		return exitRefused, err
	}
	if compared == nil || compared.Status == "bootstrap" {
		return exitRefused, fmt.Errorf("synchronization requires an existing trusted policy lineage")
	}
	if compared.Result != nil && compared.Result.Blocking() {
		return exitRefused, fmt.Errorf("baseline synchronization refuses %d regression(s): %+v", len(compared.Result.Regressions), compared.Result.Regressions)
	}
	baseSet, err := m.baseSet(ctx, base)
	if err != nil {
		return exitRefused, err
	}
	basePolicy, err := policy.Parse(baseSet.Policy)
	if err != nil {
		return exitRefused, err
	}
	p := packet.Compute(packet.Input{Base: baseSet, Ancestor: baseSet, Head: m.set, Policy: m.pol, BasePolicy: basePolicy, BaseRef: base, Branch: head})
	if !p.OK() {
		return exitRefused, fmt.Errorf("baseline synchronization refuses record transition:\n%s", p.Summary())
	}
	if code, err := enforceEvidence(ctx, m, base, env); code != exitOK || err != nil {
		return code, err
	}
	files, err := reconcile.MarshalPolicySet(next)
	if err != nil {
		return exitRefused, err
	}
	preservePolicyBytes(current, next, files)
	plan := policyWritePlan(current.Files, files)
	root, err := os.OpenRoot(m.root)
	if err != nil {
		return exitRefused, err
	}
	defer closePolicyRoot(root)
	if err := preflightPolicyWrite(ctx, m, root, current.Files, files); err != nil {
		return exitRefused, err
	}
	if err := assertHead(ctx, m, head); err != nil {
		return exitRefused, err
	}
	out := policySyncOutput{Schema: "memory-policy-sync/v1", Head: head, Base: compared, Report: report, Planned: plan, Completed: []string{}, Written: []string{}, Status: "complete", Policy: policyEvidence{Schema: next.Schema, Generator: next.Generator, Files: []policyFileIdentity{}}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.Policy.Files = append(out.Policy.Files, policyFileIdentity{Path: name, SHA256: policyDigest(files[name])})
	}
	writeErr := applyPolicyWrite(ctx, m, root, head, current.Files, files, plan, &out, beforeFile)
	if writeErr != nil {
		out.Status = "incomplete"
	}
	if err := writePolicySyncOutput(env, asJSON, out); err != nil {
		return exitRefused, errors.Join(writeErr, err)
	}
	if writeErr != nil {
		return exitRefused, fmt.Errorf("policy synchronization incomplete (%d/%d paths completed); retry the same pinned plan: %w", len(out.Completed), len(plan), writeErr)
	}
	return exitOK, nil
}

func preservePolicyBytes(current policyRead, next reconcile.PolicySet, files map[string][]byte) {
	if current.Policy.Schema != next.Schema || current.Policy.Generator != next.Generator {
		return
	}
	files[reconcile.PolicyPath] = current.Files[reconcile.PolicyPath]
	old := map[reconcile.BaselineAllowance]bool{}
	for _, a := range current.Policy.Allowances {
		old[a] = true
	}
	for _, a := range next.Allowances {
		if old[a] {
			name, err := reconcile.AllowancePath(a)
			if err == nil {
				files[name] = current.Files[name]
			} // Both sets were already validated.
		}
	}
}

func policyWritePlan(before, after map[string][]byte) []policyChange {
	names := map[string]bool{}
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	plan := []policyChange{}
	for name := range names {
		if samePolicyFile(before, after, name) {
			continue
		}
		change := policyChange{Path: name}
		if body, ok := before[name]; ok {
			change.BeforeSHA256 = policyDigest(body)
		}
		if body, ok := after[name]; ok {
			change.AfterSHA256 = policyDigest(body)
		}
		plan = append(plan, change)
	}
	sort.Slice(plan, func(i, j int) bool {
		if plan[i].Path == reconcile.PolicyPath {
			return false
		}
		if plan[j].Path == reconcile.PolicyPath {
			return true
		}
		return plan[i].Path < plan[j].Path
	})
	return plan
}

func samePolicyFile(a, b map[string][]byte, name string) bool {
	x, xok := a[name]
	y, yok := b[name]
	return xok == yok && bytes.Equal(x, y)
}

func preflightPolicyWrite(ctx context.Context, m *memory, root *os.Root, before, after map[string][]byte) error {
	changes, err := m.repo.WorkingChanges(ctx)
	if err != nil {
		return err
	}
	for _, name := range changes.Paths {
		if name != reconcile.PolicyPath && !reconcile.PolicyMemberPath(name) {
			return fmt.Errorf("commit nonpolicy work before synchronization: %s", name)
		}
	}
	working, err := workingPolicyFiles(root)
	if err != nil {
		return err
	}
	if err := matchesPolicyPlan(working, before, after); err != nil {
		return err
	}
	entries, err := m.repo.IndexEntries(ctx, reconcile.PolicyPath, reconcile.PolicyDirectory)
	if err != nil {
		return err
	}
	var paths []string
	for _, entry := range entries {
		if (entry.Path != reconcile.PolicyPath && !reconcile.PolicyMemberPath(entry.Path)) || !entry.Regular() {
			return fmt.Errorf("unexpected or nonregular staged policy member %s", entry.Path)
		}
		paths = append(paths, entry.Path)
	}
	index, err := m.repo.Files(ctx, "", paths)
	if err != nil {
		return err
	}
	if err := matchesPolicyPlan(index, before, after); err != nil {
		return fmt.Errorf("staged policy: %w", err)
	}
	return nil
}

func matchesPolicyPlan(actual, before, after map[string][]byte) error {
	names := map[string]bool{}
	for name := range actual {
		names[name] = true
	}
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	for name := range names {
		if !samePolicyFile(actual, before, name) && !samePolicyFile(actual, after, name) {
			return fmt.Errorf("policy path %s differs from both pinned input and planned output; preserve that work before synchronizing", name)
		}
	}
	return nil
}

func workingPolicyFiles(root *os.Root) (map[string][]byte, error) {
	for _, dir := range []string{".github", reconcile.PolicyDirectory} {
		info, err := root.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("policy root %s is not a regular directory", dir)
		}
	}
	names := []string{reconcile.PolicyPath}
	entries, err := fs.ReadDir(root.FS(), reconcile.PolicyDirectory)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		name := reconcile.PolicyDirectory + "/" + entry.Name()
		if !reconcile.PolicyMemberPath(name) {
			return nil, fmt.Errorf("unexpected working policy member %s", name)
		}
		names = append(names, name)
	}
	out := map[string][]byte{}
	for _, name := range names {
		info, err := root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("policy member %s is not a regular file", name)
		}
		body, err := root.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out[name] = body
	}
	return out, nil
}

func applyPolicyWrite(ctx context.Context, m *memory, root *os.Root, head string, before, after map[string][]byte, plan []policyChange, out *policySyncOutput, beforeFile func(int, string) error) error {
	for i, change := range plan {
		if beforeFile != nil {
			if err := beforeFile(i, change.Path); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := assertHead(ctx, m, head); err != nil {
			return err
		}
		// Validate the entire index/worktree once more immediately before the
		// first mutation; then compare each owned path immediately before writing.
		if i == 0 {
			if err := preflightPolicyWrite(ctx, m, root, before, after); err != nil {
				return err
			}
		}
		working, err := workingPolicyFile(root, change.Path)
		if err != nil {
			return err
		}
		if !samePolicyFile(working, before, change.Path) && !samePolicyFile(working, after, change.Path) {
			return fmt.Errorf("policy path changed during synchronization: %s", change.Path)
		}
		if !samePolicyFile(working, after, change.Path) {
			body, exists := after[change.Path]
			if exists {
				err = atomicPolicyFile(root, change.Path, body)
			} else {
				parent, openErr := policyParent(root, change.Path, false)
				if openErr != nil {
					return openErr
				}
				err = parent.Remove(path.Base(change.Path))
				err = errors.Join(err, parent.Close())
			}
			if err != nil {
				return fmt.Errorf("write %s: %w", change.Path, err)
			}
			out.Written = append(out.Written, change.Path)
		}
		out.Completed = append(out.Completed, change.Path)
	}
	return assertHead(ctx, m, head)
}

func atomicPolicyFile(root *os.Root, name string, body []byte) (err error) {
	parent, err := policyParent(root, name, true)
	if err != nil {
		return err
	}
	defer closePolicyRoot(parent)
	tmp := ".memory-policy-" + rand.Text() + ".tmp"
	f, err := parent.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		removeErr := parent.Remove(tmp)
		if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove policy temporary file: %w", removeErr))
		}
	}()
	_, writeErr := f.Write(body)
	if writeErr == nil {
		writeErr = f.Chmod(0644)
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	return parent.Rename(tmp, path.Base(name))
}

// Open every directory component without accepting redirects, keeping its
// descriptor pinned across the write even if another process renames a parent.
func policyParent(root *os.Root, name string, create bool) (*os.Root, error) {
	if name != reconcile.PolicyPath && !reconcile.PolicyMemberPath(name) {
		return nil, fmt.Errorf("unowned policy path %s", name)
	}
	parts := []string{".github"}
	if name != reconcile.PolicyPath {
		parts = append(parts, path.Base(reconcile.PolicyDirectory))
	}
	parent := root
	for _, part := range parts {
		info, err := parent.Lstat(part)
		if errors.Is(err, fs.ErrNotExist) && create {
			err = parent.Mkdir(part, 0755)
			if err == nil || errors.Is(err, fs.ErrExist) {
				info, err = parent.Lstat(part)
			}
		}
		if err != nil {
			if parent != root {
				err = errors.Join(err, parent.Close())
			}
			return nil, err
		}
		if !info.IsDir() {
			if parent != root {
				defer closePolicyRoot(parent)
			}
			return nil, fmt.Errorf("policy root %s is not a regular directory", part)
		}
		next, err := parent.OpenRoot(part)
		if parent != root {
			err = errors.Join(err, parent.Close())
		}
		if err != nil {
			if next != nil {
				closePolicyRoot(next)
			}
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			closePolicyRoot(next)
			return nil, fmt.Errorf("policy root moved while opening %s", part)
		}
		parent = next
	}
	return parent, nil
}

func workingPolicyFile(root *os.Root, name string) (map[string][]byte, error) {
	out := map[string][]byte{}
	parent, err := policyParent(root, name, false)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer closePolicyRoot(parent)
	info, err := parent.Lstat(path.Base(name))
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("policy member %s is not a regular file", name)
	}
	body, err := parent.ReadFile(path.Base(name))
	if err != nil {
		return nil, err
	}
	out[name] = body
	return out, nil
}

func writePolicySyncOutput(env Env, asJSON bool, out policySyncOutput) error {
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("write policy synchronization report: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintf(env.Stdout, "policy synchronization %s at %s: %d planned path(s), %d completed, %d written\n", out.Status, shortCommit(out.Head), len(out.Planned), len(out.Completed), len(out.Written)); err != nil {
		return err
	}
	for _, change := range out.Planned {
		if _, err := fmt.Fprintf(env.Stdout, "- %s: %s -> %s\n", change.Path, change.BeforeSHA256, change.AfterSHA256); err != nil {
			return err
		}
	}
	return nil
}

func closePolicyRoot(root *os.Root) {
	// A directory descriptor has no buffered output; closing it cannot affect
	// durability. File writes, syncs, renames, and file closes are checked above.
	_ = root.Close()
}
