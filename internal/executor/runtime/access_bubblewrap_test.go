package runtime

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// Exercise the production Linux command builder without launching a Linux
// sandbox. Kernel enforcement remains covered by the native access tests when
// they run on Linux; these cases execute on either supported host.
func TestBubblewrapPreservesMountPolicyAndPayload(t *testing.T) {
	for _, tc := range []struct {
		name             string
		readOnly, pinned bool
		mounts           []string
	}{
		{"implementation", false, false, []string{"--bind", "/work", "/work", "--bind", "/state", "/state"}},
		{"review", true, false, []string{"--bind", "/state", "/state"}},
		{"pinned implementation", false, true, []string{"--bind-fd", "4", "/work", "--bind-fd", "3", "/state"}},
		{"pinned review", true, true, []string{"--bind-fd", "3", "/state"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := accessPaths{work: "/work", state: "/state", readOnly: tc.readOnly,
				protected: []string{"/work/.git", "/parent/.git"}}
			if tc.pinned {
				paths.launchMounts = launchMounts{workFD: 4, stateFD: 3}
			}
			cmd := exec.Command("/payload", "argument with spaces", "--literal")
			err := confineBubblewrapProcess(cmd, paths, func(name string) (string, error) {
				if name != "bwrap" {
					t.Fatalf("resolved unexpected wrapper %q", name)
				}
				return "/tools/bwrap", nil
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"/tools/bwrap", "--die-with-parent", "--unshare-user", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--new-session", "--cap-drop", "ALL", "--ro-bind", "/", "/", "--proc", "/proc", "--dev", "/dev"}
			want = append(want, tc.mounts...)
			want = append(want, "--ro-bind", "/work/.git", "/work/.git", "--ro-bind", "/parent/.git", "/parent/.git", "--chdir", "/work", "--", "/payload", "argument with spaces", "--literal")
			if cmd.Path != "/tools/bwrap" || !slices.Equal(cmd.Args, want) {
				t.Fatalf("wrapper = %q %q, want %q", cmd.Path, cmd.Args, want)
			}
		})
	}
}

func TestBubblewrapMissingPrerequisiteFailsBeforePayload(t *testing.T) {
	cmd := exec.Command("/payload", "argument")
	want := slices.Clone(cmd.Args)
	err := confineBubblewrapProcess(cmd, accessPaths{}, func(string) (string, error) {
		return "", exec.ErrNotFound
	})
	if !errors.Is(err, ErrAccessUnsupported) || !strings.Contains(err.Error(), "install Bubblewrap") {
		t.Fatalf("missing Linux prerequisite diagnostic: %v", err)
	}
	if cmd.Path != "/payload" || !slices.Equal(cmd.Args, want) || cmd.Process != nil {
		t.Fatal("missing prerequisite changed or launched the payload")
	}
}
