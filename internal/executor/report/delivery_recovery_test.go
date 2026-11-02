package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type callbackHandler struct {
	*lifecycleHandler
	fn func(context.Context, Event) error
}

func (h callbackHandler) Handle(ctx context.Context, e Event) error { return h.fn(ctx, e) }

func TestDurableAdmissionMustPersistBeforeAnyDelivery(t *testing.T) {
	for _, retry := range []string{"publish", "resume"} {
		t.Run(retry, func(t *testing.T) {
			var journal bytes.Buffer
			h := &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}}
			r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), []Sink{NewArgusSink(&journal)}, []Handler{h}, WithClock(fixedClock()))
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("disk unavailable")
			r.durable.save = func() error { return failure }
			e := TaskCompleted("event", "T1", 1, "plan", "rev")
			if err := r.Publish(t.Context(), e); !errors.Is(err, failure) {
				t.Fatalf("admission error = %v", err)
			}
			if err := r.Resume(t.Context()); !errors.Is(err, failure) {
				t.Fatalf("resume ignored unpersisted admission: %v", err)
			}
			if err := r.Publish(t.Context(), e); !errors.Is(err, failure) {
				t.Fatalf("duplicate bypassed persistence: %v", err)
			}
			if len(h.calls) != 0 || journal.Len() != 0 {
				t.Fatalf("unpersisted event delivered: calls=%v journal=%s", h.calls, &journal)
			}
			r.durable.save = r.durable.persist
			if retry == "publish" {
				err = r.Publish(t.Context(), e)
			} else {
				err = r.Resume(t.Context())
			}
			if err != nil || len(h.calls) != 1 {
				t.Fatalf("recovery: calls=%v err=%v", h.calls, err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			cold, err := NewDurableReporter("next-run", r.durable.path, nil, []Handler{h})
			if err != nil {
				t.Fatal(err)
			}
			if err := cold.Resume(t.Context()); err != nil || len(h.calls) != 1 {
				t.Fatalf("completed receipt not durable: calls=%v err=%v", h.calls, err)
			}
		})
	}
}

func TestDurableOutcomeSaveFailureBlocksDependentsWithoutRepeatingSuccess(t *testing.T) {
	for _, renamed := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-rename", true: "after-rename"}[renamed], func(t *testing.T) {
			a := &lifecycleHandler{id: "a", kinds: []Kind{KindTaskCompleted}}
			b := &lifecycleHandler{id: "b", kinds: a.kinds, after: []string{"a"}}
			r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{a, b})
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("receipt sync failed")
			saves := 0
			r.durable.save = func() error {
				saves++
				if saves == 1 || renamed {
					if err := r.durable.persist(); err != nil {
						return err
					}
				}
				if saves > 1 {
					return failure
				}
				return nil
			}
			if err := r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev")); !errors.Is(err, failure) {
				t.Fatalf("outcome error = %v", err)
			}
			if err := r.Resume(t.Context()); !errors.Is(err, failure) || len(a.calls) != 1 || len(b.calls) != 0 {
				t.Fatalf("failed receipt allowed work: a=%v b=%v error=%v", a.calls, b.calls, err)
			}
			r.durable.save = r.durable.persist
			if err := r.Resume(t.Context()); err != nil || len(a.calls) != 1 || len(b.calls) != 1 {
				t.Fatalf("receipt recovery: a=%v b=%v error=%v", a.calls, b.calls, err)
			}
		})
	}
}

func TestDurableCancellationStopsNewEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	a := callbackHandler{lifecycleHandler: &lifecycleHandler{id: "a", kinds: []Kind{KindTaskCompleted}}, fn: func(context.Context, Event) error { cancel(); return nil }}
	b := &lifecycleHandler{id: "b", kinds: a.kinds}
	r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{a, b})
	if err != nil {
		t.Fatal(err)
	}
	e := TaskCompleted("event", "T1", 1, "plan", "rev")
	if err := r.Publish(ctx, e); !errors.Is(err, context.Canceled) || len(b.calls) != 0 {
		t.Fatalf("cancellation launched sibling: calls=%v err=%v", b.calls, err)
	}
	e.ID = "canceled-event"
	if err := r.Publish(ctx, e); !errors.Is(err, context.Canceled) || len(r.HandlerOutcomes()) != 2 {
		t.Fatalf("canceled publication changed ledger: %v", err)
	}
	if err := r.Resume(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resume = %v", err)
	}
	if err := r.Resume(t.Context()); err != nil || len(b.calls) != 1 {
		t.Fatalf("fresh context failed recovery: calls=%v err=%v", b.calls, err)
	}
}

