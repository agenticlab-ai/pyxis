package gate_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/journal"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

func completeDeliveryFixture() string {
	valid := strings.Replace(histDelivery, "| **Stage position** | stage 1 |", "| **Stage position** | stage 1 |\n| **Open items** | none |\n| **Park state** | none |\n| **Cumulative spend** | unavailable |", 1)
	return strings.ReplaceAll(valid, "- **Body**:", "- **Principal**: Argus\n- **Authority-contract version**: authority-contract/v1\n- **Body**:")
}

func newDeliveryRepo(t *testing.T, ledger string) *histRepo {
	t.Helper()
	r := &histRepo{t: t, root: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("docs/problems/foo/delivery.md", ledger)
	r.git("add", "docs/problems/foo/delivery.md")
	r.git("commit", "-qm", "trusted baseline")
	return r
}

func TestChangedDeliveryCheckRefusesFreshUnknownAndSameProposalRepair(t *testing.T) {
	base := completeDeliveryFixture()
	fresh := base + "\n### E-3 — finding-disposition — review result\n\n- **Date**: 2026-10-08\n- **Principal**: Argus\n- **Authority-contract version**: authority-contract/v1\n- **Body**: Recorded a review disposition.\n"
	r := newDeliveryRepo(t, base)
	r.write("docs/problems/foo/delivery.md", fresh)
	c := gate.Ctx{Root: r.root, MergeBase: "HEAD", Cfg: config.Defaults()}
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 1 || !strings.Contains(defects[0].Reason, "new E-3") {
		t.Fatalf("fresh unsupported event was admitted: %v", defects)
	}
	// The pure/local writer cannot establish acceptance. Only the gate's Git
	// baseline can decide whether a target was already merged.
	meta := journal.Metadata{Date: "2026-10-08", Principal: "Argus", AuthorityVersion: "authority-contract/v1", Reason: "Correct the disposition type."}
	if _, err := journal.CorrectFile(context.Background(), r.root, c.Cfg, "foo", []journal.CorrectionRequest{{Target: "E-3", To: "reconciliation"}}, meta, false); err != nil {
		t.Fatal(err)
	}
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 1 || !strings.Contains(defects[0].Reason, "new E-3") {
		t.Fatalf("same-proposal correction laundered a fresh unknown type: %v", defects)
	}
	if check, ok := gate.Lookup("check-delivery-types"); !ok || !check.NeedsMergeBase || !check.RunOnPyxisSource {
		t.Fatal("normal umbrella or Source exemption omitted the early check")
	}
}

func TestChangedDeliveryCheckRecoversHistoricalDamageAndReadsPushedHead(t *testing.T) {
	base := strings.Replace(completeDeliveryFixture(), "stage-entered", "finding-disposition", 1)
	r := newDeliveryRepo(t, base)
	r.write("app.go", "package app\n")
	c := gate.Ctx{Root: r.root, MergeBase: "HEAD", Cfg: config.Defaults()}
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 0 {
		t.Fatalf("unchanged legacy ledger blocked unrelated app work: %v", defects)
	}
	meta := journal.Metadata{Date: "2026-10-08", Principal: "Argus", AuthorityVersion: "authority-contract/v1", Reason: "Correct historical classification."}
	if _, err := journal.CorrectFile(context.Background(), r.root, c.Cfg, "foo", []journal.CorrectionRequest{{Target: "E-1", To: "reconciliation"}}, meta, false); err != nil {
		t.Fatal(err)
	}
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 0 {
		t.Fatal(defects)
	}
	r.git("add", "docs/problems/foo/delivery.md")
	r.git("commit", "-qm", "append exact historical correction")
	c.MergeBase, c.Head = "HEAD~1", "HEAD"
	// A different session's bad working tree must not contaminate the head
	// being pushed. The desk check still sees and refuses that bad edit.
	r.write("docs/problems/foo/delivery.md", base)
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 0 {
		t.Fatalf("pushed head was replaced by working-tree content: %v", defects)
	}
	c.Head = ""
	if defects := gate.CheckDeliveryTypes(c); len(defects) != 0 {
		// The restored baseline is unchanged; the change-specific check skips it.
		t.Fatalf("unchanged legacy content must be skipped: %v", defects)
	}
	r.write("docs/problems/foo/delivery.md", base+"\n### E-3 — not-a-type — fresh mistake\n")
	if defects := gate.CheckDeliveryTypes(c); len(defects) == 0 {
		t.Fatal("desk check ignored working-tree unknown event")
	}
	if err := os.Remove(filepath.Join(r.root, "docs/problems/foo/delivery.md")); err != nil {
		t.Fatal(err)
	}
	if defects := gate.CheckDeliveryTypes(c); len(defects) == 0 {
		t.Fatal("removed delivery ledger silently passed")
	}
}

func TestDeliveryCheckFailsClosedOnUnreadableGit(t *testing.T) {
	for _, c := range []gate.Ctx{
		{Root: t.TempDir(), Cfg: config.Defaults()},
		{Root: t.TempDir(), MergeBase: "missing", Cfg: config.Defaults()},
	} {
		if defects := gate.CheckDeliveryTypes(c); len(defects) != 1 || defects[0].Op != "git" {
			t.Fatalf("unreadable Git passed: %v", defects)
		}
	}
}
