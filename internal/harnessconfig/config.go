// Package harnessconfig parses the top-level `harness-config:` block on
// `.pyxis/config.yaml`. That block is the model catalog for every harness:
// which profile meets which requirement, and which native model and effort
// that profile launches. `.pyxis/config.local.yaml` overlays it for one machine.
//
// The block parses CLOSED (AP-6): unknown harness names, unknown roles, unknown
// profile fields, and malformed entries are errors. Sibling top-level blocks on
// the shared surface are skipped. Stdlib only.
package harnessconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
)

// Sentinel errors — part of the package contract.
var (
	ErrUnknownKey     = errors.New("harness-config: unknown key")
	ErrUnknownHarness = errors.New("harness-config: unknown harness")
	ErrMalformed      = errors.New("harness-config: malformed")
)

// Config is the parsed harness-config block. Obtain via Load or Parse.
type Config struct {
	harnesses map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec
}

type draftProfile struct {
	id     string
	fields map[string]string
}

// Load merges root/.pyxis/config.local.yaml over root/.pyxis/config.yaml and
// parses harness-config. Absent files or an absent block yield an empty Config.
func Load(root string) (Config, error) {
	b, err := configlayer.Load(root)
	if err != nil {
		return Config{}, fmt.Errorf("harness-config: load configuration: %w", err)
	}
	return Parse(b)
}

// LoadDefault reads the tracked repository catalog above the current
// directory. Machine-local overlays are applied by the caller that already
// merged configuration for a run; adapter tests and the registry see the
// shared catalog.
func LoadDefault(harness entity.Harness) (map[nativerouting.Role][]nativerouting.ProfileSpec, error) {
	root, err := reporoot.Find()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(root, configlayer.GlobalPath))
	if err != nil {
		return nil, fmt.Errorf("harness-config: read %s: %w", configlayer.GlobalPath, err)
	}
	cfg, err := Parse(b)
	if err != nil {
		return nil, err
	}
	profiles := cfg.Profiles(harness)
	if len(profiles) == 0 {
		return nil, fmt.Errorf("harness-config: %s has no model profiles", harness)
	}
	return profiles, nil
}

