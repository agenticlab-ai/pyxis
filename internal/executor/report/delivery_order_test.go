package report

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDurablePublicationOrderSurvivesRestartAndBlocksOvertaking(t *testing.T) {
	for _, dependent := range []bool{false, true} {
		t.Run(map[bool]string{false: "independent-sibling", true: "dependent-sibling"}[dependent], func(t *testing.T) {
			kinds := []Kind{KindTaskImplementationCompleted, KindTaskCompleted}
			a := &lifecycleHandler{id: "a", kinds: kinds, failures: 2}
			b := &lifecycleHandler{id: "b", kinds: kinds}
			if dependent {
				b.after = []string{"a"}
			}
			path := filepath.Join(t.TempDir(), "ledger.json")
			r, err := NewDurableReporter("run", path, nil, []Handler{a, b}, WithClock(fixedClock()))
			if err != nil {
				t.Fatal(err)
			}
			first := TaskImplementationCompleted("z-implementation", "T1", 1, "plan", "rev", "#1")
			second := TaskCompleted("a-delivery", "T1", 1, "plan", "rev")
			for _, e := range []Event{first, second} {
				if err := r.Publish(t.Context(), e); err == nil {
					t.Fatal("failed earlier transition did not surface")
				}
			}
			if !reflect.DeepEqual(a.calls, []string{first.ID, first.ID}) {
				t.Fatalf("later transition overtook failed delivery: %v", a.calls)
			}
			if dependent && len(b.calls) != 0 || !dependent && !reflect.DeepEqual(b.calls, []string{first.ID, second.ID}) {
				t.Fatalf("sibling dependency semantics changed: %v", b.calls)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			r, err = NewDurableReporter("next-run", path, nil, []Handler{a, b})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := r.Resume(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.calls, []string{first.ID, first.ID, first.ID, second.ID}) || !reflect.DeepEqual(b.calls, []string{first.ID, second.ID}) {
				t.Fatalf("restart delivery order a=%v b=%v", a.calls, b.calls)
			}
			third := TaskCompleted("0-next-task", "T2", 2, "plan", "rev")
			if err := r.Publish(t.Context(), third); err != nil || r.durable.ledger.Records[third.ID].Sequence != 3 {
				t.Fatalf("restart lost sequence high watermark: %v", err)
			}
			outcomes := r.HandlerOutcomes()
			if len(outcomes) != 6 || outcomes[0].EventID != third.ID {
				t.Fatalf("cold outcomes not stably ordered: %+v", outcomes)
			}
			r.durable.sequence = ^uint64(0)
			third.ID = "overflow"
			if err := r.Publish(t.Context(), third); err == nil {
				t.Fatal("sequence wrapped and could reorder events")
			}
		})
	}
}

func TestDurableLedgerOwnershipAcrossReportersAndProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.json")
	h := &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}, failures: 1}
	r, err := NewDurableReporter("owner", path, nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev")); err == nil {
		t.Fatal("missing pending handler failure")
	}
	if _, err := NewDurableReporter("other", path, nil, []Handler{h}); !errors.Is(err, ErrDeliveryLedgerInUse) {
		t.Fatalf("second reporter acquired same ledger: %v", err)
	}
	alias := filepath.Join(dir, "alias.json")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableReporter("alias", alias, nil, []Handler{h}); !errors.Is(err, ErrDeliveryLedgerInUse) {
		t.Fatalf("symlink bypassed ledger ownership: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestDurableDeliveryProcessHelper$")
	cmd.Env = append(os.Environ(), "PYXIS_REPORT_HELPER=locked", "PYXIS_REPORT_LEDGER_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross-process ownership check: %v\n%s", err, out)
	}
	failure := errors.New("disk unavailable")
	r.durable.dirty = true
	r.durable.save = func() error { return failure }
	if err := r.Close(); !errors.Is(err, failure) {
		t.Fatalf("close discarded unsaved receipts: %v", err)
	}
	if _, err := NewDurableReporter("other", path, nil, []Handler{h}); !errors.Is(err, ErrDeliveryLedgerInUse) {
		t.Fatalf("failed close released ownership: %v", err)
	}
	r.durable.save = r.durable.persist
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if err := r.Publish(t.Context(), Event{}); !errors.Is(err, ErrReporterClosed) {
		t.Fatalf("closed reporter published: %v", err)
	}
	if err := r.Resume(t.Context()); !errors.Is(err, ErrReporterClosed) {
		t.Fatalf("closed reporter resumed: %v", err)
	}
	recovered, err := NewDurableReporter("next-owner", path, nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := recovered.Resume(t.Context()); err != nil || len(h.calls) != 2 {
		t.Fatalf("ownership handoff lost pending event: calls=%v err=%v", h.calls, err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	if err := NewReporter("plain").Close(); err != nil {
		t.Fatalf("plain reporter close: %v", err)
	}
}

func TestDurableOwnershipSetupFailures(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableReporter("run", filepath.Join(file, "ledger.json"), nil, nil); err == nil {
		t.Fatal("invalid lock parent accepted")
	}
	path := filepath.Join(dir, "ledger.json")
	if err := os.Mkdir(path+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableReporter("run", path, nil, nil); err == nil {
		t.Fatal("unavailable lock file accepted")
	}
	cycle := filepath.Join(dir, "cycle")
	if err := os.Symlink(cycle, cycle); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableReporter("run", cycle, nil, nil); err == nil {
		t.Fatal("unresolvable ledger alias accepted")
	}
}
