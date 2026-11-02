package reviewobligation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testRecordBudget = 1 << 20

func sizedObligation(t *testing.T, size int) Obligation {
	t.Helper()
	o := Obligation{SchemaVersion: Schema, ID: "sized", Repository: "owner/repo", Scope: "feature/001", TaskID: "T1", ProposalRef: "github:owner/repo#1", Revision: "abc", Harness: "claude", TaskCharter: strings.Repeat("<\n", 32*1024-1000)}
	if size == 0 {
		return o
	}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > size {
		t.Fatalf("fixture needs %d bytes, budget %d", len(b), size)
	}
	o.TaskCharter += strings.Repeat("x", size-len(b))
	return o
}

func rowLimitStore(t *testing.T, kind string) (Store, func() []byte) {
	t.Helper()
	if kind == "file" {
		path := filepath.Join(t.TempDir(), "review.jsonl")
		store, err := NewFileStore(path)
		if err != nil {
			t.Fatal(err)
		}
		return store, func() []byte {
			b, err := os.ReadFile(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			return b
		}
	}
	remote, repo := gitStoreFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	return store, func() []byte {
		out, err := exec.Command("git", "--git-dir", remote, "rev-parse", "--verify", DefaultEvidenceRef).CombinedOutput()
		if err != nil {
			t.Fatalf("read published evidence ref: %v: %s", err, out)
		}
		return out
	}
}

func TestStoresRoundTripLargeCharter(t *testing.T) {
	for _, kind := range []string{"file", "git"} {
		for _, size := range []int{0, testRecordBudget} {
			t.Run(fmt.Sprintf("%s/%d", kind, size), func(t *testing.T) {
				store, _ := rowLimitStore(t, kind)
				o := sizedObligation(t, size)
				ctx := context.Background()
				if err := store.Append(ctx, o); err != nil {
					t.Fatal(err)
				}
				got, found, err := store.Get(ctx, o.ID)
				if err != nil || !found || got != o {
					t.Fatalf("Get did not retain full charter: found=%v err=%v", found, err)
				}
				states, err := store.Open(ctx, o.Scope)
				if err != nil || len(states) != 1 || states[0].Obligation != o || states[0].Status != "handoff_pending" {
					t.Fatalf("Open before handoff: states=%d err=%v", len(states), err)
				}
				if err := store.CompleteHandoff(ctx, o.ID); err != nil {
					t.Fatalf("CompleteHandoff: %v", err)
				}
				states, err = store.Open(ctx, o.Scope)
				if err != nil || len(states) != 1 || states[0].Obligation != o || states[0].Status != "review_pending" {
					t.Fatalf("Open after handoff: states=%d err=%v", len(states), err)
				}
			})
		}
	}
}

func TestStoresRejectOversizedCharterWithoutPublication(t *testing.T) {
	for _, kind := range []string{"file", "git"} {
		t.Run(kind, func(t *testing.T) {
			store, snapshot := rowLimitStore(t, kind)
			ctx := context.Background()
			good := sizedObligation(t, 300*1024)
			good.TaskCharter = "baseline"
			if err := store.Append(ctx, good); err != nil {
				t.Fatal(err)
			}
			before := snapshot()
			oversized := sizedObligation(t, testRecordBudget+1)
			oversized.ID = "other"
			if err := store.Append(ctx, oversized); !errors.Is(err, ErrRowTooLarge) {
				t.Fatalf("oversized serialized charter: %v", err)
			}
			if !bytes.Equal(before, snapshot()) {
				t.Fatal("rejected row changed published evidence")
			}
			got, found, err := store.Get(ctx, good.ID)
			if err != nil || !found || got != good {
				t.Fatalf("existing charter became unreadable: found=%v err=%v", found, err)
			}
		})
	}
}

func TestRecordBounds(t *testing.T) {
	if testRecordBudget != MaxRecordBytes {
		t.Fatal("record budget changed; reconsider persisted compatibility")
	}
	if _, err := marshalRecord(make(chan int)); err == nil {
		t.Fatal("unencodable row accepted")
	}
	for _, ending := range []string{"\n", "\r\n", ""} {
		for _, size := range []int{MaxRecordBytes, MaxRecordBytes + 1, MaxRecordBytes + 100} {
			t.Run(fmt.Sprintf("size=%d/ending=%q", size, ending), func(t *testing.T) {
				scan := recordScanner([]byte(strings.Repeat("x", size) + ending))
				if size > MaxRecordBytes {
					if scan.Scan() || scan.Err() == nil {
						t.Fatal("reader accepted an oversized row")
					}
					return
				}
				if !scan.Scan() || len(scan.Bytes()) != size || scan.Scan() || scan.Err() != nil {
					t.Fatalf("exact-boundary row failed: %v", scan.Err())
				}
			})
		}
	}
}

func TestStoresReplayLargeLifecycleRecords(t *testing.T) {
	for _, kind := range []string{"file", "git"} {
		t.Run(kind, func(t *testing.T) {
			store, _ := rowLimitStore(t, kind)
			ctx := context.Background()
			o := sizedObligation(t, 0)
			if err := store.Append(ctx, o); err != nil {
				t.Fatal(err)
			}
			if err := store.CompleteHandoff(ctx, o.ID); err != nil {
				t.Fatal(err)
			}
			attempt := testAttempt(o.ID, "large-attempt", "run")
			attempt.AttemptProofRef = strings.Repeat("p", 80*1024)
			lease, err := store.Begin(ctx, attempt)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := lease.Close(); err != nil {
					t.Error(err)
				}
			})
			pair := testPublicationPair(o, attempt)
			pair.NativeReceipt = strings.Repeat("r", 80*1024)
			if err := store.RecordPair(ctx, pair); err != nil {
				t.Fatal(err)
			}
			if err := store.AppendTerminal(ctx, testVerifiedTerminal(pair)); err != nil {
				t.Fatal(err)
			}
			state, found, err := store.State(ctx, o.ID)
			if err != nil || !found || state.Status != "verified" || state.Pair == nil || *state.Pair != pair {
				t.Fatalf("large lifecycle replay: found=%v status=%s err=%v", found, state.Status, err)
			}
		})
	}
}
