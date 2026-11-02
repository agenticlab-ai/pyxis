package precheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentSelectorPreservesContractReadinessAndRejectsInvalidInputs(t *testing.T) {
	root := filepath.Join("..", "plan", "testdata", "precheck-clean")
	if handled, defects := RunDocument(root, "demo"); !handled || len(defects) != 0 {
		t.Fatalf("existing contract not routed: handled=%v defects=%v", handled, defects)
	}
	for _, state := range []string{"draft", "invalid slug", "invalid config", "unreadable ledger"} {
		t.Run(state, func(t *testing.T) {
			root, feature := t.TempDir(), "widget"
			switch state {
			case "invalid slug":
				feature = "INVALID"
			case "invalid config":
				write(t, root, ".pyxis/config.yaml", []byte("[invalid YAML"))
			case "unreadable ledger":
				ledger := filepath.Join(root, "docs/prds/widget/000-base.md")
				if err := os.MkdirAll(filepath.Dir(ledger), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("000-base.md", ledger); err != nil {
					t.Fatal(err)
				}
			}
			handled, defects := RunDocument(root, feature)
			if state == "draft" {
				if handled || len(defects) != 0 {
					t.Fatal("a draft without a ledger was routed as a document contract")
				}
			} else if !handled || len(defects) == 0 {
				t.Fatalf("invalid input reached the origin route: handled=%v defects=%v", handled, defects)
			}
		})
	}
}
