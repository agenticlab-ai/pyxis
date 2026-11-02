package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

var retirementSources = []string{".pyxis/skills/example-review/SKILL.md", ".pyxis/vendor/example-tool/README.md"}

func TestStandaloneSourceRetirementUsageAndHeadMovement(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"retire-source"}, {"retire-source", "--unknown"},
		{"retire-source", "--source", "docs/product-vision.md", "--reason", "retire", "--write"},
		{"retire-source", "--source", retirementSources[0], "--source", retirementSources[0], "--reason", "retire", "--write"},
		{"retire-source", "--source", retirementSources[0], "--reason", "retire", "--check", "--write"},
		{"retire-source", "--source", retirementSources[0], "--reason", "retire", "--check", "--actor", "me"},
	} {
		h := newHarness(t, t.TempDir())
		if code, _, _ := h.run(args...); code != exitUsage {
			t.Fatalf("usage %v: %d", args, code)
		}
	}
	h := standaloneRetirementFixture(t)
	h.git("config", "--unset", "user.email")
	h.ok(retirementArgs("--check")...)
	h.git("config", "user.email", "m@example.com")
	env, _, _ := h.env("")
	moved := false
	env.Now = func() time.Time {
		if !moved {
			moved = true
			h.git("commit", "--allow-empty", "-m", "HEAD moved during preflight")
		}
		return t0.Add(time.Hour)
	}
	if code, err := runRetireSource(t.Context(), retirementArgs("--write")[1:], env); code == exitOK || err == nil {
		t.Fatalf("moving HEAD admitted: %d %v", code, err)
	}
	if h.git("status", "--porcelain=v1", "--untracked-files=all") != "" {
		t.Fatal("HEAD refusal left new memory")
	}
}

func standaloneRetirementFixture(t *testing.T) *harness {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	for _, source := range retirementSources {
		writeFixtureFile(t, h.dir, source, "# Exact historical source\n", 0644)
	}
	h.commit("introduce standalone sources")
	h.ok("reconcile", "--write")
	h.commit("capture sources")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0644)
	h.commit("exact legacy inventory")
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
	h.commit("partitioned policy")
	h.git("checkout", "-qb", "seal")
	return h
}

func retirementArgs(mode string) []string {
	return []string{"retire-source", "--source", retirementSources[0], "--source", retirementSources[1], "--reason", "Retire approved standalone sources", mode, "--json"}
}

func TestStandaloneSourceRetirementRefusesConfiguredGoverningSources(t *testing.T) {
	t.Parallel()
	h := standaloneRetirementFixture(t)
	for _, field := range []string{"product_vision", "architecture_principles", "incidents"} {
		t.Run(field, func(t *testing.T) {
			writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "paths:\n  "+field+": "+retirementSources[1]+"\n", 0644)
			h.commit("configure protected source " + field)
			h.git("branch", "-f", "main", "HEAD")
			for _, mode := range []string{"--check", "--write"} {
				if code, _, stderr := h.run(retirementArgs(mode)...); code == exitOK || !strings.Contains(stderr, "governed source") {
					t.Fatalf("configured governing source not refused: %s %d %s", mode, code, stderr)
				}
			}
			if h.git("status", "--porcelain=v1", "--untracked-files=all") != "" {
				t.Fatal("governing-source refusal wrote memory")
			}
		})
	}
}

