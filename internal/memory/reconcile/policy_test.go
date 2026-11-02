package reconcile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestPolicyAddressesAndRoundTrip(t *testing.T) {
	a := BaselineAllowance{Code: "gap<&", Scope: "docs/\"résumé\"", Feature: "x", Count: 2, Blocking: true, Writable: true}
	name, err := AllowancePath(a)
	if err != nil {
		t.Fatal(err)
	}
	// This literal locks the byte-level addressing contract, including escaping.
	sum := sha256.Sum256([]byte(`["memory-debt-address/v1","gap\u003c\u0026","docs/\"résumé\"","x"]`))
	if name != PolicyDirectory+"/"+hex.EncodeToString(sum[:])+".json" {
		t.Fatalf("address = %s", name)
	}
	changed := a
	changed.Count = 1
	changed.Blocking = false
	changed.Writable = false
	if got, _ := AllowancePath(changed); got != name {
		t.Fatal("mutable policy changed its address")
	}
	for _, allowances := range [][]BaselineAllowance{{}, {a}} {
		p := PolicySet{Schema: PolicySchema, Generator: Generator, Allowances: allowances}
		files, err := MarshalPolicySet(p)
		if err != nil {
			t.Fatal(err)
		}
		marker := files[PolicyPath]
		delete(files, PolicyPath)
		decoded, err := ParsePolicySet(marker, files)
		if err != nil || !reflect.DeepEqual(p, decoded) {
			t.Fatalf("round trip: %+v, %v", decoded, err)
		}
		if bytes.Contains(marker, []byte("sha256")) || bytes.Contains(marker, []byte("allowances")) {
			t.Fatal("marker contains derived state")
		}
	}
}

