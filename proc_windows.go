//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

// Process control, Windows flavour. See proc_unix.go for the counterpart.

// setProcAttr hides the console window Windows would otherwise flash up for
// each child process. We're a GUI app, so a python/ffmpeg console popping in
// and out on every probe looks like a glitch.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// killProcessTree terminates the command and its descendants. Windows has no
// process groups we can signal the way Unix does, so we hand the job to
// taskkill, whose /T flag walks the child tree (yt-dlp → ffmpeg).
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	setProcAttr(kill)
	if err := kill.Run(); err != nil {
		// taskkill missing or the pid already gone — try the direct kill so we
		// at least stop the parent.
		_ = cmd.Process.Kill()
	}
}
