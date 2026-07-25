//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// Process control, Unix flavour. See proc_windows.go for the counterpart.
//
// yt-dlp is not a leaf process: it spawns ffmpeg to merge streams, cut
// sections, and extract audio. Killing yt-dlp alone would leave ffmpeg running
// (and holding the output file open), so we put the child in its own process
// group and signal the whole group at once.

// setProcAttr puts the command in a fresh process group so killProcessTree can
// take down yt-dlp and everything it spawned together.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree terminates the command and its descendants. Safe to call on a
// process that has already exited — the kill simply fails and is ignored.
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Negative PID means "the process group with this id", which is the group
	// setProcAttr created (the leader's pid doubles as the group id).
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		// The group may not exist if setProcAttr wasn't applied; fall back to
		// the single process rather than leaving it running.
		_ = cmd.Process.Kill()
	}
}
