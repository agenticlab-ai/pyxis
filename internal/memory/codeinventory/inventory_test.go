package codeinventory

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildClassifiesProductionUnits(t *testing.T) {
	entries := []Entry{
		{Path: "vendor/dependency/dep.go", Mode: "100644", Type: "blob"},
		{Path: "internal/alpha/z.go", Mode: "100644", Type: "blob"},
		{Path: "install.sh", Mode: "100755", Type: "blob"},
		{Path: "cmd/tool/main_test.go", Mode: "100644", Type: "blob"},
		{Path: ".pyxis/scripts/example.mjs", Mode: "100755", Type: "blob"},
		{Path: "pkg/public/public.go", Mode: "100644", Type: "blob"},
		{Path: "internal/alpha/a.go", Mode: "100755", Type: "blob"},
		{Path: "cmd/tool/main.go", Mode: "100644", Type: "blob"},
		{Path: "cmd/tool/generated/gen.go", Mode: "100644", Type: "blob"},
		{Path: "internal/alpha/testdata/fake.go", Mode: "100644", Type: "blob"},
		{Path: "internal/alpha/fixtures/fake.go", Mode: "100644", Type: "blob"},
		{Path: "internal/alpha/tests/helper.go", Mode: "100644", Type: "blob"},
		{Path: "scripts/tests/test-release.sh", Mode: "100755", Type: "blob"},
		{Path: ".pyxis/scripts/not-executable.sh", Mode: "100644", Type: "blob"},
		{Path: "README.md", Mode: "100644", Type: "blob"},
	}
	blobs := blobsFor(entries)

	got, err := Build(entries, blobs)
	if err != nil {
		t.Fatal(err)
	}
	wantShape := []Unit{
		{Kind: ExecutableScript, Path: ".pyxis/scripts/example.mjs", Files: []string{".pyxis/scripts/example.mjs"}, SHA256: "fbce00bd27c819717a913a8751df546ef75ce6c6a1760a45176a724ea6a40fbc"},
		{Kind: GoPackage, Path: "cmd/tool", Files: []string{"cmd/tool/main.go"}, SHA256: "25dc52e51a74ce0463d4280fc501e8c94b5b7c23ad4990d912009ae088e66855"},
		{Kind: ExecutableScript, Path: "install.sh", Files: []string{"install.sh"}, SHA256: "9124c37f776309fbe603630756d2c1ecdebaf9728aaff837c3325cc27e6ad537"},
		{Kind: GoPackage, Path: "internal/alpha", Files: []string{"internal/alpha/a.go", "internal/alpha/z.go"}, SHA256: "638d18cd18ee0537fe757fad0a084d4696a0978f2d697a34f8d96bde42bc07aa"},
		{Kind: GoPackage, Path: "pkg/public", Files: []string{"pkg/public/public.go"}, SHA256: "8a54b7621da47f165fae945e5deed49a575106b2ed2003f4c668defe8d9474e9"},
	}
	if len(got.Units) != len(wantShape) {
		t.Fatalf("units = %#v, want %d", got.Units, len(wantShape))
	}
	for i, want := range wantShape {
		unit := got.Units[i]
		if unit.Kind != want.Kind || unit.Path != want.Path || !reflect.DeepEqual(unit.Files, want.Files) || unit.SHA256 != want.SHA256 {
			t.Errorf("unit %d = %#v, want shape %#v", i, unit, want)
		}
		if len(unit.SHA256) != 64 {
			t.Errorf("unit %q SHA256 = %q", unit.Path, unit.SHA256)
		}
	}
	const wantAggregate = "dd78227835a2c5dac00ea03f911dc16dd2a7e1dc4ced9933a3426b96a11f8bc8"
	if got.AggregateSHA256 != wantAggregate {
		t.Fatalf("aggregate SHA256 = %q, want %q", got.AggregateSHA256, wantAggregate)
	}
}

