// Package startup holds machine shell assets outside the managed-repo payload.
package startup

import "embed"

// Files is the same launcher source used by checkout-based shell setup.
//
//go:embed *.zsh
var Files embed.FS
