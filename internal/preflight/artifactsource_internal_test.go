package preflight

import (
	"errors"
	"strings"
	"testing"
)

// The public default needs no credential, so its refusal must not send a new
// user to `gh auth login`, and no refusal names a retired credential.
func TestRefuseArtifactSourceNamesTheRemedyForItsHost(t *testing.T) {
	for _, tc := range []struct {
		name, url, want string
	}{
		{"public GitHub", "https://github.com/agenticlab-ai/pyxis/releases/download/v1.2.3/release-manifest-v1.2.3.json", "a public release needs no credential"},
		{"mirror", "https://mirror.example/v1.2.3/release-manifest-v1.2.3.json", "bind it as a release connection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := refuseArtifactSource("release-manifest-v1.2.3.json", tc.url, errors.New("offline"))
			if out.Status != StatusRefuse || !strings.Contains(out.Remedy, tc.want) {
				t.Fatalf("outcome = %+v, want a refusal containing %q", out, tc.want)
			}
			if strings.Contains(out.Remedy, "gh auth login") {
				t.Fatalf("remedy demands a gh login: %q", out.Remedy)
			}
		})
	}
}
