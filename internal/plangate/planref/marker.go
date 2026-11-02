// Package planref validates the marker and reference grammar shared by plan-scoped trackers.
package planref

import (
	"fmt"
	"regexp"
	"strings"
)

var markerRE = regexp.MustCompile(`<!-- pyxis:plan-task (\S+) -->`)
var segmentRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var taskIDRE = regexp.MustCompile(`^T[0-9]+$`)

// ParseMarker reads the plan-local identity written by the plan projection.
func ParseMarker(brief string) (feature, version, taskID string, ok bool) {
	m := markerRE.FindStringSubmatch(brief)
	if m == nil {
		return "", "", "", false
	}
	parts := strings.Split(m[1], "/")
	if len(parts) != 3 || !segmentRE.MatchString(parts[0]) || !segmentRE.MatchString(parts[1]) || !taskIDRE.MatchString(parts[2]) {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// ParsePlanRef validates a closed plan reference, <feature>/<NNN>.
func ParsePlanRef(ref string) (feature, version string, err error) {
	ref = strings.TrimSpace(ref)
	feature, version, _, ok := ParseMarker("<!-- pyxis:plan-task " + ref + "/T1 -->")
	if !ok || feature+"/"+version != ref || !densePlanVersion(version) {
		return "", "", fmt.Errorf("invalid plan reference %q (want <feature>/<NNN>)", ref)
	}
	return feature, version, nil
}

// InPlan reports whether a task's marker names the given plan.
func InPlan(brief, feature, version string) bool {
	f, v, _, ok := ParseMarker(brief)
	return ok && f == feature && v == version
}

func densePlanVersion(version string) bool {
	if len(version) != 3 {
		return false
	}
	for _, r := range version {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
