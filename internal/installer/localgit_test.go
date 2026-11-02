package installer

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestLocalGitInitRefusesBeforeHostedRemotePreparation(t *testing.T) {
	opts := Options{Integrations: integrations.Config{SchemaVersion: integrations.SchemaVersion, Bindings: integrations.Bindings{CodeHost: integrations.Binding{Provider: integrations.LocalGit, Root: "/configured/origin.git"}, Tasks: integrations.Binding{Provider: integrations.Carina}}, CI: integrations.CI{Mode: "local"}}}
	if err := prepareIntegrations(&opts); !errors.Is(err, codehost.ErrWorkflowUnavailable) {
		t.Fatalf("incomplete local init admitted: %v", err)
	}
	if opts.AnswersProvided || opts.Answers.Coordinate != "" {
		t.Fatal("local init changed prospective answers before refusal")
	}
}
