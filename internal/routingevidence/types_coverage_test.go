package routingevidence

import "testing"

func TestPayloadMarshalRejectsEmptyAndAmbiguousUnion(t *testing.T) {
	if _, err := (Payload{}).MarshalJSON(); err == nil {
		t.Fatal("empty payload union was accepted")
	}
	p := Payload{
		RoutingResolved:         &RoutingResolvedPayload{},
		RoutingResolutionFailed: &RoutingResolutionFailedPayload{},
	}
	if _, err := p.MarshalJSON(); err == nil {
		t.Fatal("multiple payload union members were accepted")
	}
}
