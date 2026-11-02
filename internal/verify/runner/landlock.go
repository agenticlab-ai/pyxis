package runner

import (
	"fmt"
	"syscall"
	"unsafe"
)

// The Landlock syscalls (kernel >=5.13, linux/landlock.h) have no wrapper in
// the standard library's syscall package on amd64/arm64: those
// architectures' zsysnum tables were generated years before Landlock existed
// and were never regenerated (only newer-added architectures — e.g. loong64
// — carry them; confirmed against this Go toolchain's own
// zsysnum_linux_loong64.go, which lists all three at 444/445/446). The
// numbers are architecture-invariant: kernel policy began assigning new
// syscalls THE SAME number on x86_64 and the generic (arm64/riscv64/loong64)
// ABI starting in this era specifically to stop future per-arch divergence,
// which is exactly what the loong64 table's matching values confirm.
const (
	sysLandlockCreateRuleset = 444
	sysLandlockAddRule       = 445
	sysLandlockRestrictSelf  = 446
)

// landlockCreateRulesetVersion, passed as landlock_create_ruleset's flags
// with a NULL attr, makes the syscall return the running kernel's supported
// Landlock ABI version instead of creating a ruleset.
const landlockCreateRulesetVersion = 1 << 0

// landlockRuleTypePathBeneath is landlock_add_rule's rule_type for a
// landlockPathBeneathAttr rule (enum landlock_rule_type).
const landlockRuleTypePathBeneath = 1

// Access-right bits for handled_access_fs (linux/landlock.h, ABI 1 unless
// noted). Only the WRITE-class rights are ever placed in a ruleset this
// package builds (writeAccessFS) — EXECUTE, READ_FILE, and READ_DIR are
// deliberately never included in ANY ruleset here, so reads and execution
// stay completely unrestricted everywhere: DINV-5 confines writes, never
// reads (a verb still needs to read its toolchain, stdlib, and the rest of
// the filesystem system-wide).
const (
	accessFSExecute    = 1 << 0
	accessFSWriteFile  = 1 << 1
	accessFSReadFile   = 1 << 2
	accessFSReadDir    = 1 << 3
	accessFSRemoveDir  = 1 << 4
	accessFSRemoveFile = 1 << 5
	accessFSMakeChar   = 1 << 6
	accessFSMakeDir    = 1 << 7
	accessFSMakeReg    = 1 << 8
	accessFSMakeSock   = 1 << 9
	accessFSMakeFifo   = 1 << 10
	accessFSMakeBlock  = 1 << 11
	accessFSMakeSym    = 1 << 12
	accessFSRefer      = 1 << 13 // ABI 2 (kernel 5.19)
	accessFSTruncate   = 1 << 14 // ABI 3 (kernel 6.2)
)

// landlockRulesetAttr mirrors struct landlock_ruleset_attr: one u64
// access-rights bitmask. Extensible structs of this shape (a fixed prefix
// plus a size argument passed separately) are a standard Linux UAPI
// convention — passing this shorter, ABI-1-only layout works unchanged
// against any newer kernel, which zero-fills whatever the caller's size
// did not cover.
type landlockRulesetAttr struct {
	handledAccessFS uint64
}

// landlockPathBeneathAttr mirrors struct landlock_path_beneath_attr: the
// access rights granted beneath parentFd, an O_PATH descriptor on the
// allowed directory. The compiler pads this to 16 bytes (8-byte alignment
// from the leading u64) exactly as the kernel's C struct does — asserted by
// TestLandlockWriteAccessAndABILayout.
type landlockPathBeneathAttr struct {
	allowedAccess uint64
	parentFd      int32
	_             int32 // explicit trailing padding, matching the C struct's layout
}

// landlockKernel separates Linux syscall bindings from the ruleset protocol.
// Tests supply every operation explicitly; other platforms never bind Linux
// syscall numbers to their host kernel.
type landlockKernel struct {
	syscall    func(uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno)
	syscall6   func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) (uintptr, uintptr, syscall.Errno)
	openPath   func(string) (int, error)
	close      func(int) error
	noNewPrivs func() error
}

