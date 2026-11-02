package taskspec

import (
	"strings"
	"testing"
)

func TestParseRetiredCarriersFailClosed(t *testing.T) {
	native, err := RenderNative(nativeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{
		`{"goal":"legacy","acceptance":["green"]}`,
		`{"goal":"legacy","acceptance":["green"],"rung":1}`,
		`{"goal":"legacy","acceptance":["green"],"rung":3}`,
		`{"goal":"legacy","acceptance":["green"],"rung":null}`,
	} {
		legacy := "<!-- fs-spec:v1 " + payload + " -->"
		for _, comments := range [][]string{{legacy}, {native, legacy}, {legacy, native}} {
			if _, err := Parse(comments); err == nil || !strings.Contains(err.Error(), "unsupported spec version") {
				t.Fatalf("retired carrier accepted: %v", err)
			}
		}
		if _, err := ParseTask(legacy, []string{native}); err == nil {
			t.Fatal("legacy body hidden by current comment")
		}
	}
}

func TestParseIgnoresUnrelatedCommentsAndAcceptsIdenticalRepeats(t *testing.T) {
	want := nativeFixture(t)
	body, err := RenderNative(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse([]string{"hello", body, "unrelated", body})
	if err != nil || got.Goal != want.Goal {
		t.Fatalf("parse: %+v, %v", got, err)
	}
	for _, comments := range [][]string{{"ordinary comment"}, {strings.TrimSuffix(body, " -->")}, {"<!-- fs-spec:v2 not-json -->"}, {body, strings.Replace(body, `"goal":`, `"goal":"changed","goal":`, 1)}} {
		if _, err := Parse(comments); err == nil {
			t.Fatalf("accepted malformed carriers: %v", comments)
		}
	}
}
