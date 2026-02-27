package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMonitorAccountsDefaultsWhenFileMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", "")
	t.Setenv(accountsFileEnvVar, filepath.Join(tmp, "missing.json"))

	accounts, warning, err := loadMonitorAccounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Label != "default" {
		t.Fatalf("expected default label, got %q", accounts[0].Label)
	}
	expectedHome := filepath.Join(tmp, ".codex")
	if accounts[0].CodexHome != expectedHome {
		t.Fatalf("expected default codex home %q, got %q", expectedHome, accounts[0].CodexHome)
	}
}

func TestLoadMonitorAccountsFromFileWithDedup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", "")
	accountsPath := filepath.Join(tmp, "accounts.json")
	t.Setenv(accountsFileEnvVar, accountsPath)

	content := `{
  "version": 1,
  "accounts": [
    {"label":"personal","codex_home":"~/codex/a"},
    {"label":"work","codex_home":"` + filepath.Join(tmp, "codex", "b") + `"},
    {"label":"dupe","codex_home":"` + filepath.Join(tmp, "codex", "b") + `"}
  ]
}`
	if err := os.WriteFile(accountsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write accounts file: %v", err)
	}

	accounts, warning, err := loadMonitorAccounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts after dedup, got %d", len(accounts))
	}
	if accounts[0].Label != "personal" {
		t.Fatalf("expected first label personal, got %q", accounts[0].Label)
	}
	if !strings.HasSuffix(accounts[0].CodexHome, filepath.Join("codex", "a")) {
		t.Fatalf("expected expanded home path, got %q", accounts[0].CodexHome)
	}
	if accounts[1].Label != "work" {
		t.Fatalf("expected second label work, got %q", accounts[1].Label)
	}
}

func TestLoadMonitorAccountsWarnsOnEmptyAccounts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", "")
	accountsPath := filepath.Join(tmp, "accounts.json")
	t.Setenv(accountsFileEnvVar, accountsPath)
	if err := os.WriteFile(accountsPath, []byte(`{"version":1,"accounts":[]}`), 0o600); err != nil {
		t.Fatalf("write accounts file: %v", err)
	}

	accounts, warning, err := loadMonitorAccounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Label != "default" {
		t.Fatalf("expected fallback default account")
	}
	if warning == "" {
		t.Fatalf("expected warning for empty accounts list")
	}
}

func TestLoadMonitorAccountsAutoDiscoversSystemCodexHomes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", "")
	t.Setenv(accountsFileEnvVar, filepath.Join(tmp, "missing.json"))

	discoveredHome := filepath.Join(tmp, "profiles", "work", "codex-home")
	if err := os.MkdirAll(discoveredHome, 0o755); err != nil {
		t.Fatalf("mkdir discovered home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(discoveredHome, "auth.json"), []byte(`{"tokens":{"access_token":"x"}}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	accounts, _, err := loadMonitorAccounts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, account := range accounts {
		if account.CodexHome == discoveredHome {
			found = true
			if account.Label != "work" {
				t.Fatalf("expected discovered label work, got %q", account.Label)
			}
		}
	}
	if !found {
		t.Fatalf("expected discovered codex home to be included")
	}
}
