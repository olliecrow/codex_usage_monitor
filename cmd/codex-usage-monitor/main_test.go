package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunHelpIncludesCompletionAndTerminalUserInterfaceText(t *testing.T) {
	code, stdout, _ := runWithCapturedOutput(t, []string{"help"})
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout, "completion [shell]") {
		t.Fatalf("expected help to mention completion command, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "terminal user interface (TUI)") {
		t.Fatalf("expected help to expand terminal user interface term, got:\n%s", stdout)
	}
}

func TestRunCompletionDefaultIsBash(t *testing.T) {
	code, stdout, _ := runWithCapturedOutput(t, []string{"completion"})
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout, "complete -F _codex_usage_monitor_completion codex-usage-monitor") {
		t.Fatalf("expected bash completion output, got:\n%s", stdout)
	}
}

func TestRunCompletionZsh(t *testing.T) {
	code, stdout, _ := runWithCapturedOutput(t, []string{"completion", "zsh"})
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(stdout, "#compdef codex-usage-monitor") {
		t.Fatalf("expected zsh completion output, got:\n%s", stdout)
	}
}

func TestRunCompletionRejectsUnknownShell(t *testing.T) {
	code, _, stderr := runWithCapturedOutput(t, []string{"completion", "fish"})
	if code != 2 {
		t.Fatalf("expected code 2 for unsupported shell, got %d", code)
	}
	if !strings.Contains(stderr, "unsupported shell") {
		t.Fatalf("expected unsupported shell error, got:\n%s", stderr)
	}
}

func runWithCapturedOutput(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	origStdout := os.Stdout
	origStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe failed: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe failed: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code := run(args)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	stdoutBytes, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("stdout read failed: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("stderr read failed: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()
	return code, string(stdoutBytes), string(stderrBytes)
}
