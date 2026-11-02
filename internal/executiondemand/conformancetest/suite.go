// Package conformancetest owns the one capsule-media conformance harness
// every production tracker adapter must run (execution-demand-contract-v1
// §3.9). It is test support, not a tracker simulation: callers supply the
// real adapter boundary while this package fixes the origin corpus and the
// byte-preservation, authority, and tamper assertions.
package conformancetest

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

// RoundTrip carries an ordinary tracker body through one production medium's
// write/read boundary. Test doubles may replace only external process or
// network transport; the adapter's production serialization must execute.
type RoundTrip func(t *testing.T, body string) string

type goldenCapsule struct {
	CanonicalHex string `json:"canonical_hex"`
}

var fixtureFiles = []struct {
	name string
	file string
}{
	{"plan_v2", "golden_capsule_plan_v2.json"},
}

//go:embed testdata/*.json
var fixtureFS embed.FS

// Run executes the same closed origin corpus and assertions for one medium.
// Adding an origin or assertion here automatically expands every adapter's
// conformance proof.
func Run(t *testing.T, medium string, roundTrip RoundTrip) {
	t.Helper()
	for _, fixture := range loadFixtures(t) {
		t.Run(medium+"/"+fixture.name, func(t *testing.T) {
			want, err := executiondemand.Reconstruct(fixture.capsule.Origin)
			mustNoError(t, err, "reconstruct fixture %s: %v", fixture.name, err)
			body, err := executiondemand.Embed("task charter\n", fixture.capsule, want)
			mustNoError(t, err, "embed fixture %s: %v", fixture.name, err)
			got, err := executiondemand.Extract(roundTrip(t, body), want)
			mustNoError(t, err, "%s refused a capsule after its production round trip: %v", medium, err)
			mustCondition(t, reflect.DeepEqual(got, fixture.capsule), "%s altered the logical capsule:\ngot:  %+v\nwant: %+v", medium, got, fixture.capsule)
			mustNoError(t, executiondemand.VerifyCapsule(got, want), "%s capsule no longer matches frozen-source authority", medium)

			tampered := tamperMarker(t, body)
			_, err = executiondemand.Extract(roundTrip(t, tampered), want)
			mustCondition(t, err != nil, "%s silently accepted a tampered %s capsule", medium, fixture.name)
		})
	}
}

func loadFixtures(t *testing.T) []struct {
	name    string
	capsule executiondemand.Capsule
} {
	t.Helper()
	out := make([]struct {
		name    string
		capsule executiondemand.Capsule
	}, 0, len(fixtureFiles))
	for _, fixture := range fixtureFiles {
		raw, err := fixtureFS.ReadFile("testdata/" + fixture.file)
		mustNoError(t, err, "read %s: %v", fixture.file, err)
		var golden goldenCapsule
		mustNoError(t, json.Unmarshal(raw, &golden), "decode %s", fixture.file)
		wire, err := hex.DecodeString(golden.CanonicalHex)
		mustNoError(t, err, "decode canonical hex %s: %v", fixture.file, err)
		capsule, err := executiondemand.DecodeCapsuleCanonical(wire)
		mustNoError(t, err, "decode capsule %s: %v", fixture.file, err)
		out = append(out, struct {
			name    string
			capsule executiondemand.Capsule
		}{fixture.name, capsule})
	}
	return out
}

func tamperMarker(t *testing.T, body string) string {
	t.Helper()
	const markerPrefix = "<!-- pyxis:execution-demand "
	start := strings.Index(body, markerPrefix)
	mustCondition(t, start >= 0, "embedded body has no capsule marker")
	pos := start + len(markerPrefix)
	tampered := []byte(body)
	if tampered[pos] == 'A' {
		tampered[pos] = 'B'
	} else {
		tampered[pos] = 'A'
	}
	return string(tampered)
}

func mustNoError(t *testing.T, err error, format string, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf(format, args...)
	}
}

func mustCondition(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}
