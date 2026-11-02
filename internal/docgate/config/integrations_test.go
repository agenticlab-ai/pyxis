package config_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestIntegrationsForeignBlockCoexists(t *testing.T) {
	_, defects := config.Parse([]byte(`integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
  bindings:
    tasks:
      connection: local
      repository: owner/repo
paths:
  prds: docs/prds
`))
	if len(defects) != 0 {
		t.Fatalf("integrations consumer block rejected: %v", defects)
	}
}