func TestBuildIsOrderIndependentAndContentSensitive(t *testing.T) {
	entries := []Entry{
		{Path: "internal/widget/z.go", Mode: "100644", Type: "blob"},
		{Path: "tool.sh", Mode: "100755", Type: "blob"},
		{Path: "internal/widget/a.go", Mode: "100644", Type: "blob"},
		{Path: "internal/widget/a_test.go", Mode: "100644", Type: "blob"},
	}
	blobs := blobsFor(entries)
	first, err := Build(entries, blobs)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []Entry{entries[3], entries[2], entries[1], entries[0]}
	second, err := Build(reversed, blobs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("entry order changed inventory:\nfirst:  %#v\nsecond: %#v", first, second)
	}

	changedTest := cloneBlobs(blobs)
	changedTest["internal/widget/a_test.go"] = []byte("changed but excluded")
	third, err := Build(entries, changedTest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, third) {
		t.Fatalf("excluded test content changed inventory:\nfirst: %#v\nthird: %#v", first, third)
	}

	changedCode := cloneBlobs(blobs)
	changedCode["internal/widget/a.go"] = []byte("changed production code")
	fourth, err := Build(entries, changedCode)
	if err != nil {
		t.Fatal(err)
	}
	if fourth.AggregateSHA256 == first.AggregateSHA256 || fourth.Units[0].SHA256 == first.Units[0].SHA256 {
		t.Fatalf("production change did not move package and aggregate digests:\nfirst:  %#v\nfourth: %#v", first, fourth)
	}
	if fourth.Units[1].SHA256 != first.Units[1].SHA256 {
		t.Fatal("unrelated script digest moved with Go package content")
	}
}

func TestBuildIncludesSourcedZshAtRegularModes(t *testing.T) {
	const name = ".pyxis/scripts/startup/codex-worktree.zsh"
	digests := map[string]string{}
	for _, mode := range []string{"100644", "100755"} {
		t.Run(mode, func(t *testing.T) {
			entries := []Entry{{Path: name, Mode: mode, Type: "blob"}}
			got, err := Build(entries, blobsFor(entries))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{name}
			if len(got.Units) != 1 || got.Units[0].Kind != ExecutableScript || got.Units[0].Path != name || !reflect.DeepEqual(got.Units[0].Files, want) {
				t.Fatalf("zsh inventory = %#v, want one sourced-script unit for %q", got, name)
			}
			digests[mode] = got.Units[0].SHA256
		})
	}
	if len(digests) != 2 {
		t.Fatalf("zsh mode cases produced %d digests, want 2", len(digests))
	}
	if digests["100644"] == digests["100755"] {
		t.Fatal("zsh executable-bit change did not move the unit digest")
	}
}

func TestBuildLegacyV2ReproducesPreZshInventory(t *testing.T) {
	const zsh = ".pyxis/scripts/startup/codex-worktree.zsh"
	entries := []Entry{
		{Path: "internal/widget/widget.go", Mode: "100644", Type: "blob"},
		{Path: "tools/release.sh", Mode: "100755", Type: "blob"},
		{Path: zsh, Mode: "100644", Type: "blob"},
	}
	blobs := blobsFor(entries)

	legacy, err := BuildLegacyV2(entries, blobs)
	if err != nil {
		t.Fatal(err)
	}
	preZsh, err := Build(entries[:2], blobs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, preZsh) {
		t.Fatalf("legacy v2 inventory did not reproduce the exact pre-zsh closure:\nlegacy:  %#v\npre-zsh: %#v", legacy, preZsh)
	}

	current, err := Build(entries, blobs)
	if err != nil {
		t.Fatal(err)
	}
	if current.AggregateSHA256 == legacy.AggregateSHA256 || len(current.Units) != len(legacy.Units)+1 || current.Units[0].Path != zsh {
		t.Fatalf("current inventory did not add sourced zsh independently of legacy reproduction:\ncurrent: %#v\nlegacy:  %#v", current, legacy)
	}

	changed := cloneBlobs(blobs)
	changed[zsh] = []byte("changed sourced zsh\n")
	legacyChanged, err := BuildLegacyV2(entries, changed)
	if err != nil {
		t.Fatal(err)
	}
	currentChanged, err := Build(entries, changed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacyChanged, legacy) {
		t.Fatalf("zsh bytes changed a legacy v2 inventory:\nold: %#v\nnew: %#v", legacy, legacyChanged)
	}
	if currentChanged.AggregateSHA256 == current.AggregateSHA256 {
		t.Fatal("zsh bytes did not change the current inventory")
	}
}

