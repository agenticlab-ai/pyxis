// Package configlayer loads the repository configuration together with its
// machine-local overlay. It is deliberately schema-agnostic: the owning
// consumer still closed-parses and validates its merged subtree.
package configlayer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	GlobalPath = ".pyxis/config.yaml"
	LocalPath  = ".pyxis/config.local.yaml"
)

// Load reads the tracked global configuration and overlays the ignored local
// configuration recursively. A key in the local document replaces the same
// scalar or sequence globally; mapping keys merge so a machine can override a
// single leaf without copying the rest of the repository policy.
func Load(root string) ([]byte, error) {
	global, err := readOptional(filepath.Join(root, GlobalPath))
	if err != nil {
		return nil, fmt.Errorf("read global config %s: %w", GlobalPath, err)
	}
	local, err := readOptional(filepath.Join(root, LocalPath))
	if err != nil {
		return nil, fmt.Errorf("read local config %s: %w", LocalPath, err)
	}
	if len(local) == 0 {
		return global, nil
	}
	base, err := parse(global)
	if err != nil {
		return nil, fmt.Errorf("parse global config %s: %w", GlobalPath, err)
	}
	override, err := parse(local)
	if err != nil {
		return nil, fmt.Errorf("parse local config %s: %w", LocalPath, err)
	}
	merge(base, override)
	return []byte(render(base)), nil
}

func readOptional(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

type node struct {
	key      string
	value    string
	children []*node
	raw      []string
}

func parse(data []byte) (*node, error) {
	root := &node{}
	stack := []*node{root}
	indents := []int{-2}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); {
		raw := strings.TrimRight(lines[i], " \t\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		indent, err := indentation(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("line %d: sequence has no mapping key", i+1)
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("line %d: expected mapping key", i+1)
		}
		for len(indents) > 1 && indent <= indents[len(indents)-1] {
			stack = stack[:len(stack)-1]
			indents = indents[:len(indents)-1]
		}
		if indent != indents[len(indents)-1]+2 {
			return nil, fmt.Errorf("line %d: indentation must use two-space steps", i+1)
		}
		n := &node{key: strings.TrimSpace(key), value: strings.TrimSpace(value)}
		parent := stack[len(stack)-1]
		for _, sibling := range parent.children {
			if sibling.key == n.key {
				return nil, fmt.Errorf("line %d: duplicate mapping key %q", i+1, n.key)
			}
		}
		parent.children = append(parent.children, n)
		i++
		if n.value != "" {
			continue
		}
		if i < len(lines) {
			nextIndex := i
			for nextIndex < len(lines) {
				next := strings.TrimSpace(lines[nextIndex])
				if next != "" && !strings.HasPrefix(next, "#") {
					break
				}
				nextIndex++
			}
			if nextIndex < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[nextIndex]), "-") {
				i = nextIndex
				for i < len(lines) {
					line := strings.TrimRight(lines[i], " \t\r")
					t := strings.TrimSpace(line)
					if t == "" || strings.HasPrefix(t, "#") {
						i++
						continue
					}
					in, ierr := indentation(line)
					if ierr != nil {
						return nil, fmt.Errorf("line %d: %w", i+1, ierr)
					}
					if in <= indent {
						break
					}
					n.raw = append(n.raw, strings.Repeat(" ", in-indent-2)+t)
					i++
				}
				continue
			}
		}
		stack = append(stack, n)
		indents = append(indents, indent)
	}
	return root, nil
}

func indentation(line string) (int, error) {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			return 0, errors.New("tabs are not allowed in indentation")
		default:
			if n%2 != 0 {
				return 0, errors.New("indentation must use two-space steps")
			}
			return n, nil
		}
	}
	return n, nil
}

func merge(dst, src *node) {
	for _, incoming := range src.children {
		var current *node
		for _, candidate := range dst.children {
			if candidate.key == incoming.key {
				current = candidate
				break
			}
		}
		if current == nil {
			dst.children = append(dst.children, incoming)
			continue
		}
		if current.value == "" && incoming.value == "" && len(current.raw) == 0 && len(incoming.raw) == 0 {
			merge(current, incoming)
			continue
		}
		*current = *incoming
	}
}

func render(root *node) string {
	var b strings.Builder
	for _, child := range root.children {
		renderNode(&b, child, 0)
	}
	return b.String()
}

func renderNode(b *strings.Builder, n *node, indent int) {
	b.WriteString(strings.Repeat(" ", indent))
	b.WriteString(n.key)
	b.WriteByte(':')
	if n.value != "" {
		b.WriteByte(' ')
		b.WriteString(n.value)
	}
	b.WriteByte('\n')
	for _, raw := range n.raw {
		b.WriteString(strings.Repeat(" ", indent+2))
		b.WriteString(raw)
		b.WriteByte('\n')
	}
	for _, child := range n.children {
		renderNode(b, child, indent+2)
	}
}
