package lease

import (
	"crypto/sha1"
	"encoding/hex"
	"os/exec"
	"testing"
)

func TestCoverage90ConfigurationAndHelperBranches(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(Config{WorkDir: t.TempDir(), Git: gitBin, Holder: "holder", RefPrefix: "refs/custom"})
	if err != nil {
		t.Fatal(err)
	}
	if l.refPrefix != "refs/custom/" || l.Holder() != "holder" {
		t.Fatalf("lease defaults = prefix %q holder %q", l.refPrefix, l.Holder())
	}
	out, stderr, err := l.git(t.Context(), "payload", "hash-object", "--stdin")
	digest := sha1.Sum([]byte("blob 7\x00payload"))
	if err != nil || out != hex.EncodeToString(digest[:])+"\n" || stderr != "" {
		t.Fatalf("git stdin = out %q stderr %q err %v", out, stderr, err)
	}
	if got := firstLine(" \n\t"); got != "(no output)" {
		t.Fatalf("firstLine(blank) = %q", got)
	}
}
