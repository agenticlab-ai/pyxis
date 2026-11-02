package gitstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ArtifactReader exact-reads proof bytes from one named mainline object. It
// never consults the worktree and accepts only a clean repository-relative
// path, so contributor-controlled Git options cannot turn a proof ID into a
// different object or repository.
type ArtifactReader struct {
	repo string
}

func NewArtifactReader(repo string) (*ArtifactReader, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, errors.New("routing activation artifact reader: repository path is required")
	}
	return &ArtifactReader{repo: repo}, nil
}

func (r *ArtifactReader) ReadActivationArtifact(ctx context.Context, object, path string) ([]byte, error) {
	object, path = strings.TrimSpace(object), filepath.ToSlash(strings.TrimSpace(path))
	if r == nil || r.repo == "" || !lowerHexObject(object) || !safeArtifactPath(path) {
		return nil, errors.New("routing activation artifact reader: invalid object or path")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", r.repo, "show", object+":"+path)
	cmd.Env = safeGitEnvironment()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("routing activation artifact reader: git show %s: %w", path, err)
	}
	return out, nil
}

// ReadRemoteRef observes one exact remote branch object at the release seam.
// A signed statement about current mainline is not self-authenticating: the
// release gate must compare it with the code host's current ref before it can
// append an activation checkpoint.
func ReadRemoteRef(ctx context.Context, repo, remote, ref string) (string, error) {
	repo, remote, ref = strings.TrimSpace(repo), strings.TrimSpace(remote), strings.TrimSpace(ref)
	if repo == "" || remote == "" || strings.HasPrefix(remote, "-") || !strings.HasPrefix(ref, "refs/heads/") ||
		strings.ContainsAny(remote+ref, "\x00\r\n \t") {
		return "", errors.New("routing activation remote reader: invalid repository, remote, or branch ref")
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "ls-remote", "--exit-code", remote, ref)
	cmd.Env = safeGitEnvironment()
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("routing activation remote reader: git ls-remote: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	fields := strings.Fields(stdout.String())
	if len(fields) != 2 || fields[1] != ref || !lowerHexObject(fields[0]) {
		return "", errors.New("routing activation remote reader: remote returned a non-exact branch result")
	}
	return fields[0], nil
}

func lowerHexObject(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func safeArtifactPath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != filepath.FromSlash(path) || strings.Contains(path, "\\") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func safeGitEnvironment() []string {
	// The proof object and repository are command arguments, but ambient Git
	// redirection can still replace config, object storage, or executables.
	// Preserve ordinary process state while stripping only steering variables.
	var out []string
	for _, entry := range environ() {
		name := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			name = entry[:i]
		}
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_CONFIG_PARAMETERS", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR":
			continue
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

var environ = os.Environ
