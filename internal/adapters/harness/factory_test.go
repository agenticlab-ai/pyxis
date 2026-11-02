package harness_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func TestFactoryUsesEffectiveCatalogForEveryAdapter(t *testing.T) {
	base, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range entity.Harnesses() {
		t.Run(backend.String(), func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0o700); err != nil {
				t.Fatal(err)
			}
			row := base.Profiles(backend)[nativerouting.RoleImplementation][0]
			model := row.Model + "-prototype"
			overlay := fmt.Sprintf("harness-config:\n  %s:\n    implementation:\n      prototype-only:\n        model: %s\n        effort: %s\n        capability: economy\n        deliberation: light\n        context: standard\n        assurance: ordinary\n        cost_rank: 99\n", backend, model, row.Effort)
			for name, source := range map[string]string{"config.yaml": harnessconfigtest.YAML, "config.local.yaml": overlay} {
				if err := os.WriteFile(filepath.Join(root, ".pyxis", name), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			effective, err := harnessconfig.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, construct := range []func(entity.Harness, config.Config, harnessconfig.Config) (port.AgentRuntime, error){harness.New, harness.NewReview} {
				rt, err := construct(backend, config.Defaults(), effective)
				if err != nil {
					t.Fatal(err)
				}
				for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
					catalog, _, err := rt.(port.CatalogProvider).Catalog(role)
					if err != nil {
						t.Fatal(err)
					}
					profiles := effective.Profiles(backend)[role]
					if len(catalog.Entries) != len(profiles) {
						t.Fatalf("%s: adapter catalog has %d profiles, effective catalog has %d", role, len(catalog.Entries), len(profiles))
					}
					for _, profile := range profiles {
						found := false
						for _, entry := range catalog.Entries {
							if entry.NativeProfileID != profile.ID {
								continue
							}
							found = true
							if entry.NativeModel != profile.Model || entry.NativeEffort != profile.Effort {
								t.Fatalf("profile %s changed model or effort: %+v", profile.ID, entry)
							}
							if err := rt.(nativerouting.NativeCapability).ValidateProfile(entry); err != nil {
								t.Fatalf("adapter rejected effective profile %s: %v", profile.ID, err)
							}
						}
						if !found {
							t.Fatalf("adapter lost effective profile %s", profile.ID)
						}
					}
				}
			}
		})
	}
}

func TestFactoryPreservesClaudeTurnOptions(t *testing.T) {
	hc, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, turns := range []int{0, 7} {
		cfg := config.Defaults()
		if turns > 0 {
			cfg, err = config.Parse(fmt.Appendf(nil, "executor:\n  caps:\n    worker_turns: %d\n", turns))
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, review := range []bool{false, true} {
			construct := harness.New
			opts := []claude.Option{claude.WithProfiles(hc.Profiles(entity.Claude))}
			role := nativerouting.RoleImplementation
			if review {
				construct = harness.NewReview
				role = nativerouting.RoleReview
			} else {
				opts = append(opts, claude.WithMaxTurns(turns))
			}
			direct, err := claude.New(opts...)
			if err != nil {
				t.Fatal(err)
			}
			shared, err := construct(entity.Claude, cfg, hc)
			if err != nil {
				t.Fatal(err)
			}
			profile := hc.Profiles(entity.Claude)[role][0]
			spec := port.NativeLaunchSpec{Role: role, NativeModel: profile.Model, NativeEffort: profile.Effort}
			want, err := direct.CaptureNativeLaunch(spec)
			if err != nil {
				t.Fatal(err)
			}
			got, err := shared.(port.NativeLaunchCapturer).CaptureNativeLaunch(spec)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("turns=%d review=%v: got %+v, want %+v, err %v", turns, review, got, want, err)
			}
		}
	}
}

func TestFactoryFallsBackToTrackedCatalog(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".pyxis", "config.yaml")
	if err := os.WriteFile(path, []byte(harnessconfigtest.YAML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, backend := range entity.Harnesses() {
		rt, err := harness.New(backend, config.Defaults(), harnessconfig.Config{})
		if err != nil || rt == nil {
			t.Fatalf("fallback for %s: runtime=%v, error=%v", backend, rt, err)
		}
		want, err := harnessconfig.LoadDefault(backend)
		if err != nil {
			t.Fatal(err)
		}
		catalog, _, err := rt.(port.CatalogProvider).Catalog(nativerouting.RoleImplementation)
		if err != nil || len(catalog.Entries) != len(want[nativerouting.RoleImplementation]) {
			t.Fatalf("fallback catalog for %s: %+v, %v", backend, catalog, err)
		}
	}
	if err := os.WriteFile(path, []byte("branch: main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, construct := range []func(entity.Harness, config.Config, harnessconfig.Config) (port.AgentRuntime, error){harness.New, harness.NewReview} {
		rt, err := construct(entity.Codex, config.Defaults(), harnessconfig.Config{})
		if rt != nil || err == nil || !strings.Contains(err.Error(), "codex has no model profiles") {
			t.Fatalf("missing fallback catalog must refuse construction: runtime=%v, error=%v", rt, err)
		}
	}
}

func TestFactoryPropagatesAdapterCatalogDefects(t *testing.T) {
	for _, tc := range []struct{ model, effort, want string }{
		{"missing-provider", "none", "invalid model"},
		{"provider/model", "high", "effort"},
	} {
		source := fmt.Appendf(nil, "harness-config:\n  opencode:\n    implementation:\n      invalid:\n        model: %s\n        effort: %s\n        capability: economy\n        deliberation: light\n        context: standard\n        assurance: ordinary\n        cost_rank: 1\n        strongest: true\n", tc.model, tc.effort)
		hc, err := harnessconfig.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, construct := range []func(entity.Harness, config.Config, harnessconfig.Config) (port.AgentRuntime, error){harness.New, harness.NewReview} {
			rt, err := construct(entity.OpenCode, config.Defaults(), hc)
			if rt != nil || err == nil || !strings.Contains(err.Error(), "build opencode backend:") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("adapter defect must refuse construction with context: runtime=%v, error=%v", rt, err)
			}
		}
	}
}

func TestFactoryRegistryFallback(t *testing.T) {
	hc, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	want, err := harness.New(entity.Codex, config.Defaults(), hc)
	if err != nil {
		t.Fatal(err)
	}
	registered := entity.Harness("factory-test-extension:" + t.TempDir())
	runtime.Register(registered, func() (port.AgentRuntime, error) { return want, nil })
	for _, construct := range []func(entity.Harness, config.Config, harnessconfig.Config) (port.AgentRuntime, error){harness.New, harness.NewReview} {
		got, err := construct(registered, config.Defaults(), harnessconfig.Config{})
		if err != nil || got != want {
			t.Fatalf("registered extension was not resolved: runtime=%v, error=%v", got, err)
		}
		got, err = construct("factory-test-unknown", config.Defaults(), harnessconfig.Config{})
		if got != nil || !errors.Is(err, runtime.ErrUnknownHarness) {
			t.Fatalf("unknown harness must preserve registry error: runtime=%v, error=%v", got, err)
		}
	}
}
