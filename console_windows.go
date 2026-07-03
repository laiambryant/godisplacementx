//go:build windows

package main

import (
	"os"
	"syscall"
)

// attachParentConsole attaches the (GUI-subsystem) process to its parent
// console on Windows so CLI output is visible when launched from a terminal.
// It is a no-op for a process that already owns a console or has no parent
// console (e.g. launched by double-click).
func attachParentConsole() {
	const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS (-1)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := proc.Call(attachParentProcess); r == 0 {
		return // no parent console (or already attached)
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}
