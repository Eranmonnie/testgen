package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Eranmonnie/testgen/internal/analyzer"
	"github.com/Eranmonnie/testgen/internal/config"
	"github.com/Eranmonnie/testgen/internal/generator"
	"github.com/Eranmonnie/testgen/pkg/models"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"

	// Global flags
	configFile string
	verbose    bool
	dryRun     bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "testgen",
	Short: "AI-powered Go test generation tool",
	Long: `Testgen automatically generates Go tests using AI.
It can work in auto mode (triggered by git hooks) or manual mode (on-demand).`,
	Version: version,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without doing it")

	// Add subcommands
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(hooksCmd)
	rootCmd.AddCommand(statusCmd)
}

// Generate command - main functionality
var generateCmd = &cobra.Command{
	Use:   "generate [files...]",
	Short: "Generate tests for Go files",
	Long: `Generate tests for specified Go files or analyze git changes.
	
Examples:
  testgen generate                    # Analyze recent git changes
  testgen generate user.go handler.go # Generate for specific files
  testgen generate --range HEAD~3..HEAD # Analyze specific git range
  testgen generate --function ValidateUser # Generate for specific function`,
	RunE: runGenerate,
}

var (
	gitRange     string
	functionName string
	allFiles     bool
)

func init() {
	generateCmd.Flags().StringVar(&gitRange, "range", "", "git range to analyze (e.g., HEAD~1..HEAD)")
	generateCmd.Flags().StringVar(&functionName, "function", "", "specific function to generate tests for")
	generateCmd.Flags().BoolVar(&allFiles, "all", false, "generate tests for all functions in specified files")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if verbose {
		fmt.Printf("Using config: %s mode, %s provider\n", cfg.Mode, cfg.AI.Provider)
	}

	// Determine what to analyze
	var result *analyzer.AnalysisResult

	if len(args) > 0 {
		// Specific files provided
		var functions []string
		if functionName != "" {
			functions = []string{functionName}
		}

		result, err = analyzer.AnalyzeSpecificFunctions(args, functions)
		if err != nil {
			return fmt.Errorf("failed to analyze files: %w", err)
		}

		if verbose {
			fmt.Printf("Analyzing %d specific files\n", len(args))
		}
	} else {
		// Analyze git changes
		fromRef, toRef := parseGitRange(gitRange, cfg)

		result, err = analyzer.AnalyzeChanges(fromRef, toRef)
		if err != nil {
			return fmt.Errorf("failed to analyze git changes: %w", err)
		}

		if verbose {
			fmt.Printf("Analyzing git range: %s..%s\n", fromRef, toRef)
		}
	}

	// Show analysis summary
	if verbose || dryRun {
		analyzer.PrintAnalysisSummary(result)
	}

	if len(result.GenerationTargets) == 0 {
		fmt.Println("No functions found that need test generation.")
		return nil
	}

	if dryRun {
		fmt.Printf("Would generate tests for %d functions\n", len(result.GenerationTargets))
		return nil
	}

	// Generate actual tests using AI
	fmt.Printf("Generating tests for %d functions...\n", len(result.GenerationTargets))

	// Create test generator
	generator := generator.NewTestGenerator(cfg)

	// Build request context
	context := analyzer.GetProjectContext(result)

	// Create generation request
	request := models.TestGenerationRequest{
		Functions: result.GenerationTargets,
		Context:   context,
	}

	// Generate tests
	response, err := generator.GenerateTests(request)
	if err != nil {
		return fmt.Errorf("failed to generate tests: %w", err)
	}

	if verbose {
		fmt.Printf("AI Response: %s (confidence: %.2f)\n", response.Reasoning, response.Confidence)
		if len(response.Warnings) > 0 {
			fmt.Printf("Warnings: %v\n", response.Warnings)
		}
	}

	// Write test files
	if err := generator.WriteTestFiles(result.GenerationTargets, response.Tests); err != nil {
		return fmt.Errorf("failed to write test files: %w", err)
	}

	fmt.Printf("Successfully generated %d test functions\n", len(response.Tests))

	return nil
}

// Init command - setup configuration and hooks
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize testgen in current project",
	Long: `Create default configuration file and optionally install git hooks.
	
This will create a .testgen.yml file with sensible defaults.`,
	RunE: runInit,
}

var (
	installHooks bool
	autoMode     bool
)

func init() {
	initCmd.Flags().BoolVar(&installHooks, "hooks", false, "install git hooks for auto mode")
	initCmd.Flags().BoolVar(&autoMode, "auto", false, "set up for auto mode")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if config already exists
	if _, err := os.Stat(config.DefaultConfigFile); err == nil {
		fmt.Printf("Configuration file %s already exists.\n", config.DefaultConfigFile)
		return nil
	}

	// Create default config
	cfg := config.DefaultConfig()

	// Modify based on flags
	if autoMode {
		cfg.Mode = "auto"
		cfg.Hooks = []string{"post-commit"}
	}

	// Save configuration
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Created configuration file: %s\n", config.DefaultConfigFile)

	// Install hooks if requested
	if installHooks {
		if err := installGitHooks(cfg); err != nil {
			return fmt.Errorf("failed to install git hooks: %w", err)
		}
		fmt.Println("Git hooks installed successfully")
	}

	// Show next steps
	fmt.Println("\nNext steps:")
	fmt.Printf("1. Edit %s to customize settings\n", config.DefaultConfigFile)
	fmt.Println("2. Set TESTGEN_API_KEY environment variable")
	if !installHooks && autoMode {
		fmt.Println("3. Run 'testgen hooks install' to enable auto mode")
	}
	fmt.Println("4. Run 'testgen generate' to start generating tests")

	return nil
}

