package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestTrackerRefusesMalformedScopeBeforeReadingTasks(t *testing.T) {
	for _, provider := range []string{integrations.GitHub, integrations.Carina, integrations.Forgejo} {
		t.Run(provider, func(t *testing.T) {
			repo, binary, slug, label, kind := t.TempDir(), "unused-carina", "owner/repo", "agent", provider
			binding := integrations.ResolvedRepository{Provider: provider, Repository: slug, BaseURL: "https://forgejo.example.invalid"}
			var stderr bytes.Buffer
			got, code := tracker(t.Context(), binding, &repo, &binary, &stderr, "malformed-plan-ref", &slug, &label, nil, "github.com", &kind)
			if code != 2 || got != nil || !strings.Contains(stderr.String(), "plan") {
				t.Fatalf("tracker accepted malformed scope: %T, exit %d, %s", got, code, &stderr)
			}
		})
	}
}
