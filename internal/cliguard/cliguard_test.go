package cliguard_test

import (
	"errors"
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/cliguard"
)

// newFS builds a verb-shaped FlagSet with one string and one bool flag, the
// shapes the mains use.
func newFS(t *testing.T) *flag.FlagSet {
	t.Helper()
	fs := flag.NewFlagSet("verb", flag.ContinueOnError)
	fs.String("out", "", "output path")
	fs.Bool("dry-run", false, "preview only")
	return fs
}

func TestPositionals(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantPos []string
		wantErr string // "" = clean; else a substring of the refusal
	}{
		{"no args", nil, nil, ""},
		{"positionals only", []string{"a", "b"}, []string{"a", "b"}, ""},
		{"flags then positionals", []string{"--out", "x", "--dry-run", "a"}, []string{"a"}, ""},
		{"flag-like value consumed by its flag", []string{"--out", "-", "a"}, []string{"a"}, ""},
		{"flag after positional", []string{"a", "--dry-run"}, nil, `flag-like token "--dry-run" in a positional slot — flags precede positionals`},
		{"short flag after positional", []string{"a", "-x"}, nil, `flag-like token "-x"`},
		{"bare dash positional", []string{"-"}, nil, `flag-like token "-"`},
		{"unknown flag", []string{"--bogus"}, nil, "flag provided but not defined: -bogus"},
		{"malformed bool value", []string{"--dry-run=maybe"}, nil, "invalid boolean value"},
		{"flag-like stranded behind --", []string{"--", "-x"}, nil, `flag-like token "-x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := cliguard.Positionals(newFS(t), tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Positionals(%q) refused: %v", tt.args, err)
				}
				if !slices.Equal(pos, tt.wantPos) {
					t.Fatalf("Positionals(%q) = %q, want %q", tt.args, pos, tt.wantPos)
				}
				return
			}
			if err == nil {
				t.Fatalf("Positionals(%q) = %q, want refusal containing %q", tt.args, pos, tt.wantErr)
			}
			var ue *cliguard.UsageError
			if !errors.As(err, &ue) {
				t.Fatalf("Positionals(%q) error %T, want *UsageError", tt.args, err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Positionals(%q) refusal %q does not name %q", tt.args, err, tt.wantErr)
			}
			if pos != nil {
				t.Fatalf("Positionals(%q) returned positionals %q alongside a refusal", tt.args, pos)
			}
		})
	}
}

func TestPositionalsHelpUnwrapsToErrHelp(t *testing.T) {
	_, err := cliguard.Positionals(newFS(t), []string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("Positionals(-h) = %v, want a refusal unwrapping to flag.ErrHelp", err)
	}
	var ue *cliguard.UsageError
	if !errors.As(err, &ue) || ue.Verb != "verb" {
		t.Fatalf("Positionals(-h) = %#v, want *UsageError with Verb %q", err, "verb")
	}
}

func TestInterspersed(t *testing.T) {
	t.Run("flag after positional is consumed", func(t *testing.T) {
		fs := newFS(t)
		out := fs.Lookup("out")
		pos, err := cliguard.Interspersed(fs, []string{"a", "--out", "x", "b", "--dry-run"})
		if err != nil {
			t.Fatalf("Interspersed refused: %v", err)
		}
		if want := []string{"a", "b"}; !slices.Equal(pos, want) {
			t.Fatalf("positionals = %q, want %q", pos, want)
		}
		if out.Value.String() != "x" {
			t.Fatalf("--out = %q, want %q (interspersed flags must be consumed)", out.Value.String(), "x")
		}
	})
	t.Run("unknown flag refuses wherever it appears", func(t *testing.T) {
		_, err := cliguard.Interspersed(newFS(t), []string{"a", "--bogus"})
		var ue *cliguard.UsageError
		if !errors.As(err, &ue) || !strings.Contains(err.Error(), "-bogus") {
			t.Fatalf("Interspersed(a --bogus) = %v, want an unknown-flag *UsageError", err)
		}
	})
	t.Run("flag-like leftover behind -- refuses", func(t *testing.T) {
		_, err := cliguard.Interspersed(newFS(t), []string{"a", "--", "-x"})
		var ue *cliguard.UsageError
		if !errors.As(err, &ue) || !strings.Contains(err.Error(), `flag-like token "-x"`) {
			t.Fatalf("Interspersed(a -- -x) = %v, want the flag-like-leftover refusal", err)
		}
	})
}

func TestCutArgv(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOwn  []string
		wantArgv []string
		found    bool
	}{
		{"no separator", []string{"docgate", "gate"}, []string{"docgate", "gate"}, nil, false},
		{"empty args", nil, nil, nil, false},
		{"separator splits", []string{"docgate", "--", "gate", "--merge-base", "origin/main"}, []string{"docgate"}, []string{"gate", "--merge-base", "origin/main"}, true},
		{"separator first", []string{"--", "a"}, []string{}, []string{"a"}, true},
		{"separator last", []string{"docgate", "--"}, []string{"docgate"}, []string{}, true},
		{"second separator passes verbatim", []string{"docgate", "--", "fold", "--", "dir"}, []string{"docgate"}, []string{"fold", "--", "dir"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			own, argv, found := cliguard.CutArgv(tt.args)
			if found != tt.found {
				t.Fatalf("CutArgv(%q) found = %v, want %v", tt.args, found, tt.found)
			}
			if !slices.Equal(own, tt.wantOwn) {
				t.Fatalf("CutArgv(%q) own = %q, want %q", tt.args, own, tt.wantOwn)
			}
			if !slices.Equal(argv, tt.wantArgv) {
				t.Fatalf("CutArgv(%q) argv = %q, want %q (bytes through, untouched)", tt.args, argv, tt.wantArgv)
			}
		})
	}
}

func TestUsageErrorMessageCarriesVerb(t *testing.T) {
	e := &cliguard.UsageError{Verb: "exec", Reason: "boom"}
	if got := e.Error(); got != "exec: boom" {
		t.Fatalf("Error() = %q, want %q", got, "exec: boom")
	}
	bare := &cliguard.UsageError{Reason: "boom"}
	if got := bare.Error(); got != "boom" {
		t.Fatalf("Error() = %q, want %q", got, "boom")
	}
}

func TestGuardPanicsOnNonContinueFlagSet(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Positionals accepted an ExitOnError FlagSet — it must panic (programmer error: the refusal would be bypassed)")
		}
	}()
	fs := flag.NewFlagSet("verb", flag.ExitOnError)
	_, _ = cliguard.Positionals(fs, nil) // panic expected before any parse
}
