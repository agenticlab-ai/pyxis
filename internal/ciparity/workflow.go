// Package ciparity enforces the code-gate-mirror required-set invariant over
// the source repository's complete hosted workflow surface (DINV-1).
package ciparity

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// A real YAML decoder is necessary here: line searches miss flow mappings,
// quoted keys and nested jobs. The pinned YAML-org parser is the only new
// dependency; aliases/merge keys and unsupported workflow features fail closed.
type workflow struct {
	Name        string            `yaml:"name"`
	On          yaml.Node         `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Env         map[string]string `yaml:"env"`
	Jobs        map[string]job    `yaml:"jobs"`
}

type job struct {
	Name            string            `yaml:"name"`
	Needs           []string          `yaml:"needs"`
	RunsOn          string            `yaml:"runs-on"`
	Timeout         int               `yaml:"timeout-minutes"`
	If              string            `yaml:"if"`
	ContinueOnError bool              `yaml:"continue-on-error"`
	Permissions     map[string]string `yaml:"permissions"`
	Env             map[string]string `yaml:"env"`
	Steps           []step            `yaml:"steps"`
}

type step struct {
	Name            string            `yaml:"name"`
	ID              string            `yaml:"id"`
	Uses            string            `yaml:"uses"`
	Run             string            `yaml:"run"`
	If              string            `yaml:"if"`
	ContinueOnError bool              `yaml:"continue-on-error"`
	Shell           string            `yaml:"shell"`
	Directory       string            `yaml:"working-directory"`
	Timeout         int               `yaml:"timeout-minutes"`
	With            map[string]string `yaml:"with"`
	Env             map[string]string `yaml:"env"`
}

func decodeWorkflow(data []byte) (workflow, error) {
	var tree yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tree); err != nil {
		return workflow{}, fmt.Errorf("decode workflow: %w", err)
	}
	if err := validateYAML(&tree); err != nil {
		return workflow{}, err
	}
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return workflow{}, fmt.Errorf("workflow must contain exactly one YAML document")
	}
	var wf workflow
	dec = yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&wf); err != nil {
		return workflow{}, fmt.Errorf("unsupported workflow structure: %w", err)
	}
	if len(wf.Jobs) == 0 {
		return workflow{}, fmt.Errorf("workflow has no jobs")
	}
	return wf, nil
}

func validateYAML(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("line %d: YAML anchors and aliases are not supported by CI parity", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Tag != "!!str" || seen[key.Value] || key.Value == "<<" {
				return fmt.Errorf("line %d: duplicate, non-string, or merged mapping key %q", key.Line, key.Value)
			}
			seen[key.Value] = true
			if key.Value == "continue-on-error" && (value.Tag != "!!bool" || (value.Value != "true" && value.Value != "false")) {
				return fmt.Errorf("line %d: continue-on-error must be literal true or false", value.Line)
			}
		}
	}
	for _, child := range n.Content {
		if err := validateYAML(child); err != nil {
			return err
		}
	}
	return nil
}

// verificationEvents includes branch pushes and schedules as well as PRs.
// Tag-only release workflows and manual-only experiments do not gate changes.
// fullPR requires an unfiltered PR trigger: a paths-filtered job cannot stand
// in for the complete gate on every proposal.
func verificationEvents(on yaml.Node) (verify, fullPR bool, err error) {
	visit := func(name string, filter *yaml.Node) error {
		switch name {
		case "pull_request":
			verify = true
			fullPR = filter == nil || filter.Tag == "!!null" || (filter.Kind == yaml.MappingNode && len(filter.Content) == 0)
		case "pull_request_target", "merge_group", "schedule", "workflow_call":
			verify = true
		case "push":
			tagsOnly := false
			if filter != nil && filter.Kind == yaml.MappingNode {
				for i := 0; i < len(filter.Content); i += 2 {
					if filter.Content[i].Value == "tags" {
						tagsOnly = true
					}
					if filter.Content[i].Value == "branches" || filter.Content[i].Value == "branches-ignore" {
						verify = true
					}
				}
			}
			verify = verify || !tagsOnly
		case "workflow_dispatch", "release":
		default:
			return fmt.Errorf("unclassified workflow event %q", name)
		}
		return nil
	}
	switch on.Kind {
	case yaml.ScalarNode:
		err = visit(on.Value, nil)
	case yaml.SequenceNode:
		for _, event := range on.Content {
			if event.Kind != yaml.ScalarNode {
				return false, false, fmt.Errorf("workflow event list must contain names")
			}
			if err = visit(event.Value, nil); err != nil {
				break
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(on.Content); i += 2 {
			if err = visit(on.Content[i].Value, on.Content[i+1]); err != nil {
				break
			}
		}
	default:
		err = fmt.Errorf("workflow needs an explicit event declaration")
	}
	return verify, fullPR, err
}