// writeAccessFS is the write-class access-rights mask this package confines
// verbs to, widened with REFER/TRUNCATE when the running kernel's Landlock
// ABI supports them.
func writeAccessFS(abi int) uint64 {
	mask := uint64(accessFSWriteFile | accessFSRemoveDir | accessFSRemoveFile |
		accessFSMakeChar | accessFSMakeDir | accessFSMakeReg | accessFSMakeSock |
		accessFSMakeFifo | accessFSMakeBlock | accessFSMakeSym)
	if abi >= 2 {
		mask |= accessFSRefer
	}
	if abi >= 3 {
		mask |= accessFSTruncate
	}
	return mask
}

// abiVersion queries the bound kernel's supported Landlock ABI
// version. A returned 0 (with a nil error) means Landlock is compiled out or
// disabled via the LSM boot list on a kernel new enough to have the syscall
// at all but reporting no supported ABI; an error means the syscall itself
// is unavailable (kernel <5.13).
func (k landlockKernel) abiVersion() (int, error) {
	r1, _, errno := k.syscall(sysLandlockCreateRuleset, 0, 0, uintptr(landlockCreateRulesetVersion))
	if errno != 0 {
		return 0, errno
	}
	return int(r1), nil
}

// createRuleset creates a Landlock ruleset governing exactly
// handledAccessFS's rights, returning its file descriptor.
func (k landlockKernel) createRuleset(handledAccessFS uint64) (int, error) {
	attr := landlockRulesetAttr{handledAccessFS: handledAccessFS}
	r1, _, errno := k.syscall(sysLandlockCreateRuleset, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return -1, errno
	}
	return int(r1), nil
}

// addPathBeneathRule grants allowedAccess beneath path on rulesetFD. Reads
// and execution are unaffected regardless of allowedAccess's value here:
// only the rights actually present in the RULESET's own handled_access_fs
// (set at createRuleset time) are ever governed at all.
func (k landlockKernel) addPathBeneathRule(rulesetFD int, path string, allowedAccess uint64) error {
	fd, err := k.openPath(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = k.close(fd) }() // release the owned rule path descriptor

	attr := landlockPathBeneathAttr{allowedAccess: allowedAccess, parentFd: int32(fd)}
	_, _, errno := k.syscall6(sysLandlockAddRule, uintptr(rulesetFD), uintptr(landlockRuleTypePathBeneath), uintptr(unsafe.Pointer(&attr)), 0, 0, 0)
	if errno != 0 {
		return fmt.Errorf("landlock_add_rule(%s): %w", path, errno)
	}
	return nil
}

// restrictSelfToPaths applies the whole Landlock sequence to the CURRENT
// process: detect the ABI, create a write-only ruleset, grant it beneath
// every root, set no_new_privs, and seal the process with
// landlock_restrict_self. Any failure at any step — including "Landlock is
// unsupported here" — returns an error rather than silently leaving the
// process unconfined: RunConfinedExec's caller (confineFail) fails closed on
// exactly this, never falling back to executing the verb open.
func (k landlockKernel) restrictSelfToPaths(roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("landlock: no allowed roots supplied")
	}

	abi, err := k.abiVersion()
	if err != nil {
		return fmt.Errorf("landlock: query ABI version: %w", err)
	}
	if abi < 1 {
		return fmt.Errorf("landlock: unsupported by this kernel (ABI %d) — requires kernel >=5.13 with Landlock enabled", abi)
	}

	access := writeAccessFS(abi)
	rulesetFD, err := k.createRuleset(access)
	if err != nil {
		return fmt.Errorf("landlock: create ruleset: %w", err)
	}
	defer func() { _ = k.close(rulesetFD) }() // release the owned ruleset descriptor

	for _, root := range roots {
		if err := k.addPathBeneathRule(rulesetFD, root, access); err != nil {
			return fmt.Errorf("landlock: %w", err)
		}
	}

	if err := k.noNewPrivs(); err != nil {
		return fmt.Errorf("landlock: prctl(PR_SET_NO_NEW_PRIVS): %w", err)
	}

	if _, _, errno := k.syscall(sysLandlockRestrictSelf, uintptr(rulesetFD), 0, 0); errno != 0 {
		return fmt.Errorf("landlock: restrict_self: %w", errno)
	}
	return nil
}
