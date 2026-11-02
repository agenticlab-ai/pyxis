package wake

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func mustParse(t *testing.T, d Doc) *argusledger.Ledger {
	t.Helper()
	l, defects := argusledger.Parse(config.Defaults().Problems, verb, d.Path, d.Bytes)
	if len(defects) != 0 {
		t.Fatalf("fixture ledger did not parse: %v", defects)
	}
	return l
}

func TestDispose(t *testing.T) {
	cases := []struct {
		name    string
		journal string
		want    Disposition
	}{
		{
			name:    "open with an outstanding await",
			journal: stageEntered + awaitEntry("E-2", "pr:214", "merged", "2026-07-11"),
			want:    Open,
		},
		{
			name: "parked on an unanswered brief",
			journal: stageEntered +
				"### E-2 — brief-emitted — need a ruling\n\n- **Date**: 2026-07-11\n- **Body**: escalated; class: override\n\n---\n",
			want: Parked,
		},
		{
			name: "answered park is open again",
			journal: stageEntered +
				"### E-2 — brief-emitted — need a ruling\n\n- **Date**: 2026-07-11\n- **Body**: escalated\n\n---\n" +
				"### E-3 — answer-received — the human answered\n\n- **Date**: 2026-07-12\n- **Body**: cites A-2\n\n---\n",
			want: Open,
		},
		{
			name: "closed on the delivered entry",
			journal: stageEntered +
				"### E-2 — delivered — done\n\n- **Date**: 2026-07-12\n- **Body**: verified on the mainline\n\n---\n",
			want: Closed,
		},
		{
			name: "closed dominates a prior park",
			journal: stageEntered +
				"### E-2 — brief-emitted — need a ruling\n\n- **Date**: 2026-07-11\n- **Body**: escalated\n\n---\n" +
				"### E-3 — delivered — done\n\n- **Date**: 2026-07-12\n- **Body**: verified on the mainline\n\n---\n",
			want: Closed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := mustParse(t, deliveryDoc("foo", c.journal))
			if got := Dispose(l); got != c.want {
				t.Fatalf("Dispose = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDispose_NilAndNonDelivery(t *testing.T) {
	if got := Dispose(nil); got != Open {
		t.Fatalf("Dispose(nil) = %v, want Open", got)
	}
}
