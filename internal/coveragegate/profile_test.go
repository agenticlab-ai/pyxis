package coveragegate

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMinimum(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  BasisPoints
		ok    bool
	}{
		{value: "0", want: 0, ok: true},
		{value: "85", want: 8500, ok: true},
		{value: "85.5", want: 8550, ok: true},
		{value: "99.99", want: 9999, ok: true},
		{value: "100.00", want: 10000, ok: true},
		{value: "", ok: false},
		{value: "-1", ok: false},
		{value: "85.001", ok: false},
		{value: "100.01", ok: false},
		{value: "wat", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMinimum(tt.value)
			if (err == nil) != tt.ok {
				t.Fatalf("ParseMinimum(%q) error = %v, want ok=%v", tt.value, err, tt.ok)
			}
			if err == nil && got != tt.want {
				t.Fatalf("ParseMinimum(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseProfileMergesDuplicateBlocksAsUnion(t *testing.T) {
	t.Parallel()
	profile := `mode: atomic
example.com/project/a/a.go:10.1,12.2 2 0
example.com/project/a/a.go:10.1,12.2 2 1
example.com/project/a/a.go:20.1,21.2 1 0
example.com/project/b/b.go:3.1,4.2 3 1
`
	report, err := ParseProfile(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	if report.Covered != 5 || report.Total != 6 {
		t.Fatalf("report = %d/%d, want 5/6", report.Covered, report.Total)
	}
	if len(report.Packages) != 2 {
		t.Fatalf("packages = %d, want 2", len(report.Packages))
	}
	if len(report.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(report.Files))
	}
	bottom := report.Bottom(1)
	if len(bottom) != 1 || bottom[0].Path != "example.com/project/a" || bottom[0].Covered != 2 || bottom[0].Total != 3 {
		t.Fatalf("bottom = %+v, want package a at 2/3", bottom)
	}
}

func TestReportMeetsUsesExactRatio(t *testing.T) {
	t.Parallel()
	report := Report{Covered: 17, Total: 20, Files: []File{{Path: "a.go", Covered: 17, Total: 20}}}
	if !report.Meets(8500) {
		t.Fatal("17/20 must meet 85%")
	}
	if report.Meets(8501) {
		t.Fatal("17/20 must not meet 85.01%")
	}
}

func TestReportRejectsAnUnderCoveredFileEvenWhenAggregatePasses(t *testing.T) {
	t.Parallel()
	report := Report{
		Covered: 180,
		Total:   200,
		Files: []File{
			{Path: "high.go", Package: "example.com/project", Covered: 95, Total: 100},
			{Path: "low.go", Package: "example.com/project", Covered: 85, Total: 100},
		},
	}
	if !report.Meets(8500) {
		t.Fatal("both files meet 85%")
	}
	report.Files[1].Covered = 84
	report.Covered = 179
	if report.Meets(8500) {
		t.Fatal("89.5% aggregate masked a file below 85%")
	}
	below := report.Below(8500)
	if len(below) != 1 || below[0].Path != "low.go" {
		t.Fatalf("below = %+v, want only low.go", below)
	}
	if bottom := report.BottomFiles(1); len(bottom) != 1 || bottom[0].Path != "low.go" {
		t.Fatalf("bottom files = %+v, want low.go", bottom)
	}
}

func TestReportWithoutDropsExcludedPackagesFromTheVerdict(t *testing.T) {
	t.Parallel()
	report, err := ParseProfile(strings.NewReader(`mode: set
example.com/project/kept/a.go:1.1,2.1 9 1
example.com/project/kept/a.go:3.1,4.1 1 0
example.com/project/kept/only_from_skipped.go:1.1,2.1 4 0
example.com/project/skipped/b.go:1.1,2.1 10 0
example.com/project/skipped/sub/c.go:1.1,2.1 10 1
`))
	if err != nil {
		t.Fatal(err)
	}
	if report.Meets(9000) {
		t.Fatal("the uncovered excluded files must fail the full report")
	}
	rest := report.Without([]string{"example.com/project/skipped", "example.com/project/kept/only_from_skipped.go"})
	want := Report{
		Covered:  19,
		Total:    20,
		Packages: []Package{{Path: "example.com/project/kept", Covered: 9, Total: 10}, {Path: "example.com/project/skipped/sub", Covered: 10, Total: 10}},
		Files: []File{
			{Path: "example.com/project/kept/a.go", Package: "example.com/project/kept", Covered: 9, Total: 10},
			{Path: "example.com/project/skipped/sub/c.go", Package: "example.com/project/skipped/sub", Covered: 10, Total: 10},
		},
	}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("without = %+v, want %+v", rest, want)
	}
	if !rest.Meets(9000) {
		t.Fatal("the remaining files meet 90%")
	}
	if all := report.Without(nil); !reflect.DeepEqual(all, report) {
		t.Fatalf("excluding nothing changed the report: %+v", all)
	}
	if none := report.Without([]string{"example.com/project/kept", "example.com/project/skipped", "example.com/project/skipped/sub"}); none.Total != 0 || none.Meets(0) {
		t.Fatalf("excluding every package must leave nothing to meet: %+v", none)
	}
}

func TestParseProfileFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []string{
		"",
		"mode: mystery\n",
		"example.com/a.go:1.1,1.2 1 1\n",
		"mode: set\nmode: set\n",
		"mode: set\nmalformed\n",
		"mode: set\nexample.com/a.go:1.1,1.2 nope 1\n",
		"mode: set\nexample.com/a.go:1.1,1.2 1 -1\n",
		"mode: set\n",
	}
	for _, profile := range tests {
		if _, err := ParseProfile(strings.NewReader(profile)); err == nil {
			t.Fatalf("ParseProfile(%q) succeeded, want failure", profile)
		}
	}
}
