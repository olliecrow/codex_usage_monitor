package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/olliecrow/codex_usage_monitor/internal/tui"
	"github.com/olliecrow/codex_usage_monitor/internal/usage"
	"golang.org/x/term"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return runTUI(nil)
	}

	switch args[0] {
	case "tui":
		return runTUI(args[1:])
	case "snapshot", "status":
		return runSnapshot(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "completion":
		return runCompletion(args[1:])
	case "-h", "--help", "help":
		printRootUsage()
		return 0
	default:
		// Treat bare flags as TUI flags for better UX.
		if strings.HasPrefix(args[0], "-") {
			return runTUI(args)
		}
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printRootUsage()
		return 2
	}
}

func runCompletion(args []string) int {
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "error: completion accepts zero or one shell argument (bash or zsh)")
		return 2
	}
	shell := "bash"
	if len(args) == 1 {
		shell = strings.TrimSpace(args[0])
	}
	script, err := completionScript(shell)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	fmt.Print(script)
	return 0
}

func runSnapshot(args []string) int {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "output normalized JSON")
	timeout := fs.Duration("timeout", 10*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "error: --timeout must be > 0")
		return 2
	}

	fetcher := usage.NewSnapshotFetcher()
	defer fetcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	out, err := fetcher.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printSnapshotHuman(out)
	return 0
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "output doctor report as JSON")
	timeout := fs.Duration("timeout", 20*time.Second, "doctor timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "error: --timeout must be > 0")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report := usage.RunDoctor(ctx)

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to encode JSON: %v\n", err)
			return 1
		}
	} else {
		printDoctorHuman(report)
	}

	if !report.Healthy() {
		return 1
	}
	return 0
}

