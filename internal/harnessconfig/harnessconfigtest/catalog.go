// Package harnessconfigtest provides a fixed model catalog for behavior tests.
// The fixture is independent of the repository's changing operational config.
package harnessconfigtest

import (
	_ "embed"

	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
)

// YAML is an isolated test catalog, not a snapshot to keep in sync with defaults.
//
//go:embed catalog.yaml
var YAML string

// Config parses a fresh fixture catalog.
func Config() (harnessconfig.Config, error) {
	return harnessconfig.Parse([]byte(YAML))
}
