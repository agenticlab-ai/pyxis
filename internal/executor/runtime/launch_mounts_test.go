package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPinnedLaunchRejectsDirectorySwap(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work, state, outside := filepath.Join(root, "work"), filepath.Join(root, "state"), filepath.Join(root, "outside")
	for _, path := range []string{work, state, outside, filepath.Join(state, "child")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	parent, closeParent, err := openLaunchMounts(accessPaths{work: work, state: state})
	if err != nil {
		t.Fatal(err)
	}
	defer closeParent()
	paths := accessPaths{work: work, state: filepath.Join(state, "child"), ancestors: []accessPaths{parent}}
	if err := os.Rename(paths.state, paths.state+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, paths.state); err != nil {
		t.Fatal(err)
	}
	if _, _, err := openLaunchMounts(paths); err == nil {
		t.Fatal("accepted symlink outside original write roots")
	}
	if err := os.Remove(paths.state); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(paths.state+"-saved", paths.state); err != nil {
		t.Fatal(err)
	}
	paths, closeScope, err := openLaunchMounts(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer closeScope()
	if err := os.Rename(paths.state, paths.state+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, paths.state); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("unused")
	pinned, cleanup, err := pinLaunchMounts(cmd, paths)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if pinned.stateFD != 3 || pinned.workFD != 4 {
		t.Fatalf("%+v", pinned)
	}
	info, err := cmd.ExtraFiles[0].Stat()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.Stat(paths.state + "-saved")
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(info, saved) {
		t.Fatal("opened mount source changed after directory swap")
	}
	for _, paths := range []accessPaths{
		{work: work, state: outside, ancestors: []accessPaths{parent}},
		{work: work, state: state, ancestors: []accessPaths{{work: work, state: filepath.Join(root, "missing")}}},
	} {
		if _, _, err := openLaunchMounts(paths); err == nil {
			t.Fatal("accepted invalid mount scope")
		}
	}
	if _, cleanup, err := pinLaunchMounts(exec.Command("unused"), accessPaths{}); err != nil {
		t.Fatal(err)
	} else {
		cleanup()
	}
}

func TestPinnedLaunchCannotRegainWiderAncestor(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work, state := filepath.Join(root, "work"), filepath.Join(root, "state")
	child, outside := filepath.Join(state, "child"), filepath.Join(state, "sibling")
	for _, path := range []string{work, state, child, outside, filepath.Join(child, "grandchild")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	parent, closeParent, err := openLaunchMounts(accessPaths{work: work, state: state})
	if err != nil {
		t.Fatal(err)
	}
	defer closeParent()
	narrower, closeNarrower, err := openLaunchMounts(accessPaths{work: work, state: child, ancestors: []accessPaths{parent}})
	if err != nil {
		t.Fatal(err)
	}
	defer closeNarrower()
	if err := os.Rename(child, child+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, child); err != nil {
		t.Fatal(err)
	}
	// A malicious request to the wider ancestor's sibling is rejected both by
	// path validation and by the narrower capability's retained root handle.
	if _, _, err := openLaunchMounts(accessPaths{work: work, state: filepath.Join(child, "../sibling"), ancestors: []accessPaths{parent, narrower}}); err == nil {
		t.Fatal("regained a wider ancestor's directory")
	}
	grandchild := accessPaths{work: work, state: filepath.Join(child, "grandchild"), ancestors: []accessPaths{parent, narrower}}
	got, closeGrandchild, err := openLaunchMounts(grandchild)
	if err != nil {
		t.Fatal(err)
	}
	defer closeGrandchild()
	file, err := got.stateRoot.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }() // test owns the reference descriptor
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.Stat(filepath.Join(child+"-saved", "grandchild"))
	if err != nil || !os.SameFile(info, saved) {
		t.Fatal("narrowed capability followed a replaced ancestor")
	}
}

func TestPinnedLaunchFailuresReleaseDescriptors(t *testing.T) {
	_, ref, _ := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	// Test independent scope ownership without borrowing the broker's handles.
	parent, closeParent, err := openLaunchMounts(accessPaths{work: t.TempDir(), state: ref.State})
	if err != nil {
		t.Fatal(err)
	}
	defer closeParent()
	for _, paths := range []accessPaths{
		{work: parent.work, state: parent.state, ancestors: []accessPaths{{work: parent.work, state: parent.state}}},
		{work: parent.work, state: parent.work, ancestors: []accessPaths{{work: parent.work, state: parent.state, readOnly: true}}},
		{work: filepath.Join(parent.work, "missing"), state: parent.state},
	} {
		if _, _, err := openLaunchMounts(paths); err == nil {
			t.Fatal("accepted invalid scope roots")
		}
	}
	paths, closeScope, err := openLaunchMounts(accessPaths{work: parent.work, state: parent.state, ancestors: []accessPaths{parent}, readOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("unused")
	if pinned, cleanup, err := pinLaunchMounts(cmd, paths); err != nil {
		t.Fatal(err)
	} else {
		if pinned.workFD != 0 || pinned.stateFD != 3 {
			t.Fatal("read-only scope gained a writable work mount")
		}
		cleanup()
	}
	closeScope()
	if _, _, err := pinLaunchMounts(exec.Command("unused"), paths); err == nil {
		t.Fatal("accepted revoked root")
	}
	if _, _, err := pinLaunchMounts(exec.Command("unused"), accessPaths{ancestors: []accessPaths{parent}}); err == nil {
		t.Fatal("accepted unpinned root")
	}
}