func runTUI(args []string) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	interval := fs.Duration("interval", 60*time.Second, "poll interval")
	timeout := fs.Duration("timeout", 10*time.Second, "per-poll fetch timeout")
	noColor := fs.Bool("no-color", false, "disable color styling")
	noAltScreen := fs.Bool("no-alt-screen", false, "disable alternate screen mode")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "error: --interval must be > 0")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "error: --timeout must be > 0")
		return 2
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "warning: interactive TUI requires a TTY, falling back to snapshot output")
		return runSnapshot([]string{"--timeout", timeout.String()})
	}

	fetcher := usage.NewDefaultFetcher()
	defer fetcher.Close()

	err := tui.Run(tui.Options{
		Interval:  *interval,
		Timeout:   *timeout,
		NoColor:   *noColor,
		AltScreen: !*noAltScreen,
		Fetch: func(ctx context.Context) (*usage.Summary, error) {
			return fetcher.Fetch(ctx)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func printSnapshotHuman(out *usage.Summary) {
	fmt.Printf("data source: %s\n", out.Source)
	fmt.Printf("subscription plan: %s\n", out.PlanType)
	if out.AccountEmail != "" {
		fmt.Printf("account: %s\n", out.AccountEmail)
	}
	if out.AccountID != "" {
		fmt.Printf("account id: %s\n", out.AccountID)
	}
	if out.UserID != "" {
		fmt.Printf("user id: %s\n", out.UserID)
	}
	fmt.Printf("five-hour window: %d%% used", out.PrimaryWindow.UsedPercent)
	if out.PrimaryWindow.ResetsAt != nil {
		fmt.Printf(", resets at %s", out.PrimaryWindow.ResetsAt.Format(time.RFC3339))
	}
	if out.PrimaryWindow.SecondsUntilReset != nil {
		fmt.Printf(", in %s", (time.Duration(*out.PrimaryWindow.SecondsUntilReset) * time.Second).Round(time.Second))
	}
	fmt.Println()

	fmt.Printf("weekly window: %d%% used", out.SecondaryWindow.UsedPercent)
	if out.SecondaryWindow.ResetsAt != nil {
		fmt.Printf(", resets at %s", out.SecondaryWindow.ResetsAt.Format(time.RFC3339))
	}
	if out.SecondaryWindow.SecondsUntilReset != nil {
		fmt.Printf(", in %s", (time.Duration(*out.SecondaryWindow.SecondsUntilReset) * time.Second).Round(time.Second))
	}
	fmt.Println()
	if out.AdditionalLimitCount > 0 {
		fmt.Printf("additional limits: %d\n", out.AdditionalLimitCount)
	}
	if out.TotalAccounts > 0 {
		fmt.Printf("accounts: %d detected, %d reachable\n", out.TotalAccounts, out.SuccessfulAccounts)
	}
	if out.WindowAccountLabel != "" {
		fmt.Printf("window account: %s\n", out.WindowAccountLabel)
	}
	if out.ObservedTokensStatus != "" {
		fmt.Printf("observed token estimate status: %s\n", out.ObservedTokensStatus)
		fmt.Printf("five-hour tokens (sum across accounts): %s\n", formatObservedWindowShort(out.ObservedWindow5h, out.ObservedTokens5h))
		if split := formatObservedWindowSplit(out.ObservedWindow5h); split != "" {
			fmt.Printf("  estimated: %s\n", split)
		}
		fmt.Printf("weekly tokens (sum across accounts): %s\n", formatObservedWindowShort(out.ObservedWindowWeekly, out.ObservedTokensWeekly))
		if split := formatObservedWindowSplit(out.ObservedWindowWeekly); split != "" {
			fmt.Printf("  estimated: %s\n", split)
		}
	}
	for _, warning := range out.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
}

func formatObservedWindowShort(win *usage.ObservedTokenBreakdown, fallbackTotal *int64) string {
	if win == nil {
		if fallbackTotal == nil {
			return "n/a"
		}
		return formatCompactCount(*fallbackTotal)
	}
	return formatCompactCount(win.Total)
}

func formatObservedWindowSplit(win *usage.ObservedTokenBreakdown) string {
	if win == nil || !win.HasSplit {
		return ""
	}
	parts := []string{
		"input " + formatCompactCount(win.Input),
		"cached input " + formatCompactCount(win.CachedInput),
		"output " + formatCompactCount(win.Output),
	}
	if win.ReasoningOutput > 0 {
		parts = append(parts, "reasoning "+formatCompactCount(win.ReasoningOutput))
	}
	if win.HasCachedOutput && win.CachedOutput > 0 {
		parts = append(parts, "cached output "+formatCompactCount(win.CachedOutput))
	}
	return strings.Join(parts, " | ")
}

func formatCompactCount(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	if v < 1000 {
		return fmt.Sprintf("%s%d", sign, v)
	}

	units := []string{"", "k", "m", "b", "t"}
	value := float64(v)
	unitIndex := 0
	for value >= 1000 && unitIndex < len(units)-1 {
		value /= 1000
		unitIndex++
	}

	rounded := int64(math.Round(value))
	return fmt.Sprintf("%s%d%s", sign, rounded, units[unitIndex])
}

func printDoctorHuman(report usage.DoctorReport) {
	fmt.Println("codex usage monitor doctor")
	fmt.Println()
	for _, c := range report.Checks {
		state := "FAIL"
		if c.OK {
			state = "PASS"
		}
		fmt.Printf("[%s] %s\n", state, c.Name)
		fmt.Printf("  %s\n", c.Details)
	}
}

func printRootUsage() {
	fmt.Println("codex usage monitor")
	fmt.Println()
	fmt.Println("Track Codex subscription usage in a terminal user interface (TUI).")
	fmt.Println("The monitor is read-only and does not mutate Codex account data.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  codex-usage-monitor                       Run terminal user interface (default)")
	fmt.Println("  codex-usage-monitor tui [flags]           Run terminal user interface explicitly")
	fmt.Println("  codex-usage-monitor snapshot [flags]      Print one usage snapshot")
	fmt.Println("  codex-usage-monitor doctor [flags]        Run setup and source checks")
	fmt.Println("  codex-usage-monitor completion [shell]    Print shell completion script")
	fmt.Println()
	fmt.Println("Completion:")
	fmt.Println("  codex-usage-monitor completion bash > ~/.local/share/bash-completion/completions/codex-usage-monitor")
	fmt.Println("  codex-usage-monitor completion zsh > ~/.zsh/completions/_codex-usage-monitor")
	fmt.Println()
	fmt.Println("Snapshot flags:")
	fmt.Println("  --json            Output normalized JSON")
	fmt.Println("  --timeout 10s     Request timeout")
	fmt.Println()
	fmt.Println("Doctor flags:")
	fmt.Println("  --json            Output report as JSON")
	fmt.Println("  --timeout 20s     Doctor timeout")
	fmt.Println()
	fmt.Println("Terminal user interface flags:")
	fmt.Println("  --interval 60s    Poll interval")
	fmt.Println("  --timeout 10s     Per-poll fetch timeout")
	fmt.Println("  --no-color        Disable color styling")
	fmt.Println("  --no-alt-screen   Disable alternate screen mode")
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return `# bash completion for codex-usage-monitor
_codex_usage_monitor_completion() {
  local cur prev words cword
  _init_completion || return
  local commands="tui snapshot status doctor completion help"
  if [[ ${cword} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return
  fi
  case "${words[1]}" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh" -- "${cur}") )
      ;;
    snapshot|status)
      COMPREPLY=( $(compgen -W "--json --timeout" -- "${cur}") )
      ;;
    doctor)
      COMPREPLY=( $(compgen -W "--json --timeout" -- "${cur}") )
      ;;
    tui)
      COMPREPLY=( $(compgen -W "--interval --timeout --no-color --no-alt-screen" -- "${cur}") )
      ;;
    *)
      COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
      ;;
  esac
}
complete -F _codex_usage_monitor_completion codex-usage-monitor
`, nil
	case "zsh":
		return `#compdef codex-usage-monitor
_codex_usage_monitor() {
  local -a commands
  commands=(
    'tui:run terminal user interface'
    'snapshot:print one usage snapshot'
    'status:alias for snapshot'
    'doctor:run setup and source checks'
    'completion:print shell completion script'
    'help:show help text'
  )
  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi
  case "${words[2]}" in
    completion)
      _values 'shell' bash zsh
      ;;
    snapshot|status|doctor)
      _values 'flag' --json --timeout
      ;;
    tui)
      _values 'flag' --interval --timeout --no-color --no-alt-screen
      ;;
  esac
}
_codex_usage_monitor "$@"
`, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (expected bash or zsh)", shell)
	}
}
