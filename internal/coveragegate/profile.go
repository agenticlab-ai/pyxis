// Package coveragegate measures repository-wide Go statement coverage and
// applies a deterministic minimum. It consumes the standard Go coverage
// profile rather than human-formatted `go tool cover` output, so threshold
// comparisons do not depend on display rounding.
package coveragegate

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// BasisPoints is a percentage in hundredths of one percent: 8,500 is 85.00%.
type BasisPoints int64

// ParseMinimum parses a percentage in [0, 100] with at most two decimal
// places. The closed representation keeps threshold comparisons exact.
func ParseMinimum(value string) (BasisPoints, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("coverage minimum is empty")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, fmt.Errorf("coverage minimum %q is not a percentage", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > 100 {
		return 0, fmt.Errorf("coverage minimum %q must be between 0 and 100", value)
	}
	fraction := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, fmt.Errorf("coverage minimum %q may have at most two decimal places", value)
		}
		fraction, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || fraction < 0 {
			return 0, fmt.Errorf("coverage minimum %q is not a percentage", value)
		}
		if len(parts[1]) == 1 {
			fraction *= 10
		}
	}
	if whole == 100 && fraction != 0 {
		return 0, fmt.Errorf("coverage minimum %q exceeds 100", value)
	}
	return BasisPoints(whole*100 + fraction), nil
}

// Package is one Go package's statement coverage across the whole test run.
type Package struct {
	Path    string
	Covered int64
	Total   int64
}

// Percent returns the package's covered-statement percentage.
func (p Package) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return 100 * float64(p.Covered) / float64(p.Total)
}

// File is one compiled, non-test Go source file's statement coverage across
// the whole test run. Files with no executable statements do not appear in a
// Go coverage profile and therefore are not measurable units.
type File struct {
	Path    string
	Package string
	Covered int64
	Total   int64
}

// Percent returns the file's covered-statement percentage.
func (f File) Percent() float64 {
	if f.Total == 0 {
		return 0
	}
	return 100 * float64(f.Covered) / float64(f.Total)
}

// Report is the de-duplicated union of every coverage block in a profile.
// `go test -coverpkg=./... ./...` repeats blocks once per test binary; a block
// counts once in the denominator and is covered if any test binary executed it.
type Report struct {
	Covered  int64
	Total    int64
	Packages []Package
	Files    []File
}

// Percent returns repository-wide covered-statement percentage.
func (r Report) Percent() float64 {
	if r.Total == 0 {
		return 0
	}
	return 100 * float64(r.Covered) / float64(r.Total)
}

// Meets reports whether every measured Go source file meets minimum. This is
// deliberately not an aggregate comparison: well-covered files cannot mask a
// weak file in the same package or elsewhere in the repository.
func (r Report) Meets(minimum BasisPoints) bool {
	if len(r.Files) == 0 {
		return false
	}
	for _, file := range r.Files {
		if !meets(file.Covered, file.Total, minimum) {
			return false
		}
	}
	return true
}

func meets(covered, total int64, minimum BasisPoints) bool {
	return total > 0 && covered*10_000 >= total*int64(minimum)
}

// Without returns the report minus the excluded files, as if they had never
// been instrumented. Each entry names a package import path, which excludes
// that package's own files, or one source file path as the profile spells it.
func (r Report) Without(excluded []string) Report {
	var rest Report
	byPackage := make(map[string]Package)
	for _, file := range r.Files {
		if slices.Contains(excluded, file.Package) || slices.Contains(excluded, file.Path) {
			continue
		}
		rest.Files = append(rest.Files, file)
		rest.Covered += file.Covered
		rest.Total += file.Total
		row := byPackage[file.Package]
		row.Path = file.Package
		row.Covered += file.Covered
		row.Total += file.Total
		byPackage[file.Package] = row
	}
	for _, row := range byPackage {
		rest.Packages = append(rest.Packages, row)
	}
	sort.Slice(rest.Packages, func(i, j int) bool { return rest.Packages[i].Path < rest.Packages[j].Path })
	return rest
}

