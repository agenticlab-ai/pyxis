// Package policy reads memory/policy.json: the few knobs the memory needs from
// its operators. Consequence tiers per record kind, the subjects whose changes
// always count as high, when a question counts as already asked, and the
// default context budget. The file parses closed; a missing file is the
// default policy.
package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// Schema is the policy document version.
const Schema = 1

// Policy is the parsed document.
type Policy struct {
	Schema      int         `json:"schema"`
	Consequence Consequence `json:"consequence"`
	Ask         Ask         `json:"ask"`
	Pack        Pack        `json:"pack"`
}

// Consequence says how much scrutiny a change earns.
type Consequence struct {
	ByKind              map[string]string `json:"by_kind"`               // record kind → low | medium | high
	HighSubjectPrefixes []string          `json:"high_subject_prefixes"` // subjects whose changes are always high
	ManyClaims          int               `json:"many_claims"`           // a change moving at least this many claims goes up one tier (0: never)
}

// Ask says when a question counts as already asked.
type Ask struct {
	Similarity float64 `json:"similarity"` // 0..1 share of shared words
}

// Pack says how much context an agent gets by default.
type Pack struct {
	DefaultBudgetTokens int `json:"default_budget_tokens"`
}

// Default is the policy used when no file exists.
func Default() Policy {
	return Policy{
		Schema: Schema,
		Consequence: Consequence{
			ByKind: map[string]string{
				record.Need: record.Low, record.Promise: record.Medium, record.Expectation: record.Medium, record.Decision: record.Medium,
				record.Assumption: record.Low, record.Policy: record.High, record.Document: record.Low, record.Learning: record.Low, record.Plan: record.Low,
				record.Question: record.Low, record.Answer: record.Low, record.Revocation: record.Medium,
			},
			HighSubjectPrefixes: []string{"governance/", "security/"},
			ManyClaims:          10,
		},
		Ask:  Ask{Similarity: 0.6},
		Pack: Pack{DefaultBudgetTokens: 12000},
	}
}

// Parse reads a policy document; nil bytes mean the default. Unknown keys and
// unknown tiers are errors.
func Parse(b []byte) (Policy, error) {
	if b == nil {
		return Default(), nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("policy.json: %w", err)
	}
	if p.Schema != Schema {
		return Policy{}, fmt.Errorf("policy.json: schema %d is not %d", p.Schema, Schema)
	}
	d := Default()
	if p.Consequence.ByKind == nil {
		p.Consequence.ByKind = d.Consequence.ByKind
	}
	for k, tier := range p.Consequence.ByKind {
		if !record.IsKind(k) {
			return Policy{}, fmt.Errorf("policy.json: consequence.by_kind names unknown kind %q", k)
		}
		if Rank(tier) < 0 {
			return Policy{}, fmt.Errorf("policy.json: consequence.by_kind[%s] = %q is not low, medium or high", k, tier)
		}
	}
	if p.Consequence.HighSubjectPrefixes == nil {
		p.Consequence.HighSubjectPrefixes = d.Consequence.HighSubjectPrefixes
	}
	if p.Ask.Similarity == 0 {
		p.Ask.Similarity = d.Ask.Similarity
	}
	if p.Ask.Similarity < 0 || p.Ask.Similarity > 1 {
		return Policy{}, fmt.Errorf("policy.json: ask.similarity %v is not within 0..1", p.Ask.Similarity)
	}
	if p.Pack.DefaultBudgetTokens == 0 {
		p.Pack.DefaultBudgetTokens = d.Pack.DefaultBudgetTokens
	}
	if p.Pack.DefaultBudgetTokens < 0 {
		return Policy{}, fmt.Errorf("policy.json: pack.default_budget_tokens is negative")
	}
	return p, nil
}

// Marshal renders the policy as the file bytes.
func (p Policy) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	return append(b, '\n'), nil
}

// Rank orders tiers: low 0, medium 1, high 2; unknown -1.
func Rank(tier string) int {
	switch tier {
	case record.Low:
		return 0
	case record.Medium:
		return 1
	case record.High:
		return 2
	}
	return -1
}

// Max returns the higher of two tiers.
func Max(a, b string) string {
	if Rank(b) > Rank(a) {
		return b
	}
	return a
}

// Raise moves a tier up one step.
func Raise(tier string) string {
	switch tier {
	case record.Low:
		return record.Medium
	default:
		return record.High
	}
}

// ForRecord is the consequence of one record: its kind's tier, raised to high
// on a high subject, never lower than what the author declared.
func (p Policy) ForRecord(r *record.Record) string {
	tier := p.Consequence.ByKind[r.Kind]
	if tier == "" {
		tier = record.Low
	}
	for _, prefix := range p.Consequence.HighSubjectPrefixes {
		if strings.HasPrefix(r.Subject, prefix) {
			tier = record.High
		}
	}
	return Max(tier, r.Consequence)
}
