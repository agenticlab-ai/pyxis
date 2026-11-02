package taskspec

import "testing"

func TestNativeCarrierRejectsInvalidProducer(t *testing.T) {
	for _, mutate := range []func(*Spec){
		func(s *Spec) { s.Requirements = nil },
		func(s *Spec) { s.Acceptance = nil },
	} {
		s := nativeFixture(t)
		mutate(&s)
		if _, err := RenderNative(s); err == nil {
			t.Fatal("rendered invalid native producer")
		}
	}
}
