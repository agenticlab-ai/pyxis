package integrations

import (
	"slices"
	"strconv"
	"strings"
)

type configNode struct {
	value    string
	children map[string]*configNode
}

// Parse validates only the integrations subtree. Other top-level consumers own
// their blocks. This deliberately supports the repository's two-space mapping
// subset, and refuses aliases, flow collections, duplicate keys and unknowns.
func Parse(data []byte) (Config, error) {
	c := Config{Connections: map[string]Connection{}}
	root, present, err := subtree(data)
	if err != nil || !present {
		return c, err
	}
	if err := keys(root, "schema_version", "connections", "bindings", "ci"); err != nil {
		return c, err
	}
	c.SchemaVersion, err = scalar(root, "schema_version")
	if err != nil {
		return c, err
	}
	if c.SchemaVersion != SchemaVersion {
		return c, invalid("schema_version must be %s", SchemaVersion)
	}
	if conns := root.children["connections"]; conns != nil {
		if conns.value != "" {
			return c, invalid("connections must be a mapping")
		}
		for name, n := range conns.children {
			if !namePattern.MatchString(name) {
				return c, invalid("invalid connection name")
			}
			if err := keys(n, "provider", "base_url", "token_env", "draft_prefix"); err != nil {
				return c, err
			}
			var conn Connection
			for key, dst := range map[string]*string{"provider": &conn.Provider, "base_url": &conn.BaseURL, "token_env": &conn.TokenEnv, "draft_prefix": &conn.DraftPrefix} {
				*dst, err = scalar(n, key)
				if err != nil {
					return c, err
				}
			}
			conn, err = normalizeConnection(conn)
			if err != nil {
				return c, err
			}
			c.Connections[name] = conn
		}
	}
	if bindings := root.children["bindings"]; bindings != nil {
		if err := keys(bindings, "tasks", "code_host", "releases", "artifacts"); err != nil {
			return c, err
		}
		for key, dst := range map[string]*Binding{"tasks": &c.Bindings.Tasks, "code_host": &c.Bindings.CodeHost, "artifacts": &c.Bindings.Artifacts} {
			if n := bindings.children[key]; n != nil {
				*dst, err = parseBinding(n)
				if err != nil {
					return c, err
				}
				r, e := c.Resolve(*dst)
				if e != nil {
					return c, e
				}
				if key != "tasks" && r.Provider == Carina {
					return c, invalid("%s cannot use carina", key)
				}
				if key != "code_host" && r.Provider == LocalGit {
					return c, invalid("%s cannot use local-git; it is only a code_host binding", key)
				}
			}
		}
		if rel := bindings.children["releases"]; rel != nil {
			if err := keys(rel, "source", "distribution"); err != nil {
				return c, err
			}
			for key, dst := range map[string]*Binding{"source": &c.Bindings.Releases.Source, "distribution": &c.Bindings.Releases.Distribution} {
				if n := rel.children[key]; n != nil {
					*dst, err = parseBinding(n)
					if err != nil {
						return c, err
					}
					r, e := c.Resolve(*dst)
					if e != nil {
						return c, e
					}
					if r.Provider == Carina || r.Provider == LocalGit {
						return c, invalid("release binding requires github or forgejo")
					}
				}
			}
		}
	}
	if ci := root.children["ci"]; ci != nil {
		if err := keys(ci, "mode", "runner_label"); err != nil {
			return c, err
		}
		c.CI.Mode, err = scalar(ci, "mode")
		if err != nil {
			return c, err
		}
		c.CI.RunnerLabel, err = scalar(ci, "runner_label")
		if err != nil {
			return c, err
		}
		if c.CI.Mode != "" && c.CI.Mode != "local" && c.CI.Mode != "actions" && c.CI.Mode != "both" {
			return c, invalid("ci.mode must be local, actions, or both")
		}
		if strings.ContainsAny(c.CI.RunnerLabel, "\r\n\x00") {
			return c, invalid("runner_label must be a single line")
		}
	}
	return c, nil
}

func parseBinding(n *configNode) (Binding, error) {
	var b Binding
	if err := keys(n, "connection", "repository", "provider", "root"); err != nil {
		return b, err
	}
	for key, dst := range map[string]*string{"connection": &b.Connection, "repository": &b.Repository, "provider": &b.Provider, "root": &b.Root} {
		v, err := scalar(n, key)
		if err != nil {
			return b, err
		}
		*dst = v
	}
	if b == (Binding{}) {
		return b, invalid("binding must name a connection or provider")
	}
	return b, nil
}

func keys(n *configNode, allowed ...string) error {
	if n.value != "" {
		return invalid("expected a mapping")
	}
	for key := range n.children {
		if !slices.Contains(allowed, key) {
			return invalid("unknown key %q", key)
		}
	}
	return nil
}

func scalar(n *configNode, key string) (string, error) {
	v := n.children[key]
	if v == nil {
		return "", nil
	}
	if len(v.children) != 0 || v.value == "" {
		return "", invalid("%s must be a non-empty scalar", key)
	}
	return v.value, nil
}

func subtree(data []byte) (*configNode, bool, error) {
	root := &configNode{children: map[string]*configNode{}}
	stack := []*configNode{root}
	present, inside := false, false
	for i, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if indent == 0 {
			inside = false
			key, val, ok := strings.Cut(trim, ":")
			if !ok || strings.TrimSpace(key) != "integrations" {
				continue
			}
			if present {
				return nil, false, invalid("duplicate integrations block")
			}
			if strings.TrimSpace(stripComment(val)) != "" {
				return nil, false, invalid("integrations must be a mapping")
			}
			present, inside = true, true
			continue
		}
		if !inside {
			continue
		}
		if strings.Contains(raw[:indent], "\t") || indent%2 != 0 {
			return nil, false, invalid("line %d: use two-space indentation", i+1)
		}
		level := indent / 2
		if level > len(stack) {
			return nil, false, invalid("line %d: unexpected indentation", i+1)
		}
		stack = stack[:level]
		parent := stack[level-1]
		if parent.value != "" {
			return nil, false, invalid("line %d: scalar cannot contain children", i+1)
		}
		key, val, ok := strings.Cut(trim, ":")
		key = strings.TrimSpace(key)
		if !ok || !namePattern.MatchString(key) {
			return nil, false, invalid("line %d: expected a mapping key", i+1)
		}
		if _, ok := parent.children[key]; ok {
			return nil, false, invalid("line %d: duplicate key %q", i+1, key)
		}
		value, err := parseScalar(strings.TrimSpace(stripComment(val)))
		if err != nil {
			return nil, false, invalid("line %d: malformed scalar", i+1)
		}
		n := &configNode{value: value, children: map[string]*configNode{}}
		parent.children[key] = n
		stack = append(stack, n)
	}
	return root, present, nil
}

func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote == '"' && ch == '\\' {
			i++
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
		}
		if ch == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}
	return s
}

func parseScalar(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if s[0] == '"' {
		return strconv.Unquote(s)
	}
	if s[0] == '\'' {
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return "", ErrInvalid
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'"), nil
	}
	if strings.ContainsAny(s, "{}[]\n\r\x00") || strings.HasPrefix(s, "&") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, "!") || strings.HasPrefix(s, "|") || strings.HasPrefix(s, ">") || s == "null" || s == "~" {
		return "", ErrInvalid
	}
	return s, nil
}