func TestDurableProgressDoesNotRetryLifecycleEffects(t *testing.T) {
	h := &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}, failures: 1}
	var journal bytes.Buffer
	r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), []Sink{NewArgusSink(&journal)}, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev")); err == nil {
		t.Fatal("missing handler failure")
	}
	if err := r.Publish(t.Context(), CIFixAttempt("T2", 1, 3)); err != nil || len(h.calls) != 1 {
		t.Fatalf("progress retried lifecycle work: calls=%v err=%v", h.calls, err)
	}
	if len(journalEvents(t, &journal)) != 2 {
		t.Fatal("ordinary progress was not reported")
	}
	if err := r.Resume(t.Context()); err != nil || len(h.calls) != 2 {
		t.Fatalf("explicit recovery: calls=%v err=%v", h.calls, err)
	}
}

func TestDurableIdentityBindsPayloadAndCopiesCounts(t *testing.T) {
	h := callbackHandler{lifecycleHandler: &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}}, fn: func(_ context.Context, e Event) error { e.Counts.Succeeded = 99; return nil }}
	r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{h}, WithClock(fixedClock()))
	if err != nil {
		t.Fatal(err)
	}
	e := TaskCompleted("event", "T1", 1, "plan", "rev")
	e.Counts = &Counts{Succeeded: 1}
	if err := r.Publish(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	if e.Counts.Succeeded != 1 {
		t.Fatal("handler mutated producer payload")
	}
	e.Time, e.RunID = fixedClock()().Add(time.Hour), "retry-run"
	if err := r.Publish(t.Context(), e); err != nil {
		t.Fatalf("transport correlation changed identity: %v", err)
	}
	e.Reason = "different meaning"
	if err := r.Publish(t.Context(), e); err == nil {
		t.Fatal("accepted same identity with different handler-visible payload")
	}
	e.Reason = ""
	e.Counts.Succeeded = 2
	if err := r.Publish(t.Context(), e); err == nil {
		t.Fatal("producer mutation rewrote ledger payload")
	}
	e.ID, e.Time = "invalid-time", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := r.Publish(t.Context(), e); err == nil || len(r.HandlerOutcomes()) != 1 {
		t.Fatalf("unserializable event poisoned ledger: %v", err)
	}
}

func TestDurableRegistrationRejectsInvalidInputs(t *testing.T) {
	var typedNil *lifecycleHandler
	for _, h := range []Handler{nil, typedNil, &lifecycleHandler{}, &lifecycleHandler{id: "a"}, &lifecycleHandler{id: "a", kinds: []Kind{""}}} {
		if _, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{h}); err == nil {
			t.Fatalf("invalid handler accepted: %#v", h)
		}
	}
	if _, err := NewDurableReporter("run", " \t", nil, nil); err == nil {
		t.Fatal("empty path accepted")
	}
	plain := NewReporter("run")
	if err := plain.Resume(t.Context()); err != nil || plain.HandlerOutcomes() != nil {
		t.Fatalf("plain reporter recovery: %v", err)
	}
}

