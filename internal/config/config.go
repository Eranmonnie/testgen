package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the complete testgen configuration
type Config struct {
	Mode      string        `yaml:"mode"`      // "auto" or "manual"
	Hooks     []string      `yaml:"hooks"`     // git hooks to install
	Triggers  TriggerConfig `yaml:"triggers"`  // when to trigger generation
	AI        AIConfig      `yaml:"ai"`        // AI model settings
	Output    OutputConfig  `yaml:"output"`    // output settings
	Filtering FilterConfig  `yaml:"filtering"` // function filtering rules
}

// TriggerConfig defines when test generation should trigger
type TriggerConfig struct {
	Auto   AutoTrigger   `yaml:"auto"`   // auto mode settings
	Manual ManualTrigger `yaml:"manual"` // manual mode settings
}

type AutoTrigger struct {
	FilePatterns []string `yaml:"file_patterns"` // patterns that trigger auto generation
	ExcludeFiles []string `yaml:"exclude_files"` // files to exclude
	OnCommit     bool     `yaml:"on_commit"`     // trigger on commit
	OnPush       bool     `yaml:"on_push"`       // trigger on push
}

type ManualTrigger struct {
	DefaultRange string `yaml:"default_range"` // default git range for manual mode
}

// AIConfig defines AI model settings
type AIConfig struct {
	Provider    string  `yaml:"provider"`    // "openai", "anthropic", "local"
	Model       string  `yaml:"model"`       // specific model name
	APIKey      string  `yaml:"api_key"`     // API key (or use env var)
	BaseURL     string  `yaml:"base_url"`    // for custom endpoints
	Temperature float64 `yaml:"temperature"` // creativity level 0-1
	MaxTokens   int     `yaml:"max_tokens"`  // max response length
	Timeout     int     `yaml:"timeout"`     // timeout in seconds
}

// OutputConfig defines where and how tests are generated
type OutputConfig struct {
	Directory      string `yaml:"directory"`       // test output directory
	Suffix         string `yaml:"suffix"`          // test file suffix
	Overwrite      bool   `yaml:"overwrite"`       // overwrite existing tests
	BackupExisting bool   `yaml:"backup_existing"` // backup before overwriting
	TestTemplate   string `yaml:"test_template"`   // custom test template
}

// FilterConfig defines function filtering rules
type FilterConfig struct {
	IncludeUnexported bool     `yaml:"include_unexported"` // include private functions
	MaxComplexity     int      `yaml:"max_complexity"`     // max cyclomatic complexity
	MinComplexity     int      `yaml:"min_complexity"`     // min complexity to test
	SkipPatterns      []string `yaml:"skip_patterns"`      // function name patterns to skip
	RequireParams     bool     `yaml:"require_params"`     // require functions to have parameters
	RequireReturns    bool     `yaml:"require_returns"`    // require functions to have returns
}

const (
	DefaultConfigFile = ".testgen.yml"
	GlobalConfigFile  = "testgen.yml"
	ConfigEnvVar      = "TESTGEN_CONFIG"
)

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Mode:  "manual",
		Hooks: []string{},
		Triggers: TriggerConfig{
			Auto: AutoTrigger{
				FilePatterns: []string{"*.go"},
				ExcludeFiles: []string{"*_test.go", "vendor/*", ".git/*"},
				OnCommit:     true,
				OnPush:       false,
			},
			Manual: ManualTrigger{
				DefaultRange: "HEAD~1..HEAD",
			},
		},
		AI: AIConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.2,
			MaxTokens:   2000,
			Timeout:     30,
		},
		Output: OutputConfig{
			Directory:      "", // same directory as source
			Suffix:         "_test.go",
			Overwrite:      false,
			BackupExisting: true,
			TestTemplate:   "default",
		},
		Filtering: FilterConfig{
			IncludeUnexported: false,
			MaxComplexity:     15,
			MinComplexity:     1,
			SkipPatterns:      []string{"main", "init"},
			RequireParams:     false,
			RequireReturns:    false,
		},
	}
}

