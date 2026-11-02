package payload

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func WorkflowForRoot(root string, profiles ...*integrations.Config) (source, target string, err error) {
	var c integrations.Config
	if len(profiles) > 0 && profiles[0] != nil {
		c = *profiles[0]
	} else {
		c, err = integrations.Load(root)
	}
	if err != nil {
		return "", "", err
	}
	if c.Bindings.CodeHost != (integrations.Binding{}) {
		h, err := c.CodeHost("")
		if err != nil {
			return "", "", err
		}
		if h.Provider == integrations.Forgejo {
			return ForgejoWorkflow, ForgejoWorkflowTarget, nil
		}
	}
	return InstalledWorkflow, InstalledWorkflowTarget, nil
}

// unsafeWorkflowValue reports a rendered value that could leave the quoted YAML
// scalar a workflow template holds it in.
func unsafeWorkflowValue(v string) bool {
	return strings.ContainsAny(v, "'\"\r\n")
}

var actionsExpression = regexp.MustCompile(`\$\{\{[^{}\r\n]+\}\}`)

// RenderForgejoWorkflow preserves expressions authored in the trusted Actions
// template while the ordinary closed substitution rejects remnants in values.
func RenderForgejoWorkflow(template []byte, values map[string]string) ([]byte, error) {
	var expressions []string
	masked := actionsExpression.ReplaceAllStringFunc(string(template), func(s string) string {
		expressions = append(expressions, s)
		return fmt.Sprintf("PYXIS_ACTION_EXPRESSION_%d_END", len(expressions)-1)
	})
	for key, v := range values {
		if unsafeWorkflowValue(v) || strings.Contains(v, "PYXIS_ACTION_EXPRESSION_") {
			return nil, fmt.Errorf("workflow: unsafe value for %s", key)
		}
	}
	out, err := Substitute([]byte(masked), values)
	if err != nil {
		return nil, err
	}
	s := string(out)
	for i, e := range expressions {
		s = strings.ReplaceAll(s, fmt.Sprintf("PYXIS_ACTION_EXPRESSION_%d_END", i), e)
	}
	return []byte(s), nil
}
