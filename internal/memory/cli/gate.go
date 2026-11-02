package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/rewriteauth"
)

// This is the same bounded historical replay used by the hosted ratchet. The
// reconciler still proves the exact trusted baseline and all its input hashes.
const initialBaselineReplay = "3ed3ffb1cf6e4a593bf9990178bb10afe88dce02"

// RunGate is the memory-gate entry point, shared by the desk and hosted CI.
// It composes memory's existing CLI predicates without changing their grammar.
func RunGate(ctx context.Context, args []string, env Env) int {
	code, err := runMemoryGate(ctx, args, env)
	if err != nil {
		if _, writeErr := fmt.Fprintf(env.Stderr, "memory-gate: %v\n", err); writeErr != nil {
			return exitRefused
		}
	}
	return code
}

// runMemoryGate composes existing predicates; it never generates a baseline,
// adds an allowance, fetches history, or weakens either constituent check.
func runMemoryGate(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("gate", env)
	baseFlag := fs.String("base", "", "trusted base ref (defaults to CI input or the local mainline)")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 0 {
		return exitUsage, usage("gate accepts only --base")
	}
	m, err := open(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	// The packet and reconciliation steps below open the memory again. They
	// reuse what this invocation has already read from pinned commits.
	env.pinned = m.pinned
	// Sources and the baseline are pinned to HEAD by reconcile. Refuse to
	// certify an older committed tree while edits or new files await a commit.
	dirty, err := m.repo.Dirty(ctx, ".")
	if err != nil {
		return exitRefused, err
	}
	if dirty {
		return exitRefused, fmt.Errorf("memory gate checks committed HEAD; commit the working-tree changes before running it")
	}
	reports, err := m.repo.ListTree(ctx, "HEAD", ".pyxis/evidence/memory")
	if err != nil {
		return exitRefused, fmt.Errorf("check tracked memory reports: %w", err)
	}
	if len(reports) > 0 {
		return exitRefused, fmt.Errorf("generated memory verification reports must not be tracked: %s; remove .pyxis/evidence/memory from Git", reports[0].Path)
	}
	base, inventoryOnly, err := memoryGateBase(ctx, m, *baseFlag, env)
	if err != nil {
		return exitRefused, err
	}
	if inventoryOnly {
		base, err = m.repo.Head(ctx)
		if err != nil {
			return exitRefused, err
		}
	}
	auth := rewriteauth.Verify(m.root, base, "HEAD")
	if handled, code, err := authorizeDirectRewrite(auth, env); handled {
		return code, err
	}
	evidence := m.evidenceCheck(ctx, base)
	if code, err := emitEvidence(env, evidence, false); code != exitOK || err != nil {
		return code, err
	}
	reconcileArgs := []string{"--check", "--baseline", reconciliationBaselinePath}
	if inventoryOnly {
		if _, err := fmt.Fprintln(env.Stdout, "memory gate: mainline inventory (no proposed change to compare)"); err != nil {
			return exitRefused, err
		}
	} else {
		if _, err := fmt.Fprintf(env.Stdout, "memory gate: trusted base %s\n", base); err != nil {
			return exitRefused, err
		}
		if code, err := runPacket(ctx, []string{"--base", base}, env, true); code != exitOK || err != nil {
			return code, err
		}
		reconcileArgs = append(reconcileArgs, "--baseline-ref", base, "--baseline-replay-ref", initialBaselineReplay)
	}
	return runReconcile(ctx, reconcileArgs, env)
}

func authorizeDirectRewrite(auth rewriteauth.Result, env Env) (bool, int, error) {
	if !auth.Active {
		return false, 0, nil
	}
	if auth.Err != nil {
		return true, exitRefused, fmt.Errorf("direct rewrite authorization: %w", auth.Err)
	}
	if !auth.AllowsMemoryGate() {
		return true, exitRefused, fmt.Errorf("direct rewrite authorization is incomplete")
	}
	if _, err := fmt.Fprintln(env.Stdout, "memory gate: exact operator-authorized direct rewrite"); err != nil {
		return true, exitRefused, err
	}
	return true, exitOK, nil
}

func memoryGateBase(ctx context.Context, m *memory, explicit string, env Env) (string, bool, error) {
	event, supplied := env.Getenv("PYXIS_CI_EVENT"), env.Getenv("PYXIS_CI_BASE")
	if event != "" {
		if explicit != "" {
			return "", false, usage("CI supplies the trusted base; --base cannot override it")
		}
		switch event {
		case "schedule":
			if supplied != "" {
				return "", false, usage("scheduled memory gate must not supply a base")
			}
			return "", true, nil
		case "pull_request", "push":
			if len(supplied) != 40 || strings.Trim(supplied, "0123456789abcdef") != "" || strings.Trim(supplied, "0") == "" {
				return "", false, usage("%s memory gate needs PYXIS_CI_BASE from the trusted event (a full commit SHA)", event)
			}
		default:
			return "", false, usage("unsupported memory gate CI event %q", event)
		}
	} else if env.Getenv("GITHUB_ACTIONS") == "true" {
		return "", false, usage("hosted memory gate needs PYXIS_CI_EVENT and PYXIS_CI_BASE")
	}
	ref := explicit
	if ref == "" {
		ref = supplied
	}
	defaulted := ref == ""
	if defaulted {
		ref = m.repo.DefaultBase(ctx, m.remote, m.branch)
		if ref == "" {
			return "", false, usage("no trusted mainline is available locally; fetch it or pass --base")
		}
	}
	base, err := m.repo.ResolveCommit(ctx, ref)
	if err != nil {
		return "", false, err
	}
	head, err := m.repo.Head(ctx)
	if err != nil {
		return "", false, err
	}
	// Only an implicit desk run on the mainline itself uses inventory mode.
	// Explicit and hosted self-comparisons still reach the strict refusal.
	return base, defaulted && base == head, nil
}
