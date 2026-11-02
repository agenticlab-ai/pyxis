package reconcile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
)

const (
	PolicySchema    = "memory-reconciliation-baseline/v2"
	PolicyPath      = ".github/memory-reconciliation-baseline.json"
	PolicyDirectory = ".github/memory-reconciliation-baseline"
)

// PolicySet separates accepted debt from the identities of an invocation's
// inputs. Legacy retains the v1 binding until an explicit data-only migration.
type PolicySet struct {
	Schema     string
	Generator  string
	Allowances []BaselineAllowance
	Legacy     *Baseline
}

type policyMarker struct {
	Schema    string `json:"schema_version"`
	Generator string `json:"generator"`
}

// AllowancePath addresses a finding, never its mutable count or severity.
func AllowancePath(a BaselineAllowance) (string, error) {
	if err := validateAllowance(a); err != nil {
		return "", err
	}
	body, err := json.Marshal([4]string{"memory-debt-address/v1", a.Code, a.Scope, a.Feature})
	if err != nil {
		return "", fmt.Errorf("encode allowance address: %w", err)
	}
	sum := sha256.Sum256(body)
	return PolicyDirectory + "/" + hex.EncodeToString(sum[:]) + ".json", nil
}

// PolicyMemberPath admits only direct, fully addressed members of the fixed root.
func PolicyMemberPath(name string) bool {
	return path.Dir(name) == PolicyDirectory && strings.HasSuffix(name, ".json") && validDigest(strings.TrimSuffix(path.Base(name), ".json"))
}

// ParsePolicySet reads both formats without fallback. The caller verifies Git
// modes and directory entries before supplying regular member blobs.
func ParsePolicySet(marker []byte, members map[string][]byte) (PolicySet, error) {
	var version struct {
		Schema string `json:"schema_version"`
	}
	if err := json.Unmarshal(marker, &version); err != nil {
		return PolicySet{}, fmt.Errorf("decode policy version: %w", err)
	}
	if version.Schema == BaselineSchema {
		if len(members) != 0 {
			return PolicySet{}, fmt.Errorf("mixed v1 baseline and v2 policy members; use an explicit migration")
		}
		b, err := ParseBaseline(marker)
		if err != nil {
			return PolicySet{}, err
		}
		if !knownLegacyGenerator(b.Generator) {
			return PolicySet{}, fmt.Errorf("unsupported baseline evaluator %q; install a reader with an explicit compatibility mapping", b.Generator)
		}
		return PolicySet{Schema: b.Schema, Generator: b.Generator, Allowances: b.Allowances, Legacy: &b}, nil
	}
	var m policyMarker
	if err := decodePolicyObject(marker, map[string]bool{"schema_version": true, "generator": true}, []string{"schema_version", "generator"}, &m); err != nil {
		return PolicySet{}, fmt.Errorf("decode policy marker: %w", err)
	}
	if m.Schema != PolicySchema {
		return PolicySet{}, fmt.Errorf("unsupported policy schema_version %q", m.Schema)
	}
	if !knownPolicyGenerator(m.Generator) {
		return PolicySet{}, fmt.Errorf("unsupported v2 policy evaluator %q; install a reader with an explicit compatibility mapping", m.Generator)
	}
	p := PolicySet{Schema: m.Schema, Generator: m.Generator, Allowances: []BaselineAllowance{}}
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !PolicyMemberPath(name) {
			return PolicySet{}, fmt.Errorf("unexpected policy member %q", name)
		}
		var a BaselineAllowance
		keys := map[string]bool{"code": true, "scope": true, "feature": true, "count": true, "blocking": true, "writable": true}
		if err := decodePolicyObject(members[name], keys, []string{"code", "scope", "count", "blocking", "writable"}, &a); err != nil {
			return PolicySet{}, fmt.Errorf("policy member %s: %w", name, err)
		}
		want, err := AllowancePath(a)
		if err != nil {
			return PolicySet{}, fmt.Errorf("policy member %s: %w", name, err)
		}
		if name != want {
			return PolicySet{}, fmt.Errorf("policy member %s has an inconsistent address; expected %s", name, want)
		}
		p.Allowances = append(p.Allowances, a)
	}
	sort.Slice(p.Allowances, func(i, j int) bool { return compareAllowanceAddress(p.Allowances[i], p.Allowances[j]) < 0 })
	if err := p.validate(); err != nil {
		return PolicySet{}, err
	}
	return p, nil
}

