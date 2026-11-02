// Package localgit implements read-only code-host diagnostics on one local
// bare origin. It has no hosted API or network transport.
package localgit

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// RunGit is a process seam, not a code-host semantic interface.
type RunGit func(context.Context, string, ...string) ([]byte, error)

type Options struct {
	Root     string
	Origin   string
	Remote   string
	Mainline string
	Run      RunGit
}

type Diagnostics struct{ options Options }

func NewDiagnostics(options Options) *Diagnostics { return &Diagnostics{options: options} }

var _ codehost.Diagnostics = (*Diagnostics)(nil)

func (d *Diagnostics) Inspect(ctx context.Context) (readiness codehost.Readiness, err error) {
	defer func() {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
	}()
	if err := ctx.Err(); err != nil {
		return codehost.Readiness{}, err
	}
	o := d.options
	if o.Root == "" || o.Origin == "" || o.Remote == "" || strings.HasPrefix(o.Remote, "-") || strings.ContainsAny(o.Remote, "\r\n\x00") || o.Mainline == "" {
		return codehost.Readiness{}, originError("root, origin, remote, and mainline must be explicitly resolved from repository configuration")
	}
	run := o.Run
	if run == nil {
		run = runGit
	}
	git := func(dir string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return run(ctx, dir, append([]string{"-c", "protocol.allow=never"}, args...)...)
	}
	mainline := "refs/heads/" + o.Mainline
	if _, err := git(o.Root, "check-ref-format", mainline); err != nil {
		return codehost.Readiness{}, originError("invalid configured mainline %q: %v", o.Mainline, err)
	}
	inside, err := git(o.Root, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(inside)) != "true" {
		return codehost.Readiness{}, originError("operand %q is not a Git working repository", o.Root)
	}
	origin, err := localPath(o.Root, o.Origin)
	if err != nil {
		return codehost.Readiness{}, originError("configured bare origin %q: %v", o.Origin, err)
	}
	for _, direction := range []string{"fetch", "push"} {
		args := []string{"remote", "get-url", "--all"}
		if direction == "push" {
			args = append(args, "--push")
		}
		out, err := git(o.Root, append(args, o.Remote)...)
		if err != nil {
			return codehost.Readiness{}, originError("configured remote %q has no readable %s URL; configure it to %q: %v", o.Remote, direction, origin, err)
		}
		urls := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(urls) != 1 || urls[0] == "" {
			return codehost.Readiness{}, originError("remote %q needs exactly one local %s URL matching %q", o.Remote, direction, origin)
		}
		path, err := localPath(o.Root, urls[0])
		if err != nil {
			return codehost.Readiness{}, originError("remote %q %s URL is not a readable local origin: %v", o.Remote, direction, err)
		}
		if path != origin {
			return codehost.Readiness{}, originError("remote %q %s origin %q differs from configured origin %q; correct bindings.code_host.root or the remote before retrying", o.Remote, direction, path, origin)
		}
	}
	bare, err := git(origin, "rev-parse", "--is-bare-repository")
	if err != nil || strings.TrimSpace(string(bare)) != "true" {
		return codehost.Readiness{}, originError("configured origin %q is not a bare Git repository; point bindings.code_host.root and remote %q at an existing bare origin", origin, o.Remote)
	}
	out, err := git(origin, "for-each-ref", "--format=%(refname) %(objectname) %(objecttype)", mainline)
	if err != nil {
		return codehost.Readiness{}, originError("read configured mainline %s: %v", mainline, err)
	}
	r := codehost.Readiness{
		SchemaVersion: "code-host-readiness/v1", Provider: integrations.LocalGit,
		Origin: origin, Remote: o.Remote, MainlineRef: mainline,
		ProposalRemedy: "local draft preparation, operator review recording, and operator lock landing are available; implementation execution and watch integration remain pending",
		Trust:          codehost.Trust{SharedAccount: true},
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return codehost.Readiness{}, originError("malformed configured mainline response")
		}
		if fields[0] != mainline {
			continue
		}
		if fields[2] != "commit" || !objectID(fields[1]) {
			return codehost.Readiness{}, originError("configured mainline %s does not name a full commit OID", mainline)
		}
		r.MainlineOID = fields[1]
	}
	if r.MainlineOID == "" {
		r.MainlineRemedy = fmt.Sprintf("complete the separate first configured-mainline push to remote %q branch %q through the pinned first-push gate, then retry; draft preparation cannot initialize mainline", o.Remote, o.Mainline)
	}
	return r, nil
}

func localPath(root, value string) (string, error) {
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("origin must be a local filesystem path")
	}
	if strings.HasPrefix(value, "file:") {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "file" || u.Opaque != "" || u.User != nil || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" || !filepath.IsAbs(u.Path) || strings.ContainsAny(u.Path, "\r\n\x00") {
			return "", fmt.Errorf("file origin must name an absolute path on this machine")
		}
		value = u.Path
	} else if colon := strings.IndexByte(value, ':'); colon >= 0 && !filepath.IsAbs(value) && !strings.Contains(value[:colon], string(filepath.Separator)) {
		// Git treats a colon before the first slash as scp-style syntax.
		// Colons inside explicit local paths are ordinary filename characters.
		return "", fmt.Errorf("hosted and SSH origins are not permitted in local-git mode")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func objectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func originError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", codehost.ErrOrigin, fmt.Sprintf(format, args...))
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Repository selection must come from the explicit operand/origin paths.
	// Process-global Git overrides could otherwise make -C/Dir probe another repo.
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS":
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	return cmd.Output()
}
