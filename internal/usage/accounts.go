package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultAccountsRelativePath = ".codex-usage-monitor/accounts.json"
	accountsFileEnvVar          = "CODEX_USAGE_MONITOR_ACCOUNTS_FILE"
)

type accountFile struct {
	Version  int           `json:"version"`
	Accounts []accountItem `json:"accounts"`
}

type accountItem struct {
	Label     string `json:"label"`
	CodexHome string `json:"codex_home"`
}

type MonitorAccount struct {
	Label     string `json:"label"`
	CodexHome string `json:"codex_home"`
}

func loadMonitorAccounts() ([]MonitorAccount, string, error) {
	defaultHome, err := defaultCodexHome()
	if err != nil {
		return nil, "", err
	}

	collector := newAccountCollector()
	collector.add("default", defaultHome, 50, false)

	if envHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); envHome != "" {
		expanded, expandErr := expandPath(envHome)
		if expandErr != nil {
			collector.warnf("could not resolve CODEX_HOME: %v", expandErr)
		} else {
			collector.add("active", expanded, 40, true)
		}
	}

	fileAccounts, fileWarning, fileErr := loadAccountsFromFile()
	if fileErr != nil {
		collector.warnf("accounts file could not be read: %v", fileErr)
	} else {
		if fileWarning != "" {
			collector.warnf("%s", fileWarning)
		}
		for _, account := range fileAccounts {
			collector.add(account.Label, account.CodexHome, 100, true)
		}
	}

	autoAccounts, autoWarning, autoErr := discoverMonitorAccountsFromFilesystem()
	if autoErr != nil {
		collector.warnf("auto discovery error: %v", autoErr)
	} else {
		if autoWarning != "" {
			collector.warnf("%s", autoWarning)
		}
		for _, account := range autoAccounts {
			collector.add(account.Label, account.CodexHome, 30, false)
		}
	}

	out := collector.toAccounts()
	if len(out) == 0 {
		out = append(out, MonitorAccount{
			Label:     "default",
			CodexHome: defaultHome,
		})
	}

	return out, collector.warningString(), nil
}

func loadAccountsFromFile() ([]MonitorAccount, string, error) {
	accountsPath, err := resolveAccountsFilePath()
	if err != nil {
		return nil, "", fmt.Errorf("resolve accounts file: %w", err)
	}

	data, err := os.ReadFile(accountsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read accounts file %s: %w", accountsPath, err)
	}

	var raw accountFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, "", fmt.Errorf("decode accounts file %s: %w", accountsPath, err)
	}
	if len(raw.Accounts) == 0 {
		return nil, fmt.Sprintf("accounts file %s is empty", accountsPath), nil
	}

	out := make([]MonitorAccount, 0, len(raw.Accounts))
	for i, a := range raw.Accounts {
		label := strings.TrimSpace(a.Label)
		if label == "" {
			label = fmt.Sprintf("account-%d", i+1)
		}
		home, err := expandPath(strings.TrimSpace(a.CodexHome))
		if err != nil {
			return nil, "", fmt.Errorf("resolve codex_home for account %q: %w", label, err)
		}
		if strings.TrimSpace(home) == "" {
			return nil, "", fmt.Errorf("account %q has empty codex_home", label)
		}
		out = append(out, MonitorAccount{
			Label:     label,
			CodexHome: filepath.Clean(home),
		})
	}
	return out, "", nil
}

func discoverMonitorAccountsFromFilesystem() ([]MonitorAccount, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve home directory: %w", err)
	}
	paths, warnings, err := discoverCodexHomesFromSystem(home)
	if err != nil {
		return nil, "", err
	}

	out := make([]MonitorAccount, 0, len(paths))
	for _, path := range paths {
		if !hasUsageSignals(path) {
			continue
		}
		out = append(out, MonitorAccount{
			Label:     labelForDiscoveredHome(path),
			CodexHome: filepath.Clean(path),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, strings.Join(dedupeStrings(warnings), "; "), nil
}

func discoverCodexHomesFromSystem(home string) ([]string, []string, error) {
	candidates := map[string]struct{}{}
	var warnings []string

	patterns := []string{
		filepath.Join(home, ".codex*"),
	}

	for depth := 1; depth <= 5; depth++ {
		patterns = append(patterns, homePatternWithSuffix(home, depth, "codex-home"))
		patterns = append(patterns, homePatternWithSuffix(home, depth, ".codex"))
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("invalid discovery pattern %q: %v", pattern, err))
			continue
		}
		for _, match := range matches {
			if !dirExists(match) {
				continue
			}
			candidates[filepath.Clean(match)] = struct{}{}
		}
	}

	out := make([]string, 0, len(candidates))
	for candidate := range candidates {
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out, warnings, nil
}

func homePatternWithSuffix(home string, depth int, suffix string) string {
	parts := make([]string, 0, depth+2)
	parts = append(parts, home)
	for i := 0; i < depth; i++ {
		parts = append(parts, "*")
	}
	parts = append(parts, suffix)
	return filepath.Join(parts...)
}

func labelForDiscoveredHome(codexHome string) string {
	base := filepath.Base(codexHome)
	switch {
	case base == "codex-home":
		parent := filepath.Base(filepath.Dir(codexHome))
		if strings.TrimSpace(parent) != "" && parent != "." && parent != string(filepath.Separator) {
			return safeLabel(parent)
		}
	case strings.HasPrefix(base, ".codex"):
		if base == ".codex" {
			return "default"
		}
		return safeLabel(strings.TrimPrefix(base, "."))
	}
	return safeLabel(base)
}

func hasUsageSignals(codexHome string) bool {
	if fileExists(filepath.Join(codexHome, "auth.json")) {
		return true
	}
	if dirExists(filepath.Join(codexHome, "sessions")) {
		return true
	}
	if dirExists(filepath.Join(codexHome, "archived_sessions")) {
		return true
	}
	return false
}

type accountCollector struct {
	byHome   map[string]accountCandidate
	warnings []string
}

type accountCandidate struct {
	account  MonitorAccount
	priority int
}

func newAccountCollector() *accountCollector {
	return &accountCollector{
		byHome: map[string]accountCandidate{},
	}
}

func (c *accountCollector) add(label, codexHome string, priority int, allowWithoutSignals bool) {
	normalized := filepath.Clean(strings.TrimSpace(codexHome))
	if normalized == "" {
		return
	}
	if !allowWithoutSignals && !hasUsageSignals(normalized) {
		return
	}
	if existing, ok := c.byHome[normalized]; ok {
		if existing.priority >= priority {
			return
		}
	}
	c.byHome[normalized] = accountCandidate{
		account: MonitorAccount{
			Label:     safeLabel(label),
			CodexHome: normalized,
		},
		priority: priority,
	}
}

func (c *accountCollector) warnf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		return
	}
	c.warnings = append(c.warnings, msg)
}

func (c *accountCollector) warningString() string {
	deduped := dedupeStrings(c.warnings)
	return strings.Join(deduped, "; ")
}

func (c *accountCollector) toAccounts() []MonitorAccount {
	out := make([]MonitorAccount, 0, len(c.byHome))
	for _, candidate := range c.byHome {
		out = append(out, candidate.account)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].CodexHome < out[j].CodexHome
	})
	return out
}

func safeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "account"
	}
	return label
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func resolveAccountsFilePath() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv(accountsFileEnvVar)); explicit != "" {
		return expandPath(explicit)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultAccountsRelativePath), nil
}

func defaultCodexHome() (string, error) {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		return expandPath(codexHome)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
