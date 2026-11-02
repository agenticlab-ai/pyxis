package ecosystem

import "testing"

// These tests moved here from internal/verify/checks/falsification_test.go
// when the go focused-test strategy (span extraction + `go test -json`
// outcome parsing) was relocated out of the stack-blind falsification check
// into this ecosystem data entry (T10 / FR-16). Behavior is unchanged; only
// touchedBy's argument shape changed (a []int of line numbers, so ecosystem
// stays free of a diffset dependency).

func TestGoStrategy_Registered(t *testing.T) {
	strat, ok := New().FocusedTestStrategyFor("go")
	if !ok {
		t.Fatal("go stack has no focused-test strategy, want go-test-json")
	}
	if strat.TargetablePath == nil || strat.ExtractTargetNames == nil || strat.BuildCommand == nil || strat.ParseOutcomes == nil {
		t.Fatal("go strategy has a nil member")
	}
	if !strat.TargetablePath("pkg/foo_test.go") || strat.TargetablePath("pkg/foo.go") {
		t.Error("TargetablePath must accept _test.go files only")
	}
	if got := strat.BuildCommand("./pkg", []string{"TestA", "TestB"}); got != "go test -json -run '^(TestA|TestB)$' ./pkg" {
		t.Errorf("BuildCommand = %q", got)
	}
	if got := strat.BuildCommand("./pkg", nil); got != "go test -json ./pkg" {
		t.Errorf("BuildCommand (whole-package) = %q", got)
	}
}

func TestGoStrategy_UnknownStackHasNoStrategy(t *testing.T) {
	if _, ok := New().FocusedTestStrategyFor("python"); ok {
		t.Error("an unregistered stack must report no focused-test strategy (⇒ whole-suite fallback)")
	}
}

func TestGoTestFuncSpans_MultipleFunctionsOrderedWithSpans(t *testing.T) {
	content := []byte("package pkg\n\nfunc TestA(t *testing.T) {\n\tx := 1\n\t_ = x\n}\n\nfunc TestB(t *testing.T) {\n\ty := 2\n\t_ = y\n}\n")
	spans := goTestFuncSpans(content)
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2: %+v", len(spans), spans)
	}
	if spans[0].name != "TestA" || spans[0].start != 3 {
		t.Errorf("spans[0] = %+v, want name=TestA start=3", spans[0])
	}
	if spans[1].name != "TestB" {
		t.Errorf("spans[1].name = %q, want TestB", spans[1].name)
	}
	// TestA's span must cover its body lines (4-5), ending right before TestB's
	// func line, so a change to line 4 (body-only) attributes to TestA, not TestB.
	if !spans[0].touchedBy([]int{5}) {
		t.Error("spans[0] (TestA) should be touched by an added line inside its own body")
	}
	if spans[0].touchedBy([]int{spans[1].start}) {
		t.Error("spans[0] (TestA) must not be touched by a line inside TestB's span")
	}
}

func TestGoStrategy_ExtractTargetNames(t *testing.T) {
	content := []byte("package pkg\n\nfunc TestA(t *testing.T) {\n\tx := 1\n\t_ = x\n}\n\nfunc TestB(t *testing.T) {\n\ty := 2\n\t_ = y\n}\n")
	// Added file: every top-level test is a target.
	if got := goExtractTargetNames(content, nil, true); len(got) != 2 || got[0] != "TestA" || got[1] != "TestB" {
		t.Errorf("added-file targets = %v, want [TestA TestB]", got)
	}
	// Modified file: only the function whose span a changed line falls in.
	if got := goExtractTargetNames(content, []int{4}, false); len(got) != 1 || got[0] != "TestA" {
		t.Errorf("modified-file targets = %v, want [TestA]", got)
	}
}

func TestParseGoTestOutcomes_PassFailSkipAndMissingName(t *testing.T) {
	stream := `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"pass","Package":"p","Test":"TestA"}
{"Action":"run","Package":"p","Test":"TestB"}
{"Action":"fail","Package":"p","Test":"TestB"}
{"Action":"run","Package":"p","Test":"TestS"}
{"Action":"skip","Package":"p","Test":"TestS"}
{"Action":"pass","Package":"p"}
`
	outcomes := parseGoTestOutcomes(stream)
	if outcomes["TestA"] != "pass" {
		t.Errorf(`outcomes["TestA"] = %q, want "pass"`, outcomes["TestA"])
	}
	if outcomes["TestB"] != "fail" {
		t.Errorf(`outcomes["TestB"] = %q, want "fail"`, outcomes["TestB"])
	}
	if outcomes["TestS"] != "skip" {
		t.Errorf(`outcomes["TestS"] = %q, want "skip" (a skip is not-applicable, distinct from a fail)`, outcomes["TestS"])
	}
	if _, ok := outcomes["TestC"]; ok {
		t.Errorf(`outcomes["TestC"] = %q present, want absent (never ran to a terminal outcome)`, outcomes["TestC"])
	}
}

func TestParseGoTestOutcomes_PassIsAuthoritative(t *testing.T) {
	stream := `{"Action":"pass","Package":"p","Test":"TestA"}
{"Action":"fail","Package":"p","Test":"TestA"}
{"Action":"skip","Package":"p","Test":"TestB"}
{"Action":"pass","Package":"p","Test":"TestB"}
`
	outcomes := parseGoTestOutcomes(stream)
	if outcomes["TestA"] != "pass" {
		t.Errorf(`outcomes["TestA"] = %q, want "pass" (a pass must not regress to a later fail)`, outcomes["TestA"])
	}
	if outcomes["TestB"] != "pass" {
		t.Errorf(`outcomes["TestB"] = %q, want "pass" (a later pass overrides an earlier skip)`, outcomes["TestB"])
	}
}

func TestParseGoTestOutcomes_MalformedOutputDegradesGracefully(t *testing.T) {
	outcomes := parseGoTestOutcomes("not json at all")
	if len(outcomes) != 0 {
		t.Errorf("outcomes = %v, want empty on unparseable input", outcomes)
	}
}
