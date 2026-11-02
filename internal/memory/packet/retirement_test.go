package packet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestBookkeepingRetirementProtectsRetainedKnowledge(t *testing.T) {
	const oldAnchor = "Need-01M26FGPM9DGT3K9Z3YG65YD6J"
	const latestAnchor = "Need-01M31W585PD5F0KTWARECF191Y"
	const summary = "Learning-01M26FGS3Z116P1CXJ9B46S6GZ"
	const withdrawn = "Learning-01M3B2Z6JDSFNED71V03YY7KB8"
	for _, name := range []string{
		"complete cleanup", "keep referring summary", "keep predecessor", "keep revoked target",
		"changed base", "changed preimage", "arbitrary requirement", "new code inventory",
		"concurrent relation", "textual reference", "document embed", "strip reference",
	} {
		t.Run(name, func(t *testing.T) {
			ancestor := retirementTree(t)
			original := ancestor.set()
			if name == "changed preimage" {
				path := original.Paths[oldAnchor]
				ancestor.files[path] = append(append([]byte(nil), ancestor.files[path]...), '\n')
			}
			if name == "strip reference" {
				addRetirementReference(ancestor, oldAnchor, true)
			}
			if name == "arbitrary requirement" {
				ancestor.add(record.Promise, "prd/retained", "FR-1", "Preserve a product obligation")
			}
			if name == "new code inventory" {
				r := *original.Get(oldAnchor)
				r.ID = record.NewID(record.Need, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), [10]byte{1})
				r.Subject = "code/another-inventory"
				body, err := record.Marshal(&r)
				if err != nil {
					t.Fatal(err)
				}
				ancestor.files[r.Path()] = body
			}
			base, head := ancestor.clone(), newTree(t)
			switch name {
			case "keep referring summary":
				path := original.Paths[summary]
				head.files[path] = ancestor.files[path]
			case "keep predecessor":
				path := original.Paths[oldAnchor]
				head.files[path] = ancestor.files[path]
			case "keep revoked target":
				path := original.Paths[withdrawn]
				head.files[path] = ancestor.files[path]
			case "changed base":
				path := original.Paths[oldAnchor]
				base.files[path] = append(append([]byte(nil), base.files[path]...), '\n')
			case "concurrent relation":
				addRetirementReference(base, latestAnchor, true)
			case "textual reference":
				addRetirementReference(base, oldAnchor, false)
			case "document embed":
				base.doc("guide/retained", "manual", "Keep the reference", "{{code/unit-scripts-cut-pyxis-release.sh-61de6e44d147#anchor}}\n")
			case "strip reference":
				for _, r := range ancestor.set().Records {
					if r.Subject == "learning/retained-reason" {
						copy := *r
						copy.Relations = nil
						copy.Statement = "A reference was stripped"
						body, err := record.Marshal(&copy)
						if err != nil {
							t.Fatal(err)
						}
						head.files[copy.Path()] = body
					}
				}
			}
			p := Compute(Input{Ancestor: ancestor.set(), Base: base.set(), Head: head.set(), Policy: policy.Default(), BasePolicy: policy.Default()})
			blocked := strings.Contains(codes(p.Blockers), "record-deleted")
			if blocked != (name != "complete cleanup") {
				t.Fatalf("unexpected deletion verdict: %s", p.Summary())
			}
			if name == "complete cleanup" && (!p.OK() || len(p.Deleted) != len(original.Records) || p.Consequence != record.High || !strings.Contains(codes(p.Warnings), "bookkeeping-retirement")) {
				t.Fatalf("complete approved deletion failed: %s", p.Summary())
			}
		})
	}
}

func retirementTree(t *testing.T) *tree {
	t.Helper()
	tr := newTree(t)
	files, err := os.ReadDir("testdata/bookkeeping")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join("testdata/bookkeeping", file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var r record.Record
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatal(err)
		}
		tr.files[r.Path()] = body
	}
	return tr
}

func addRetirementReference(tr *tree, target string, typed bool) {
	tr.t.Helper()
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	r := record.Record{
		Schema: record.Schema, ID: record.NewID(record.Learning, at, [10]byte{1}),
		Kind: record.Learning, Subject: "learning/retained-reason", Statement: "The evidence is " + target,
		Issuer: "test", CreatedAt: at.Format(time.RFC3339),
	}
	if typed {
		r.Statement = "A retained external constraint depends on this evidence"
		r.Relations = []record.Relation{{Type: record.DependsOn, Target: target}}
	}
	body, err := record.Marshal(&r)
	if err != nil {
		tr.t.Fatal(err)
	}
	tr.files[r.Path()] = body
}
