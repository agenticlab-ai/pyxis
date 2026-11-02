package ecosystem

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestResolveSecretPatterns(t *testing.T) {
	d := New()

	// No manifest extension -> the ecosystem set, unmodified.
	base, err := d.ResolveSecretPatterns(nil)
	if err != nil {
		t.Fatalf("ResolveSecretPatterns(nil): %v", err)
	}
	if len(base) != len(d.SecretPatterns) {
		t.Errorf("expected %d base patterns, got %d", len(d.SecretPatterns), len(base))
	}

	// A manifest extra pattern is compiled and appended.
	extended, err := d.ResolveSecretPatterns([]string{`internal-token-[0-9]+`})
	if err != nil {
		t.Fatalf("ResolveSecretPatterns(extra): %v", err)
	}
	if len(extended) != len(d.SecretPatterns)+1 {
		t.Errorf("expected %d patterns, got %d", len(d.SecretPatterns)+1, len(extended))
	}
	if !extended[len(extended)-1].MatchString("internal-token-42") {
		t.Error("manifest-extended pattern did not match its own fixture")
	}

	// A malformed manifest pattern is an error, not a silently-dropped detector.
	if _, err := d.ResolveSecretPatterns([]string{"("}); err == nil {
		t.Error("expected an error for a malformed manifest secret pattern")
	}
}

func TestResolveForbiddenArtifactRules(t *testing.T) {
	d := New()

	// nil cfg -> ecosystem defaults unmodified.
	base, err := d.ResolveForbiddenArtifactRules(nil)
	if err != nil {
		t.Fatalf("ResolveForbiddenArtifactRules(nil): %v", err)
	}
	if base.MaxFileBytes != d.ForbiddenArtifacts.MaxFileBytes {
		t.Errorf("MaxFileBytes = %d, want %d", base.MaxFileBytes, d.ForbiddenArtifacts.MaxFileBytes)
	}
	if len(base.AbsolutePathPatterns) != len(d.ForbiddenArtifacts.AbsolutePathPatterns) {
		t.Errorf("expected %d base absolute-path patterns, got %d", len(d.ForbiddenArtifacts.AbsolutePathPatterns), len(base.AbsolutePathPatterns))
	}

	// A manifest override raises the ceiling and adds a pattern.
	cfg := &schema.ForbiddenArtifactsConfig{
		MaxFileBytes:              2048,
		ExtraAbsolutePathPatterns: []string{`/opt/acme/`},
	}
	resolved, err := d.ResolveForbiddenArtifactRules(cfg)
	if err != nil {
		t.Fatalf("ResolveForbiddenArtifactRules(cfg): %v", err)
	}
	if resolved.MaxFileBytes != 2048 {
		t.Errorf("MaxFileBytes = %d, want 2048", resolved.MaxFileBytes)
	}
	if len(resolved.AbsolutePathPatterns) != len(d.ForbiddenArtifacts.AbsolutePathPatterns)+1 {
		t.Errorf("expected %d absolute-path patterns, got %d", len(d.ForbiddenArtifacts.AbsolutePathPatterns)+1, len(resolved.AbsolutePathPatterns))
	}
	found := false
	for _, p := range resolved.AbsolutePathPatterns {
		if p.MatchString("/opt/acme/secrets.env") {
			found = true
		}
	}
	if !found {
		t.Error("manifest-extended absolute-path pattern did not match its own fixture")
	}

	// A zero MaxFileBytes override (the field's absent value) leaves the
	// ecosystem ceiling untouched rather than disabling the check.
	resolved2, err := d.ResolveForbiddenArtifactRules(&schema.ForbiddenArtifactsConfig{})
	if err != nil {
		t.Fatalf("ResolveForbiddenArtifactRules(empty cfg): %v", err)
	}
	if resolved2.MaxFileBytes != d.ForbiddenArtifacts.MaxFileBytes {
		t.Errorf("empty override MaxFileBytes = %d, want ecosystem default %d", resolved2.MaxFileBytes, d.ForbiddenArtifacts.MaxFileBytes)
	}

	// A malformed manifest pattern is an error.
	if _, err := d.ResolveForbiddenArtifactRules(&schema.ForbiddenArtifactsConfig{ExtraAbsolutePathPatterns: []string{"["}}); err == nil {
		t.Error("expected an error for a malformed manifest absolute-path pattern")
	}

	// Resolving with an extension must never mutate the shared ecosystem
	// defaults: the base set's length must be exactly what it was before any
	// resolve call, even after several manifest-extended resolves ran.
	originalLen := len(d.ForbiddenArtifacts.AbsolutePathPatterns)
	if _, err := d.ResolveForbiddenArtifactRules(cfg); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if len(d.ForbiddenArtifacts.AbsolutePathPatterns) != originalLen {
		t.Errorf("shared ecosystem defaults mutated: now %d patterns, want %d", len(d.ForbiddenArtifacts.AbsolutePathPatterns), originalLen)
	}
}