func TestBuildExcludesZshFromNonProductionTrees(t *testing.T) {
	entries := []Entry{
		{Path: ".pyxis/scripts/tests/helper.zsh", Mode: "100644", Type: "blob"},
		{Path: ".pyxis/vendor/tool/startup.zsh", Mode: "100755", Type: "blob"},
		{Path: "internal/widget/fixtures/setup.zsh", Mode: "100644", Type: "blob"},
		{Path: "internal/widget/generated/setup.zsh", Mode: "100644", Type: "blob"},
		{Path: "internal/widget/testdata/setup.zsh", Mode: "100644", Type: "blob"},
	}
	got, err := Build(entries, blobsFor(entries))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != 0 {
		t.Fatalf("excluded zsh files produced inventory units: %#v", got.Units)
	}
}

func TestBuildFailsClosedOnDefectiveInScopeEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		blobs   map[string][]byte
		want    string
	}{
		{
			name:    "go symlink",
			entries: []Entry{{Path: "internal/widget/widget.go", Mode: "120000", Type: "blob"}},
			blobs:   map[string][]byte{"internal/widget/widget.go": []byte("elsewhere")},
			want:    "not a regular file",
		},
		{
			name:    "script submodule",
			entries: []Entry{{Path: "tools/run.sh", Mode: "160000", Type: "commit"}},
			blobs:   map[string][]byte{},
			want:    "not a regular file",
		},
		{
			name:    "sourced zsh symlink",
			entries: []Entry{{Path: ".pyxis/scripts/startup/codex-worktree.zsh", Mode: "120000", Type: "blob"}},
			blobs:   map[string][]byte{".pyxis/scripts/startup/codex-worktree.zsh": []byte("elsewhere")},
			want:    "not a regular file",
		},
		{
			name:    "missing blob",
			entries: []Entry{{Path: "pkg/widget/widget.go", Mode: "100644", Type: "blob"}},
			blobs:   map[string][]byte{},
			want:    "has no blob bytes",
		},
		{
			name: "duplicate path",
			entries: []Entry{
				{Path: "cmd/tool/main.go", Mode: "100644", Type: "blob"},
				{Path: "cmd/tool/main.go", Mode: "100644", Type: "blob"},
			},
			blobs: map[string][]byte{"cmd/tool/main.go": []byte("package main")},
			want:  "duplicate entry",
		},
		{
			name:    "noncanonical path",
			entries: []Entry{{Path: "internal/../internal/widget.go", Mode: "100644", Type: "blob"}},
			blobs:   map[string][]byte{},
			want:    "not a canonical repository-relative path",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(tc.entries, tc.blobs)
			if err == nil || !errors.Is(err, ErrInvalidTree) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Build = %#v, %v; want ErrInvalidTree containing %q", got, err, tc.want)
			}
			if len(got.Units) != 0 || got.AggregateSHA256 != "" {
				t.Fatalf("defective tree returned partial inventory: %#v", got)
			}
		})
	}
}

func TestBuildIgnoresOutOfScopeIrregularEntries(t *testing.T) {
	entries := []Entry{
		{Path: "internal/widget/testdata/link.go", Mode: "120000", Type: "blob"},
		{Path: "docs/example.go", Mode: "120000", Type: "blob"},
		{Path: "vendor/tool.sh", Mode: "160000", Type: "commit"},
		{Path: "notes.txt", Mode: "160000", Type: "commit"},
		{Path: "scripts/nonexec.sh", Mode: "100644", Type: "blob"},
	}
	got, err := Build(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Units) != 0 || len(got.AggregateSHA256) != 64 {
		t.Fatalf("empty inventory = %#v", got)
	}
}

func blobsFor(entries []Entry) map[string][]byte {
	out := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755") {
			out[entry.Path] = []byte("contents of " + entry.Path + "\n")
		}
	}
	return out
}

func cloneBlobs(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for name, body := range in {
		out[name] = append([]byte(nil), body...)
	}
	return out
}
