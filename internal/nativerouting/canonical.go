package nativerouting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func canonicalControls(in []NativeControl) []NativeControl {
	out := append([]NativeControl(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func canonicalQualification(q Qualification) Qualification {
	q.Controls = canonicalControls(q.Controls)
	q.Claims = append([]QualificationClaim(nil), q.Claims...)
	sort.Slice(q.Claims, func(i, j int) bool {
		if q.Claims[i].Component != q.Claims[j].Component {
			return q.Claims[i].Component < q.Claims[j].Component
		}
		left, _ := json.Marshal(q.Claims[i])
		right, _ := json.Marshal(q.Claims[j])
		return string(left) < string(right)
	})
	q.Validity.Invalidations = append([]Invalidation(nil), q.Validity.Invalidations...)
	sort.Slice(q.Validity.Invalidations, func(i, j int) bool {
		if q.Validity.Invalidations[i].At.Equal(q.Validity.Invalidations[j].At) {
			return q.Validity.Invalidations[i].EventID < q.Validity.Invalidations[j].EventID
		}
		return q.Validity.Invalidations[i].At.Before(q.Validity.Invalidations[j].At)
	})
	return q
}

func canonicalCatalog(c Catalog) Catalog {
	c.Entries = append([]CatalogEntry(nil), c.Entries...)
	for i := range c.Entries {
		c.Entries[i].Controls = canonicalControls(c.Entries[i].Controls)
	}
	sort.Slice(c.Entries, func(i, j int) bool { return c.Entries[i].NativeProfileID < c.Entries[j].NativeProfileID })
	return c
}

func digestJSON(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// QualificationDigest binds an entry to the complete canonical evidence
// artifact, independent of claim, control, invalidation, or JSON field order.
func QualificationDigest(q Qualification) string {
	return digestJSON(canonicalQualification(q))
}

// PolicyIdentity binds the canonical catalog and every qualification it
// references. Collection ordering never affects the result.
func PolicyIdentity(c Catalog, qualifications QualificationSet) string {
	byID := make(map[string]Qualification, len(qualifications))
	for _, q := range qualifications {
		byID[q.QualificationID] = canonicalQualification(q)
	}
	referenced := make([]Qualification, 0, len(c.Entries))
	for _, entry := range canonicalCatalog(c).Entries {
		if q, ok := byID[entry.QualificationID]; ok {
			referenced = append(referenced, q)
		}
	}
	sort.Slice(referenced, func(i, j int) bool { return referenced[i].QualificationID < referenced[j].QualificationID })
	return digestJSON(struct {
		Catalog        Catalog          `json:"catalog"`
		Qualifications QualificationSet `json:"qualifications"`
	}{canonicalCatalog(c), referenced})
}
