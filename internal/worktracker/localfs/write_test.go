package localfs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestSweepTempsSparesLiveWriters pins the pid-aware temp sweep (write.go): an
// orphan temp (no live writer) is reaped, but a temp whose writer pid is still
// alive is left untouched — reaping it would make a concurrent writer's rename
// fail and lose the write (the parallel-create contract, DINV-2/UB-5). The
// conformance ParallelCreate row exercises this across real processes; this row
// pins the branch deterministically.
func TestSweepTempsSparesLiveWriters(t *testing.T) {
	s := New(t.TempDir(), fixedClock())
	dir := s.itemsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir items: %v", err)
	}

	// A live writer's temp: names this process's pid, which is alive.
	live := filepath.Join(dir, tempPrefix+strconv.Itoa(os.Getpid())+"-1")
	// An orphan: a name whose pid does not parse, treated as a dead orphan.
	orphan := filepath.Join(dir, tempPrefix+"notapid")
	for _, p := range []string{live, orphan} {
		if err := os.WriteFile(p, []byte("staged"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	s.sweepTemps()

	if _, err := os.Stat(live); err != nil {
		t.Fatalf("sweep reaped a live writer's temp: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("sweep did not reap the orphan temp (err=%v)", err)
	}
}

// TestTempPID parses the writer pid out of a temp name and rejects non-temp or
// malformed names.
func TestTempPID(t *testing.T) {
	cases := []struct {
		name    string
		wantPID int
		wantOK  bool
	}{
		{tempPrefix + "4321-7", 4321, true},
		{tempPrefix + "notapid", 0, false},
		{tempPrefix + "-5", 0, false},
		{tempPrefix + "0-1", 0, false},
		{"1.md", 0, false},
	}
	for _, c := range cases {
		gotPID, gotOK := tempPID(c.name)
		if gotPID != c.wantPID || gotOK != c.wantOK {
			t.Errorf("tempPID(%q) = (%d, %v), want (%d, %v)", c.name, gotPID, gotOK, c.wantPID, c.wantOK)
		}
	}
}
