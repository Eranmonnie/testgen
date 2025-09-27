package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eranmonnie/testgen/internal/config"
)

func TestParseGitRange(t *testing.T) {
	cfg := &config.Config{
		Triggers: config.TriggerConfig{
			Manual: config.ManualTrigger{
				DefaultRange: "HEAD~2..HEAD",
			},
		},
	}

	tests := []struct {
		name         string
		rangeFlag    string
		expectedFrom string
		expectedTo   string
	}{
		{
			name:         "explicit range",
			rangeFlag:    "main..feature",
			expectedFrom: "main",
			expectedTo:   "feature",
		},
		{
			name:         "empty range uses config default",
			rangeFlag:    "",
			expectedFrom: "HEAD~2",
			expectedTo:   "HEAD",
		},
		{
			name:         "HEAD range",
			rangeFlag:    "HEAD~3..HEAD",
			expectedFrom: "HEAD~3",
			expectedTo:   "HEAD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := parseGitRange(tt.rangeFlag, cfg)
			if from != tt.expectedFrom {
				t.Errorf("Expected from '%s', got '%s'", tt.expectedFrom, from)
			}
			if to != tt.expectedTo {
				t.Errorf("Expected to '%s', got '%s'", tt.expectedTo, to)
			}
		})
	}
}

func TestInstallGitHooks(t *testing.T) {
	// Create a temporary git repository
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to temp directory
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create .git directory
	err = os.MkdirAll(".git/hooks", 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Test config with hooks
	cfg := &config.Config{
		Hooks: []string{"post-commit", "pre-push"},
	}

	// Install hooks
	err = installGitHooks(cfg)
	if err != nil {
		t.Fatalf("Failed to install git hooks: %v", err)
	}

	// Verify hooks were created
	for _, hookName := range cfg.Hooks {
		hookPath := filepath.Join(".git", "hooks", hookName)

		// Check file exists
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s was not created", hookName)
			continue
		}

		// Check file is executable
		info, err := os.Stat(hookPath)
		if err != nil {
			t.Errorf("Failed to stat hook %s: %v", hookName, err)
			continue
		}

		if info.Mode()&0111 == 0 {
			t.Errorf("Hook %s is not executable", hookName)
		}

		// Check content contains testgen
		content, err := os.ReadFile(hookPath)
		if err != nil {
			t.Errorf("Failed to read hook %s: %v", hookName, err)
			continue
		}

		if !strings.Contains(string(content), "testgen") {
			t.Errorf("Hook %s does not contain testgen command", hookName)
		}
	}
}

func TestUninstallGitHooks(t *testing.T) {
	// Create a temporary git repository
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to temp directory
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create .git directory
	hooksDir := ".git/hooks"
	err = os.MkdirAll(hooksDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create testgen hooks
	testgenHook := `#!/bin/sh
# testgen post-commit hook
exec testgen generate
`

	nonTestgenHook := `#!/bin/sh
# Some other hook
echo "other hook"
`

	// Install hooks
	err = os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(testgenHook), 0755)
	if err != nil {
		t.Fatalf("Failed to create testgen hook: %v", err)
	}

	err = os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte(nonTestgenHook), 0755)
	if err != nil {
		t.Fatalf("Failed to create other hook: %v", err)
	}

	// Uninstall hooks
	err = uninstallGitHooks()
	if err != nil {
		t.Fatalf("Failed to uninstall git hooks: %v", err)
	}

	// Verify testgen hook was removed
	if _, err := os.Stat(filepath.Join(hooksDir, "post-commit")); !os.IsNotExist(err) {
		t.Error("Testgen post-commit hook was not removed")
	}

	// Verify non-testgen hook was preserved
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-push")); os.IsNotExist(err) {
		t.Error("Non-testgen pre-push hook was incorrectly removed")
	}
}

func TestShowHooksStatus(t *testing.T) {
	// Create a temporary git repository
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to temp directory
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create .git directory
	hooksDir := ".git/hooks"
	err = os.MkdirAll(hooksDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create a testgen hook
	testgenHook := `#!/bin/sh
# testgen post-commit hook
exec testgen generate
`

	err = os.WriteFile(filepath.Join(hooksDir, "post-commit"), []byte(testgenHook), 0755)
	if err != nil {
		t.Fatalf("Failed to create testgen hook: %v", err)
	}

	// Test showHooksStatus (this mainly tests it doesn't crash)
	err = showHooksStatus()
	if err != nil {
		t.Errorf("showHooksStatus failed: %v", err)
	}

	// Note: In a real test, we'd capture stdout and verify the output,
	// but for simplicity we're just testing that it doesn't error
}

func TestLoadConfig(t *testing.T) {
	// Test loading default config
	cfg, err := loadConfig()
	if err != nil {
		// This might fail if there's no config file, which is OK
		// We're mainly testing the function doesn't panic
		t.Logf("Config load failed (this may be expected): %v", err)
	} else {
		// Basic sanity check
		if cfg.Mode != "auto" && cfg.Mode != "manual" {
			t.Errorf("Invalid mode in loaded config: %s", cfg.Mode)
		}
	}

	// Test loading from explicit file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "testgen.yml")

	configContent := `mode: manual
ai:
  provider: openai
  model: gpt-4
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set global configFile variable
	originalConfigFile := configFile
	configFile = configPath
	defer func() { configFile = originalConfigFile }()

	cfg, err = loadConfig()
	if err != nil {
		t.Fatalf("Failed to load explicit config: %v", err)
	}

	if cfg.Mode != "manual" {
		t.Errorf("Expected mode 'manual', got '%s'", cfg.Mode)
	}

	if cfg.AI.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", cfg.AI.Provider)
	}
}

// Mock config types for testing (to avoid import issues)
type Config struct {
	Mode     string
	Hooks    []string
	Triggers TriggerConfig
	AI       AIConfig
}

type TriggerConfig struct {
	Manual ManualTrigger
}

type ManualTrigger struct {
	DefaultRange string
}

type AIConfig struct {
	Provider string
	Model    string
}
