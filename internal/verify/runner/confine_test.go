package runner

import (
	"strings"
	"testing"
)

// --- detectWriteScopeViolation -------------------------------------------

func TestDetectWriteScopeViolation_GoStyleBarePathExtracted(t *testing.T) {
	path, ok := detectWriteScopeViolation("", `open /outside/scope/file.txt: permission denied`)
	if !ok {
		t.Fatal("want a violation signature match")
	}
	if path != "/outside/scope/file.txt" {
		t.Errorf("path = %q, want /outside/scope/file.txt (stopping before the colon)", path)
	}
}

func TestDetectWriteScopeViolation_SingleQuotedPathExtracted(t *testing.T) {
	path, ok := detectWriteScopeViolation("", `mkdir: cannot create directory '/etc/evil': Permission denied`)
	if !ok {
		t.Fatal("want a violation signature match")
	}
	if path != "/etc/evil" {
		t.Errorf("path = %q, want /etc/evil", path)
	}
}

func TestDetectWriteScopeViolation_EACCES_CaseInsensitive(t *testing.T) {
	path, ok := detectWriteScopeViolation("Error: eacces: permission denied, open '/tmp/outside/x'", "")
	if !ok {
		t.Fatal("want a violation signature match (case-insensitive EACCES)")
	}
	if path != "/tmp/outside/x" {
		t.Errorf("path = %q, want /tmp/outside/x", path)
	}
}

func TestDetectWriteScopeViolation_SignatureWithNoExtractablePath(t *testing.T) {
	_, ok := detectWriteScopeViolation("", "permission denied")
	if !ok {
		t.Fatal("want a violation signature match even without a recoverable path")
	}
}

func TestDetectWriteScopeViolation_NoSignature_NoMatch(t *testing.T) {
	_, ok := detectWriteScopeViolation("go: build failed", "some_test.go:12: undefined: Foo")
	if ok {
		t.Error("want no violation match on ordinary build/test output")
	}
}

func TestDetectWriteScopeViolation_ScansBothStreams(t *testing.T) {
	_, ok := detectWriteScopeViolation("nothing here", "open /x: permission denied")
	if !ok {
		t.Error("want a match found in stderr even when stdout is clean")
	}
}

// --- wrapperFailureReason -------------------------------------------------

func TestWrapperFailureReason_ExitCodeAndMarkerMatch(t *testing.T) {
	res := Result{ExitCode: confineWrapperFailExitCode, Stderr: confineWrapperFailMarker + "landlock: unsupported by this kernel (ABI 0)\n"}
	reason, ok := wrapperFailureReason(res)
	if !ok {
		t.Fatal("want a recognized wrapper failure")
	}
	if reason != "landlock: unsupported by this kernel (ABI 0)" {
		t.Errorf("reason = %q, want the marker's trimmed payload", reason)
	}
}

func TestWrapperFailureReason_RightExitCodeWrongText_NotRecognized(t *testing.T) {
	// A REAL verb that happens to exit with the same numeric code for its own
	// unrelated reasons must never be misread as a confinement failure: the
	// marker text must ALSO match, not the number alone.
	res := Result{ExitCode: confineWrapperFailExitCode, Stderr: "some tool's own unrelated error text\n"}
	if _, ok := wrapperFailureReason(res); ok {
		t.Error("want no match: marker text absent even though the exit code coincides")
	}
}

func TestWrapperFailureReason_MarkerPresentWrongExitCode_NotRecognized(t *testing.T) {
	res := Result{ExitCode: 1, Stderr: confineWrapperFailMarker + "should not matter\n"}
	if _, ok := wrapperFailureReason(res); ok {
		t.Error("want no match: exit code does not match the reserved sentinel")
	}
}

func TestWrapperFailureReason_AlreadyClassed_NotRecognized(t *testing.T) {
	res := Result{Class: ClassTimeout, ExitCode: confineWrapperFailExitCode, Stderr: confineWrapperFailMarker + "x\n"}
	if _, ok := wrapperFailureReason(res); ok {
		t.Error("want no match: a result that already carries a Class is never a wrapper-failure candidate")
	}
}

// --- finalizeConfinement ---------------------------------------------------

func TestFinalizeConfinement_NotConfined_PassthroughUnchanged(t *testing.T) {
	in := Result{ExitCode: 1, Stderr: "permission denied: /outside"}
	out := finalizeConfinement(false, in)
	if out.Class != "" {
		t.Errorf("Class = %q, want unchanged empty (confinement was never active)", out.Class)
	}
}

func TestFinalizeConfinement_CleanPass_NeverReclassified(t *testing.T) {
	in := Result{Passed: true, ExitCode: 0, Stdout: "unrelated log line mentioning permission denied in a test name"}
	out := finalizeConfinement(true, in)
	if out.Class != "" {
		t.Errorf("Class = %q, want empty: a clean pass must never be reclassified as a violation", out.Class)
	}
}

func TestFinalizeConfinement_AlreadyClassed_NeverOverridden(t *testing.T) {
	in := Result{Class: ClassTimeout, Stderr: "permission denied: /outside"}
	out := finalizeConfinement(true, in)
	if out.Class != ClassTimeout {
		t.Errorf("Class = %q, want ClassTimeout preserved (its own distinct, already-correct classification)", out.Class)
	}
}

func TestFinalizeConfinement_WrapperFailure_ReclassifiedInfra(t *testing.T) {
	in := Result{ExitCode: confineWrapperFailExitCode, Stderr: confineWrapperFailMarker + "landlock: create ruleset: bad\n"}
	out := finalizeConfinement(true, in)
	if out.Class != ClassInfra {
		t.Fatalf("Class = %q, want ClassInfra", out.Class)
	}
	if out.Err == nil || !strings.Contains(out.Err.Error(), "landlock: create ruleset: bad") {
		t.Errorf("Err = %v, want it to carry the wrapper's reported reason", out.Err)
	}
}

func TestFinalizeConfinement_PlainFailureWithViolationSignature_ReclassifiedViolation(t *testing.T) {
	in := Result{ExitCode: 1, Stderr: "open /outside/scope/x.txt: permission denied\n"}
	out := finalizeConfinement(true, in)
	if out.Class != ClassViolation {
		t.Fatalf("Class = %q, want ClassViolation", out.Class)
	}
	if out.Err == nil || !strings.Contains(out.Err.Error(), "/outside/scope/x.txt") {
		t.Errorf("Err = %v, want it to name the offending path", out.Err)
	}
}

func TestFinalizeConfinement_PlainFailureWithoutSignature_Unchanged(t *testing.T) {
	in := Result{ExitCode: 1, Stdout: "some_test.go:12: assertion failed"}
	out := finalizeConfinement(true, in)
	if out.Class != "" {
		t.Errorf("Class = %q, want unchanged empty: an ordinary check failure is never reclassified", out.Class)
	}
}
