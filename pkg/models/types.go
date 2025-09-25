package models

// Config represents the testgen configuration
type Config struct {
	Mode     string        `yaml:"mode"`  // "auto" or "manual"
	Hooks    []string      `yaml:"hooks"` // git hooks to install
	Triggers TriggerConfig `yaml:"triggers"`
	AI       AIConfig      `yaml:"ai"`
	Output   OutputConfig  `yaml:"output"`
}

// TriggerConfig defines when test generation should trigger
type TriggerConfig struct {
	Auto   []string `yaml:"auto"` // file patterns for auto mode
	Manual struct {
		DefaultRange string `yaml:"default_range"`
	} `yaml:"manual"`
}

// AIConfig defines AI model settings
type AIConfig struct {
	Model       string  `yaml:"model"`       // "gpt-4", "gpt-3.5-turbo", etc.
	Temperature float64 `yaml:"temperature"` // creativity level
	MaxTokens   int     `yaml:"max_tokens"`  // response length limit
	APIKey      string  `yaml:"api_key"`     // or use env var
}

// OutputConfig defines where and how tests are generated
type OutputConfig struct {
	Directory string `yaml:"directory"` // where to put test files
	Suffix    string `yaml:"suffix"`    // test file suffix, default "_test.go"
	Overwrite bool   `yaml:"overwrite"` // overwrite existing tests
}

// FunctionInfo represents a Go function to generate tests for
type FunctionInfo struct {
	Name       string          `json:"name"`
	Package    string          `json:"package"`
	File       string          `json:"file"`
	Signature  string          `json:"signature"`
	Parameters []ParameterInfo `json:"parameters"`
	Returns    []ReturnInfo    `json:"returns"`
	IsMethod   bool            `json:"is_method"`
	Receiver   *ReceiverInfo   `json:"receiver,omitempty"`
	Comments   []string        `json:"comments"`
	Complexity ComplexityInfo  `json:"complexity"`
}

// ParameterInfo represents a function parameter
type ParameterInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ReturnInfo represents a return value
type ReturnInfo struct {
	Name string `json:"name,omitempty"` // named returns
	Type string `json:"type"`
}

// ReceiverInfo represents method receiver
type ReceiverInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ComplexityInfo provides hints for test generation
type ComplexityInfo struct {
	HasErrors            bool     `json:"has_errors"`            // returns error
	HasPointers          bool     `json:"has_pointers"`          // uses pointers
	HasInterfaces        bool     `json:"has_interfaces"`        // uses interfaces
	HasChannels          bool     `json:"has_channels"`          // uses channels
	HasGoroutines        bool     `json:"has_goroutines"`        // spawns goroutines
	Dependencies         []string `json:"dependencies"`          // external dependencies
	CyclomaticComplexity int      `json:"cyclomatic_complexity"` // rough estimate
}

// TestGenerationRequest represents a request to generate tests
type TestGenerationRequest struct {
	Functions []FunctionInfo `json:"functions"`
	Context   RequestContext `json:"context"`
}

// RequestContext provides additional context for test generation
type RequestContext struct {
	ProjectName   string            `json:"project_name"`
	PackageName   string            `json:"package_name"`
	ExistingTests []string          `json:"existing_tests"` // existing test function names
	Imports       []string          `json:"imports"`        // package imports
	Constants     map[string]string `json:"constants"`      // relevant constants
	GitContext    GitContext        `json:"git_context"`
}

// GitContext provides git-related context
type GitContext struct {
	CommitMessage string   `json:"commit_message"`
	ChangedLines  []int    `json:"changed_lines"`
	Author        string   `json:"author"`
	Branch        string   `json:"branch"`
	FilesDiff     []string `json:"files_diff"`
}

// TestGenerationResponse represents the AI's test generation response
type TestGenerationResponse struct {
	Tests      []GeneratedTest `json:"tests"`
	Reasoning  string          `json:"reasoning"`  // why these tests were chosen
	Confidence float64         `json:"confidence"` // AI's confidence level
	Warnings   []string        `json:"warnings"`   // potential issues
}

// GeneratedTest represents a single generated test
type GeneratedTest struct {
	Name        string   `json:"name"`        // test function name
	Code        string   `json:"code"`        // complete test code
	Description string   `json:"description"` // what the test does
	TestType    TestType `json:"test_type"`   // unit, integration, etc.
	Coverage    []string `json:"coverage"`    // what scenarios it covers
}

// TestType represents different types of tests
type TestType string

const (
	UnitTest        TestType = "unit"
	IntegrationTest TestType = "integration"
	BenchmarkTest   TestType = "benchmark"
	ExampleTest     TestType = "example"
	FuzzTest        TestType = "fuzz"
)

// GenerationStats tracks test generation statistics
type GenerationStats struct {
	FilesProcessed  int            `json:"files_processed"`
	FunctionsFound  int            `json:"functions_found"`
	TestsGenerated  int            `json:"tests_generated"`
	SuccessRate     float64        `json:"success_rate"`
	ProcessingTime  int64          `json:"processing_time_ms"`
	AITokensUsed    int            `json:"ai_tokens_used"`
	ErrorsByType    map[string]int `json:"errors_by_type"`
	FunctionsByType map[string]int `json:"functions_by_type"`
}
