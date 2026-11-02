package packet

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestSessionLogMigrationCannotDeleteDurableMemory(t *testing.T) {
	envelope, err := os.ReadFile("../sessionlog/testdata/empty.json")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../sessionlog/testdata/empty.md")
	if err != nil {
		t.Fatal(err)
	}
	var imported record.Record
	if err := json.Unmarshal(envelope, &imported); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"approved pair", "body only", "envelope only", "changed base", "referenced", "requirement", "new empty import"} {
		t.Run(name, func(t *testing.T) {
			ancestor := newTree(t)
			ancestor.files[imported.Path()] = envelope
			ancestor.files[imported.BodyPath()] = body
			if name == "requirement" {
				ancestor.add(record.Promise, "prd/widget", "FR-1", "Keep this requirement")
			}
			if name == "new empty import" {
				r := imported
				r.Statement = "Another empty import"
				b, err := record.Marshal(&r)
				if err != nil {
					t.Fatal(err)
				}
				ancestor.files[r.Path()] = b
			}
			base, head := ancestor.clone(), ancestor.clone()
			delete(head.files, imported.Path())
			delete(head.files, imported.BodyPath())
			switch name {
			case "body only":
				head.files[imported.Path()] = envelope
			case "envelope only":
				head.files[imported.BodyPath()] = body
			case "changed base":
				base.files[imported.Path()] = append(append([]byte(nil), envelope...), ' ')
			case "requirement":
				for p := range head.files {
					if strings.HasPrefix(p, "prd/") {
						delete(head.files, p)
					}
				}
			}
			as, bs, hs := ancestor.set(), base.set(), head.set()
			if name == "referenced" {
				bs.To[imported.ID] = []store.Edge{{}}
			}
			p := Compute(Input{Ancestor: as, Base: bs, Head: hs, Policy: policy.Default(), BasePolicy: policy.Default()})
			blocked := strings.Contains(codes(p.Blockers), "record-deleted")
			if blocked != (name != "approved pair") {
				t.Fatalf("blockers = %s", p.Summary())
			}
			if name == "approved pair" && !p.OK() {
				t.Fatal(p.Summary())
			}
		})
	}
}
