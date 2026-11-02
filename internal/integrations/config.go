// Package integrations owns capability bindings on the shared Pyxis config.
// Connections identify servers and environment credential references; bindings
// select a repository independently for each consumer capability.
package integrations

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
)

const (
	GitHub        = "github"
	Forgejo       = "forgejo"
	Carina        = "carina"
	LocalGit      = "local-git"
	SchemaVersion = "pyxis-integrations/v1"
)

var ErrInvalid = errors.New("invalid integrations configuration")

type Connection struct {
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url"`
	TokenEnv    string `json:"token_env"`
	DraftPrefix string `json:"draft_prefix"`
}

type Binding struct {
	Connection string `json:"connection,omitempty"`
	Repository string `json:"repository,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Root       string `json:"root,omitempty"` // Carina operand root or configured local Git bare origin
}

type ReleaseBindings struct {
	Source       Binding `json:"source"`
	Distribution Binding `json:"distribution"`
}

type Bindings struct {
	Tasks     Binding         `json:"tasks"`
	CodeHost  Binding         `json:"code_host"`
	Releases  ReleaseBindings `json:"releases"`
	Artifacts Binding         `json:"artifacts"`
}

type CI struct {
	Mode        string `json:"mode,omitempty"`
	RunnerLabel string `json:"runner_label,omitempty"`
}

type Config struct {
	SchemaVersion string                `json:"schema_version,omitempty"`
	Connections   map[string]Connection `json:"connections,omitempty"`
	Bindings      Bindings              `json:"bindings"`
	CI            CI                    `json:"ci"`
	root          string                // resolution context; never serialized into the portable profile
}

type ResolvedRepository struct {
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url,omitempty"`
	Repository  string `json:"repository,omitempty"`
	TokenEnv    string `json:"token_env,omitempty"`
	DraftPrefix string `json:"draft_prefix,omitempty"`
	Root        string `json:"root,omitempty"`
}

// Token obtains the credential only at use time; it is never a config value.
func (r ResolvedRepository) Token() string { return os.Getenv(r.TokenEnv) }

// Key includes the server origin (including port and path), so equal native
// repository and item numbers on different servers cannot share an identity.
func (r ResolvedRepository) Key() string {
	if r.Provider == Carina || r.Provider == LocalGit {
		return r.Provider + ":" + filepath.Clean(r.Root)
	}
	return r.Provider + ":" + strings.TrimRight(r.BaseURL, "/") + ":" + r.Repository
}

func Load(root string) (Config, error) {
	b, err := configlayer.Load(root)
	if err != nil {
		return Config{}, fmt.Errorf("load integrations: %w", err)
	}
	c, err := Parse(b)
	if err != nil {
		return Config{}, err
	}
	c.root, err = filepath.Abs(root)
	if err != nil {
		return Config{}, fmt.Errorf("load integrations root: %w", err)
	}
	return c, nil
}

func (c Config) Resolve(b Binding) (ResolvedRepository, error) {
	if b.Provider == LocalGit {
		if b.Connection != "" || b.Repository != "" {
			return ResolvedRepository{}, invalid("local-git binding names a bare origin root, not a connection or hosted repository")
		}
		if strings.TrimSpace(b.Root) == "" || strings.ContainsAny(b.Root, "\r\n\x00") || strings.Contains(b.Root, "://") {
			return ResolvedRepository{}, invalid("local-git binding requires root as a filesystem path to its bare origin")
		}
		root := filepath.Clean(b.Root)
		if !filepath.IsAbs(root) && c.root != "" {
			root = filepath.Join(c.root, root)
		}
		return ResolvedRepository{Provider: LocalGit, Root: root}, nil
	}
	if b.Provider == Carina {
		if b.Connection != "" || b.Repository != "" {
			return ResolvedRepository{}, invalid("carina binding cannot name a connection or repository")
		}
		root := b.Root
		if root != "" && !filepath.IsAbs(root) && c.root != "" {
			root = filepath.Join(c.root, root)
		}
		return ResolvedRepository{Provider: Carina, Root: root}, nil
	}
	if b.Root != "" {
		return ResolvedRepository{}, invalid("root is only valid for carina or local-git")
	}
	conn := Connection{Provider: b.Provider}
	if b.Connection != "" {
		var ok bool
		conn, ok = c.Connections[b.Connection]
		if !ok {
			return ResolvedRepository{}, invalid("binding refers to unknown connection %q", b.Connection)
		}
		if b.Provider != "" && b.Provider != conn.Provider {
			return ResolvedRepository{}, invalid("binding provider conflicts with connection %q", b.Connection)
		}
	}
	if conn.Provider == "" {
		return ResolvedRepository{}, invalid("binding requires a connection or provider")
	}
	conn, err := normalizeConnection(conn)
	if err != nil {
		return ResolvedRepository{}, err
	}
	if err := ValidateRepository(b.Repository); err != nil {
		return ResolvedRepository{}, err
	}
	return ResolvedRepository{Provider: conn.Provider, BaseURL: conn.BaseURL, Repository: b.Repository, TokenEnv: conn.TokenEnv, DraftPrefix: conn.DraftPrefix}, nil
}

