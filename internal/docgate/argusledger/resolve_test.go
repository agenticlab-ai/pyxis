package argusledger

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const deliveryFixture = `# demo — delivery ledger

| Field | Value |
|---|---|
| **Slug** | demo |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | Stage 1 |

## Journal

### E-1 — stage-entered — begin

- **Body**: one

### E-2 — ruling-recorded — a call

- **Body**: two
`

func TestResolveAnchor(t *testing.T) {
	l, defects := ParseAs("t", "docs/problems/demo/delivery.md",
		schema.LedgerClasses["delivery-ledger"], []byte(deliveryFixture))
	if len(defects) != 0 {
		t.Fatalf("unexpected parse defects: %v", defects)
	}
	cases := []struct {
		anchor string
		want   AnchorResolution
	}{
		{"E-1", AnchorJournal},
		{"E-2", AnchorJournal},
		{"E-9", AnchorUnresolved},
		{"Status", AnchorDerived},
		{"Stage position", AnchorDerived},
		{"Nonexistent", AnchorUnresolved},
	}
	for _, c := range cases {
		if got := l.ResolveAnchor(c.anchor); got != c.want {
			t.Errorf("ResolveAnchor(%q) = %d, want %d", c.anchor, got, c.want)
		}
	}
}

func TestDerivedFields(t *testing.T) {
	l, _ := ParseAs("t", "docs/problems/demo/delivery.md",
		schema.LedgerClasses["delivery-ledger"], []byte(deliveryFixture))
	fields := DerivedFields(l.Derived.Bytes)
	want := map[string]bool{"Status": true, "Stage position": true}
	if len(fields) != len(want) {
		t.Fatalf("got fields %v, want %v", fields, want)
	}
	for _, f := range fields {
		if !want[f] {
			t.Errorf("unexpected derived field %q (a value-cell bold span must not be a field)", f)
		}
	}
}
