package integrations

import (
	"fmt"
	"sort"
	"strings"
)

// Render writes the owned subtree with deterministic ordering and quoted
// scalars. Only environment references are serialized; Token is never read.
func (c Config) Render() ([]byte, error) {
	if c.SchemaVersion == "" {
		return nil, nil
	}
	var out strings.Builder
	out.WriteString("integrations:\n")
	line := func(indent int, key, value string) {
		if value != "" {
			fmt.Fprintf(&out, "%s%s: %q\n", strings.Repeat(" ", indent), key, value)
		}
	}
	line(2, "schema_version", c.SchemaVersion)
	if len(c.Connections) > 0 {
		out.WriteString("  connections:\n")
		names := make([]string, 0, len(c.Connections))
		for name := range c.Connections {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if !namePattern.MatchString(name) {
				return nil, invalid("invalid connection name")
			}
			conn := c.Connections[name]
			fmt.Fprintf(&out, "    %s:\n", name)
			line(6, "provider", conn.Provider)
			line(6, "base_url", conn.BaseURL)
			line(6, "token_env", conn.TokenEnv)
			line(6, "draft_prefix", conn.DraftPrefix)
		}
	}
	if c.Bindings != (Bindings{}) {
		out.WriteString("  bindings:\n")
		binding := func(indent int, name string, b Binding) {
			if b == (Binding{}) {
				return
			}
			fmt.Fprintf(&out, "%s%s:\n", strings.Repeat(" ", indent), name)
			line(indent+2, "connection", b.Connection)
			line(indent+2, "repository", b.Repository)
			line(indent+2, "provider", b.Provider)
			line(indent+2, "root", b.Root)
		}
		binding(4, "tasks", c.Bindings.Tasks)
		binding(4, "code_host", c.Bindings.CodeHost)
		if c.Bindings.Releases != (ReleaseBindings{}) {
			out.WriteString("    releases:\n")
			binding(6, "source", c.Bindings.Releases.Source)
			binding(6, "distribution", c.Bindings.Releases.Distribution)
		}
		binding(4, "artifacts", c.Bindings.Artifacts)
	}
	if c.CI != (CI{}) {
		out.WriteString("  ci:\n")
		line(4, "mode", c.CI.Mode)
		line(4, "runner_label", c.CI.RunnerLabel)
	}
	b := []byte(out.String())
	if _, err := Parse(b); err != nil {
		return nil, err
	}
	return b, nil
}
