package admission

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
	"github.com/agenticlab-ai/pyxis/internal/statedir"
)

const sourceRef = "refs/pyxis/maintenance-source"

type ControlSource struct {
	Repository    string
	GitCommon     string
	RemoteURL     string
	ResolveRemote func(context.Context, string) (string, error)
}

type control struct {
	repository string
	source     ExecutionSource
	owned      bool
}

func (s ControlSource) Capture(ctx context.Context, source jobs.SourceBinding) (Published, error) {
	remoteURL := s.RemoteURL
	if s.ResolveRemote != nil {
		var err error
		remoteURL, err = s.ResolveRemote(ctx, source.Remote)
		if err != nil {
			return Published{}, fmt.Errorf("resolve installed source remote: %w", err)
		}
	}
	if !filepath.IsAbs(s.Repository) || !filepath.IsAbs(s.GitCommon) || remoteURL == "" || strings.HasPrefix(remoteURL, "-") || strings.ContainsAny(remoteURL, "\x00\r\n") {
		return Published{}, errors.New("maintenance control storage requires canonical repository, Git common directory, and remote URL")
	}
	remoteURL = enrolledRemoteURL(s.Repository, remoteURL)
	base, err := statedir.Sub("maintenance", "control")
	if err != nil {
		return Published{}, err
	}
	base = filepath.Clean(base)
	if overlaps(base, s.Repository) || overlaps(base, s.GitCommon) {
		return Published{}, errors.New("maintenance control storage must be outside the enrolled checkout and Git common directory")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return Published{}, fmt.Errorf("create maintenance control parent: %w", err)
	}
	base, err = filepath.EvalSymlinks(base)
	common, commonErr := filepath.EvalSymlinks(s.GitCommon)
	info, infoErr := os.Stat(base)
	if err != nil || commonErr != nil || infoErr != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 || overlaps(base, s.Repository) || overlaps(base, common) {
		return Published{}, errors.Join(errors.New("maintenance control storage must be private and outside the enrolled checkout and Git common directory"), err, commonErr, infoErr)
	}
	directory, err := os.MkdirTemp(base, "admission-")
	if err != nil {
		return Published{}, fmt.Errorf("create private maintenance control repository: %w", err)
	}
	c := &control{repository: directory, source: ExecutionSource{ControlRepository: directory,
		EnrolledRepository: s.Repository, EnrolledGitCommon: s.GitCommon}, owned: true}
	failed := true
	defer func() {
		if failed {
			_ = c.Close() // Preserve the capture error; only this invocation's fresh directory can be removed.
		}
	}()
	if err := c.git(ctx, "init", "--bare", "--quiet", directory); err != nil {
		return Published{}, fmt.Errorf("initialize maintenance control repository: %w", err)
	}
	if err := c.git(ctx, "-C", directory, "config", "remote.installed.url", remoteURL); err != nil {
		return Published{}, fmt.Errorf("bind installed maintenance remote: %w", err)
	}
	store, err := gitrefstore.New(directory, "installed", sourceRef)
	if err != nil {
		return Published{}, err
	}
	commit, err := store.FetchBranch(ctx, source.Branch)
	if err != nil {
		return Published{}, err
	}
	files, err := store.ReadExact(ctx, commit, ".pyxis/maintenance/jobs.yaml", ".pyxis/config.yaml")
	if err != nil {
		return Published{}, fmt.Errorf("read published maintenance policy at %s: %w", commit, err)
	}
	failed = false
	return Published{Commit: commit, Jobs: files[".pyxis/maintenance/jobs.yaml"], Config: files[".pyxis/config.yaml"], Control: c}, nil
}

// enrolledRemoteURL anchors only Git's plain relative filesystem transport.
// URLs, remote helpers, scp syntax, absolute paths, and home-relative syntax
// retain their original spelling and Git-defined handling.
func enrolledRemoteURL(repository, remoteURL string) string {
	if filepath.IsAbs(remoteURL) || strings.HasPrefix(remoteURL, "~") {
		return remoteURL
	}
	if colon := strings.IndexByte(remoteURL, ':'); colon >= 0 && !strings.Contains(remoteURL[:colon], string(filepath.Separator)) {
		return remoteURL
	}
	// Preserve path components: symlinks must resolve before a following "..".
	return strings.TrimRight(repository, string(filepath.Separator)) + string(filepath.Separator) + remoteURL
}

func (c *control) RefStore(ref string) (*gitrefstore.Store, error) {
	if c == nil || !c.owned {
		return nil, errors.New("maintenance control repository is closed")
	}
	return gitrefstore.New(c.repository, "installed", ref)
}

func (c *control) ExecutionSource() (ExecutionSource, error) {
	if c == nil || !c.owned {
		return ExecutionSource{}, errors.New("maintenance control repository is closed")
	}
	return c.source, nil
}

func (c *control) Close() error {
	if c == nil || !c.owned {
		return nil
	}
	c.owned = false
	return os.RemoveAll(c.repository)
}

func (c *control) git(ctx context.Context, args ...string) error {
	command := exec.CommandContext(ctx, "git", args...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "GIT_") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git control operation: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func overlaps(path, protected string) bool {
	rel, err := filepath.Rel(protected, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