func TestPolicyClosedParser(t *testing.T) {
	a := BaselineAllowance{Code: "gap", Scope: "scope", Count: 1, Blocking: true, Writable: true}
	name, _ := AllowancePath(a)
	body, _ := json.Marshal(a)
	marker := `{"schema_version":"memory-reconciliation-baseline/v2","generator":"memory-reconcile/v3"}`
	for _, tc := range []struct{ name, marker, body, path string }{
		{name: "unknown marker key", marker: strings.TrimSuffix(marker, "}") + `,"extra":0}`},
		{name: "duplicate version", marker: strings.TrimSuffix(marker, "}") + `,"schema_version":"memory-reconciliation-baseline/v2"}`},
		{name: "marker trailing value", marker: marker + ` {}`},
		{name: "unknown evaluator", marker: strings.ReplaceAll(marker, "memory-reconcile/v3", "memory-reconcile/v99")},
		{name: "old evaluator in v2", marker: strings.ReplaceAll(marker, "memory-reconcile/v3", "memory-reconcile/v1")},
		{name: "unsupported schema", marker: strings.ReplaceAll(marker, "baseline/v2", "baseline/v99")},
		{name: "member duplicate", body: `{"code":"gap","scope":"scope","count":1,"blocking":true,"writable":true,"count":2}`},
		{name: "member trailing", body: string(body) + ` {}`},
		{name: "member unknown", body: strings.TrimSuffix(string(body), "}") + `,"extra":0}`},
		{name: "case alias", body: strings.ReplaceAll(string(body), "blocking", "Blocking")},
		{name: "missing severity", body: `{"code":"gap","scope":"scope","count":1,"writable":true}`},
		{name: "null writable", body: strings.ReplaceAll(string(body), `"writable":true`, `"writable":null`)},
		{name: "null feature", body: strings.TrimSuffix(string(body), "}") + `,"feature":null}`},
		{name: "zero count", body: strings.ReplaceAll(string(body), `"count":1`, `"count":0`)},
		{name: "negative count", body: strings.ReplaceAll(string(body), `"count":1`, `"count":-1`)},
		{name: "fractional count", body: strings.ReplaceAll(string(body), `"count":1`, `"count":1.5`)},
		{name: "wrong address", body: strings.ReplaceAll(string(body), "scope", "other")},
		{name: "wrong filename", path: PolicyDirectory + "/" + strings.Repeat("0", 64) + ".json"},
		{name: "uppercase filename", path: PolicyDirectory + "/" + strings.Repeat("A", 64) + ".json"},
		{name: "subdirectory", path: PolicyDirectory + "/nested/" + strings.Repeat("a", 64) + ".json"},
		{name: "nonmember", path: PolicyDirectory + "/README.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, b, p := marker, string(body), name
			if tc.marker != "" {
				m = tc.marker
			}
			if tc.body != "" {
				b = tc.body
			}
			if tc.path != "" {
				p = tc.path
			}
			if _, err := ParsePolicySet([]byte(m), map[string][]byte{p: []byte(b)}); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
	legacy := NewBaseline(Report{Generator: Generator, SourceClosureSHA256: strings.Repeat("a", 64), CodeInventorySHA256: strings.Repeat("b", 64), MemoryClosureSHA256: strings.Repeat("c", 64)})
	v1, err := MarshalBaseline(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePolicySet(v1, map[string][]byte{name: body}); err == nil {
		t.Fatal("mixed formats accepted")
	}
	legacy.Generator = "memory-reconcile/v99"
	v1, err = MarshalBaseline(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePolicySet(v1, nil); err == nil {
		t.Fatal("unknown legacy evaluator accepted")
	}
	for _, bad := range []PolicySet{
		{},
		{Schema: PolicySchema, Generator: Generator},
		{Schema: PolicySchema, Generator: Generator, Allowances: []BaselineAllowance{{Code: "a\x00b", Scope: "scope", Count: 1}}},
		{Schema: PolicySchema, Generator: Generator, Allowances: []BaselineAllowance{a, a}},
		{Schema: PolicySchema, Generator: Generator, Allowances: []BaselineAllowance{{Code: "a", Scope: "s", Count: math.MaxInt}, {Code: "b", Scope: "s", Count: 1}}},
	} {
		if _, err := MarshalPolicySet(bad); err == nil {
			t.Fatal("invalid normalized policy accepted")
		}
	}
	if _, err := ParsePolicySet([]byte(`{"schema_version":"memory-reconciliation-baseline/v1"}`), nil); err == nil {
		t.Fatal("invalid legacy input accepted")
	}
}

func TestPolicyPreservesLegacyRatchetAndPhaseAdvances(t *testing.T) {
	for _, pair := range [][2]string{{"missing-prd-memory", "unserved-prd-memory"}, {"missing-design-target-memory", "missing-serves-memory"}, {"unmapped-prd-unit", "open-prd-question"}} {
		oldReport := Report{Generator: Generator, Findings: []Finding{{Code: pair[0], Scope: "s", Feature: "f", Blocking: true, Writable: true}}}
		newReport := Report{Generator: Generator, Findings: []Finding{{Code: pair[1], Scope: "s", Feature: "f", Blocking: true, Writable: false}}}
		prior, next := NewPolicySet(oldReport), NewPolicySet(newReport)
		if CompareBaselines(prior.Normalized(), next.Normalized()).Blocking() {
			t.Fatal("recognized one-way phase advance refused")
		}
		if !CompareBaselines(next.Normalized(), prior.Normalized()).Blocking() {
			t.Fatal("phase reversal accepted")
		}
		both := newReport
		both.Findings = append(append([]Finding{}, newReport.Findings...), oldReport.Findings...)
		if !CompareBaseline(prior.Normalized(), both).Blocking() {
			t.Fatal("coexisting predecessor accepted")
		}
		grown := newReport
		grown.Findings = append(append([]Finding{}, newReport.Findings...), newReport.Findings...)
		if !CompareBaseline(prior.Normalized(), grown).Blocking() {
			t.Fatal("growing phase advance accepted")
		}
	}
	seed := Report{Generator: Generator, Findings: []Finding{{Code: "a", Scope: "first", Writable: true}, {Code: "b", Scope: "second", Writable: true}}}
	before, _ := MarshalPolicySet(NewPolicySet(seed))
	for index := range 2 {
		reduced := seed
		reduced.Findings = []Finding{seed.Findings[1-index]}
		after, _ := MarshalPolicySet(NewPolicySet(reduced))
		deleted := maps.Clone(before)
		for name, body := range after {
			if !bytes.Equal(before[name], body) {
				t.Fatal("unaffected member changed")
			}
			delete(deleted, name)
		}
		if len(deleted) != 1 {
			t.Fatalf("reduction touches %d members", len(deleted))
		}
		if !CompareBaseline(NewPolicySet(reduced).Normalized(), seed).Blocking() {
			t.Fatal("retired debt returned")
		}
	}
}
