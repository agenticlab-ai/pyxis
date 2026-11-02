package codegatemirror

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// PrefetchMainline fetches the configured mainline so history-sensitive
// checks can see objects that landed while this workspace was idle. It never
// merges. A missing remote, a failed fetch, or a mainline that is not an
// ancestor of HEAD is reported to stderr and does not fail the gate — the
// checks still decide. CI already has a fetched checkout; a no-op fetch is
// cheap. The remote/branch pair comes from the pyxis config surface
// (config.MainlineRef), never a baked literal.
func PrefetchMainline(ctx context.Context, workspace string, stderr io.Writer) {
	if gitOne(ctx, workspace, "remote") == "" {
		warnPrefetch(stderr, "code-gate: skip mainline fetch: no remotes\n")
		return
	}
	cfg, _ := config.Load(workspace)
	remote, branch, ref := cfg.Remote, cfg.Branch, cfg.MainlineRef()
	if upstream := strings.TrimSpace(gitOne(ctx, workspace, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")); upstream != "" {
		if parts := strings.SplitN(upstream, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			remote, branch, ref = parts[0], parts[1], parts[0]+"/"+parts[1]
		}
	}
	if remote == "" || branch == "" {
		warnPrefetch(stderr, "code-gate: skip mainline fetch: empty configured remote or branch\n")
		return
	}
	if out, err := gitCmd(ctx, workspace, nil, "fetch", "--no-tags", "--quiet", remote, branch); err != nil {
		warnPrefetch(stderr, "code-gate: mainline fetch failed (continuing): %v\n%s", err, strings.TrimSpace(string(out)))
		return
	}
	if _, err := gitCmd(ctx, workspace, nil, "merge-base", "--is-ancestor", ref, "HEAD"); err != nil {
		warnPrefetch(stderr, "code-gate: %s has commits not in HEAD; merge before the long gate to avoid a stale-base miss\n", ref)
	}
}

// warnPrefetch is advisory only; a closed stderr must not fail the gate.
func warnPrefetch(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, format, args...)
}

func gitOne(ctx context.Context, workspace string, args ...string) string {
	out, err := gitCmd(ctx, workspace, nil, args...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
