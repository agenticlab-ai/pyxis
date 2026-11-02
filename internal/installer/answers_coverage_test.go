package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestAnswersGrammarCoversClosedForms(t *testing.T) {
	valid := []byte("tracker: github\nci_mode: both\nwiring: { repo: use, protection: required-checks }\ncli_surfaces: [claude, codex]\nargus: false\ngit_name: Ada\ngit_email: ada@example.com\ncoordinate: owner/repo\n")
	ans, err := ParseAnswers(valid)
	if err != nil || ans.Tracker != "github" || ans.WiringProtection != "required-checks" || len(ans.CLISurfaces) != 2 || ans.Argus == nil || *ans.Argus {
		t.Fatalf("ParseAnswers(valid) = %+v, %v", ans, err)
	}
	path := filepath.Join(t.TempDir(), "answers.yaml")
	if err := os.WriteFile(path, valid, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAnswersFile(path); err != nil {
		t.Fatalf("LoadAnswersFile(valid): %v", err)
	}
	if _, err := LoadAnswersFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("LoadAnswersFile(missing): want error")
	}

	invalid := []string{
		"  tracker: github",
		"tracker github",
		"wiring: block",
		"wiring: { repo use }",
		"wiring: { future: value }",
		"cli_surfaces: block",
		"cli_surfaces: [claude, ]",
		"argus: perhaps",
		"future: value",
	}
	for _, body := range invalid {
		if _, err := ParseAnswers([]byte(body)); err == nil {
			t.Errorf("ParseAnswers(%q) succeeded", body)
		}
	}
	if got, err := parseSurfacesFlow("[]"); err != nil || len(got) != 0 {
		t.Fatalf("parseSurfacesFlow(empty) = %v, %v", got, err)
	}
	if got, err := parseSurfacesFlow(surfaceAuto); err != nil || len(got) != 1 || got[0] != surfaceAuto {
		t.Fatalf("parseSurfacesFlow(auto) = %v, %v", got, err)
	}
}

func TestAnswersFlagHelpersCoverEmptyMalformedAndQuotedValues(t *testing.T) {
	if repo, protection, err := ParseWiringArg(""); err != nil || repo != "" || protection != "" {
		t.Fatalf("ParseWiringArg(empty) = %q %q %v", repo, protection, err)
	}
	if repo, protection, err := ParseWiringArg("repo=create,, protection=checks+review"); err != nil || repo != "create" || protection != "checks+review" {
		t.Fatalf("ParseWiringArg(valid) = %q %q %v", repo, protection, err)
	}
	for _, value := range []string{"repo", "future=value"} {
		if _, _, err := ParseWiringArg(value); err == nil {
			t.Errorf("ParseWiringArg(%q) succeeded", value)
		}
	}
	if SplitSurfacesArg("") != nil || strings.Join(SplitSurfacesArg("auto"), ",") != "auto" || strings.Join(SplitSurfacesArg("claude, codex"), ",") != "claude,codex" {
		t.Fatal("SplitSurfacesArg did not preserve its three forms")
	}
	if got := answersUnquote(` "Ada" # operator`); got != "Ada" {
		t.Fatalf("answersUnquote = %q", got)
	}
}

func TestWriteAdoptionConfigSuccessAndWriteFailure(t *testing.T) {
	pin := Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}
	target := t.TempDir()
	digest, err := WriteAdoptionConfig(target, pin, "https://example.invalid/release.tar.gz", strings.Repeat("b", 40))
	if err != nil || len(digest) != 64 {
		t.Fatalf("WriteAdoptionConfig = %q, %v", digest, err)
	}
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, ".pyxis"), []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAdoptionConfig(blocked, pin, "source", "commit"); err == nil {
		t.Fatal("WriteAdoptionConfig with blocked parent: want error")
	}
}

func TestWriteAdoptionConfigRejectsRendererParserDrift(t *testing.T) {
	original := parseAdoptionConfig
	parseAdoptionConfig = func([]byte) (config.Config, error) {
		return config.Config{}, errors.New("parser drift")
	}
	defer func() { parseAdoptionConfig = original }()
	pin := Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}
	if _, err := WriteAdoptionConfig(t.TempDir(), pin, "source", "commit"); err == nil {
		t.Fatal("WriteAdoptionConfig swallowed parser drift")
	}
}
