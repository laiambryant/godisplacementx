//go:build !desktop && !bindings

package main

import (
	"context"
	"os"
	"testing"
)

func withArgs(t *testing.T, args ...string) {
	t.Helper()
	old := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = old })
}

func TestRunDispatchesToCLI(t *testing.T) {
	withArgs(t, "godisplacementx", "version")
	if err := run(); err != nil {
		t.Fatal(err)
	}
}

func TestRunWithoutArgsHitsGUIStub(t *testing.T) {
	withArgs(t, "godisplacementx")
	if err := run(); err == nil {
		t.Fatal("the CLI-only build must refuse to open a GUI")
	}
}

func TestMainSuccess(t *testing.T) {
	withArgs(t, "godisplacementx", "version")
	main()
}

func TestMainExitsNonZeroOnError(t *testing.T) {
	withArgs(t, "godisplacementx")
	oldExit := exitFunc
	code := -1
	exitFunc = func(c int) { code = c }
	t.Cleanup(func() { exitFunc = oldExit })
	main()
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestNewAppStartup(t *testing.T) {
	a := NewApp()
	a.startup(context.Background())
	if a.ctx == nil {
		t.Fatal("startup must store the runtime context")
	}
}
