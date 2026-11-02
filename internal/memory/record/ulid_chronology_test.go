package record

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBatchTimeOrdersFullIDsIndependentOfEntropyAndKind(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 123456789, time.UTC)
	var high [10]byte
	for i := range high {
		high[i] = 0xff
	}
	for _, tc := range []struct {
		name         string
		predecessors []string
		want         time.Time
	}{
		{"past source", []string{NewID(Promise, now.Add(-time.Millisecond), high)}, time.UnixMilli(now.UnixMilli())},
		{"equal millisecond high entropy", []string{NewID(Promise, now, high), NewID(Answer, now, [10]byte{})}, time.UnixMilli(now.UnixMilli() + 1)},
		{"future source in same created_at second", []string{NewID(Promise, now.Add(2*time.Millisecond), high), NewID(Answer, now.Add(3*time.Millisecond), [10]byte{})}, time.UnixMilli(now.UnixMilli() + 4)},
		{"clock behind immutable source", []string{NewID(Policy, now.AddDate(1, 0, 0), high)}, time.UnixMilli(now.AddDate(1, 0, 0).UnixMilli() + 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, err := BatchTime(now, 3, tc.predecessors...)
			if err != nil || !first.Equal(tc.want) {
				t.Fatalf("batch start=%v err=%v, want earliest safe millisecond %v", first, err, tc.want)
			}
			previous := ""
			for n, kind := range []string{Decision, Promise, Answer} {
				id := NewID(kind, first.Add(time.Duration(n)*time.Millisecond), [10]byte{})
				for _, source := range tc.predecessors {
					if !IDBefore(source, id) {
						t.Fatalf("source %s is not before new %s", source, id)
					}
				}
				if previous != "" && !IDBefore(previous, id) {
					t.Fatalf("local dependency %s is not before new %s", previous, id)
				}
				previous = id
			}
		})
	}
}

func TestBatchTimeRejectsInvalidOrUnrepresentableChronology(t *testing.T) {
	last := time.Date(9999, 12, 31, 23, 59, 59, 999000000, time.UTC)
	for _, tc := range []struct {
		name        string
		now         time.Time
		count       uint64
		predecessor string
	}{
		{"negative ULID time", time.UnixMilli(-1), 1, ""},
		{"created_at exceeds RFC3339", last.Add(time.Millisecond), 1, ""},
		{"malformed source ID", t0, 1, "Promise-invalid"},
		{"unknown source kind", t0, 1, NewID("Unknown", t0, [10]byte{})},
		{"source ULID exceeds 128 bits", t0, 1, "Promise-8" + strings.Repeat("0", 25)},
		{"source beyond created_at range", t0, 1, NewID(Promise, time.UnixMilli((1<<48)-1), [10]byte{})},
		{"source has no representable successor", t0, 1, NewID(Promise, last, [10]byte{})},
		{"whole range must fit", last.Add(-time.Millisecond), 3, ""},
		{"count must not overflow", t0, ^uint64(0), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var predecessors []string
			if tc.predecessor != "" {
				predecessors = append(predecessors, tc.predecessor)
			}
			if _, err := BatchTime(tc.now, tc.count, predecessors...); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid chronology accepted: %v", err)
			}
		})
	}
}

func TestBatchTimeFinalRepresentableRangeAndEmptyBatch(t *testing.T) {
	last := time.Date(9999, 12, 31, 23, 59, 59, 999000000, time.UTC)
	first, err := BatchTime(last.Add(-2*time.Millisecond), 3)
	if err != nil {
		t.Fatal(err)
	}
	for n := range 3 {
		at := first.Add(time.Duration(n) * time.Millisecond)
		r := valid()
		r.ID, r.CreatedAt = NewID(Promise, at, [10]byte{}), at.Format(time.RFC3339)
		if err := r.Validate(); err != nil {
			t.Fatalf("final representable range minted invalid record: %v", err)
		}
	}
	if !first.Add(2 * time.Millisecond).Equal(last) {
		t.Fatal("final representable millisecond was not available")
	}
	if first, err := BatchTime(t0, 0, NewID(Promise, last, [10]byte{})); err != nil || !first.Equal(t0) {
		t.Fatalf("empty batch unnecessarily needs a successor: %v %v", first, err)
	}
}
