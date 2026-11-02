package conformancetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// GenerateDomain crosses every live registered harness, both closed roles,
// and every profile the supplied derivation policy can produce. Input order is
// discarded before identities or cells are created.
func GenerateDomain(policy executiondemand.DerivationPolicy, registry RuntimeRegistry, roles Roles) (Domain, error) {
	if registry == nil {
		return Domain{}, errors.New("native routing conformance: runtime registry is required")
	}
	policyIdentity, err := executiondemand.DerivationPolicyDigest(policy)
	if err != nil {
		return Domain{}, fmt.Errorf("native routing conformance: derivation policy: %w", err)
	}
	profiles, err := executiondemand.DerivableProfiles(policy)
	if err != nil {
		return Domain{}, fmt.Errorf("native routing conformance: derive profiles: %w", err)
	}
	harnesses, err := canonicalStrings(harnessNames(registry.Harnesses()))
	if err != nil {
		return Domain{}, fmt.Errorf("native routing conformance: live harness domain: %w", err)
	}
	if len(harnesses) == 0 {
		return Domain{}, errors.New("native routing conformance: live harness domain is empty")
	}
	canonicalRoles, err := canonicalRoles(roles)
	if err != nil {
		return Domain{}, err
	}
	if len(profiles) == 0 {
		return Domain{}, errors.New("native routing conformance: derivation policy produced no profiles")
	}

	domain := Domain{
		SchemaVersion: DomainSchema, PolicyVersion: policy.Version, PolicyDigest: policyIdentity,
		Harnesses: harnesses, Roles: canonicalRoles, Profiles: profiles,
		Cells: make([]Cell, 0, len(harnesses)*len(canonicalRoles)*len(profiles)),
	}
	for _, harness := range harnesses {
		for _, role := range canonicalRoles {
			for _, profile := range profiles {
				profileID := SemanticProfileID(profile)
				domain.Cells = append(domain.Cells, Cell{
					CellID:  harness + "/" + string(role) + "/" + profileID,
					Harness: entity.Harness(harness), Role: role, SemanticProfileID: profileID, RequestedProfile: profile,
				})
			}
		}
	}
	domain.DomainIdentity = contentIdentity(domainWithoutIdentity(domain))
	if err := ValidateDomain(domain); err != nil {
		return Domain{}, err
	}
	return domain, nil
}

func SemanticProfileID(profile executiondemand.ProfileMinimum) string {
	return strings.Join([]string{string(profile.Capability), string(profile.Deliberation), string(profile.Context), string(profile.Assurance)}, ".")
}

func harnessNames(values []entity.Harness) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.String()
	}
	return out
}

func canonicalStrings(values []string) ([]string, error) {
	out := append([]string(nil), values...)
	sort.Strings(out)
	for i, value := range out {
		if value == "" {
			return nil, errors.New("harness identity is empty")
		}
		if i > 0 && value == out[i-1] {
			return nil, fmt.Errorf("duplicate harness %q", value)
		}
	}
	return out, nil
}

func canonicalRoles(roles Roles) ([]nativerouting.Role, error) {
	out := append([]nativerouting.Role(nil), roles...)
	slices.Sort(out)
	if len(out) != 2 || out[0] != nativerouting.RoleImplementation || out[1] != nativerouting.RoleReview {
		return nil, errors.New("native routing conformance: roles must contain implementation and review exactly once")
	}
	return out, nil
}

func domainWithoutIdentity(domain Domain) Domain {
	domain.DomainIdentity = ""
	return domain
}

func contentIdentity(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("native routing conformance: canonical identity: %v", err))
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// ValidateDomain rejects any omitted, duplicated, foreign, or reordered cell.
func ValidateDomain(domain Domain) error {
	if domain.SchemaVersion != DomainSchema || domain.PolicyVersion == "" || domain.PolicyDigest == "" || domain.DomainIdentity == "" {
		return errors.New("native routing conformance: incomplete domain identity")
	}
	if domain.DomainIdentity != contentIdentity(domainWithoutIdentity(domain)) {
		return errors.New("native routing conformance: domain identity does not bind its content")
	}
	harnesses, err := canonicalStrings(domain.Harnesses)
	if err != nil || !equalStrings(harnesses, domain.Harnesses) {
		return errors.New("native routing conformance: harness domain is not canonical and unique")
	}
	roles, err := canonicalRoles(domain.Roles)
	if err != nil || !equalRoles(roles, domain.Roles) {
		return errors.New("native routing conformance: role domain is not canonical and complete")
	}
	if len(domain.Profiles) == 0 || len(domain.Cells) != len(harnesses)*len(roles)*len(domain.Profiles) {
		return errors.New("native routing conformance: profile or cell domain is incomplete")
	}
	seenProfiles := map[string]executiondemand.ProfileMinimum{}
	previousProfileID := ""
	for _, profile := range domain.Profiles {
		id := SemanticProfileID(profile)
		if _, duplicate := seenProfiles[id]; duplicate {
			return fmt.Errorf("native routing conformance: duplicate semantic profile %q", id)
		}
		if previousProfileID != "" && id < previousProfileID {
			return errors.New("native routing conformance: semantic profile domain is not canonical")
		}
		seenProfiles[id] = profile
		previousProfileID = id
	}
	cellIndex := 0
	for _, harness := range harnesses {
		for _, role := range roles {
			for _, profile := range domain.Profiles {
				id := SemanticProfileID(profile)
				want := Cell{CellID: harness + "/" + string(role) + "/" + id, Harness: entity.Harness(harness), Role: role, SemanticProfileID: id, RequestedProfile: profile}
				if domain.Cells[cellIndex] != want {
					return fmt.Errorf("native routing conformance: cell %d is omitted, duplicated, or out of canonical order", cellIndex)
				}
				cellIndex++
			}
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalRoles(a, b []nativerouting.Role) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
