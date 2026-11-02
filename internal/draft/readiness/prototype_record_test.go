package readiness

import (
	"os"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestPrototypeRecordCompilesAndFindingCitationLeavesReadinessUnchanged(t *testing.T) {
	baseline := readyFiles(t)
	baselineVerdict := judgeFiles(t, baseline)
	if !baselineVerdict.Ready || len(baselineVerdict.Unmet) != 0 {
		t.Fatalf("baseline fixture is not ready: %+v", baselineVerdict.Unmet)
	}

	files := readyFiles(t)
	prototypeRecord, err := os.ReadFile("testdata/prototype-record.md")
	if err != nil {
		t.Fatal(err)
	}
	files["brainstorm/001.md"] = prototypeRecord
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\nThe selected layout handles brainstorm:widget#L-1.\n")...)
	refreshReviews(t, files)

	compiled, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatalf("compile draft with prototype record: %v", err)
	}
	foundRecord := false
	for _, entry := range compiled.Records {
		if entry.Draft.Kind == record.Document && entry.Draft.Subject == "brainstorm/widget" && reflect.DeepEqual(entry.Draft.Body, prototypeRecord) {
			foundRecord = true
			break
		}
	}
	if !foundRecord {
		t.Fatal("compiled draft does not contain the prototype record")
	}
	if !citationResolved(compiled, Sources{}, nil, gate.Citation{Grammar: "brainstorm", Payload: "widget#L-1"}) {
		t.Fatal("brainstorm citation to the first prototype finding did not resolve")
	}
	verdict := Judge(compiled, Sources{})
	if !verdict.Ready || !reflect.DeepEqual(verdict.Unmet, baselineVerdict.Unmet) {
		t.Fatalf("prototype record changed draft readiness: baseline=%+v with record=%+v", baselineVerdict.Unmet, verdict.Unmet)
	}
}
