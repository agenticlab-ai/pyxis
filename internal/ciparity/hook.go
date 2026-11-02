package ciparity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkHook(ctx context.Context, root string) error {
	// Resolve the repository's .githooks directory symlink before asking Git
	// for its tracked target. Filesystem permissions alone miss index-only mode
	// changes and checkouts with core.fileMode=false.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve hook repository: %w", err)
	}
	hook, err := filepath.EvalSymlinks(filepath.Join(root, ".githooks/pre-push"))
	if err != nil {
		return fmt.Errorf("resolve local pre-push: %w", err)
	}
	rel, err := filepath.Rel(root, hook)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("local pre-push must resolve inside the repository")
	}
	info, err := os.Stat(hook)
	if err != nil {
		return fmt.Errorf("stat local pre-push: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("local pre-push must be an executable regular file")
	}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--stage", "-z", "--", filepath.ToSlash(rel))
	cmd.Dir = root
	tracked, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read tracked local pre-push mode: %w", err)
	}
	entry, path, ok := strings.Cut(strings.TrimSuffix(string(tracked), "\x00"), "\t")
	fields := strings.Fields(entry)
	if !ok || path != filepath.ToSlash(rel) || len(fields) != 3 || fields[0] != "100755" || fields[2] != "0" {
		return fmt.Errorf("local pre-push must be tracked with executable mode 100755 and no conflicts")
	}
	body, err := os.ReadFile(hook)
	if err != nil {
		return fmt.Errorf("read local pre-push: %w", err)
	}
	first, _, _ := strings.Cut(string(body), "\n")
	if first != "#!/bin/sh" {
		return fmt.Errorf("local pre-push must begin with the exact interpreter #!/bin/sh")
	}
	var commands []string
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}
	want := "set -e\nROOT=\"$(git rev-parse --show-toplevel)\"\n\"$ROOT/.pyxis/scripts/docgate-resolve.sh\" pre-push\nexec \"$ROOT/.pyxis/scripts/code-gate-resolve.sh\" run"
	if strings.Join(commands, "\n") != want {
		return fmt.Errorf("local pre-push must delegate exactly to docgate pre-push and the complete code-gate")
	}
	return nil
}
