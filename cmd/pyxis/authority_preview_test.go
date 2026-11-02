package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

func TestInitPreviewNamesOnceOnlyFloorAuthoritySeed(t *testing.T) {
	var out bytes.Buffer
	printInitReport(&out, installer.Report{Preview: true, WouldSeed: []installer.SeededDoc{{Path: payload.AuthorityContractTarget, Source: "template:authority-contract-template.md"}}})
	for _, want := range []string{payload.AuthorityContractTarget, "create once at floor", "existing policy refuses", "nothing was performed"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("preview omitted %q: %s", want, out.String())
		}
	}
}
