package precheck

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
)

// latestVersion reads the remote lock namespace, including the peeled lines
// emitted for annotated tags. A gap is an invalid history, not a new version.
func latestVersion(ctx context.Context, root, remote, feature string, run lifecycle.RunCommand) (int, error) {
	if run == nil || remote == "" || !featureName.MatchString(feature) {
		return 0, fmt.Errorf("list binding locks: process adapter, remote, and feature are required")
	}
	prefix := "refs/tags/lock/" + feature + "/v"
	out, err := run(ctx, root, nil, "git", "ls-remote", "--tags", remote, prefix+"*")
	if err != nil {
		return 0, fmt.Errorf("list binding locks for %s: %w", feature, err)
	}
	versions := map[int]bool{}
	maxVersion := 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("malformed remote lock line %q", line)
		}
		ref := strings.TrimSuffix(fields[1], "^{}")
		raw, ok := strings.CutPrefix(ref, prefix)
		if !ok {
			return 0, fmt.Errorf("unexpected lock ref %q", ref)
		}
		version, err := strconv.Atoi(raw)
		if err != nil || version < 1 {
			return 0, fmt.Errorf("invalid lock version in %q", ref)
		}
		versions[version] = true
		maxVersion = max(maxVersion, version)
	}
	for version := 1; version <= maxVersion; version++ {
		if !versions[version] {
			return 0, fmt.Errorf("binding locks for %s skip version %d", feature, version)
		}
	}
	return maxVersion, nil
}
