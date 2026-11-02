package runner

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestLandlockWriteAccessAndABILayout(t *testing.T) {
	base := uint64(accessFSWriteFile | accessFSRemoveDir | accessFSRemoveFile |
		accessFSMakeChar | accessFSMakeDir | accessFSMakeReg | accessFSMakeSock |
		accessFSMakeFifo | accessFSMakeBlock | accessFSMakeSym)
	for _, tt := range []struct {
		abi  int
		want uint64
	}{{1, base}, {2, base | accessFSRefer}, {3, base | accessFSRefer | accessFSTruncate}} {
		if got := writeAccessFS(tt.abi); got != tt.want {
			t.Errorf("writeAccessFS(%d) = %#x, want %#x", tt.abi, got, tt.want)
		}
		if writeAccessFS(tt.abi)&(accessFSExecute|accessFSReadFile|accessFSReadDir) != 0 {
			t.Fatal("write-only ruleset restricted reads or execution")
		}
	}
	if got := unsafe.Sizeof(landlockRulesetAttr{}); got != 8 {
		t.Fatalf("ruleset attribute size = %d, want 8", got)
	}
	if got := unsafe.Sizeof(landlockPathBeneathAttr{}); got != 16 {
		t.Fatalf("path-beneath attribute size = %d, want 16", got)
	}
}

func TestLandlockRawWrappersReportKernelResults(t *testing.T) {
	t.Run("ABI success and error", func(t *testing.T) {
		k := landlockKernel{}
		k.syscall = func(trap, a1, a2, a3 uintptr) (uintptr, uintptr, syscall.Errno) {
			if trap != sysLandlockCreateRuleset || a1 != 0 || a2 != 0 || a3 != landlockCreateRulesetVersion {
				t.Fatalf("ABI syscall = (%d, %d, %d, %d)", trap, a1, a2, a3)
			}
			return 3, 0, 0
		}
		if got, err := k.abiVersion(); err != nil || got != 3 {
			t.Fatalf("ABI = %d, %v", got, err)
		}
		k.syscall = func(uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno) {
			return 0, 0, syscall.ENOSYS
		}
		if _, err := k.abiVersion(); err != syscall.ENOSYS {
			t.Fatalf("ABI error = %v, want %v", err, syscall.ENOSYS)
		}
	})

	t.Run("ruleset success and error", func(t *testing.T) {
		k := landlockKernel{}
		k.syscall = func(trap, attr, size, flags uintptr) (uintptr, uintptr, syscall.Errno) {
			if trap != sysLandlockCreateRuleset || attr == 0 || size != unsafe.Sizeof(landlockRulesetAttr{}) || flags != 0 {
				t.Fatalf("create syscall = (%d, %d, %d, %d)", trap, attr, size, flags)
			}
			return 17, 0, 0
		}
		if got, err := k.createRuleset(writeAccessFS(3)); err != nil || got != 17 {
			t.Fatalf("createRuleset = %d, %v", got, err)
		}
		k.syscall = func(uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno) {
			return 0, 0, syscall.EPERM
		}
		if _, err := k.createRuleset(writeAccessFS(1)); err != syscall.EPERM {
			t.Fatalf("createRuleset error = %v, want %v", err, syscall.EPERM)
		}
	})

	t.Run("path rule open and syscall failures", func(t *testing.T) {
		k := landlockKernel{}
		k.openPath = func(string) (int, error) { return -1, syscall.ENOENT }
		if err := k.addPathBeneathRule(3, "/missing", writeAccessFS(1)); err == nil || !strings.Contains(err.Error(), "open /missing") {
			t.Fatalf("open failure = %v", err)
		}

		closed := -1
		k.openPath = func(path string) (int, error) {
			if path != "/allowed" {
				t.Fatalf("open path = %q", path)
			}
			return 23, nil
		}
		k.close = func(fd int) error { closed = fd; return nil }
		k.syscall6 = func(trap, ruleset, ruleType, attr, a4, a5, a6 uintptr) (uintptr, uintptr, syscall.Errno) {
			if trap != sysLandlockAddRule || ruleset != 3 || ruleType != landlockRuleTypePathBeneath || attr == 0 {
				t.Fatalf("add-rule syscall = (%d, %d, %d, %d)", trap, ruleset, ruleType, attr)
			}
			return 0, 0, syscall.EINVAL
		}
		if err := k.addPathBeneathRule(3, "/allowed", writeAccessFS(2)); err == nil || !strings.Contains(err.Error(), "landlock_add_rule(/allowed)") {
			t.Fatalf("add-rule failure = %v", err)
		}
		if closed != 23 {
			t.Fatalf("closed fd = %d, want 23", closed)
		}
	})
}

func TestRestrictSelfToPathsFailsClosedAtEveryKernelBoundary(t *testing.T) {
	k := landlockKernel{}
	if err := k.restrictSelfToPaths(nil); err == nil || !strings.Contains(err.Error(), "no allowed roots") {
		t.Fatalf("empty roots error = %v", err)
	}

	for _, failAt := range []string{"abi", "unsupported", "create", "add", "prctl", "restrict", ""} {
		name := failAt
		if name == "" {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			k := landlockKernel{}
			k.openPath = func(string) (int, error) {
				if failAt == "add" {
					return -1, syscall.ENOENT
				}
				return 29, nil
			}
			k.close = func(int) error { return nil }
			k.syscall = func(trap, a1, a2, a3 uintptr) (uintptr, uintptr, syscall.Errno) {
				switch trap {
				case sysLandlockCreateRuleset:
					if a1 == 0 {
						if failAt == "abi" {
							return 0, 0, syscall.ENOSYS
						}
						if failAt == "unsupported" {
							return 0, 0, 0
						}
						return 3, 0, 0
					}
					if failAt == "create" {
						return 0, 0, syscall.EPERM
					}
					return 31, 0, 0
				case sysLandlockRestrictSelf:
					if failAt == "restrict" {
						return 0, 0, syscall.EBADF
					}
					return 0, 0, 0
				default:
					t.Fatalf("unexpected syscall trap %d", trap)
					return 0, 0, syscall.ENOSYS
				}
			}
			k.syscall6 = func(trap, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, uintptr, syscall.Errno) {
				switch trap {
				case sysLandlockAddRule:
					return 0, 0, 0
				default:
					t.Fatalf("unexpected syscall6 trap %d", trap)
					return 0, 0, syscall.ENOSYS
				}
			}

			k.noNewPrivs = func() error {
				if failAt == "prctl" {
					return syscall.EPERM
				}
				return nil
			}

			err := k.restrictSelfToPaths([]string{"/allowed"})
			if failAt == "" {
				if err != nil {
					t.Fatalf("success = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s boundary unexpectedly succeeded", failAt)
			}
		})
	}
}
