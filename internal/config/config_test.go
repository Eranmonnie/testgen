package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test default values
	if config.Mode != "manual" {
		t.Errorf("Expected default mode 'manual', got '%s'", config.Mode)
	}

	if config.AI.Provider != "openai" {
		t.Errorf("Expected default provider 'openai', got '%s'", config.AI.Provider)
	}

	if config.AI.Model != "gpt-4" {
		t.Errorf("Expected default model 'gpt-4', got '%s'", config.AI.Model)
	}

	if config.AI.Temperature != 0.2 {
		t.Errorf("Expected default temperature 0.2, got %f", config.AI.Temperature)
	}

	if config.Output.Suffix != "_test.go" {
		t.Errorf("Expected default suffix '_test.go', got '%s'", config.Output.Suffix)
	}

	if config.Filtering.MaxComplexity != 15 {
		t.Errorf("Expected default max complexity 15, got %d", config.Filtering.MaxComplexity)
	}

	// Test slice defaults
	expectedPatterns := []string{"*.go"}
	if len(config.Triggers.Auto.FilePatterns) != len(expectedPatterns) {
		t.Errorf("Expected %d file patterns, got %d", len(expectedPatterns), len(config.Triggers.Auto.FilePatterns))
	}

	expectedExcludes := []string{"*_test.go", "vendor/*", ".git/*"}
	if len(config.Triggers.Auto.ExcludeFiles) != len(expectedExcludes) {
		t.Errorf("Expected %d exclude patterns, got %d", len(expectedExcludes), len(config.Triggers.Auto.ExcludeFiles))
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "testgen.yml")

	configContent := `mode: auto
hooks:
  - post-commit
  - pre-push

ai:
  provider: anthropic
  model: claude-3-sonnet
  temperature: 0.3
  max_tokens: 1500

output:
  directory: tests
  suffix: .test.go
  overwrite: true

filtering:
  include_unexported: true
  max_complexity: 20
  skip_patterns:
    - "helper*"
    - "internal*"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	config, err := LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test loaded values
	if config.Mode != "auto" {
		t.Errorf("Expected mode 'auto', got '%s'", config.Mode)
	}

	if len(config.Hooks) != 2 {
		t.Errorf("Expected 2 hooks, got %d", len(config.Hooks))
	}

	if config.AI.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", config.AI.Provider)
	}

	if config.AI.Model != "claude-3-sonnet" {
		t.Errorf("Expected model 'claude-3-sonnet', got '%s'", config.AI.Model)
	}

	if config.AI.Temperature != 0.3 {
		t.Errorf("Expected temperature 0.3, got %f", config.AI.Temperature)
	}

	if config.Output.Directory != "tests" {
		t.Errorf("Expected output directory 'tests', got '%s'", config.Output.Directory)
	}

	if config.Output.Suffix != ".test.go" {
		t.Errorf("Expected suffix '.test.go', got '%s'", config.Output.Suffix)
	}

	if !config.Output.Overwrite {
		t.Error("Expected overwrite to be true")
	}

	if !config.Filtering.IncludeUnexported {
		t.Error("Expected include_unexported to be true")
	}

	if config.Filtering.MaxComplexity != 20 {
		t.Errorf("Expected max complexity 20, got %d", config.Filtering.MaxComplexity)
	}

	expectedSkipPatterns := []string{"helper*", "internal*"}
	if len(config.Filtering.SkipPatterns) != len(expectedSkipPatterns) {
		t.Errorf("Expected %d skip patterns, got %d", len(expectedSkipPatterns), len(config.Filtering.SkipPatterns))
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "invalid mode",
			config: &Config{
				Mode:      "invalid",
				AI:        DefaultConfig().AI,
				Filtering: DefaultConfig().Filtering,
			},
			expectError: true,
			errorMsg:    "mode must be 'auto' or 'manual'",
		},
		{
			name: "invalid provider",
			config: &Config{
				Mode: "manual",
				AI: AIConfig{
					Provider:    "invalid",
					Temperature: 0.5,
					MaxTokens:   1000,
				},
				Filtering: DefaultConfig().Filtering,
			},
			expectError: true,
			errorMsg:    "unsupported AI provider",
		},
		{
			name: "invalid temperature too low",
			config: &Config{
				Mode: "manual",
				AI: AIConfig{
					Provider:    "openai",
					Temperature: -0.1,
					MaxTokens:   1000,
				},
				Filtering: DefaultConfig().Filtering,
			},
			expectError: true,
			errorMsg:    "temperature must be between 0 and 1",
		},
		{
			name: "invalid temperature too high",
			config: &Config{
				Mode: "manual",
				AI: AIConfig{
					Provider:    "openai",
					Temperature: 1.1,
					MaxTokens:   1000,
				},
				Filtering: DefaultConfig().Filtering,
			},
			expectError: true,
			errorMsg:    "temperature must be between 0 and 1",
		},
		{
			name: "invalid max tokens",
			config: &Config{
				Mode: "manual",
				AI: AIConfig{
					Provider:    "openai",
					Temperature: 0.5,
					MaxTokens:   -1,
				},
				Filtering: DefaultConfig().Filtering,
			},
			expectError: true,
			errorMsg:    "max_tokens must be positive",
		},
		{
			name: "invalid complexity range",
			config: &Config{
				Mode: "manual",
				AI:   DefaultConfig().AI,
				Filtering: FilterConfig{
					MinComplexity: 10,
					MaxComplexity: 5,
				},
			},
			expectError: true,
			errorMsg:    "min_complexity (10) cannot be greater than max_complexity (5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errorMsg)
				} else if !contains([]string{err.Error()}, tt.errorMsg) && tt.errorMsg != "" {
					// Check if error message contains expected text
					if len(tt.errorMsg) > 0 && !stringContains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("TESTGEN_MODE", "auto")
	os.Setenv("TESTGEN_API_KEY", "test-key-123")
	os.Setenv("TESTGEN_MODEL", "gpt-3.5-turbo")
	os.Setenv("TESTGEN_PROVIDER", "anthropic")
	defer func() {
		os.Unsetenv("TESTGEN_MODE")
		os.Unsetenv("TESTGEN_API_KEY")
		os.Unsetenv("TESTGEN_MODEL")
		os.Unsetenv("TESTGEN_PROVIDER")
	}()

	config := DefaultConfig()
	overrideWithEnv(config)

	if config.Mode != "auto" {
		t.Errorf("Expected mode 'auto' from env, got '%s'", config.Mode)
	}

	if config.AI.APIKey != "test-key-123" {
		t.Errorf("Expected API key 'test-key-123' from env, got '%s'", config.AI.APIKey)
	}

	if config.AI.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected model 'gpt-3.5-turbo' from env, got '%s'", config.AI.Model)
	}

	if config.AI.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic' from env, got '%s'", config.AI.Provider)
	}
}

func TestGetTestOutputPath(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		sourceFile string
		expected   string
	}{
		{
			name:       "default output directory",
			config:     DefaultConfig(),
			sourceFile: "/path/to/user.go",
			expected:   "/path/to/user_test.go",
		},
		{
			name: "custom output directory",
			config: &Config{
				Output: OutputConfig{
					Directory: "tests",
					Suffix:    "_test.go",
				},
			},
			sourceFile: "/path/to/user.go",
			expected:   "tests/user_test.go",
		},
		{
			name: "custom suffix",
			config: &Config{
				Output: OutputConfig{
					Directory: "",
					Suffix:    ".test.go",
				},
			},
			sourceFile: "/path/to/user.go",
			expected:   "/path/to/user.test.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetTestOutputPath(tt.sourceFile)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestShouldIncludeFunction(t *testing.T) {
	config := &Config{
		Filtering: FilterConfig{
			IncludeUnexported: false,
			MinComplexity:     2,
			MaxComplexity:     10,
			SkipPatterns:      []string{"helper*", "internal*", "temp"},
		},
	}

	tests := []struct {
		name       string
		funcName   string
		isExported bool
		complexity int
		expected   bool
	}{
		{
			name:       "exported function within complexity range",
			funcName:   "ValidateUser",
			isExported: true,
			complexity: 5,
			expected:   true,
		},
		{
			name:       "unexported function with include_unexported=false",
			funcName:   "validateUser",
			isExported: false,
			complexity: 5,
			expected:   false,
		},
		{
			name:       "function too complex",
			funcName:   "ComplexFunction",
			isExported: true,
			complexity: 15,
			expected:   false,
		},
		{
			name:       "function not complex enough",
			funcName:   "SimpleFunction",
			isExported: true,
			complexity: 1,
			expected:   false,
		},
		{
			name:       "function matches skip pattern (wildcard)",
			funcName:   "helperFunction",
			isExported: true,
			complexity: 5,
			expected:   false,
		},
		{
			name:       "function matches skip pattern (exact)",
			funcName:   "temp",
			isExported: true,
			complexity: 5,
			expected:   false,
		},
		{
			name:       "function matches skip pattern (contains)",
			funcName:   "internalHelper",
			isExported: true,
			complexity: 5,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ShouldIncludeFunction(tt.funcName, tt.isExported, tt.complexity)
			if result != tt.expected {
				t.Errorf("ShouldIncludeFunction(%s, %t, %d) = %t, expected %t",
					tt.funcName, tt.isExported, tt.complexity, result, tt.expected)
			}
		})
	}
}

func TestShouldTriggerOnFile(t *testing.T) {
	config := &Config{
		Mode: "auto",
		Triggers: TriggerConfig{
			Auto: AutoTrigger{
				FilePatterns: []string{"*.go", "src/*.go"},
				ExcludeFiles: []string{"*_test.go", "vendor/*"},
			},
		},
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "go file should trigger",
			filePath: "user.go",
			expected: true,
		},
		{
			name:     "vendor file should be excluded",
			filePath: "vendor/pkg/file.go",
			expected: false,
		},
		{
			name:     "src go file should trigger",
			filePath: "src/handler.go",
			expected: true,
		},
		{
			name:     "non-go file should not trigger",
			filePath: "README.md",
			expected: false,
		},
		{
			name:     "nested go file should trigger",
			filePath: "internal/service.go",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ShouldTriggerOnFile(tt.filePath)
			if result != tt.expected {
				t.Errorf("ShouldTriggerOnFile(%s) = %t, expected %t", tt.filePath, result, tt.expected)
			}
		})
	}

	// Test manual mode doesn't trigger
	config.Mode = "manual"
	result := config.ShouldTriggerOnFile("user.go")
	if result != false {
		t.Error("Manual mode should not trigger on any file")
	}
}

func TestIsAutoMode(t *testing.T) {
	config := DefaultConfig()

	// Default is manual
	if config.IsAutoMode() {
		t.Error("Default config should be manual mode")
	}

	// Test auto mode
	config.Mode = "auto"
	if !config.IsAutoMode() {
		t.Error("Config with mode='auto' should return true for IsAutoMode()")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to temp directory
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create a custom config
	config := &Config{
		Mode:  "auto",
		Hooks: []string{"post-commit"},
		AI: AIConfig{
			Provider:    "anthropic",
			Model:       "claude-3-sonnet",
			Temperature: 0.3,
			MaxTokens:   1500,
		},
		Output: OutputConfig{
			Directory: "test_output",
			Suffix:    ".test.go",
			Overwrite: true,
		},
		Filtering: FilterConfig{
			IncludeUnexported: true,
			MaxComplexity:     20,
			SkipPatterns:      []string{"helper*"},
		},
	}

	// Save the config
	err = SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load it back
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Compare values
	if loadedConfig.Mode != config.Mode {
		t.Errorf("Expected mode '%s', got '%s'", config.Mode, loadedConfig.Mode)
	}

	if loadedConfig.AI.Provider != config.AI.Provider {
		t.Errorf("Expected provider '%s', got '%s'", config.AI.Provider, loadedConfig.AI.Provider)
	}

	if loadedConfig.AI.Model != config.AI.Model {
		t.Errorf("Expected model '%s', got '%s'", config.AI.Model, loadedConfig.AI.Model)
	}

	if loadedConfig.Output.Directory != config.Output.Directory {
		t.Errorf("Expected output directory '%s', got '%s'", config.Output.Directory, loadedConfig.Output.Directory)
	}

	if loadedConfig.Filtering.IncludeUnexported != config.Filtering.IncludeUnexported {
		t.Errorf("Expected include_unexported %t, got %t", config.Filtering.IncludeUnexported, loadedConfig.Filtering.IncludeUnexported)
	}
}

// Helper function to check if a string contains a substring
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) <= len(s) && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
