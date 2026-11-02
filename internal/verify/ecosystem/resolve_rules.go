package ecosystem

import (
	"fmt"
	"regexp"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// ResolveSecretPatterns returns the effective secret-pattern set for the
// secret-scan check: the compiled ecosystem defaults plus any
// manifest-declared extra patterns — the manifest extension point (design
// codegate-v1 §3.7/§3.10). A manifest pattern that fails to compile is
// returned as an error rather than silently dropped (a silently-dropped
// pattern is a fail-open detector); UnmarshalManifest already rejects this
// at parse time, so this is a defense-in-depth re-check for callers that
// construct a Manifest directly.
func (d *Defaults) ResolveSecretPatterns(manifestExtra []string) ([]*regexp.Regexp, error) {
	if len(manifestExtra) == 0 {
		return d.SecretPatterns, nil
	}
	resolved := make([]*regexp.Regexp, 0, len(d.SecretPatterns)+len(manifestExtra))
	resolved = append(resolved, d.SecretPatterns...)
	for _, p := range manifestExtra {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("manifest secret_scan.extra_patterns %q: %w", p, err)
		}
		resolved = append(resolved, re)
	}
	return resolved, nil
}

// ResolveForbiddenArtifactRules returns the effective forbidden-artifact
// rules: the ecosystem ceiling and absolute-path patterns, overridden or
// extended by the manifest's forbidden_artifacts extension point. A nil cfg
// (no manifest declaration) returns the ecosystem defaults unmodified.
func (d *Defaults) ResolveForbiddenArtifactRules(cfg *schema.ForbiddenArtifactsConfig) (ArtifactRules, error) {
	resolved := ArtifactRules{
		MaxFileBytes:         d.ForbiddenArtifacts.MaxFileBytes,
		AbsolutePathPatterns: d.ForbiddenArtifacts.AbsolutePathPatterns,
	}
	if cfg == nil {
		return resolved, nil
	}
	if cfg.MaxFileBytes > 0 {
		resolved.MaxFileBytes = cfg.MaxFileBytes
	}
	if len(cfg.ExtraAbsolutePathPatterns) > 0 {
		extra := make([]*regexp.Regexp, 0, len(cfg.ExtraAbsolutePathPatterns))
		for _, p := range cfg.ExtraAbsolutePathPatterns {
			re, err := regexp.Compile(p)
			if err != nil {
				return ArtifactRules{}, fmt.Errorf("manifest forbidden_artifacts.extra_absolute_path_patterns %q: %w", p, err)
			}
			extra = append(extra, re)
		}
		combined := make([]*regexp.Regexp, 0, len(resolved.AbsolutePathPatterns)+len(extra))
		combined = append(combined, resolved.AbsolutePathPatterns...)
		combined = append(combined, extra...)
		resolved.AbsolutePathPatterns = combined
	}
	return resolved, nil
}
