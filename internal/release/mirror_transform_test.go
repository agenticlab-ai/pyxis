package release

import (
	"errors"
	"strings"
	"testing"
)

// The retarget transform: every source-repo reference becomes the dist
// repo, and a reference already naming the dist repo survives untouched —
// the corruption case a bare global replacement fails (dist contains
// source as a prefix, so "o/r-dist" would degrade to "o/r-dist-dist").
func TestTransformInstallSH(t *testing.T) {
	const src, dist = "agenticlab-ai/pyxis", "agenticlab-ai/pyxis-dist"
	cases := []struct{ name, in, want string }{
		{"plain references", `REPO="agenticlab-ai/pyxis"` + "\n# gh api repos/agenticlab-ai/pyxis/contents/install.sh\n",
			`REPO="agenticlab-ai/pyxis-dist"` + "\n# gh api repos/agenticlab-ai/pyxis-dist/contents/install.sh\n"},
		{"existing dist reference survives", "# the mirror lives at agenticlab-ai/pyxis-dist\nREPO=\"agenticlab-ai/pyxis\"\n",
			"# the mirror lives at agenticlab-ai/pyxis-dist\nREPO=\"agenticlab-ai/pyxis-dist\"\n"},
		{"no references", "PINNED_VERSION=\"v1.2.3\"\n", "PINNED_VERSION=\"v1.2.3\"\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := transformInstallSH(c.in, src, dist); got != c.want {
				t.Errorf("transform = %q, want %q", got, c.want)
			}
		})
	}
}

// Repository distribution must fail closed on a malformed listing or a
// refused update, without replacing an installer's bytes on uncertain reads.
func TestBoundInstallerListingFailures(t *testing.T) {
	cases := []coverageGHResult{
		{err: errors.New("transport refused")},
		{stdout: "{"},
		{stdout: `{"encoding":"utf8"}`},
		{stdout: `{"encoding":"base64","content":"%%%"}`},
		{code: 1, stderr: "forbidden"},
	}
	for i, result := range cases {
		_, err := refreshDistInstallSH(t.Context(), MirrorOptions{Tag: "v1.2.3", DistRepo: "owner/dist", Runner: &coverageGHRunner{results: []coverageGHResult{result}}}, "wanted")
		if err == nil {
			t.Errorf("listing failure %d succeeded", i)
		}
	}
	putFailure := &coverageGHRunner{results: []coverageGHResult{{code: 1, stderr: "not found"}, {code: 1, stderr: "write refused"}}}
	if _, err := refreshDistInstallSH(t.Context(), MirrorOptions{Tag: "v1.2.3", DistRepo: "owner/dist", Runner: putFailure}, "wanted"); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatalf("put failure = %v", err)
	}
}