func knownLegacyGenerator(g string) bool {
	return g == "memory-reconcile/v1" || g == "memory-reconcile/v2" || knownPolicyGenerator(g)
}

func knownPolicyGenerator(g string) bool {
	return g == "memory-reconcile/v3" || g == "memory-reconcile/v4" || g == "memory-reconcile/v5" || g == "memory-reconcile/v6" || g == Generator
}

func decodePolicyObject(body []byte, allowed map[string]bool, required []string, dst any) error {
	keys := func(location string) map[string]bool {
		if location == "$" {
			return allowed
		}
		return map[string]bool{}
	}
	if err := scanJSONValueKeys(json.NewDecoder(bytes.NewReader(body)), "$", 0, keys); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := consumeBaselineJSONEOF(dec); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	for _, key := range required {
		if raw, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("required field %q is missing or null", key)
		}
	}
	if raw, ok := fields["feature"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("feature must be a string")
	}
	return nil
}

func validateAllowance(a BaselineAllowance) error {
	if a.Code == "" || a.Scope == "" || a.Count < 1 {
		return fmt.Errorf("allowance has an empty address or non-positive count")
	}
	if strings.ContainsRune(a.Code+a.Scope+a.Feature, '\x00') {
		return fmt.Errorf("allowance address contains a NUL byte")
	}
	return nil
}

func (p PolicySet) validate() error {
	if p.Schema != PolicySchema || !knownPolicyGenerator(p.Generator) || p.Legacy != nil {
		return fmt.Errorf("v2 policy requires schema %s and evaluator %s", PolicySchema, Generator)
	}
	if p.Allowances == nil {
		return fmt.Errorf("policy allowances must be a set, not null")
	}
	total := 0
	for i, a := range p.Allowances {
		if err := validateAllowance(a); err != nil {
			return err
		}
		if i > 0 && compareAllowanceAddress(p.Allowances[i-1], a) >= 0 {
			return fmt.Errorf("policy allowances are repeated or not in canonical address order")
		}
		if a.Count > math.MaxInt-total {
			return fmt.Errorf("policy allowance count overflows")
		}
		total += a.Count
	}
	return nil
}

// Normalized reuses the existing exact/monotonic comparison without inventing
// global input hashes for v2. Only a legacy policy has those stored identities.
func (p PolicySet) Normalized() Baseline {
	if p.Legacy != nil {
		return *p.Legacy
	}
	b := Baseline{Schema: p.Schema, Generator: p.Generator, Allowances: p.Allowances}
	for _, a := range p.Allowances {
		b.FindingCount += a.Count
	}
	return b
}

// NewPolicySet captures the report's exact inventory under its evaluator.
func NewPolicySet(report Report) PolicySet {
	return PolicySet{Schema: PolicySchema, Generator: report.Generator, Allowances: allowancesFor(report.Findings)}
}

// MarshalPolicySet emits only policy, never aggregate identities or a manifest.
func MarshalPolicySet(p PolicySet) (map[string][]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	marker, err := json.MarshalIndent(policyMarker{p.Schema, p.Generator}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode policy marker: %w", err)
	}
	out := map[string][]byte{PolicyPath: append(marker, '\n')}
	for _, a := range p.Allowances {
		name, err := AllowancePath(a)
		if err != nil {
			return nil, err
		}
		body, err := json.MarshalIndent(a, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode policy member: %w", err)
		}
		out[name] = append(body, '\n')
	}
	return out, nil
}
