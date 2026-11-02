package acting

import (
	"errors"
	"testing"
)

func TestCoverage90ActFailureAndHelperFallbacks(t *testing.T) {
	p := policyFor(true, "refs/backstop@1", false, fleetTask)
	machine := &fakeMachine{id: machineID, actErr: errors.New("merge unavailable")}
	a := &Adapter{Medium: fullMedium(), Recorder: newRecorder(), Machine: machine}
	res, err := a.Handle(t.Context(), autoEvent("proposal"), p, Preconditions{RefProtectionPresent: true})
	if err != nil {
		t.Fatalf("Handle act failure returned infrastructure error: %v", err)
	}
	if res.Acted || !refusalMentions(res, "did not complete") {
		t.Fatalf("Handle act failure result = %+v", res)
	}
	if _, ok := findClass(p, "missing"); ok {
		t.Fatal("findClass found an absent class")
	}
	if got := quote("value"); got != "'value'" {
		t.Fatalf("quote = %q", got)
	}
}
