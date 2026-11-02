package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// This Go test executable doubles as a transparent Git process observer. It
// never fabricates object data: every operation goes to the installed Git.
func runEvidenceGitProxy() {
	f, err := os.OpenFile(os.Getenv("PYXIS_TEST_GIT_OPERATIONS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(91)
	}
	if err := json.NewEncoder(f).Encode(os.Args[1:]); err != nil {
		os.Exit(92)
	}
	if err := f.Close(); err != nil {
		os.Exit(93)
	}
	cmd := exec.Command(os.Getenv("PYXIS_TEST_REAL_GIT"), os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(94)
	}
	os.Exit(0)
}

func TestEvidenceHistoryAndSourceReadsAreBatched(t *testing.T) {
	var counts []map[string]int
	for _, size := range []int{2, 20} {
		h := initRepo(t)
		h.ok("init")
		h.commit("base")
		for i := range size {
			r := memoryTestRecord(record.Learning, "learning/batched", "", "Pinned witness "+strconv.Itoa(i), i+1)
			r.Provenance.Source = "README.md"
			r.Provenance.Evidence = &record.Evidence{Schema: record.EvidenceSchema, Locator: record.EvidenceIntroduction, Files: []record.EvidenceFile{{Path: "README.md", Mode: "100644", SHA256: fmtDigest([]byte("repo\n"))}}}
			writeEvidenceRecord(t, h, r)
		}
		h.commit("introduce all witnesses")
		gitPath, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		if err := os.Symlink(exe, filepath.Join(bin, "git")); err != nil {
			t.Fatal(err)
		}
		log := filepath.Join(t.TempDir(), "operations.jsonl")
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Setenv("PYXIS_TEST_REAL_GIT", gitPath)
			t.Setenv("PYXIS_TEST_GIT_OPERATIONS", log)
			t.Setenv("PYXIS_TEST_GIT_PROXY", "1")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			h.ok("evidence", "--check")
		})
		f, err := os.Open(log)
		if err != nil {
			t.Fatal(err)
		}
		count := map[string]int{}
		scan := bufio.NewScanner(f)
		for scan.Scan() {
			var args []string
			if err := json.Unmarshal(scan.Bytes(), &args); err != nil {
				t.Fatal(err)
			}
			for _, arg := range args {
				if arg == "log" || arg == "cat-file" || arg == "ls-tree" {
					count[arg]++
				}
			}
		}
		if err := scan.Err(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, count)
	}
	if !reflect.DeepEqual(counts[0], counts[1]) || counts[1]["log"] != 1 || counts[1]["cat-file"] != 2 {
		t.Fatalf("read processes grew with record count: %+v", counts)
	}
}

func TestEvidenceFileModesAndReadOnlyOutputFailures(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("base")
	h.git("checkout", "-qb", "topic")
	writeFixtureFile(t, h.dir, ".pyxis/scripts/source.sh", "exit 0\n", 0o755)
	h.commit("executable source")
	id := h.add("--kind", "Learning", "--subject", "learning/mode", "--statement", "Preserve executable mode.", "--source", ".pyxis/scripts/source.sh")
	r, _ := readEvidenceRecord(t, h, id)
	if r.Provenance.Evidence.Files[0].Mode != "100755" {
		t.Fatal("mode was normalized away")
	}
	for _, jsonFlag := range []bool{false, true} {
		env, _, _ := h.env("")
		env.Stdout = gateBrokenWriter{}
		args := []string{"evidence", "--check"}
		if jsonFlag {
			args = append(args, "--json")
		}
		if code := Run(t.Context(), args, env); code != exitRefused {
			t.Fatal("lost output failure")
		}
	}
	if err := os.Chmod(filepath.Join(h.dir, ".pyxis/scripts/source.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.commit("drop executable mode")
	if code, out, _ := h.run("evidence", "--check"); code != exitRefused || !strings.Contains(out, "evidence-content-mismatch") {
		t.Fatalf("mode change passed: %d %s", code, out)
	}
}
