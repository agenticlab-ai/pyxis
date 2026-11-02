package drivetogreen

import (
	"regexp"
	"strings"
	"testing"
)

// The redaction drill (ABUSE-3): a seeded secret in a failed-CI log must be
// scrubbed before the log could reach a fixer. Each case names a secret shape and
// a substring that must not survive.
func TestRedactor_ScrubsSeededSecrets(t *testing.T) {
	r := NewRedactor(0, nil)
	cases := []struct {
		name    string
		log     string
		leaked  string // the secret substring that must be gone
		keepKey string // a key/prefix that should be preserved (optional)
	}{
		{
			name:   "github classic token",
			log:    "fatal: could not read Password for 'https://ghp_abcdEFGH0123456789ijklMNOPqrstUVWX@github.com'",
			leaked: "ghp_abcdEFGH0123456789ijklMNOPqrstUVWX",
		},
		{
			name:   "github fine-grained PAT",
			log:    "using token github_pat_11ABCDEFG0aBcDeFgHiJkL_mNoPqRsTuVwXyZ0123456789",
			leaked: "github_pat_11ABCDEFG0aBcDeFgHiJkL_mNoPqRsTuVwXyZ0123456789",
		},
		{
			name:    "aws secret env assignment",
			log:     "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY\nnext line",
			leaked:  "wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY",
			keepKey: "AWS_SECRET_ACCESS_KEY",
		},
		{
			name:   "aws access key id",
			log:    "detected key AKIAIOSFODNN7EXAMPLE in the environment",
			leaked: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:    "authorization header with a bearer token",
			log:     "> Authorization: Bearer eyJhbGciOiJimSInR5cCImaJ9.payloadpayload.signaturesig",
			leaked:  "eyJhbGciOiJimSInR5cCImaJ9.payloadpayload.signaturesig",
			keepKey: "Authorization",
		},
		{
			name:    "standalone bearer token",
			log:     "retrying with bearer sk-live-0123456789abcdefXYZ token",
			leaked:  "sk-live-0123456789abcdefXYZ",
			keepKey: "bearer",
		},
		{
			name:    "generic TOKEN assignment with quotes",
			log:     `CI_JOB_TOKEN: "s3cr3t-tokenvalue-should-vanish"`,
			leaked:  "s3cr3t-tokenvalue-should-vanish",
			keepKey: "CI_JOB_TOKEN",
		},
		{
			name:    "url query token",
			log:     "GET https://api.example.com/v1/thing?access_token=deadbeefcafef00d1234 HTTP/2",
			leaked:  "deadbeefcafef00d1234",
			keepKey: "access_token=",
		},
		{
			name:   "url userinfo password",
			log:    "cloning https://ci-bot:sup3rSecretPassw0rd@git.example.com/repo.git",
			leaked: "sup3rSecretPassw0rd",
		},
		{
			name:   "PEM private key block",
			log:    "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAA\nSECRETKEYMATERIAL\n-----END OPENSSH PRIVATE KEY-----",
			leaked: "SECRETKEYMATERIAL",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.Redact(tc.log)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("secret survived redaction:\n input:  %q\n output: %q\n leaked: %q", tc.log, got, tc.leaked)
			}
			if !strings.Contains(got, redactionMark) {
				t.Errorf("expected a %s marker where the secret was scrubbed: %q", redactionMark, got)
			}
			if tc.keepKey != "" && !strings.Contains(got, tc.keepKey) {
				t.Errorf("the key/prefix %q should survive so the fixer knows which value leaked: %q", tc.keepKey, got)
			}
		})
	}
}

// Redaction must NOT scrub genuinely useful build context — commit SHAs, file
// paths, and stack frames — or the fixer loses the signal it needs. The default
// set is shape-based, not blanket high-entropy scrubbing.
func TestRedactor_KeepsUsefulContext(t *testing.T) {
	r := NewRedactor(0, nil)
	log := strings.Join([]string{
		"commit a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4",
		"--- FAIL: TestWidget (0.01s)",
		"    widget_test.go:42: got 3, want 4",
		"internal/widget/widget.go:17 +0x1a",
	}, "\n")
	got := r.Redact(log)
	for _, keep := range []string{
		"a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4",
		"widget_test.go:42",
		"internal/widget/widget.go:17",
		"got 3, want 4",
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("useful context %q was scrubbed: %q", keep, got)
		}
	}
	if strings.Contains(got, redactionMark) {
		t.Errorf("nothing secret-shaped here; no redaction expected: %q", got)
	}
}

