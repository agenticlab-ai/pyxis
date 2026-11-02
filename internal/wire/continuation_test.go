package wire

import (
	"context"
	"strings"
	"testing"
)

func TestWiringContinuationPreservesHandoffsAndRepositoryState(t *testing.T) {
	for _, deferredRepo := range []bool{false, true} {
		name := "labels-handed-off"
		if deferredRepo {
			name = "repository-handed-off"
		}
		t.Run(name, func(t *testing.T) {
			ops := Table(onboardingConfig("required-checks"))
			split := 0
			for i, op := range ops {
				if op.Name == OpSetProtection {
					split = i
					break
				}
			}
			fake := newFake(t)
			if deferredRepo {
				fake.on(1, "", "HTTP 403: Resource not accessible by integration", 1, "repo create")
			} else {
				fake.on(1, "", "HTTP 403: Resource not accessible by integration", 1, "label create")
			}
			withDefaults(fake)
			prefix, err := Execute(context.Background(), ops[:split], fake, execOpts(&sleepRecorder{})...)
			if err != nil {
				t.Fatal(err)
			}
			options := append(execOpts(&sleepRecorder{}), WithPriorAudit(prefix))
			audit, err := Execute(context.Background(), ops[split:], fake, options...)
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(audit.OperationNames(), opNames(ops)) || !audit.Partial || !strings.Contains(audit.HandOff, prefix.HandOff) {
				t.Fatalf("continuation lost the declared history or handoff: %+v", audit)
			}
			if deferredRepo {
				for _, op := range ops[split:] {
					if r := findResult(t, audit, op.Name); r.Outcome != OutcomeHandedOff {
						t.Fatalf("deferred repository lost across push boundary: %+v", r)
					}
				}
				if fake.countCalls("issue", "create") != 0 || fake.countCalls("api", "PUT") != 0 {
					t.Fatal("continuation mutated a repository whose creation was deferred")
				}
			} else if fake.countCalls("issue", "create", "gh label create") != 1 {
				t.Fatal("onboarding omitted the prefix's outstanding label commands")
			}
		})
	}
}