// LoadConfig loads configuration from file, with fallback to defaults
func LoadConfig() (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Try to find and load config file
	configPath, err := findConfigFile()
	if err != nil {
		// No config file found, use defaults
		return config, nil
	}

	// Load and merge with defaults
	if err := loadConfigFromFile(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	// Override with environment variables
	overrideWithEnv(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadConfigFromFile loads configuration from a specific file
func LoadConfigFromFile(filePath string) (*Config, error) {
	config := DefaultConfig()

	if err := loadConfigFromFile(filePath, config); err != nil {
		return nil, err
	}

	overrideWithEnv(config)

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to the default location
func SaveConfig(config *Config) error {
	configPath := DefaultConfigFile

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration saved to %s\n", configPath)
	return nil
}

// findConfigFile looks for config file in various locations
func findConfigFile() (string, error) {
	// 1. Check environment variable
	if configPath := os.Getenv(ConfigEnvVar); configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	// 2. Check current directory
	if _, err := os.Stat(DefaultConfigFile); err == nil {
		return DefaultConfigFile, nil
	}

	// 3. Check project root (look for go.mod)
	if projectRoot := findProjectRoot(); projectRoot != "" {
		configPath := filepath.Join(projectRoot, DefaultConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	// 4. Check home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPath := filepath.Join(homeDir, GlobalConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	return "", fmt.Errorf("no config file found")
}

// findProjectRoot looks for project root by finding go.mod
func findProjectRoot() string {
	dir, _ := os.Getwd()

	for dir != "/" && dir != "." {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}

	return ""
}

// loadConfigFromFile loads config from file and merges with existing config
func loadConfigFromFile(filePath string, config *Config) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// overrideWithEnv overrides config with environment variables
func overrideWithEnv(config *Config) {
	if mode := os.Getenv("TESTGEN_MODE"); mode != "" {
		config.Mode = mode
	}

	if apiKey := os.Getenv("TESTGEN_API_KEY"); apiKey != "" {
		config.AI.APIKey = apiKey
	}

	if model := os.Getenv("TESTGEN_MODEL"); model != "" {
		config.AI.Model = model
	}

	if provider := os.Getenv("TESTGEN_PROVIDER"); provider != "" {
		config.AI.Provider = provider
	}

	if baseURL := os.Getenv("TESTGEN_BASE_URL"); baseURL != "" {
		config.AI.BaseURL = baseURL
	}
}

// validateConfig validates the configuration for common errors
func validateConfig(config *Config) error {
	// Validate mode
	if config.Mode != "auto" && config.Mode != "manual" {
		return fmt.Errorf("mode must be 'auto' or 'manual', got '%s'", config.Mode)
	}

	// Validate AI provider
	validProviders := []string{"openai", "anthropic", "groq", "local"}
	if !contains(validProviders, config.AI.Provider) {
		return fmt.Errorf("unsupported AI provider '%s', must be one of: %s",
			config.AI.Provider, strings.Join(validProviders, ", "))
	}

	// Validate temperature
	if config.AI.Temperature < 0 || config.AI.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0 and 1, got %f", config.AI.Temperature)
	}

	// Validate max tokens
	if config.AI.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive, got %d", config.AI.MaxTokens)
	}

	// Validate complexity bounds
	if config.Filtering.MinComplexity > config.Filtering.MaxComplexity {
		return fmt.Errorf("min_complexity (%d) cannot be greater than max_complexity (%d)",
			config.Filtering.MinComplexity, config.Filtering.MaxComplexity)
	}

	// Warn if API key is missing for remote providers
	if (config.AI.Provider == "openai" || config.AI.Provider == "anthropic") && config.AI.APIKey == "" {
		fmt.Printf("Warning: No API key configured for provider '%s'. Set TESTGEN_API_KEY environment variable.\n",
			config.AI.Provider)
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetTestOutputPath returns the full path where test file should be created
func (c *Config) GetTestOutputPath(sourceFile string) string {
	dir := filepath.Dir(sourceFile)
	if c.Output.Directory != "" {
		dir = c.Output.Directory
	}

	baseName := strings.TrimSuffix(filepath.Base(sourceFile), ".go")
	testFileName := baseName + c.Output.Suffix

	return filepath.Join(dir, testFileName)
}

// ShouldIncludeFunction determines if a function should be included based on filtering rules
func (c *Config) ShouldIncludeFunction(funcName string, isExported bool, complexity int) bool {
	// Check export status
	if !isExported && !c.Filtering.IncludeUnexported {
		return false
	}

	// Check complexity bounds
	if complexity < c.Filtering.MinComplexity || complexity > c.Filtering.MaxComplexity {
		return false
	}

	// Check skip patterns
	for _, pattern := range c.Filtering.SkipPatterns {
		if matched, _ := filepath.Match(pattern, funcName); matched {
			return false
		}
		// Simple string contains check as fallback
		if strings.Contains(strings.ToLower(funcName), strings.ToLower(pattern)) {
			return false
		}
	}

	return true
}

// IsAutoMode returns true if running in auto mode
func (c *Config) IsAutoMode() bool {
	return c.Mode == "auto"
}

// ShouldTriggerOnFile checks if a file should trigger auto generation
func (c *Config) ShouldTriggerOnFile(filePath string) bool {
	// Only trigger in auto mode
	if !c.IsAutoMode() {
		return false
	}

	// Normalize path separators
	filePath = filepath.ToSlash(filePath)

	// Check exclude patterns first
	for _, pattern := range c.Triggers.Auto.ExcludeFiles {
		pattern = filepath.ToSlash(pattern)

		// Check if the pattern matches the full path
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return false
		}

		// Check if the pattern matches just the filename
		if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
			return false
		}

		// Handle wildcard patterns like "vendor/*"
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(filePath, prefix+"/") {
				return false
			}
		}

		// Handle exact directory matches like "vendor"
		if strings.HasPrefix(filePath, pattern+"/") {
			return false
		}
	}

	// Check include patterns
	for _, pattern := range c.Triggers.Auto.FilePatterns {
		pattern = filepath.ToSlash(pattern)

		// Check base filename
		if matched, _ := filepath.Match(pattern, filepath.Base(filePath)); matched {
			return true
		}

		// Check full path
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}

		// Handle glob patterns with **
		if strings.Contains(pattern, "**") {
			if strings.HasSuffix(pattern, "*.go") && strings.HasSuffix(filePath, ".go") {
				prefix := strings.TrimSuffix(pattern, "**/*.go")
				if prefix == "" || strings.HasPrefix(filePath, prefix) {
					return true
				}
			}
		}
	}

	return false
}

// PrintConfig prints the current configuration in a readable format
func PrintConfig(config *Config) {
	fmt.Printf("Testgen Configuration:\n")
	fmt.Printf("======================\n")
	fmt.Printf("Mode: %s\n", config.Mode)
	fmt.Printf("Git Hooks: %v\n", config.Hooks)
	fmt.Printf("\n")

	fmt.Printf("AI Settings:\n")
	fmt.Printf("  Provider: %s\n", config.AI.Provider)
	fmt.Printf("  Model: %s\n", config.AI.Model)
	fmt.Printf("  Temperature: %.2f\n", config.AI.Temperature)
	fmt.Printf("  Max Tokens: %d\n", config.AI.MaxTokens)
	if config.AI.APIKey != "" {
		fmt.Printf("  API Key: %s***\n", config.AI.APIKey[:min(8, len(config.AI.APIKey))])
	}
	fmt.Printf("\n")

	fmt.Printf("Output Settings:\n")
	fmt.Printf("  Directory: %s\n", orDefault(config.Output.Directory, "same as source"))
	fmt.Printf("  Suffix: %s\n", config.Output.Suffix)
	fmt.Printf("  Overwrite: %t\n", config.Output.Overwrite)
	fmt.Printf("  Backup: %t\n", config.Output.BackupExisting)
	fmt.Printf("\n")

	fmt.Printf("Filtering Rules:\n")
	fmt.Printf("  Include Unexported: %t\n", config.Filtering.IncludeUnexported)
	fmt.Printf("  Complexity Range: %d-%d\n", config.Filtering.MinComplexity, config.Filtering.MaxComplexity)
	fmt.Printf("  Skip Patterns: %v\n", config.Filtering.SkipPatterns)
	fmt.Printf("\n")
}

func orDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
