package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerdictInvariantDINV1BindingsPresent(t *testing.T) {
	tests := []struct {
		name    string
		binding Bindings
		wantErr bool
	}{
		{
			name: "all bindings present",
			binding: Bindings{
				HeadSHA:         "abc123",
				DiffBase:        "def456",
				ManifestHash:    "ghi789",
				BaselineHash:    "jkl012",
				CodegateVersion: "1.0.0",
				GitVersion:      "2.40.0",
			},
			wantErr: false,
		},
		{
			name: "missing head_sha",
			binding: Bindings{
				DiffBase:        "def456",
				ManifestHash:    "ghi789",
				BaselineHash:    "jkl012",
				CodegateVersion: "1.0.0",
				GitVersion:      "2.40.0",
			},
			wantErr: true,
		},
		{
			name: "missing diff_base",
			binding: Bindings{
				HeadSHA:         "abc123",
				ManifestHash:    "ghi789",
				BaselineHash:    "jkl012",
				CodegateVersion: "1.0.0",
				GitVersion:      "2.40.0",
			},
			wantErr: true,
		},
		{
			name: "missing codegate_version",
			binding: Bindings{
				HeadSHA:      "abc123",
				DiffBase:     "def456",
				ManifestHash: "ghi789",
				BaselineHash: "jkl012",
				GitVersion:   "2.40.0",
			},
			wantErr: true,
		},
		{
			name: "missing git_version",
			binding: Bindings{
				HeadSHA:         "abc123",
				DiffBase:        "def456",
				ManifestHash:    "ghi789",
				BaselineHash:    "jkl012",
				CodegateVersion: "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.binding.BindingsPresent()
			if (err != nil) != tt.wantErr {
				t.Errorf("BindingsPresent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerdictInvariantDINV3PassClassExclusion(t *testing.T) {
	tests := []struct {
		name    string
		verdict *Verdict
		wantErr bool
	}{
		{
			name: "pass state with passing checks",
			verdict: &Verdict{
				State: "green",
				Rungs: map[string]*Rung{
					"0": {
						Name:  "0",
						State: "pass",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "pass"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "green-degraded with skipped checks",
			verdict: &Verdict{
				State: "green-degraded",
				Rungs: map[string]*Rung{
					"0": {
						Name:       "0",
						State:      "pass",
						SkipReason: "verb-absent",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "skip", SkipReason: "verb-absent"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pass state with failed check - DINV-3 violation",
			verdict: &Verdict{
				State: "green",
				Rungs: map[string]*Rung{
					"0": {
						Name:  "0",
						State: "pass",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "pass"},
							"check-2": {CheckID: "check-2", State: "fail"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "green-degraded with error check - DINV-3 violation",
			verdict: &Verdict{
				State: "green-degraded",
				Rungs: map[string]*Rung{
					"0": {
						Name:  "0",
						State: "pass",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "pass"},
							"check-2": {CheckID: "check-2", State: "error"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "pass state with skip:not-reached rung - DINV-3 violation",
			verdict: &Verdict{
				State: "green",
				Rungs: map[string]*Rung{
					"0": {
						Name:  "0",
						State: "pass",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "pass"},
						},
					},
					"1": {
						Name:       "1",
						Skipped:    true,
						SkipReason: "not-reached",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "red state with failed check",
			verdict: &Verdict{
				State: "red",
				Rungs: map[string]*Rung{
					"0": {
						Name:  "0",
						State: "fail",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "fail"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verdict.ValidateVerdictInvariants()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVerdictInvariants() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerdictInvariantDINV4RunCompleteness(t *testing.T) {
	tests := []struct {
		name    string
		verdict *Verdict
		wantErr bool
	}{
		{
			name: "rung with checks",
			verdict: &Verdict{
				State: "green",
				Rungs: map[string]*Rung{
					"0": {
						Name: "0",
						Checks: map[string]*CheckResult{
							"check-1": {CheckID: "check-1", State: "pass"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "rung skipped with reason",
			verdict: &Verdict{
				State: "green-degraded",
				Rungs: map[string]*Rung{
					"0": {
						Name:       "0",
						Skipped:    true,
						SkipReason: "verb-absent",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "rung with no checks and not skipped - DINV-4 violation",
			verdict: &Verdict{
				State: "green",
				Rungs: map[string]*Rung{
					"0": {
						Name:    "0",
						Checks:  map[string]*CheckResult{},
						Skipped: false,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verdict.ValidateVerdictInvariants()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVerdictInvariants() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestClosedParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid manifest",
			input: `{
				"version": "1.0.0",
				"stack": "go",
				"verbs": {
					"format-check": {
						"command": "gofmt -l ."
					}
				},
				"baseline_path": "testdata/baseline.json"
			}`,
			wantErr: false,
		},
		{
			name: "manifest with env and time bounds",
			input: `{
				"version": "1.0.0",
				"stack": "go",
				"verbs": {
					"test": {
						"command": "go test ./...",
						"env": {
							"GOCACHE": {"mode": "pin", "value": ".codegate/cache"},
							"HOME": {"mode": "pass"}
						},
						"timeout_s": 300,
						"grace_s": 10
					}
				},
				"test_globs": ["**/*_test.go"],
				"state_dirs": ["vendor"],
				"baseline_path": ".codegate/baseline.json"
			}`,
			wantErr: false,
		},
		{
			name: "missing version",
			input: `{
				"stack": "go",
				"verbs": {}
			}`,
			wantErr: true,
		},
		{
			name: "missing stack",
			input: `{
				"version": "1.0.0",
				"verbs": {}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalManifest([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestClosedParseUnknownKey(t *testing.T) {
	input := `{
		"version": "1.0.0",
		"stack": "go",
		"verbs": {},
		"unknown_field": "should_fail"
	}`

	_, err := UnmarshalManifest([]byte(input))
	if err == nil {
		t.Error("UnmarshalManifest() should reject unknown fields, but did not")
	}
}

func TestAtomicVerdictWrite(t *testing.T) {
	v := &Verdict{
		RunID:     "run-123",
		Workspace: "/workspace",
		Mode:      "full",
		State:     "green",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Rungs: map[string]*Rung{
			"0": {
				Name:  "0",
				Order: 0,
				State: "pass",
				Checks: map[string]*CheckResult{
					"format": {CheckID: "format", State: "pass"},
				},
			},
		},
		Findings: []Finding{},
		Bindings: Bindings{
			HeadSHA:         "abc123def456",
			DiffBase:        "prev789abc",
			ManifestHash:    "manifest_hash",
			BaselineHash:    "baseline_hash",
			CodegateVersion: "1.0.0",
			GitVersion:      "2.40.0",
		},
	}

	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Marshal() produced empty data")
	}

	var v2 Verdict
	if err := json.Unmarshal(data, &v2); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if v2.RunID != v.RunID {
		t.Errorf("RunID mismatch: got %s, want %s", v2.RunID, v.RunID)
	}
	if v2.State != v.State {
		t.Errorf("State mismatch: got %s, want %s", v2.State, v.State)
	}
	if v2.Bindings.HeadSHA != v.Bindings.HeadSHA {
		t.Errorf("HeadSHA mismatch: got %s, want %s", v2.Bindings.HeadSHA, v.Bindings.HeadSHA)
	}
}

func TestVerdictDeterministicMarshal(t *testing.T) {
	v1 := &Verdict{
		RunID:     "run-123",
		Workspace: "/workspace",
		Mode:      "full",
		State:     "green",
		StartTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC),
		Rungs: map[string]*Rung{
			"0": {
				Name:  "0",
				Order: 0,
				State: "pass",
				Checks: map[string]*CheckResult{
					"format": {CheckID: "format", State: "pass"},
				},
			},
		},
		Findings: []Finding{},
		Bindings: Bindings{
			HeadSHA:         "abc123def456",
			DiffBase:        "prev789abc",
			ManifestHash:    "manifest_hash",
			BaselineHash:    "baseline_hash",
			CodegateVersion: "1.0.0",
			GitVersion:      "2.40.0",
		},
	}

	data1, _ := v1.Marshal()
	data2, _ := v1.Marshal()

	if string(data1) != string(data2) {
		t.Error("Verdict marshal is not deterministic")
	}
}

func TestAtomicVerdictFileWrite(t *testing.T) {
	tmpDir := t.TempDir()
	verdictPath := filepath.Join(tmpDir, "verdict.json")

	v := &Verdict{
		RunID:     "run-456",
		Workspace: "/workspace",
		Mode:      "incremental",
		State:     "green-degraded",
		StartTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC),
		Rungs: map[string]*Rung{
			"0": {
				Name:  "0",
				Order: 0,
				State: "pass",
				Checks: map[string]*CheckResult{
					"check-1": {CheckID: "check-1", State: "pass"},
				},
			},
		},
		Findings: []Finding{},
		Bindings: Bindings{
			HeadSHA:         "def789ghi012",
			DiffBase:        "abc123def456",
			ManifestHash:    "new_manifest_hash",
			BaselineHash:    "new_baseline_hash",
			CodegateVersion: "1.0.0",
			GitVersion:      "2.41.0",
		},
	}

	if err := v.WriteAtomicToFile(verdictPath); err != nil {
		t.Fatalf("WriteAtomicToFile() error: %v", err)
	}

	if _, err := os.Stat(verdictPath); os.IsNotExist(err) {
		t.Fatal("Verdict file was not created")
	}

	data, err := os.ReadFile(verdictPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var v2 Verdict
	if err := json.Unmarshal(data, &v2); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if v2.RunID != v.RunID {
		t.Errorf("RunID mismatch: got %s, want %s", v2.RunID, v.RunID)
	}
	if v2.State != v.State {
		t.Errorf("State mismatch: got %s, want %s", v2.State, v.State)
	}
	if v2.Bindings.HeadSHA != v.Bindings.HeadSHA {
		t.Errorf("HeadSHA mismatch: got %s, want %s", v2.Bindings.HeadSHA, v.Bindings.HeadSHA)
	}
}

func TestAtomicWriteNoPartialFile(t *testing.T) {
	tmpDir := t.TempDir()
	verdictPath := filepath.Join(tmpDir, "verdict.json")

	v := &Verdict{
		RunID:     "run-789",
		Workspace: "/workspace",
		State:     "green",
		Rungs: map[string]*Rung{
			"0": {
				Name:  "0",
				State: "pass",
				Checks: map[string]*CheckResult{
					"check-1": {CheckID: "check-1", State: "pass"},
				},
			},
		},
		Bindings: Bindings{
			HeadSHA:         "abc123",
			DiffBase:        "def456",
			ManifestHash:    "ghi789",
			BaselineHash:    "jkl012",
			CodegateVersion: "1.0.0",
			GitVersion:      "2.40.0",
		},
	}

	if err := v.WriteAtomicToFile(verdictPath); err != nil {
		t.Fatalf("WriteAtomicToFile() error: %v", err)
	}

	if _, err := os.Stat(verdictPath); os.IsNotExist(err) {
		t.Fatal("Final verdict file should exist")
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}

	for _, file := range files {
		if file.Name() != "verdict.json" {
			t.Errorf("Unexpected file in temp dir: %s (should only have verdict.json)", file.Name())
		}
	}
}

// #402 review F5: per-verb env entries must declare pass-through vs pinned, and a
// declared verb must carry a command.
func TestManifestVerbValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"pinned value ok", `{"version":"1","stack":"go","verbs":{"test":{"command":"go test","env":{"GOFLAGS":{"mode":"pin","value":"-mod=mod"}}}}}`, false},
		{"pass-through ok", `{"version":"1","stack":"go","verbs":{"test":{"command":"go test","env":{"HOME":{"mode":"pass"}}}}}`, false},
		{"unknown env mode rejected", `{"version":"1","stack":"go","verbs":{"test":{"command":"go test","env":{"HOME":{"mode":"inherit"}}}}}`, true},
		{"pass-through pinning a value rejected", `{"version":"1","stack":"go","verbs":{"test":{"command":"go test","env":{"HOME":{"mode":"pass","value":"x"}}}}}`, true},
		{"verb without a command rejected", `{"version":"1","stack":"go","verbs":{"test":{"timeout_s":30}}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := UnmarshalManifest([]byte(tt.input)); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// T4: the rung-0 manifest extension points (secret_scan, forbidden_artifacts)
// must compile their patterns at parse time, never lazily at check-run time.
func TestManifestExtensionPointValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"valid secret_scan extra pattern",
			`{"version":"1","stack":"go","verbs":{},"secret_scan":{"extra_patterns":["internal-token-[0-9]+"]}}`,
			false,
		},
		{
			"malformed secret_scan extra pattern rejected",
			`{"version":"1","stack":"go","verbs":{},"secret_scan":{"extra_patterns":["("]}}`,
			true,
		},
		{
			"valid forbidden_artifacts override",
			`{"version":"1","stack":"go","verbs":{},"forbidden_artifacts":{"max_file_bytes":2048,"extra_absolute_path_patterns":["/opt/acme/"]}}`,
			false,
		},
		{
			"malformed forbidden_artifacts pattern rejected",
			`{"version":"1","stack":"go","verbs":{},"forbidden_artifacts":{"extra_absolute_path_patterns":["["]}}`,
			true,
		},
		{
			"negative max_file_bytes rejected",
			`{"version":"1","stack":"go","verbs":{},"forbidden_artifacts":{"max_file_bytes":-1}}`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := UnmarshalManifest([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.name == "valid secret_scan extra pattern" {
				if m.SecretScan == nil || len(m.SecretScan.ExtraPatterns) != 1 {
					t.Errorf("SecretScan not round-tripped: %+v", m.SecretScan)
				}
			}
		})
	}

	// Absent extension points must not force any change on an existing minimal
	// manifest — additive evolution (AP-6) holds.
	m, err := UnmarshalManifest([]byte(`{"version":"1","stack":"go","verbs":{}}`))
	if err != nil {
		t.Fatalf("UnmarshalManifest() unexpected error: %v", err)
	}
	if m.SecretScan != nil || m.ForbiddenArtifacts != nil {
		t.Errorf("expected nil extension points on a manifest that declares none, got %+v / %+v", m.SecretScan, m.ForbiddenArtifacts)
	}
}

// #402 review F3: completeness against the configured ladder — a verdict may not
// silently omit a configured rung or a registered check.
func TestValidateCompleteness(t *testing.T) {
	expected := map[string][]string{"0": {"format", "lint"}, "1": {"build"}}
	rung0 := func() *Rung {
		return &Rung{Name: "0", Checks: map[string]*CheckResult{
			"format": {CheckID: "format", State: "pass"},
			"lint":   {CheckID: "lint", State: "pass"},
		}}
	}

	full := &Verdict{Rungs: map[string]*Rung{
		"0": rung0(),
		"1": {Name: "1", Checks: map[string]*CheckResult{"build": {CheckID: "build", State: "pass"}}},
	}}
	if err := full.ValidateCompleteness(expected); err != nil {
		t.Errorf("complete verdict rejected: %v", err)
	}

	missingRung := &Verdict{Rungs: map[string]*Rung{"0": rung0()}}
	if err := missingRung.ValidateCompleteness(expected); err == nil {
		t.Error("a verdict omitting configured rung 1 must fail completeness")
	}

	missingCheck := &Verdict{Rungs: map[string]*Rung{
		"0": {Name: "0", Checks: map[string]*CheckResult{"format": {CheckID: "format", State: "pass"}}},
		"1": {Name: "1", Checks: map[string]*CheckResult{"build": {CheckID: "build", State: "pass"}}},
	}}
	if err := missingCheck.ValidateCompleteness(expected); err == nil {
		t.Error("a verdict omitting registered check 'lint' must fail completeness")
	}

	skipped := &Verdict{Rungs: map[string]*Rung{
		"0": rung0(),
		"1": {Name: "1", Skipped: true, SkipReason: "not-reached"},
	}}
	if err := skipped.ValidateCompleteness(expected); err != nil {
		t.Errorf("a skipped rung satisfies completeness by presence: %v", err)
	}
}

// #402 review F6: duration_ms must serialize as milliseconds, not time.Duration's
// raw nanoseconds.
func TestDurationSerializesMilliseconds(t *testing.T) {
	r := &Rung{Name: "0", Duration: DurationMS(1500 * time.Millisecond)}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"duration_ms":1500`) {
		t.Errorf("duration_ms should serialize as 1500 (ms), got %s", data)
	}
	var r2 Rung
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if time.Duration(r2.Duration) != 1500*time.Millisecond {
		t.Errorf("round-trip = %v, want 1.5s", time.Duration(r2.Duration))
	}
}

// #402 review F2/F4b: the write fails closed on an invalid verdict and leaves NO
// artifact and NO temp residue behind (no partial verdict).
func TestWriteRefusesInvalidVerdict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verdict.json")

	invalid := &Verdict{RunID: "run-x", State: "green"} // no rungs, no bindings
	if err := invalid.WriteAtomicToFile(path); err == nil {
		t.Fatal("WriteAtomicToFile must refuse an invalid verdict")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a refused write must leave no verdict file")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("a refused write must leave no temp residue, found %d file(s)", len(files))
	}
}
