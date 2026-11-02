package host

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// Explicit hosted bindings require durable full URLs. Bare references remain
// contextual only for legacy repositories without a binding; switching a
// provider must never silently reinterpret an existing await's native number.
func (p Prober) resolveBinding(ref string, task bool) (string, string, integrations.ResolvedRepository, error) {
	cfg, err := integrations.Load(p.Root)
	if err != nil {
		return "", "", integrations.ResolvedRepository{}, err
	}
	selected := cfg.Bindings.CodeHost
	if task {
		selected = cfg.Bindings.Tasks
	} else if selected.Provider == integrations.LocalGit {
		return "", "", integrations.ResolvedRepository{}, fmt.Errorf("%w: local Git wake proposal probes are not available in this build", codehost.ErrWorkflowUnavailable)
	}
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		return resolveURLBinding(cfg, selected, ref, task)
	}
	if selected != (integrations.Binding{}) {
		return "", "", integrations.ResolvedRepository{}, errors.New("wake: an explicit integration binding requires a full issue or pull-request URL in durable awaits; replace the contextual bare reference")
	}
	repo, num, err := p.resolve(ref)
	return repo, num, integrations.ResolvedRepository{}, err
}

func resolveURLBinding(cfg integrations.Config, selected integrations.Binding, ref string, task bool) (string, string, integrations.ResolvedRepository, error) {
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", integrations.ResolvedRepository{}, errors.New("wake: invalid full goal-object URL")
	}
	names := make([]string, 0, len(cfg.Connections))
	for name := range cfg.Connections {
		names = append(names, name)
	}
	// A provider-only binding is itself an explicit configured connection.
	if selected.Connection == "" && (selected.Provider == integrations.GitHub || selected.Provider == integrations.Forgejo) {
		names = append(names, "")
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == selected.Connection || names[j] == selected.Connection {
			return names[i] == selected.Connection
		}
		return names[i] < names[j]
	})
	var found integrations.ResolvedRepository
	var number string
	var foundName string
	for _, name := range names {
		candidate := integrations.Binding{Connection: name, Repository: "placeholder/repository"}
		if name == "" {
			candidate.Provider = selected.Provider
		}
		binding, resolveErr := cfg.Resolve(candidate)
		if resolveErr != nil {
			return "", "", found, resolveErr
		}
		base, parseErr := url.Parse(binding.BaseURL)
		if parseErr != nil || !strings.EqualFold(base.Host, u.Host) || base.Scheme != u.Scheme {
			continue
		}
		prefix := strings.TrimRight(base.Path, "/") + "/"
		if !strings.HasPrefix(u.Path, prefix) {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(u.Path, prefix), "/")
		kind := "pulls"
		if task {
			kind = "issues"
		} else if binding.Provider == integrations.GitHub {
			kind = "pull"
		}
		if len(parts) != 4 || parts[2] != kind {
			continue
		}
		repo := parts[0] + "/" + parts[1]
		n, numberErr := strconv.Atoi(parts[3])
		if integrations.ValidateRepository(repo) != nil || numberErr != nil || n <= 0 || strconv.Itoa(n) != parts[3] {
			return "", "", found, errors.New("wake: URL contains an invalid repository or item number")
		}
		binding.Repository = repo
		if found.Provider != "" && foundName != selected.Connection && name != selected.Connection {
			return "", "", found, errors.New("wake: full URL matches multiple connections; select its connection in the capability binding")
		}
		if found.Provider == "" || name == selected.Connection {
			found, number, foundName = binding, parts[3], name
		}
	}
	if found.Provider == "" {
		return "", "", found, fmt.Errorf("wake: full URL %q does not match a configured connection and object kind", ref)
	}
	return found.Repository, number, found, nil
}