// Config command - manage configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and manage testgen configuration settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		config.PrintConfig(cfg)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Println("Configuration is valid ✓")
		if cfg.AI.APIKey == "" {
			fmt.Println("Warning: No API key configured")
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
}

// Hooks command - manage git hooks
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks",
	Long:  `Install, uninstall, or check git hooks for auto mode.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install git hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		return installGitHooks(cfg)
	},
}

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall git hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return uninstallGitHooks()
	},
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show git hooks status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showHooksStatus()
	},
}

func init() {
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)
	hooksCmd.AddCommand(hooksStatusCmd)
}

// Status command - show overall status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show testgen status",
	Long:  `Show configuration, git hooks status, and recent activity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Testgen Status\n")
		fmt.Printf("==============\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Mode: %s\n", cfg.Mode)
		fmt.Printf("AI Provider: %s (%s)\n", cfg.AI.Provider, cfg.AI.Model)

		if cfg.AI.APIKey != "" {
			fmt.Printf("API Key: configured ✓\n")
		} else {
			fmt.Printf("API Key: not configured ✗\n")
		}

		fmt.Printf("\nGit Hooks:\n")
		if err := showHooksStatus(); err != nil {
			fmt.Printf("  Error checking hooks: %v\n", err)
		}

		// Show recent changes
		fmt.Printf("\nRecent Changes:\n")
		result, err := analyzer.AnalyzeChanges("HEAD~1", "HEAD")
		if err != nil {
			fmt.Printf("  Error analyzing recent changes: %v\n", err)
		} else {
			if len(result.GenerationTargets) > 0 {
				fmt.Printf("  %d functions ready for test generation\n", len(result.GenerationTargets))
			} else {
				fmt.Printf("  No functions need test generation\n")
			}
		}

		return nil
	},
}

// Helper functions

func loadConfig() (*config.Config, error) {
	if configFile != "" {
		return config.LoadConfigFromFile(configFile)
	}
	return config.LoadConfig()
}

func parseGitRange(rangeFlag string, cfg *config.Config) (string, string) {
	if rangeFlag != "" {
		parts := strings.Split(rangeFlag, "..")
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}

	// Use default from config
	defaultRange := cfg.Triggers.Manual.DefaultRange
	parts := strings.Split(defaultRange, "..")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// Fallback
	return "HEAD~1", "HEAD"
}

func installGitHooks(cfg *config.Config) error {
	// Check if .git directory exists
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository")
	}

	// Create hooks directory if it doesn't exist
	hooksDir := ".git/hooks"
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Install each configured hook
	for _, hookName := range cfg.Hooks {
		hookPath := fmt.Sprintf("%s/%s", hooksDir, hookName)

		// Create hook script
		hookContent := fmt.Sprintf(`#!/bin/sh
# testgen %s hook
exec testgen generate
`, hookName)

		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			return fmt.Errorf("failed to install %s hook: %w", hookName, err)
		}

		fmt.Printf("Installed %s hook\n", hookName)
	}

	return nil
}

func uninstallGitHooks() error {
	hooksDir := ".git/hooks"
	hookNames := []string{"post-commit", "pre-push", "pre-commit"}

	for _, hookName := range hookNames {
		hookPath := fmt.Sprintf("%s/%s", hooksDir, hookName)

		// Check if it's our hook
		if content, err := os.ReadFile(hookPath); err == nil {
			if strings.Contains(string(content), "testgen") {
				if err := os.Remove(hookPath); err != nil {
					fmt.Printf("Warning: failed to remove %s hook: %v\n", hookName, err)
				} else {
					fmt.Printf("Removed %s hook\n", hookName)
				}
			}
		}
	}

	return nil
}

func showHooksStatus() error {
	hooksDir := ".git/hooks"
	hookNames := []string{"post-commit", "pre-push", "pre-commit"}

	for _, hookName := range hookNames {
		hookPath := fmt.Sprintf("%s/%s", hooksDir, hookName)

		if _, err := os.Stat(hookPath); err == nil {
			// Check if it's our hook
			if content, err := os.ReadFile(hookPath); err == nil {
				if strings.Contains(string(content), "testgen") {
					fmt.Printf("  %s: installed ✓\n", hookName)
				} else {
					fmt.Printf("  %s: other hook installed\n", hookName)
				}
			}
		} else {
			fmt.Printf("  %s: not installed\n", hookName)
		}
	}

	return nil
}
