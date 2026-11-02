package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/drivetogreen"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

type localStatusFake struct {
	executionHost
	head                                    string
	state, hosted                           port.CheckState
	statusErr, headErr, openErr, inspectErr error
	writes, opens                           int
	reads                                   int
	moveAfter                               int
}

func (f *localStatusFake) ProposalHead(context.Context, port.Proposal) (string, error) {
	f.reads++
	if f.moveAfter > 0 && f.reads >= f.moveAfter {
		return "moved", nil
	}
	return f.head, f.headErr
}
func (f *localStatusFake) SetStatus(_ context.Context, sha, name, state, _, _ string) error {
	f.writes++
	if sha != f.head || name != localGateContext || state != "success" {
		return errors.New("wrong local status")
	}
	if f.statusErr == nil {
		f.state = port.CheckGreen
	}
	return f.statusErr
}
func (f *localStatusFake) ReadStatus(context.Context, string, string) (port.CheckState, error) {
	return f.state, f.statusErr
}
func (f *localStatusFake) OpenProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (port.Proposal, error) {
	f.opens++
	return port.Proposal{Number: 7}, f.openErr
}
func (f *localStatusFake) Checks(context.Context, port.Proposal) (port.CheckState, error) {
	return f.hosted, nil
}
func (f *localStatusFake) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return port.LandingOpen, f.head, f.hosted, f.inspectErr
}

func localFixture(t *testing.T, mode string) (*localVerifiedHost, *localStatusFake, *healWorkdir) {
	t.Helper()
	fake := &localStatusFake{head: "abc", hosted: port.CheckGreen}
	bound, err := withLocalCI(fake, mode)
	if err != nil {
		t.Fatal(err)
	}
	h := bound.(*localVerifiedHost)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pyxis", "config.yaml"), []byte("code-gate:\n  unit:\n    command: go test ./...\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h.git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "status" {
			return nil, nil
		}
		return []byte("abc\n"), nil
	}
	h.run = func(context.Context, string, []string) drivetogreen.CommandResult {
		return drivetogreen.CommandResult{Passed: true}
	}
	return h, fake, &healWorkdir{dir: dir}
}