func TestDurableLoadFailsClosedOnCorruptState(t *testing.T) {
	valid := `{"schema":"executor-lifecycle-delivery/v1","records":{"e":{"sequence":1,"event":{"id":"e","kind":"task.completed","time":"2026-09-10T00:00:00Z"},"handlers":{"h":{"event_id":"e","event_kind":"task.completed","handler_id":"h","status":"pending","attempts":0,"updated_at":"2026-09-10T00:00:00Z"}}}}}`
	for _, tc := range []struct{ name, body string }{
		{"invalid-json", "{"},
		{"unknown-field", strings.Replace(valid, `"schema":`, `"unknown":1,"schema":`, 1)},
		{"trailing-json", valid + " {}"},
		{"wrong-schema", strings.Replace(valid, "delivery/v1", "delivery/v2", 1)},
		{"null-records", `{"schema":"executor-lifecycle-delivery/v1","records":null}`},
		{"wrong-event-id", strings.Replace(valid, `"id":"e"`, `"id":"other"`, 1)},
		{"missing-sequence", strings.Replace(valid, `"sequence":1,`, "", 1)},
		{"wrong-outcome-event", strings.Replace(valid, `"event_id":"e"`, `"event_id":"other"`, 1)},
		{"wrong-handler-id", strings.Replace(valid, `"handler_id":"h"`, `"handler_id":"other"`, 1)},
		{"wrong-event-kind", strings.Replace(valid, `"event_kind":"task.completed"`, `"event_kind":"plan.completed"`, 1)},
		{"unknown-status", strings.Replace(valid, `"pending"`, `"lost"`, 1)},
		{"negative-attempts", strings.Replace(valid, `"attempts":0`, `"attempts":-1`, 1)},
		{"unattempted-completion", strings.Replace(valid, `"pending"`, `"completed"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ledger.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			h := &lifecycleHandler{id: "h", kinds: []Kind{KindTaskCompleted}}
			if _, err := NewDurableReporter("run", path, nil, []Handler{h}); err == nil {
				t.Fatal("corrupt ledger accepted")
			}
		})
	}
}

func TestDurableRestartValidatesPendingHandlerRegistration(t *testing.T) {
	h := &lifecycleHandler{id: "h", kinds: []Kind{KindTaskCompleted}, failures: 1}
	path := filepath.Join(t.TempDir(), "ledger.json")
	r, err := NewDurableReporter("run", path, nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev")); err == nil {
		t.Fatal("missing failure")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDurableReporter("run", path, nil, nil); err == nil {
		t.Fatal("pending handler removed silently")
	}
	h.kinds = []Kind{KindPlanCompleted}
	if _, err := NewDurableReporter("run", path, nil, []Handler{h}); err == nil {
		t.Fatal("pending handler subscription changed silently")
	}
	h.kinds, h.after = []Kind{KindTaskCompleted}, []string{"dep"}
	dep := &lifecycleHandler{id: "dep", kinds: h.kinds}
	if _, err := NewDurableReporter("run", path, nil, []Handler{h, dep}); err == nil {
		t.Fatal("pending handler gained unrecorded dependency")
	}
	h.after = nil
	r, err = NewDurableReporter("run", path, nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Resume(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if cold, err := NewDurableReporter("run", path, nil, nil); err != nil || len(cold.HandlerOutcomes()) != 1 {
		t.Fatalf("completed historical handler cannot be removed: %v", err)
	}
}

type failingDeliveryFile struct {
	writeErr, syncErr, closeErr error
	short, synced, closed       bool
}

func (f *failingDeliveryFile) Write(b []byte) (int, error) {
	if f.short {
		return 0, nil
	}
	return len(b), f.writeErr
}
func (f *failingDeliveryFile) Sync() error  { f.synced = true; return f.syncErr }
func (f *failingDeliveryFile) Close() error { f.closed = true; return f.closeErr }

func TestDeliveryWritePropagatesEveryFailure(t *testing.T) {
	failure := errors.New("disk failure")
	for _, tc := range []struct {
		name string
		file failingDeliveryFile
		want error
	}{
		{"write", failingDeliveryFile{writeErr: failure}, failure},
		{"short-write", failingDeliveryFile{short: true}, io.ErrShortWrite},
		{"sync", failingDeliveryFile{syncErr: failure}, failure},
		{"close", failingDeliveryFile{closeErr: failure}, failure},
		{"success", failingDeliveryFile{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := writeDeliveryFile(&tc.file, []byte("ledger")); !errors.Is(err, tc.want) || !tc.file.closed {
				t.Fatalf("write = %v, closed=%v", err, tc.file.closed)
			}
			if (tc.file.writeErr != nil || tc.file.short) && tc.file.synced {
				t.Fatal("failed write was synced as if successful")
			}
		})
	}
}

func TestDeliveryFilesystemFailures(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewDurableReporter("run", dir, nil, nil); err == nil {
		t.Fatal("directory loaded as ledger")
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("blocker"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &durableHandlers{path: filepath.Join(blocker, "ledger.json"), ledger: deliveryLedger{Schema: deliverySchema, Records: map[string]deliveryRecord{}}}
	if err := d.persist(); err == nil {
		t.Fatal("invalid parent accepted")
	}
	d.path = dir
	if err := d.persist(); err == nil {
		t.Fatal("ledger replaced directory")
	}
	files, err := filepath.Glob(filepath.Join(dir, ".lifecycle-delivery-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("failed save leaked temporary files: %v %v", files, err)
	}
	d.ledger.Records["invalid"] = deliveryRecord{Event: Event{Time: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}}
	if err := d.persist(); err == nil {
		t.Fatal("invalid ledger serialized")
	}
}

func TestDurableEmitSurfacesErrors(t *testing.T) {
	for _, observe := range []bool{true, false} {
		t.Run(map[bool]string{true: "observer", false: "default-log"}[observe], func(t *testing.T) {
			var out bytes.Buffer
			prior := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&out, nil)))
			t.Cleanup(func() { slog.SetDefault(prior) })
			h := &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}, failures: 1}
			var observed error
			var opts []Option
			if observe {
				opts = append(opts, WithPublicationErrorHandler(func(_ Event, err error) { observed = err }))
			}
			r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{h}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			r.Emit(TaskCompleted("event", "T1", 1, "plan", "rev"))
			if observe && observed == nil || !observe && !strings.Contains(out.String(), "temporary outage") {
				t.Fatalf("Emit silently lost error: observer=%v log=%s", observed, &out)
			}
		})
	}
}

func TestDurableConcurrentPublicationDeduplicates(t *testing.T) {
	h := &lifecycleHandler{id: "effect", kinds: []Kind{KindTaskCompleted}}
	r, err := NewDurableReporter("run", filepath.Join(t.TempDir(), "ledger.json"), nil, []Handler{h})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if err := r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev")); err != nil {
				t.Error(err)
			}
			_ = r.HandlerOutcomes() // Exercise concurrent observation under the race detector.
		})
	}
	wg.Wait()
	if len(h.calls) != 1 {
		t.Fatalf("concurrent duplicate repeated effect: %v", h.calls)
	}
}

func TestDurableDeliveryRecoversAfterProcessCrash(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, phase := range []string{"crash", "recover"} {
		cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestDurableDeliveryProcessHelper$")
		cmd.Env = append(os.Environ(), "PYXIS_REPORT_HELPER="+phase, "PYXIS_REPORT_LEDGER_DIR="+dir)
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if phase == "crash" {
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
				t.Fatalf("crash phase = %v\n%s", err, out)
			}
		} else if err != nil {
			t.Fatalf("recovery phase = %v\n%s", err, out)
		}
	}
	for _, name := range []string{"a", "b"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(body) != "effect\n" {
			t.Fatalf("effect %s repeated or lost: %q %v", name, body, err)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ledger deliveryLedger
	if err := json.Unmarshal(body, &ledger); err != nil {
		t.Fatal(err)
	}
	for _, outcome := range ledger.Records["event"].Handlers {
		if outcome.Status != "completed" {
			t.Fatalf("cold receipt not complete: %+v", outcome)
		}
	}
}

func TestDurableDeliveryProcessHelper(t *testing.T) {
	phase := os.Getenv("PYXIS_REPORT_HELPER")
	if phase == "" {
		return
	}
	dir := os.Getenv("PYXIS_REPORT_LEDGER_DIR")
	if phase == "locked" {
		if _, err := NewDurableReporter("other", filepath.Join(dir, "ledger.json"), nil, nil); !errors.Is(err, ErrDeliveryLedgerInUse) {
			t.Fatalf("second process acquired owned ledger: %v", err)
		}
		return
	}
	a := callbackHandler{lifecycleHandler: &lifecycleHandler{id: "a", kinds: []Kind{KindTaskCompleted}}, fn: func(context.Context, Event) error {
		f, err := os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		return writeDeliveryFile(f, []byte("effect\n"))
	}}
	b := callbackHandler{lifecycleHandler: &lifecycleHandler{id: "b", kinds: a.kinds, after: []string{"a"}}, fn: func(context.Context, Event) error {
		// An external idempotency key protects the unavoidable effect/receipt
		// crash window. The reporter must still skip the recorded sibling.
		f, err := os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := writeDeliveryFile(f, []byte("effect\n")); err != nil {
			return err
		}
		if phase == "crash" {
			os.Exit(23)
		}
		return nil
	}}
	r, err := NewDurableReporter(phase, filepath.Join(dir, "ledger.json"), nil, []Handler{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if phase == "crash" {
		err = r.Publish(t.Context(), TaskCompleted("event", "T1", 1, "plan", "rev"))
	} else {
		err = r.Resume(t.Context())
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleCompletionReachesEverySink(t *testing.T) {
	var operator, flow, journal bytes.Buffer
	r := NewReporterWith("run", []Sink{captureOperator(&operator), NewFlowSink(&flow), NewArgusSink(&journal)}, WithClock(fixedClock()))
	events := []Event{TaskImplementationCompleted("i", "T1", 1, "plan", "head", "#7"), TaskCompleted("d", "T1", 1, "plan", "landed"), PlanCompleted("p", "plan", "landed")}
	for _, e := range events {
		r.Emit(e)
	}
	for _, want := range []string{"implementation completed", "task completed", "plan completed", "event_id=i", "plan=plan", "revision=landed", "proposal=#7"} {
		if !strings.Contains(operator.String(), want) {
			t.Errorf("operator missing %q: %s", want, &operator)
		}
	}
	for _, want := range []string{"implementation verified; awaiting merge", "delivery verified", "plan plan"} {
		if !strings.Contains(flow.String(), want) {
			t.Errorf("flow missing %q: %s", want, &flow)
		}
	}
	got := journalEvents(t, &journal)
	for i := range events {
		events[i].Time, events[i].RunID = fixedClock()(), "run"
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatalf("journal payloads changed: %#v", got)
	}
}
