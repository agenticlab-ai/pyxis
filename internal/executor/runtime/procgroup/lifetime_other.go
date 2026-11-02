//go:build !darwin && !linux

package procgroup

import "os/exec"

type Lifetime struct{ cmd *exec.Cmd }

func NewLifetime(cmd *exec.Cmd) *Lifetime { return &Lifetime{cmd: cmd} }
func (*Lifetime) ObserveStart() error     { return nil }
func (s *Lifetime) Kill() error           { return KillChecked(s.cmd, true) }