func (c Config) CodeHost(legacyRepo string) (ResolvedRepository, error) {
	b := c.Bindings.CodeHost
	if b == (Binding{}) {
		b = Binding{Provider: GitHub, Repository: legacyRepo}
	}
	r, err := c.Resolve(b)
	if err == nil && r.Provider == Carina {
		return ResolvedRepository{}, invalid("code_host requires github, forgejo, or local-git")
	}
	return r, err
}

func (c Config) Tasks(legacyRepo, legacyTracker, carinaRoot string) (ResolvedRepository, error) {
	b := c.Bindings.Tasks
	if b == (Binding{}) {
		switch legacyTracker {
		case "", "gh", GitHub:
			b = Binding{Provider: GitHub, Repository: legacyRepo}
		case Carina:
			b = Binding{Provider: Carina, Root: carinaRoot}
		default:
			return ResolvedRepository{}, invalid("unsupported legacy tracker %q; configure a task connection", legacyTracker)
		}
	}
	r, err := c.Resolve(b)
	if err == nil && r.Provider == LocalGit {
		return ResolvedRepository{}, invalid("local-git is only valid for the code_host binding")
	}
	if err == nil && r.Provider == Carina && r.Root == "" {
		r.Root = carinaRoot
	}
	return r, err
}

func (c Config) Artifacts() (ResolvedRepository, bool, error) {
	if c.Bindings.Artifacts == (Binding{}) {
		return ResolvedRepository{}, false, nil
	}
	r, err := c.Resolve(c.Bindings.Artifacts)
	if err == nil && (r.Provider == Carina || r.Provider == LocalGit) {
		err = invalid("artifacts requires github or forgejo")
	}
	return r, true, err
}

func (c Config) Release(target, legacyRepo string) (ResolvedRepository, error) {
	var b Binding
	switch target {
	case "", "source":
		b = c.Bindings.Releases.Source
	case "distribution":
		b = c.Bindings.Releases.Distribution
	default:
		return ResolvedRepository{}, invalid("unknown release target %q", target)
	}
	if b == (Binding{}) {
		b = Binding{Provider: GitHub, Repository: legacyRepo}
	}
	r, err := c.Resolve(b)
	if err == nil && (r.Provider == Carina || r.Provider == LocalGit) {
		return ResolvedRepository{}, invalid("releases requires github or forgejo")
	}
	return r, err
}

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var repoPartPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

func ValidateRepository(repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || !repoPartPattern.MatchString(parts[0]) || !repoPartPattern.MatchString(parts[1]) || parts[0] == "." || parts[1] == "." || parts[0] == ".." || parts[1] == ".." {
		return invalid("repository must be an owner/name coordinate")
	}
	return nil
}

func normalizeConnection(c Connection) (Connection, error) {
	switch c.Provider {
	case GitHub:
		if c.BaseURL == "" {
			c.BaseURL = "https://github.com"
		}
	case Forgejo:
		if c.BaseURL == "" {
			return c, invalid("forgejo connection requires base_url")
		}
		if c.TokenEnv == "" {
			c.TokenEnv = "PYXIS_FORGEJO_TOKEN"
		}
		if c.DraftPrefix == "" {
			c.DraftPrefix = "WIP:"
		}
	default:
		return c, invalid("connection provider must be github or forgejo")
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return c, invalid("base_url must be an HTTP(S) server URL without credentials, query, or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "http" && host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback()) {
		return c, invalid("plain HTTP base_url is restricted to loopback hosts")
	}
	if u.Path != "" && path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/") && u.Path != "/" {
		return c, invalid("base_url path must not contain traversal or redundant segments")
	}
	if strings.ContainsAny(u.Path, "\\\x00\r\n") || strings.Contains(u.EscapedPath(), "%2f") || strings.Contains(u.EscapedPath(), "%2F") {
		return c, invalid("base_url has an unsafe path")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	c.BaseURL = strings.TrimRight(u.String(), "/")
	if c.Provider == GitHub && c.BaseURL != "https://github.com" {
		return c, invalid("GitHub connections currently require https://github.com; use forgejo for a self-hosted Forgejo server")
	}
	if c.TokenEnv != "" && !envPattern.MatchString(c.TokenEnv) {
		return c, invalid("token_env must name an environment variable")
	}
	if strings.ContainsAny(c.DraftPrefix, "\r\n\x00") {
		return c, invalid("draft_prefix must be a single line")
	}
	return c, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
