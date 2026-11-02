package runner

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForExitDeadlineAndGrace(t *testing.T) {
	for _, scenario := range []string{"completed", "cancelled", "term exits", "kill required"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const timeout, grace = 30 * time.Minute, 5 * time.Second
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				exited := make(chan error, 1)
				defer func() {
					select {
					case exited <- nil:
					default:
					}
				}()
				var signals []bool
				var mu sync.Mutex
				observed := func() []bool {
					mu.Lock()
					defer mu.Unlock()
					return slices.Clone(signals)
				}
				var class Class
				var err error
				returned := make(chan struct{})
				go func() {
					class, err = waitForExit(ctx, exited, timeout, grace, func(force bool) {
						mu.Lock()
						defer mu.Unlock()
						signals = append(signals, force)
					})
					close(returned)
				}()
				synctest.Wait()
				var wantSignals []bool
				var wantClass Class
				switch scenario {
				case "completed":
				case "cancelled":
					cancel()
					synctest.Wait()
					wantSignals, wantClass = []bool{true}, ClassInfra
				default:
					time.Sleep(timeout - time.Nanosecond)
					synctest.Wait()
					if got := observed(); len(got) != 0 {
						t.Fatalf("signals before deadline: %v", got)
					}
					time.Sleep(time.Nanosecond)
					synctest.Wait()
					if got := observed(); !slices.Equal(got, []bool{false}) {
						t.Fatalf("at deadline: %v, want TERM only", got)
					}
					wantSignals, wantClass = []bool{false}, ClassTimeout
					if scenario == "kill required" {
						time.Sleep(grace - time.Nanosecond)
						synctest.Wait()
						if got := observed(); !slices.Equal(got, wantSignals) {
							t.Fatalf("KILL before grace ended: %v", got)
						}
						time.Sleep(time.Nanosecond)
						synctest.Wait()
						wantSignals = []bool{false, true}
					}
				}
				if got := observed(); !slices.Equal(got, wantSignals) {
					t.Fatalf("signals=%v, want %v", got, wantSignals)
				}
				select {
				case <-returned:
					t.Fatal("returned before process exit acknowledgment")
				default:
				}
				exited <- nil
				<-returned
				if class != wantClass {
					t.Fatalf("class=%q, want %q", class, wantClass)
				}
				if scenario == "cancelled" {
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("err=%v, want cancellation", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