// Clamp bounds an oversized log to about the budget, keeping the head and the tail
// (where the error lands) with an elision marker between.
func TestRedactor_ClampsOversizeLog(t *testing.T) {
	const max = 512
	r := NewRedactor(max, nil)
	head := "HEAD-MARKER first line of the build log\n"
	tail := "\nTAIL-MARKER --- FAIL: the actual error is here"
	big := head + strings.Repeat("noise line filler filler filler\n", 400) + tail

	got := r.Redact(big)
	if len(got) > max*2 { // marker + snapping adds a little; must stay bounded
		t.Fatalf("clamped log length %d is not bounded near max %d", len(got), max)
	}
	if len(got) >= len(big) {
		t.Fatalf("clamp did not shrink the log: got %d, input %d", len(got), len(big))
	}
	if !strings.Contains(got, "HEAD-MARKER") {
		t.Error("clamp dropped the head; the invocation/context should survive")
	}
	if !strings.Contains(got, "TAIL-MARKER") {
		t.Error("clamp dropped the tail; the error region should survive")
	}
	if !strings.Contains(got, "clamped") {
		t.Error("clamp must mark the elision")
	}
}

// A secret sitting in the elided middle must not survive clamping: redaction runs
// over the WHOLE input before the middle is dropped (order matters).
func TestRedactor_ScrubsBeforeClamp(t *testing.T) {
	const max = 256
	r := NewRedactor(max, nil)
	secret := "ghp_middleSECRETtoken0123456789abcdefZZZZ"
	big := strings.Repeat("head filler line\n", 40) +
		"leaked: " + secret + "\n" +
		strings.Repeat("tail filler line\n", 40)

	got := r.Redact(big)
	if strings.Contains(got, secret) {
		t.Fatalf("a secret in the clamped-away middle survived: %q", got)
	}
}

// A short log is returned unchanged (no marker), and an empty log stays empty.
func TestRedactor_ShortAndEmpty(t *testing.T) {
	r := NewRedactor(0, nil)
	if got := r.Redact(""); got != "" {
		t.Errorf("Redact(empty) = %q, want empty", got)
	}
	short := "--- FAIL: TestX\n  x.go:1: boom\n"
	if got := r.Redact(short); got != short {
		t.Errorf("Redact(short) = %q, want unchanged %q", got, short)
	}
}

// Repo-specific extra patterns are honored alongside the built-ins.
func TestRedactor_ExtraPatterns(t *testing.T) {
	extra := regexp.MustCompile(`ACME-[0-9A-F]{12}`)
	r := NewRedactor(0, []*regexp.Regexp{extra})
	got := r.Redact("license ACME-0123456789AB expired")
	if strings.Contains(got, "ACME-0123456789AB") {
		t.Fatalf("extra pattern not applied: %q", got)
	}
	if !strings.Contains(got, redactionMark) {
		t.Errorf("expected redaction mark: %q", got)
	}
}

func TestClampHelpersCoverLineAndUTF8Boundaries(t *testing.T) {
	if got := snapForward("first\npartial"); got != "first" {
		t.Fatalf("snapForward = %q", got)
	}
	if got := snapBackward("partial\nlast"); got != "last" {
		t.Fatalf("snapBackward = %q", got)
	}
	if got := tailBytes("short", 10); got != "short" {
		t.Fatalf("tailBytes within budget = %q", got)
	}
	if got := tailBytes("a€z", 3); got != "z" {
		t.Fatalf("tailBytes split UTF-8 rune = %q, want z", got)
	}
	if !utf8Start('a') || utf8Start(0x80) {
		t.Fatal("utf8Start classified a leading or continuation byte incorrectly")
	}
	if got := clamp("a€z", 2); got != "z" {
		t.Fatalf("tiny clamp = %q, want z", got)
	}
	_ = NewRedactor(1, []*regexp.Regexp{nil})
}
