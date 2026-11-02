package claude

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func TestCoverage90OptionsAndProcessFailures(t *testing.T) {
	if _, err := New(WithFenceAuthority(nil)); err == nil {
		t.Fatal("nil fence authority was accepted")
	}

	want := errors.New("kill refused")
	awaitHandle := &handle{proc: &fakeProcess{stdout: []byte(resultJSON), killErr: want}}
	if _, err := mustAdapter(t).Await(t.Context(), awaitHandle); !errors.Is(err, want) {
		t.Fatalf("await quiesce error = %v", err)
	}
	killHandle := &handle{proc: &fakeProcess{killErr: want}}
	if err := mustAdapter(t).Kill(t.Context(), killHandle); !errors.Is(err, want) {
		t.Fatalf("kill error = %v", err)
	}
}

func TestCoverage90ResultDecodeAndCredentialDefaults(t *testing.T) {
	for _, raw := range [][]byte{[]byte("{"), []byte("[")} {
		if _, err := parseResult(raw); err == nil {
			t.Fatalf("malformed result %q parsed", raw)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", t.TempDir())
	original := keychainCredentialExists
	keychainCredentialExists = func() bool { return false }
	t.Cleanup(func() { keychainCredentialExists = original })
	if credentialStoreExists() {
		t.Fatal("empty default credential store reported present")
	}
}

func TestCoverage90NativeProfileControlRefusals(t *testing.T) {
	a := mustAdapter(t)
	one := int64(1)
	base := nativerouting.CatalogEntry{
		NativeModel: "claude-opus-5", NativeEffort: "max", AdapterGeneration: adapterGeneration,
		Controls: []nativerouting.NativeControl{
			{Name: "output_format", Type: nativerouting.ControlString, StringValue: "json"},
			{Name: "permission_mode", Type: nativerouting.ControlString, StringValue: "plan"},
		},
	}
	if err := a.ValidateProfile(base); err != nil {
		t.Fatalf("valid control fixture rejected: %v", err)
	}
	cases := []struct {
		name     string
		controls []nativerouting.NativeControl
		defect   string
	}{
		{"duplicate", append(append([]nativerouting.NativeControl{}, base.Controls...), base.Controls[0]), "invalid_typed_control"},
		{"output format", []nativerouting.NativeControl{{Name: "output_format", Type: nativerouting.ControlString, StringValue: "text"}, base.Controls[1]}, "unknown_native_control"},
		{"permission mode", []nativerouting.NativeControl{base.Controls[0], {Name: "permission_mode", Type: nativerouting.ControlString, StringValue: "unsafe"}}, "unknown_native_control"},
		{"nil turn limit", append(append([]nativerouting.NativeControl{}, base.Controls...), nativerouting.NativeControl{Name: "max_turns", Type: nativerouting.ControlInteger, IntegerValue: nil}), "unknown_native_control"},
		{"missing required controls", []nativerouting.NativeControl{{Name: "max_turns", Type: nativerouting.ControlInteger, IntegerValue: &one}}, "missing_native_control"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := base
			entry.Controls = tc.controls
			var defect nativerouting.NativeProfileDefect
			if err := a.ValidateProfile(entry); !errors.As(err, &defect) || defect.Defect != tc.defect {
				t.Fatalf("ValidateProfile = %v, want defect %q", err, tc.defect)
			}
		})
	}
	if _, err := (&Adapter{}).Build(nativerouting.SelectedProfile{Harness: harnessID, Role: nativerouting.RoleImplementation}); err == nil {
		t.Fatal("empty native profile identity built")
	}
	if _, err := (&Adapter{}).LaunchCapture(foreignHandle{}); err == nil {
		t.Fatal("foreign launch capture succeeded")
	}
}