func TestStandaloneSourceRetirementBatchSquashAndTransportClone(t *testing.T) {
	t.Parallel()
	h := standaloneRetirementFixture(t)
	policy := policyFilesOnDisk(t, h)
	var ready, sealed, retry retirementOutput
	decodeJSON(t, h.ok(retirementArgs("--check")...), &ready)
	if ready.Status != "ready" || len(ready.Sources) != 2 {
		t.Fatalf("preflight: %+v", ready)
	}
	decodeJSON(t, h.ok(retirementArgs("--write")...), &sealed)
	decodeJSON(t, h.ok(retirementArgs("--write")...), &retry)
	if sealed.Status != "sealed" || retry.Status != "already-sealed" || !reflect.DeepEqual(sealed.Sources, retry.Sources) {
		t.Fatalf("batch retry: %+v %+v", sealed, retry)
	}
	for _, source := range retirementSources {
		if string(mustRead(t, filepath.Join(h.dir, source))) != "# Exact historical source\n" {
			t.Fatal("writer altered source")
		}
	}
	h.commit("seal both sources without deleting either")
	sealHead := h.git("rev-parse", "HEAD")
	h.git("rm", "--", retirementSources[0], retirementSources[1])
	h.commit("attempt self-authorized deletion")
	if code, _, _ := h.gate("--base", "main"); code == exitOK {
		t.Fatal("same-PR deletion authorized itself")
	}
	if code, _, _ := h.run("verify", "--offline"); code == exitOK {
		t.Fatal("offline verifier accepted an unlanded seal")
	}
	h.git("reset", "--hard", sealHead)
	h.git("checkout", "main")
	h.git("merge", "--squash", "seal")
	h.commit("land exact retirement evidence")
	h.git("branch", "-D", "seal")
	var landedCheck, landedWrite retirementOutput
	decodeJSON(t, h.ok(retirementArgs("--check")...), &landedCheck)
	decodeJSON(t, h.ok(retirementArgs("--write")...), &landedWrite)
	if landedCheck.Status != "already-sealed" || landedWrite.Status != "sealed" || !reflect.DeepEqual(landedCheck.Sources, landedWrite.Sources) {
		t.Fatalf("landed seal retry: %+v %+v", landedCheck, landedWrite)
	}
	writeFixtureFile(t, h.dir, "unrelated", "dirty", 0644)
	if code, _, stderr := h.run(retirementArgs("--write")...); code == exitOK || !strings.Contains(stderr, "requires clean sources and memory") {
		t.Fatalf("landed seal accepted unrelated dirty state: %d %s", code, stderr)
	}
	if err := os.Remove(filepath.Join(h.dir, "unrelated")); err != nil {
		t.Fatal(err)
	}
	trusted := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "retire")
	h.git("rm", "--", retirementSources[0], retirementSources[1])
	h.commit("retire sources using landed seals")
	h.gateOK("--base", trusted)
	h.ok("verify", "--offline")
	if !reflect.DeepEqual(policy, policyFilesOnDisk(t, h)) {
		t.Fatal("retirement changed debt policy")
	}
	h.git("checkout", "main")
	h.git("merge", "--squash", "retire")
	h.commit("land source deletion")
	h.git("branch", "-D", "retire")
	dir := filepath.Join(t.TempDir(), "transport")
	h.git("clone", "--no-local", "file://"+h.dir, dir)
	if _, err := os.Stat(filepath.Join(dir, ".git/objects/info/alternates")); !os.IsNotExist(err) {
		t.Fatalf("clone uses alternates: %v", err)
	}
	clone := newHarness(t, dir)
	clone.gateOK()
	clone.ok("verify", "--offline")
	for _, source := range retirementSources {
		if _, err := os.Stat(filepath.Join(dir, source)); !os.IsNotExist(err) {
			t.Fatalf("source remains: %s: %v", source, err)
		}
	}
}

func TestStandaloneSourceRetirementRefusesInvalidBatchesBeforeWriting(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"dirty source", "unrelated memory", "topic HEAD", "missing capture", "symlink", "partial retry", "altered retry", "reformatted retry", "extended retry"} {
		t.Run(mode, func(t *testing.T) {
			h := standaloneRetirementFixture(t)
			args := retirementArgs("--write")
			switch mode {
			case "dirty source":
				writeFixtureFile(t, h.dir, retirementSources[1], "changed", 0644)
			case "unrelated memory":
				h.ok("add", "--kind", "Learning", "--subject", "meta/unrelated", "--statement", "unrelated")
			case "topic HEAD":
				h.git("commit", "--allow-empty", "-m", "topic-only")
			case "missing capture":
				env, _, _ := h.env("")
				m, err := open(t.Context(), env)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range m.set.IDs() {
					r := m.set.Get(id)
					if r.Provenance.Source == retirementSources[1] {
						h.ok("revoke", id, "--reason", "withdraw capture")
					}
				}
				h.commit("withdraw capture")
				h.git("branch", "-f", "main", "HEAD")
			case "symlink":
				if err := os.Remove(filepath.Join(h.dir, retirementSources[1])); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../../skills/conversation/SKILL.md", filepath.Join(h.dir, retirementSources[1])); err != nil {
					t.Fatal(err)
				}
				h.commit("symlink source")
				h.git("branch", "-f", "main", "HEAD")
			case "partial retry", "altered retry", "reformatted retry", "extended retry":
				var out retirementOutput
				decodeJSON(t, h.ok(args...), &out)
				if mode == "extended retry" {
					args = append(args, "--source", ".pyxis/skills/another/SKILL.md")
				} else {
					item := out.Sources[0]
					name := filepath.Join(h.dir, "memory", reconcile.SourceRetirementSubject(item.Receipt.Path), item.RecordID+".json")
					switch mode {
					case "partial retry":
						if err := os.Remove(name); err != nil {
							t.Fatal(err)
						}
					case "reformatted retry":
						writeFixtureFile(t, h.dir, strings.TrimPrefix(name, h.dir+"/"), " "+string(mustRead(t, name)), 0644)
					default:
						writeFixtureFile(t, h.dir, strings.TrimPrefix(name, h.dir+"/"), strings.Replace(string(mustRead(t, name)), "Preserve exact source", "Altered exact source", 1), 0644)
					}
				}
			}
			before := h.git("status", "--porcelain=v1", "--untracked-files=all")
			if code, _, stderr := h.run(args...); code == exitOK {
				t.Fatalf("invalid batch accepted: %s", stderr)
			}
			if after := h.git("status", "--porcelain=v1", "--untracked-files=all"); before != after {
				t.Fatalf("failed preflight wrote files:\n%s\n%s", before, after)
			}
		})
	}
}
