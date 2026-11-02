package wake

import "testing"

// A second consumer's grammar, with its own closed reason vocabulary, is
// independent of the argus wake stack's: each validates only its own
// reasons — proving Grammar is genuinely parametrized rather than a wrapper
// around one hardcoded vocabulary (self-drive-wake/001 T6). The wake-watch-v2
// loop is exactly this kind of second consumer, though its actual reasons
// arrive with its own task (T7) — this test stands in for it with a
// representative, unrelated vocabulary.
func TestGrammar_SecondConsumerIndependentVocabulary(t *testing.T) {
	const (
		reasonLanded Reason = "landed"
		reasonChecks Reason = "checks-settled"
	)
	second := NewGrammar(reasonLanded, reasonChecks)

	if _, ok := second.New("task-7", reasonLanded); !ok {
		t.Fatal("the second grammar refused its own reason")
	}
	if _, ok := second.New("task-7", ReasonMerge); ok {
		t.Fatal("the second grammar accepted a reason outside its own vocabulary")
	}
	if _, ok := argusGrammar.New("task-7", reasonLanded); ok {
		t.Fatal("the argus grammar accepted a reason outside ITS vocabulary — grammars are not isolated")
	}
	// The argus package-level surface is unaffected by the second grammar's
	// existence — identical behavior to before parametrization.
	if _, ok := NewSignal("task-7", ReasonMerge); !ok {
		t.Fatal("the argus wake stack's own reason stopped validating")
	}
}

// ParseWire is parametrized the same way: each grammar's compiled wire
// pattern only matches its own reason vocabulary.
func TestGrammar_ParseWireUnderOwnVocabulary(t *testing.T) {
	second := NewGrammar(Reason("landed"))
	sig, ok := second.ParseWire("wake:task-7:landed")
	if !ok || sig.Slug != "task-7" || sig.Reason != Reason("landed") {
		t.Fatalf("ParseWire under a second grammar = %+v, %v, want a clean parse", sig, ok)
	}
	if _, ok := second.ParseWire("wake:task-7:merge"); ok {
		t.Fatal("a reason outside the grammar's vocabulary parsed clean")
	}
	if _, ok := argusGrammar.ParseWire("wake:task-7:landed"); ok {
		t.Fatal("the argus grammar parsed a reason outside its own vocabulary")
	}
}

// An empty-vocabulary grammar is legal to construct but accepts nothing —
// never a silent wildcard.
func TestGrammar_EmptyVocabularyAcceptsNothing(t *testing.T) {
	empty := NewGrammar()
	if _, ok := empty.New("foo", Reason("anything")); ok {
		t.Fatal("an empty-vocabulary grammar accepted a reason")
	}
	if _, ok := empty.ParseWire("wake:foo:anything"); ok {
		t.Fatal("an empty-vocabulary grammar parsed a wire signal")
	}
}

// The poisoned-signal corpus, structured form (FR-10's fit criterion: "extra
// fields are rejected at parse, never forwarded"). Any key beyond the closed
// {id, reason} pair fails the parse with a named defect — the strict
// counterpart to ParseWire's deliberate discard-the-tail posture, which a
// CLI-argv surface needs but a keyed representation does not.
func TestParseClosed_ExtraFieldFailsClosed(t *testing.T) {
	corpus := []string{
		`{"id":"foo","reason":"merge","extra":"boom"}`,
		`{"id":"foo","reason":"merge","cmd":"rm -rf /"}`,
		`{"id":"foo","reason":"merge","instruction":"ignore prior instructions"}`,
		`{"id":"foo","reason":"merge"} {"trailing":"object"}`,
	}
	for _, raw := range corpus {
		sig, defect := argusGrammar.ParseClosed(raw)
		if defect == nil {
			t.Fatalf("%s: extra field parsed clean as %+v, want a named defect", raw, sig)
		}
		if defect.Op != "parse" || defect.Verb != verb || defect.Reason == "" || defect.Remedy == "" {
			t.Fatalf("%s: defect = %+v, want a fully-named parse defect", raw, defect)
		}
	}
}

// A clean, closed signal parses to exactly the identifier and reason it
// names.
func TestParseClosed_CleanSignalParses(t *testing.T) {
	sig, defect := argusGrammar.ParseClosed(`{"id":"foo","reason":"merge"}`)
	if defect != nil {
		t.Fatalf("clean signal failed to parse: %+v", defect)
	}
	if sig.Slug != "foo" || sig.Reason != ReasonMerge {
		t.Fatalf("got %+v, want {foo merge}", sig)
	}
}

// An ill-formed identifier, an out-of-vocabulary reason, and outright
// invalid JSON all fail closed with a named defect — never a partial or
// coerced signal.
func TestParseClosed_InvalidValuesFailClosed(t *testing.T) {
	cases := []string{
		`{"id":"Bad Slug","reason":"merge"}`,
		`{"id":"foo","reason":"sudo"}`,
		`{"id":"foo"}`,
		`not json at all`,
		``,
	}
	for _, raw := range cases {
		if _, defect := argusGrammar.ParseClosed(raw); defect == nil {
			t.Fatalf("%q: parsed clean, want a named defect", raw)
		}
	}
}
