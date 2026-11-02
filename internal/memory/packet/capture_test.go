package packet

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestDerivedCaptureBlocksNewRecordsButPreservesHistory(t *testing.T) {
	base := newTree(t)
	head := base.clone()
	r := head.add(record.Need, "code/widget", "anchor", "Production code unit internal/widget exists.")
	r.Tags = []string{"reconcile:anchor"}
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	head.files[r.Path()] = b
	if p := Compute(Input{Base: base.set(), Head: head.set()}); !strings.Contains(codes(p.Blockers), "derived-memory") {
		t.Fatalf("manual file bypassed capture admission: %+v", p.Blockers)
	}
	if p := Compute(Input{Base: head.set(), Head: head.set()}); !p.OK() {
		t.Fatalf("inherited immutable history was rejected: %+v", p.Blockers)
	}
}