// Bottom returns up to n packages ordered by increasing coverage, with path as
// the stable tie-breaker. Packages with no executable statements are absent
// from a Go coverage profile and therefore absent from the report.
func (r Report) Bottom(n int) []Package {
	if n <= 0 {
		return nil
	}
	rows := append([]Package(nil), r.Packages...)
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Covered * rows[j].Total
		right := rows[j].Covered * rows[i].Total
		if left != right {
			return left < right
		}
		return rows[i].Path < rows[j].Path
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// BottomFiles returns up to n files ordered by increasing coverage, with path
// as the stable tie-breaker.
func (r Report) BottomFiles(n int) []File {
	if n <= 0 {
		return nil
	}
	rows := append([]File(nil), r.Files...)
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Covered * rows[j].Total
		right := rows[j].Covered * rows[i].Total
		if left != right {
			return left < right
		}
		return rows[i].Path < rows[j].Path
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// Below returns every file below minimum in stable lowest-first order.
func (r Report) Below(minimum BasisPoints) []File {
	rows := r.BottomFiles(len(r.Files))
	cut := 0
	for _, file := range rows {
		if meets(file.Covered, file.Total, minimum) {
			break
		}
		cut++
	}
	return rows[:cut]
}

type block struct {
	filePath    string
	packagePath string
	statements  int64
	covered     bool
}

// ParseProfile reads the standard text profile emitted by `go test
// -coverprofile`. Duplicate blocks are merged as a union, which is required for
// repository-wide -coverpkg profiles produced by multiple test binaries.
func ParseProfile(r io.Reader) (Report, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	blocks := make(map[string]block)
	modeSeen := false
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if mode, ok := strings.CutPrefix(line, "mode:"); ok {
			if modeSeen || !validMode(strings.TrimSpace(mode)) {
				return Report{}, fmt.Errorf("coverage profile line %d: invalid or duplicate mode", lineNumber)
			}
			modeSeen = true
			continue
		}
		if !modeSeen {
			return Report{}, fmt.Errorf("coverage profile line %d: data precedes mode", lineNumber)
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return Report{}, fmt.Errorf("coverage profile line %d: want location, statement count, execution count", lineNumber)
		}
		separator := strings.LastIndex(fields[0], ":")
		if separator <= 0 {
			return Report{}, fmt.Errorf("coverage profile line %d: invalid location %q", lineNumber, fields[0])
		}
		statements, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || statements < 0 {
			return Report{}, fmt.Errorf("coverage profile line %d: invalid statement count %q", lineNumber, fields[1])
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || count < 0 {
			return Report{}, fmt.Errorf("coverage profile line %d: invalid execution count %q", lineNumber, fields[2])
		}
		filename := fields[0][:separator]
		key := fields[0] + " " + fields[1]
		candidate := block{filePath: filename, packagePath: path.Dir(filename), statements: statements, covered: count > 0}
		if current, exists := blocks[key]; exists {
			if current.filePath != candidate.filePath || current.packagePath != candidate.packagePath || current.statements != candidate.statements {
				return Report{}, fmt.Errorf("coverage profile line %d: inconsistent duplicate block %q", lineNumber, key)
			}
			current.covered = current.covered || candidate.covered
			blocks[key] = current
			continue
		}
		blocks[key] = candidate
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("read coverage profile: %w", err)
	}
	if !modeSeen {
		return Report{}, fmt.Errorf("coverage profile has no mode")
	}

	byPackage := make(map[string]Package)
	byFile := make(map[string]File)
	var report Report
	for _, item := range blocks {
		row := byPackage[item.packagePath]
		row.Path = item.packagePath
		row.Total += item.statements
		report.Total += item.statements
		if item.covered {
			row.Covered += item.statements
			report.Covered += item.statements
		}
		byPackage[item.packagePath] = row

		file := byFile[item.filePath]
		file.Path = item.filePath
		file.Package = item.packagePath
		file.Total += item.statements
		if item.covered {
			file.Covered += item.statements
		}
		byFile[item.filePath] = file
	}
	if report.Total == 0 {
		return Report{}, fmt.Errorf("coverage profile contains no executable statements")
	}
	report.Packages = make([]Package, 0, len(byPackage))
	for _, row := range byPackage {
		if row.Total > 0 {
			report.Packages = append(report.Packages, row)
		}
	}
	sort.Slice(report.Packages, func(i, j int) bool { return report.Packages[i].Path < report.Packages[j].Path })
	report.Files = make([]File, 0, len(byFile))
	for _, file := range byFile {
		if file.Total > 0 {
			report.Files = append(report.Files, file)
		}
	}
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].Path < report.Files[j].Path })
	return report, nil
}

func validMode(mode string) bool {
	switch mode {
	case "set", "count", "atomic":
		return true
	default:
		return false
	}
}
