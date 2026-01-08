//go:build linux

package crun

import "testing"

func TestExecOptionWithDetach(t *testing.T) {
	cfg := &execConfig{}
	opt := WithDetach()
	opt(cfg)

	if !cfg.detach {
		t.Error("WithDetach should set detach to true")
	}
}

func TestExecOptionWithExecTTY(t *testing.T) {
	cfg := &execConfig{}
	opt := WithExecTTY()
	opt(cfg)

	if !cfg.terminal {
		t.Error("WithExecTTY should set terminal to true")
	}
}

func TestExecOptionWithWorkingDir(t *testing.T) {
	cfg := &execConfig{}
	opt := WithWorkingDir("/home/user")
	opt(cfg)

	if cfg.cwd != "/home/user" {
		t.Errorf("cwd = %q, want /home/user", cfg.cwd)
	}
}

