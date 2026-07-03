//go:build !windows

package main

// attachParentConsole is a no-op on non-Windows platforms.
func attachParentConsole() {}
