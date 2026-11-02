package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// RunCommand is the consumer-side port for Git and code-host operations.
// Adapters must keep credentials out of argv and returned errors.
type RunCommand func(context.Context, string, []string, string, ...string) ([]byte, error)

// Inspect reads current PRD ledgers, remote lock tags, and open pull requests.
// The caller supplies an authenticated code-host command runner when needed.
func Inspect(ctx context.Context, root string, cfg config.Config, run RunCommand) (Snapshot, error) {
	if run == nil {
		return Snapshot{}, fmt.Errorf("draft process adapter is required")
	}
	s := Snapshot{OpenDrafts: map[string]bool{}, LockedVersions: map[string]int{}, PRDLedgers: map[string]bool{}}
	entries, err := os.ReadDir(filepath.Join(root, cfg.PRDs))
	if err != nil && !os.IsNotExist(err) {
		return s, fmt.Errorf("read PRD ledgers: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, cfg.PRDs, entry.Name(), "000-base.md")); err == nil {
			s.PRDLedgers[entry.Name()] = true
		} else if !os.IsNotExist(err) {
			return s, fmt.Errorf("inspect PRD ledger %s: %w", entry.Name(), err)
		}
	}
	tags, err := run(ctx, root, nil, "git", "ls-remote", "--tags", cfg.Remote, "refs/tags/lock/*/v*")
	if err != nil {
		return s, fmt.Errorf("list remote lock tags: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(tags)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return s, fmt.Errorf("malformed remote lock tag line %q", line)
		}
		tag := strings.TrimPrefix(strings.TrimSuffix(fields[1], "^{}"), "refs/tags/")
		feature, raw, ok := strings.Cut(strings.TrimPrefix(tag, "lock/"), "/v")
		if !ok || !validFeature(feature) {
			continue
		}
		version, err := strconv.Atoi(raw)
		if err == nil && version > s.LockedVersions[feature] {
			s.LockedVersions[feature] = version
		}
	}
	prs, err := run(ctx, root, nil, "gh", "api", "--paginate", "repos/{owner}/{repo}/pulls?state=open&per_page=100", "--jq", ".[] | select(.head.repo.full_name == .base.repo.full_name) | .head.ref")
	if err != nil {
		return s, fmt.Errorf("list open draft pull requests: %w", err)
	}
	for branch := range strings.SplitSeq(strings.TrimSpace(string(prs)), "\n") {
		feature, ok := draftFeature(branch, cfg.Draft.BranchPrefix, s.LockedVersions)
		if ok {
			s.OpenDrafts[feature] = true
		}
	}
	return s, nil
}

func draftFeature(branch, prefix string, locked map[string]int) (string, bool) {
	feature := strings.TrimPrefix(branch, prefix)
	if feature == branch || !validFeature(feature) {
		return "", false
	}
	if i := strings.LastIndex(feature, "-v"); i > 0 && validFeature(feature[:i]) {
		base, rawVersion := feature[:i], feature[i+2:]
		if version, err := strconv.Atoi(rawVersion); err == nil && version > 1 && locked[base] == version-1 {
			return base, true
		}
	}
	return feature, true
}

func validFeature(feature string) bool {
	if feature == "" || feature[0] < 'a' || feature[0] > 'z' {
		return false
	}
	for _, r := range feature[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}
