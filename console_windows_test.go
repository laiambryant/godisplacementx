//go:build windows && !desktop && !bindings

package main

import (
	"os"
	"testing"
)

func TestBindConsoleStreamsRestorable(t *testing.T) {
	oldOut, oldErr, oldIn := os.Stdout, os.Stderr, os.Stdin
	t.Cleanup(func() {
		os.Stdout, os.Stderr, os.Stdin = oldOut, oldErr, oldIn
	})
	bindConsoleStreams()
}