func TestLocalCIPublishesOnlyVerifiedExactRevision(t *testing.T) {
	h, fake, work := localFixture(t, "local")
	p, err := h.OpenProposal(t.Context(), port.Task{}, work, prdescription.Description{})
	if err != nil || fake.writes != 1 || fake.opens != 1 {
		t.Fatalf("open=%+v %v writes=%d", p, err, fake.writes)
	}
	state, err := h.Checks(t.Context(), p)
	if err != nil || state != port.CheckGreen {
		t.Fatalf("checks=%v %v", state, err)
	}
	if err := h.publishLocal(t.Context(), work, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, _, state, err := h.InspectProposal(t.Context(), p); err != nil || state != port.CheckGreen {
		t.Fatalf("inspect=%v %v", state, err)
	}
	if logs, err := h.FetchFailedLogs(t.Context(), p); err != nil || logs != "" {
		t.Fatalf("logs=%q %v", logs, err)
	}
}

func TestLocalCIUsesBothSharedGroupsThroughProductionRunner(t *testing.T) {
	for _, failedGroup := range []string{"", "default", "tests"} {
		t.Run("failure="+failedGroup, func(t *testing.T) {
			h, host, work := localFixture(t, "local")
			command := func(group string) string {
				if group == failedGroup {
					return "false"
				}
				return "true"
			}
			config := "code-gate:\n  quality:\n    command: " + command("default") + "\n  test:\n    group: tests\n    command: " + command("tests") + "\n"
			if err := os.WriteFile(filepath.Join(work.Dir(), ".pyxis/config.yaml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			// Only the code-host transport is fake. Run the actual local-CI
			// adapter, config loader, group scheduler and subprocess checks.
			h.run = codeGateRunner
			err := h.publishLocal(t.Context(), work, "abc")
			if failedGroup == "" {
				if err != nil || host.writes != 1 {
					t.Fatalf("green groups: %v, status writes %d", err, host.writes)
				}
			} else if err == nil || host.writes != 0 {
				t.Fatalf("red group %s certified: %v, status writes %d", failedGroup, err, host.writes)
			}
		})
	}
}

func TestLocalCIRefusesEmptyAdvisoryFailedDirtyOrChangedWork(t *testing.T) {
	for _, fault := range []string{"missing worktree", "empty checks", "advisory", "bad config", "dirty", "git error", "empty revision", "changed", "gate red", "gate error", "remote head", "read head", "open", "publish"} {
		t.Run(fault, func(t *testing.T) {
			h, fake, work := localFixture(t, "local")
			switch fault {
			case "missing worktree":
				work = nil
			case "empty checks", "advisory", "bad config":
				body := ""
				if fault == "advisory" {
					body = "code-gate:\n  only:\n    command: go test\n    class: advisory\n"
				}
				if fault == "bad config" {
					body = "code-gate:\n  invalid: value\n"
				}
				if err := os.WriteFile(filepath.Join(work.dir, ".pyxis", "config.yaml"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			case "dirty":
				h.git = func(context.Context, string, ...string) ([]byte, error) { return []byte(" M file"), nil }
			case "git error":
				h.git = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("git unavailable") }
			case "empty revision":
				h.git = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
			case "changed":
				reads := 0
				h.git = func(_ context.Context, _ string, args ...string) ([]byte, error) {
					if args[0] == "status" {
						return nil, nil
					}
					reads++
					if reads > 1 {
						return []byte("changed"), nil
					}
					return []byte("abc"), nil
				}
			case "gate red":
				h.run = func(context.Context, string, []string) drivetogreen.CommandResult {
					return drivetogreen.CommandResult{Output: "tests failed"}
				}
			case "gate error":
				h.run = func(context.Context, string, []string) drivetogreen.CommandResult {
					return drivetogreen.CommandResult{Err: errors.New("could not execute")}
				}
			case "remote head":
				fake.head = "foreign"
			case "read head":
				fake.headErr = errors.New("offline")
			case "open":
				fake.openErr = errors.New("write failed")
			case "publish":
				fake.statusErr = errors.New("status write failed")
			}
			var target port.Workdir = work
			if work == nil {
				target = nil
			}
			if _, err := h.OpenProposal(t.Context(), port.Task{}, target, prdescription.Description{}); err == nil {
				t.Fatal("unverified local result published")
			}
			if fault != "publish" && fake.writes != 0 {
				t.Fatalf("wrote before verification: %d", fake.writes)
			}
		})
	}
}

func TestLocalCIReadbackRequiresBothModesAndFreshHead(t *testing.T) {
	for _, fault := range []string{"pending", "red", "hosted red", "status error", "head error", "head moved", "inspection error", "inspection moved"} {
		t.Run(fault, func(t *testing.T) {
			h, fake, _ := localFixture(t, "both")
			fake.state = port.CheckGreen
			switch fault {
			case "pending":
				fake.state = port.CheckPending
			case "red":
				fake.state = port.CheckRed
			case "hosted red":
				fake.hosted = port.CheckRed
			case "status error":
				fake.statusErr = errors.New("offline")
			case "head error":
				fake.headErr = errors.New("offline")
			case "head moved":
				fake.moveAfter = 2
			case "inspection error":
				fake.inspectErr = errors.New("offline")
			case "inspection moved":
				fake.moveAfter = 3
			}
			var state port.CheckState
			var err error
			if strings.HasPrefix(fault, "inspection") {
				_, _, state, err = h.InspectProposal(t.Context(), port.Proposal{Number: 7})
			} else {
				state, err = h.Checks(t.Context(), port.Proposal{Number: 7})
			}
			if err == nil && state == port.CheckGreen {
				t.Fatal("unverified checks returned green")
			}
		})
	}
	h, _, work := localFixture(t, "local")
	if err := h.publishLocal(t.Context(), work, "other"); err == nil {
		t.Fatal("wrong pushed revision accepted")
	}
	if err := h.publishLocal(t.Context(), nil, "abc"); err == nil {
		t.Fatal("nil worktree accepted")
	}
	if got, err := withLocalCI(nil, "actions"); err != nil || got != nil {
		t.Fatalf("actions wrapper=%v %v", got, err)
	}
	if _, err := withLocalCI(nil, "local"); err == nil {
		t.Fatal("missing status provider accepted")
	}
}

func (f *localStatusFake) DescribeProposal(context.Context, port.Task, port.Workdir, prdescription.Description) (bool, error) {
	return false, nil
}
