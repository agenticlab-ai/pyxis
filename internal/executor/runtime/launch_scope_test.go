package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNestedLaunchCannotExpandParentPolicy(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	work, state, outside := filepath.Join(root, "work"), filepath.Join(root, "state"), filepath.Join(root, "outside")
	git := filepath.Join(work, ".git")
	for _, path := range []string{work, state, outside, git, filepath.Join(state, "child")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(state, "escape")
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	parent := accessPaths{work: work, state: state, protected: []string{git}}
	for _, tc := range []struct {
		name     string
		request  launchPaths
		readonly bool
		wantErr  bool
	}{
		{"narrow", launchPaths{Work: work, State: filepath.Join(state, "child")}, false, false},
		{"prerequisite", launchPaths{Work: "/", State: state, ReadOnly: true}, false, false},
		{"outside work", launchPaths{Work: outside, State: state}, false, true},
		{"outside state", launchPaths{Work: work, State: outside}, false, true},
		{"symlink", launchPaths{Work: work, State: alias}, false, true},
		{"Git as state", launchPaths{Work: work, State: git}, false, true},
		{"readonly parent", launchPaths{Work: work, State: state}, true, true},
		{"readonly child", launchPaths{Work: work, State: state, ReadOnly: true}, true, false},
		{"invalid work", launchPaths{Work: "relative", State: state}, false, true},
		{"invalid state", launchPaths{Work: work, State: "relative"}, false, true},
		{"invalid protected", launchPaths{Work: work, State: state, Protected: []string{"relative"}}, false, true},
		{"missing protected", launchPaths{Work: work, State: state, Protected: []string{filepath.Join(work, "missing")}}, false, true},
		{"extra denial", launchPaths{Work: work, State: state, Protected: []string{work}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := parent
			p.readOnly = tc.readonly
			got, err := narrowLaunchPaths(p, tc.request)
			if (err != nil) != tc.wantErr {
				t.Fatalf("paths=%+v err=%v", got, err)
			}
			if err == nil && (len(got.protected) == 0 || got.protected[0] != git) {
				t.Fatalf("lost ancestor Git denial: %+v", got)
			}
		})
	}
}

func TestLaunchCapabilityClosedParsingAndReplacement(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ref := launchCapability{Schema: launchSchema, Address: "127.0.0.1:1234", Token: strings.Repeat("a", 64), Proxy: exe, State: t.TempDir()}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "{}", "{", string(data) + " {}", strings.Replace(string(data), "127.0.0.1", "192.0.2.1", 1), strings.Replace(string(data), launchSchema, "unknown", 1), strings.TrimSuffix(string(data), "}") + `,"unknown":true}`} {
		t.Setenv(launchCapabilityEnv, value)
		if _, present, err := currentLaunchCapability(); !present || err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	t.Setenv(launchCapabilityEnv, string(data))
	if got, present, err := currentLaunchCapability(); err != nil || !present || got != ref {
		t.Fatalf("got %+v %v %v", got, present, err)
	}
	env := withLaunchCapability([]string{"KEEP=value", launchCapabilityEnv + "=forged"}, ref)
	if len(env) != 2 || env[0] != "KEEP=value" || env[1] != launchCapabilityEnv+"="+string(data) {
		t.Fatal(env)
	}
	clean := cleanLaunchEnv([]string{"KEEP=value", "LD_PRELOAD=bad", "DYLD_INSERT_LIBRARIES=bad", "GCONV_PATH=bad", launchCapabilityEnv + "=forged"})
	if len(clean) != 1 || clean[0] != "KEEP=value" {
		t.Fatal(clean)
	}
}
