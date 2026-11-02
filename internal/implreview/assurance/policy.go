// Package assurance owns impl-review's planner-assurance by proposal-band join.
// Routing evidence may validate a claimed process against this pure function;
// it does not define or reinterpret the policy.
package assurance

import "fmt"

const (
	DepthStandard = "standard"
	DepthDeep     = "deep"
)

// Required returns the exact effective depth and top-level pass count from the
// approved impl-review assurance matrix.
func Required(plannedAssurance, proposalBand string) (string, int, error) {
	switch plannedAssurance {
	case "ordinary":
		switch proposalBand {
		case "skip", "standard":
			return DepthStandard, 1, nil
		case "deep":
			return DepthDeep, 1, nil
		}
	case "elevated":
		if validBand(proposalBand) {
			return DepthDeep, 1, nil
		}
	case "critical":
		if validBand(proposalBand) {
			return DepthDeep, 2, nil
		}
	}
	return "", 0, fmt.Errorf("impl-review assurance: unknown join %q x %q", plannedAssurance, proposalBand)
}

func validBand(value string) bool {
	return value == "skip" || value == "standard" || value == "deep"
}