// ResolveProfiles returns an explicit catalog, or the repository default when
// the caller did not supply one.
func ResolveProfiles(harness entity.Harness, explicit bool, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (map[nativerouting.Role][]nativerouting.ProfileSpec, error) {
	if !explicit {
		return LoadDefault(harness)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("harness-config: %s has no model profiles", harness)
	}
	return nativerouting.CloneProfiles(profiles), nil
}

// Profiles returns a copy of one harness's role catalogs. An absent harness
// returns nil so the caller can fall back to the repository default.
func (c Config) Profiles(harness entity.Harness) map[nativerouting.Role][]nativerouting.ProfileSpec {
	return nativerouting.CloneProfiles(c.harnesses[harness])
}

// Parse validates the closed harness-config grammar over cfg bytes.
//
//	harness-config:
//	  <harness>:                 # claude | codex | cursor | opencode
//	    implementation:          # and review
//	      <profile-id>:
//	        model: <native model>
//	        effort: <native effort>
//	        capability: economy|standard|strong|frontier
//	        deliberation: light|standard|deep|maximum
//	        context: standard|large|extended
//	        assurance: ordinary|elevated|critical
//	        cost_rank: <positive integer>
//	        strongest: true      # exactly one profile in the role
func Parse(cfg []byte) (Config, error) {
	out := Config{harnesses: map[entity.Harness]map[nativerouting.Role][]nativerouting.ProfileSpec{}}

	const (
		stOutside = iota
		stBlock
		stHarness
		stRole
		stProfile
	)
	state := stOutside
	var curName entity.Harness
	var curRole nativerouting.Role
	var roles map[nativerouting.Role][]nativerouting.ProfileSpec
	var cur draftProfile

	flushProfile := func(lineNo int) error {
		if cur.id == "" {
			return nil
		}
		spec, err := finishProfile(lineNo, cur)
		if err != nil {
			return err
		}
		for _, existing := range roles[curRole] {
			if existing.ID == spec.ID {
				return fmt.Errorf("%w: line %d: duplicate profile %q", ErrMalformed, lineNo, spec.ID)
			}
		}
		roles[curRole] = append(roles[curRole], spec)
		cur = draftProfile{}
		return nil
	}
	flushRole := func(lineNo int) error {
		if curRole == "" {
			return nil
		}
		if err := flushProfile(lineNo); err != nil {
			return err
		}
		if err := validateRole(lineNo, curName, curRole, roles[curRole]); err != nil {
			return err
		}
		curRole = ""
		return nil
	}
	flushHarness := func(lineNo int) error {
		if curName == "" {
			return nil
		}
		if err := flushRole(lineNo); err != nil {
			return err
		}
		if len(roles) == 0 {
			return fmt.Errorf("%w: line %d: harness %s has no roles", ErrMalformed, lineNo, curName)
		}
		out.harnesses[curName] = roles
		curName, roles = "", nil
		return nil
	}

	for i, raw := range strings.Split(strings.ReplaceAll(string(cfg), "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineNo := i + 1
		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.ContainsRune(lead, '\t') {
			return Config{}, fmt.Errorf("%w: line %d: tab in indentation (use spaces)", ErrMalformed, lineNo)
		}
		indent := len(lead)

		if indent == 0 {
			key, val, ok := splitKV(trimmed)
			if !ok {
				return Config{}, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformed, lineNo, trimmed)
			}
			if state != stOutside {
				if err := flushHarness(lineNo); err != nil {
					return Config{}, err
				}
			}
			state = stOutside
			if key != "harness-config" {
				continue
			}
			if val != "" {
				return Config{}, fmt.Errorf("%w: line %d: harness-config is a block, not a scalar", ErrMalformed, lineNo)
			}
			state = stBlock
			continue
		}
		if state == stOutside {
			continue
		}
		if indent%2 != 0 || indent > 8 {
			return Config{}, fmt.Errorf("%w: line %d: unexpected indentation %d", ErrMalformed, lineNo, indent)
		}

		switch indent {
		case 2:
			if err := flushHarness(lineNo); err != nil {
				return Config{}, err
			}
			key, val, ok := splitKV(trimmed)
			if !ok || val != "" {
				return Config{}, fmt.Errorf("%w: line %d: expected harness name block under harness-config", ErrMalformed, lineNo)
			}
			parsed, err := entity.ParseHarness(key)
			if err != nil {
				return Config{}, fmt.Errorf("%w: line %d: %q (expected claude, codex, cursor, or opencode)", ErrUnknownHarness, lineNo, key)
			}
			if _, dup := out.harnesses[parsed]; dup {
				return Config{}, fmt.Errorf("%w: line %d: duplicate harness %q", ErrMalformed, lineNo, key)
			}
			curName = parsed
			roles = map[nativerouting.Role][]nativerouting.ProfileSpec{}
			state = stHarness
		case 4:
			if curName == "" {
				return Config{}, fmt.Errorf("%w: line %d: dangling key under harness-config", ErrMalformed, lineNo)
			}
			if err := flushRole(lineNo); err != nil {
				return Config{}, err
			}
			key, val, ok := splitKV(trimmed)
			if !ok {
				return Config{}, fmt.Errorf("%w: line %d: expected implementation or review block", ErrMalformed, lineNo)
			}
			role := nativerouting.Role(key)
			if role != nativerouting.RoleImplementation && role != nativerouting.RoleReview {
				return Config{}, fmt.Errorf("%w: line %d: %q under harness %q (expected implementation or review)", ErrUnknownKey, lineNo, key, curName)
			}
			if val != "" {
				return Config{}, fmt.Errorf("%w: line %d: expected implementation or review block", ErrMalformed, lineNo)
			}
			if _, dup := roles[role]; dup {
				return Config{}, fmt.Errorf("%w: line %d: duplicate role %q", ErrMalformed, lineNo, key)
			}
			roles[role] = nil
			curRole = role
			state = stRole
		case 6:
			if curRole == "" {
				return Config{}, fmt.Errorf("%w: line %d: profile outside a role", ErrMalformed, lineNo)
			}
			if err := flushProfile(lineNo); err != nil {
				return Config{}, err
			}
			key, val, ok := splitKV(trimmed)
			if !ok || val != "" {
				return Config{}, fmt.Errorf("%w: line %d: expected a profile block", ErrMalformed, lineNo)
			}
			cur = draftProfile{id: key, fields: map[string]string{}}
			state = stProfile
		case 8:
			if cur.id == "" {
				return Config{}, fmt.Errorf("%w: line %d: profile field outside a profile", ErrMalformed, lineNo)
			}
			key, val, ok := splitKV(trimmed)
			if !ok || val == "" {
				return Config{}, fmt.Errorf("%w: line %d: profile field %q needs a value", ErrMalformed, lineNo, key)
			}
			if _, dup := cur.fields[key]; dup {
				return Config{}, fmt.Errorf("%w: line %d: duplicate field %q", ErrMalformed, lineNo, key)
			}
			switch key {
			case "model", "effort", "capability", "deliberation", "context", "assurance", "cost_rank", "strongest":
				cur.fields[key] = val
			default:
				return Config{}, fmt.Errorf("%w: line %d: %q under profile %q", ErrUnknownKey, lineNo, key, cur.id)
			}
		}
	}
	if err := flushHarness(0); err != nil {
		return Config{}, err
	}
	return out, nil
}

func finishProfile(lineNo int, cur draftProfile) (nativerouting.ProfileSpec, error) {
	need := func(key string) (string, error) {
		val := strings.TrimSpace(cur.fields[key])
		if val == "" {
			return "", fmt.Errorf("%w: line %d: profile %q requires %s", ErrMalformed, lineNo, cur.id, key)
		}
		return val, nil
	}
	model, err := need("model")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	effort, err := need("effort")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	if strings.ContainsAny(model, " \t\r\n\x00") || strings.ContainsAny(effort, " \t\r\n\x00") {
		return nativerouting.ProfileSpec{}, fmt.Errorf("%w: line %d: profile %q model and effort must be single tokens", ErrMalformed, lineNo, cur.id)
	}
	capability, err := need("capability")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	deliberation, err := need("deliberation")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	context, err := need("context")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	assurance, err := need("assurance")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	rankText, err := need("cost_rank")
	if err != nil {
		return nativerouting.ProfileSpec{}, err
	}
	rank, convErr := strconv.Atoi(rankText)
	if convErr != nil || rank <= 0 {
		return nativerouting.ProfileSpec{}, fmt.Errorf("%w: line %d: profile %q cost_rank must be a positive integer", ErrMalformed, lineNo, cur.id)
	}
	provides := nativerouting.Provides{
		Capability:   executiondemand.Capability(capability),
		Deliberation: executiondemand.Deliberation(deliberation),
		Context:      executiondemand.Context(context),
		Assurance:    executiondemand.Assurance(assurance),
	}
	if !validLevel(capability, "economy", "standard", "strong", "frontier") ||
		!validLevel(deliberation, "light", "standard", "deep", "maximum") ||
		!validLevel(context, "standard", "large", "extended") ||
		!validLevel(assurance, "ordinary", "elevated", "critical") {
		return nativerouting.ProfileSpec{}, fmt.Errorf("%w: line %d: profile %q has an unknown requirement level", ErrMalformed, lineNo, cur.id)
	}
	strongest := false
	if raw, ok := cur.fields["strongest"]; ok {
		parsed, convErr := strconv.ParseBool(raw)
		if convErr != nil {
			return nativerouting.ProfileSpec{}, fmt.Errorf("%w: line %d: profile %q strongest must be true or false", ErrMalformed, lineNo, cur.id)
		}
		strongest = parsed
	}
	return nativerouting.ProfileSpec{
		ID: cur.id, Model: model, Effort: effort, Provides: provides, CostRank: rank, Strongest: strongest,
	}, nil
}

func validateRole(lineNo int, harness entity.Harness, role nativerouting.Role, specs []nativerouting.ProfileSpec) error {
	if len(specs) == 0 {
		return fmt.Errorf("%w: line %d: %s %s has no profiles", ErrMalformed, lineNo, harness, role)
	}
	ranks := map[int]string{}
	strongest := 0
	for _, spec := range specs {
		if prior, dup := ranks[spec.CostRank]; dup {
			return fmt.Errorf("%w: line %d: %s %s cost_rank %d is used by %s and %s", ErrMalformed, lineNo, harness, role, spec.CostRank, prior, spec.ID)
		}
		ranks[spec.CostRank] = spec.ID
		if spec.Strongest {
			strongest++
		}
	}
	if strongest != 1 {
		return fmt.Errorf("%w: line %d: %s %s needs exactly one strongest profile", ErrMalformed, lineNo, harness, role)
	}
	return nil
}

func validLevel(got string, allowed ...string) bool {
	return slices.Contains(allowed, got)
}

func splitKV(s string) (key, val string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:i])
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	return key, strings.TrimSpace(s[i+1:]), true
}
