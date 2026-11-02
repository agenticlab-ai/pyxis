package boundary

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestVerifiedFeatureItemExtensionIsOneCanonicalPath(t *testing.T) {
	cfg := config.Config{WorkItems: ".workitems", Problems: "docs/problems", PRDs: "docs/prds", Brainstorms: "docs/brainstorms", Reviews: "docs/reviews", Wiki: "docs/wiki", TechDesigns: "docs/tech-design"}
	base := ResolveFirstPR(cfg, "acceptance-and-proposal")
	item := ".workitems/items/8.md"
	if base.Owns(item) {
		t.Fatal("first-PR scope admitted tracker item without ownership proof")
	}
	got := WithVerifiedFeatureItem(cfg, base, item)
	if !got.Owns(item) || base.Owns(item) {
		t.Fatal("verified item extension failed or mutated shared scope")
	}
	for _, other := range []string{".workitems/items/7.md", ".workitems/items/08.md", ".workitems/items/8.txt", ".workitems/index.md", "internal/engine/core.go"} {
		if got.Owns(other) || IsFeatureItemPath(cfg, other) && other != ".workitems/items/7.md" {
			t.Fatalf("extension admitted %s", other)
		}
	}
	for _, alias := range []string{".workitems/items/../items/8.md", "./.workitems/items/8.md"} {
		if IsFeatureItemPath(cfg, alias) {
			t.Fatalf("noncanonical item path accepted: %s", alias)
		}
	}
	if WithVerifiedFeatureItem(cfg, base, "internal/engine/core.go").Owns("internal/engine/core.go") {
		t.Fatal("non-item path widened scope")
	}
}
