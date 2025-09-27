package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Eranmonnie/testgen/internal/config"
	"github.com/Eranmonnie/testgen/pkg/models"
)

// TestGenerator handles AI-powered test generation
type TestGenerator struct {
	config *config.Config
	client *http.Client
}

// NewTestGenerator creates a new test generator
func NewTestGenerator(cfg *config.Config) *TestGenerator {
	return &TestGenerator{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.AI.Timeout) * time.Second,
		},
	}
}

// GenerateTests generates tests for the given functions
func (tg *TestGenerator) GenerateTests(request models.TestGenerationRequest) (*models.TestGenerationResponse, error) {
	switch tg.config.AI.Provider {
	case "openai":
		return tg.generateWithOpenAI(request)
	case "anthropic":
		return tg.generateWithAnthropic(request)
	case "local":
		return tg.generateWithLocal(request)
	case "groq":
		return tg.generateWithGroq(request)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", tg.config.AI.Provider)
	}
}

// WriteTestFiles writes generated tests to files
func (tg *TestGenerator) WriteTestFiles(functions []models.FunctionInfo, tests []models.GeneratedTest) error {
	// Group tests by source file
	testsByFile := make(map[string][]models.GeneratedTest)
	functionsByFile := make(map[string][]models.FunctionInfo)

	for i, fn := range functions {
		if i < len(tests) {
			testsByFile[fn.File] = append(testsByFile[fn.File], tests[i])
			functionsByFile[fn.File] = append(functionsByFile[fn.File], fn)
		}
	}

	// Write test files
	for sourceFile, fileTests := range testsByFile {
		if err := tg.writeTestFile(sourceFile, functionsByFile[sourceFile], fileTests); err != nil {
			return fmt.Errorf("failed to write test file for %s: %w", sourceFile, err)
		}
	}

	return nil
}

// generateWithOpenAI generates tests using OpenAI API
func (tg *TestGenerator) generateWithOpenAI(request models.TestGenerationRequest) (*models.TestGenerationResponse, error) {
	if tg.config.AI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	prompt := tg.buildPrompt(request)

	// OpenAI API request structure
	openAIRequest := map[string]interface{}{
		"model": tg.config.AI.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert Go test writer. Generate comprehensive, idiomatic Go tests based on the provided function information.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": tg.config.AI.Temperature,
		"max_tokens":  tg.config.AI.MaxTokens,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	// Fixed: Pass separate header name and value
	return tg.makeAPIRequest("https://api.openai.com/v1/chat/completions", openAIRequest, "Authorization", "Bearer "+tg.config.AI.APIKey)
}

// generateWithAnthropic generates tests using Anthropic Claude API
func (tg *TestGenerator) generateWithAnthropic(request models.TestGenerationRequest) (*models.TestGenerationResponse, error) {
	if tg.config.AI.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured")
	}

	prompt := tg.buildPrompt(request)

	// Anthropic API request structure
	anthropicRequest := map[string]interface{}{
		"model":       tg.config.AI.Model,
		"max_tokens":  tg.config.AI.MaxTokens,
		"temperature": tg.config.AI.Temperature,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	// Fixed: Pass correct header name and value
	return tg.makeAPIRequest("https://api.anthropic.com/v1/messages", anthropicRequest, "x-api-key", tg.config.AI.APIKey)
}

// generateWithLocal generates tests using local AI (placeholder)
func (tg *TestGenerator) generateWithLocal(request models.TestGenerationRequest) (*models.TestGenerationResponse, error) {
	// This would integrate with local models like Ollama, LM Studio, etc.
	return nil, fmt.Errorf("local AI provider not implemented yet")
}

// Add Groq provider
func (tg *TestGenerator) generateWithGroq(request models.TestGenerationRequest) (*models.TestGenerationResponse, error) {
	if tg.config.AI.APIKey == "" {
		return nil, fmt.Errorf("Groq API key not configured")
	}

	prompt := tg.buildPrompt(request)

	// Groq API request (OpenAI-compatible)
	groqRequest := map[string]interface{}{
		"model": tg.config.AI.Model, // e.g., "llama3-8b-8192"
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an expert Go test writer. Generate comprehensive, idiomatic Go tests.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": tg.config.AI.Temperature,
		"max_tokens":  tg.config.AI.MaxTokens,
	}

	return tg.makeAPIRequest("https://api.groq.com/openai/v1/chat/completions", groqRequest, "Authorization", "Bearer "+tg.config.AI.APIKey)
}

// filepath: [test.go](http://_vscodecontentref_/0)
// buildPrompt creates the AI prompt from the request
func (tg *TestGenerator) buildPrompt(request models.TestGenerationRequest) string {
	var prompt strings.Builder

	prompt.WriteString("Generate comprehensive Go tests for the following functions. ")
	prompt.WriteString("You must return ONLY a valid JSON object with no markdown formatting, no code blocks, and no backticks.\n\n")

	// Add context information
	prompt.WriteString("Project Context:\n")
	prompt.WriteString(fmt.Sprintf("- Package: %s\n", request.Context.PackageName))
	prompt.WriteString(fmt.Sprintf("- Project: %s\n", request.Context.ProjectName))

	if len(request.Context.Imports) > 0 {
		prompt.WriteString(fmt.Sprintf("- Imports: %s\n", strings.Join(request.Context.Imports, ", ")))
	}

	if request.Context.GitContext.CommitMessage != "" {
		prompt.WriteString(fmt.Sprintf("- Recent commit: %s\n", request.Context.GitContext.CommitMessage))
	}

	prompt.WriteString("\nFunctions to test:\n")

	// Add function details
	for i, fn := range request.Functions {
		prompt.WriteString(fmt.Sprintf("\n%d. Function: %s\n", i+1, fn.Name))
		prompt.WriteString(fmt.Sprintf("   Signature: %s\n", fn.Signature))

		if len(fn.Parameters) > 0 {
			prompt.WriteString("   Parameters:\n")
			for _, param := range fn.Parameters {
				prompt.WriteString(fmt.Sprintf("     - %s %s\n", param.Name, param.Type))
			}
		}

		if len(fn.Returns) > 0 {
			prompt.WriteString("   Returns:\n")
			for _, ret := range fn.Returns {
				if ret.Name != "" {
					prompt.WriteString(fmt.Sprintf("     - %s %s\n", ret.Name, ret.Type))
				} else {
					prompt.WriteString(fmt.Sprintf("     - %s\n", ret.Type))
				}
			}
		}

		if fn.IsMethod {
			prompt.WriteString(fmt.Sprintf("   Method receiver: %s %s\n", fn.Receiver.Name, fn.Receiver.Type))
		}

		// Add complexity hints
		complexity := fn.Complexity
		var hints []string
		if complexity.HasErrors {
			hints = append(hints, "handles errors")
		}
		if complexity.HasPointers {
			hints = append(hints, "uses pointers")
		}
		if complexity.HasGoroutines {
			hints = append(hints, "uses goroutines")
		}
		if complexity.HasChannels {
			hints = append(hints, "uses channels")
		}
		if len(hints) > 0 {
			prompt.WriteString(fmt.Sprintf("   Complexity: %s\n", strings.Join(hints, ", ")))
		}

		if len(fn.Comments) > 0 {
			prompt.WriteString("   Comments:\n")
			for _, comment := range fn.Comments {
				prompt.WriteString(fmt.Sprintf("     %s\n", strings.TrimSpace(comment)))
			}
		}
	}

	// Add instructions
	prompt.WriteString("\nGenerate tests that:\n")
	prompt.WriteString("1. Follow Go testing conventions\n")
	prompt.WriteString("2. Test both happy path and edge cases\n")
	prompt.WriteString("3. Include table-driven tests when appropriate\n")
	prompt.WriteString("4. Test error conditions if the function returns errors\n")
	prompt.WriteString("5. Use meaningful test names (TestFunctionName_Scenario)\n")
	prompt.WriteString("6. Include setup and cleanup when needed\n")
	prompt.WriteString("7. Test nil pointer cases if function uses pointers\n")
	prompt.WriteString("8. Are readable and well-commented\n\n")

	// Specify response format more clearly
	prompt.WriteString("IMPORTANT: Return only valid JSON in this exact format (no markdown, no code blocks, no backticks):\n")
	prompt.WriteString(`{"tests":[{"name":"TestFunctionName_Scenario","code":"func TestFunctionName_Scenario(t *testing.T) { /* test code */ }","description":"what this test validates","test_type":"unit","coverage":["scenario1","scenario2"]}],"reasoning":"explanation of testing approach","confidence":0.85,"warnings":["any potential issues"]}`)

	return prompt.String()
}

// makeAPIRequest makes HTTP request to AI API
func (tg *TestGenerator) makeAPIRequest(url string, requestData map[string]interface{}, authHeaderName, authHeaderValue string) (*models.TestGenerationResponse, error) {
	// Marshal request
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Fixed: Properly set auth header
	req.Header.Set(authHeaderName, authHeaderValue)

	// Special headers for Anthropic
	if strings.Contains(url, "anthropic.com") {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	// Make request
	resp, err := tg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response based on provider
	return tg.parseAPIResponse(body, url)
}

// parseAPIResponse parses AI API response into our format
func (tg *TestGenerator) parseAPIResponse(body []byte, url string) (*models.TestGenerationResponse, error) {
	if strings.Contains(url, "openai.com") || strings.Contains(url, "groq.com") {
		return tg.parseOpenAIResponse(body) // Groq uses OpenAI-compatible format
	} else if strings.Contains(url, "anthropic.com") {
		return tg.parseAnthropicResponse(body)
	}

	return nil, fmt.Errorf("unknown API response format")
}

// parseOpenAIResponse parses OpenAI API response
func (tg *TestGenerator) parseOpenAIResponse(body []byte) (*models.TestGenerationResponse, error) {
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	// Clean the content - remove markdown code blocks if present
	content := openAIResp.Choices[0].Message.Content
	content = tg.cleanJSONResponse(content)

	// Parse the JSON content
	var response models.TestGenerationResponse
	if err := json.Unmarshal([]byte(content), &response); err != nil {
		// Log the actual content for debugging
		fmt.Printf("DEBUG: Failed to parse JSON. Content: %s\n", content)
		return nil, fmt.Errorf("failed to parse test generation response: %w", err)
	}

	return &response, nil
}

// parseAnthropicResponse parses Anthropic API response
func (tg *TestGenerator) parseAnthropicResponse(body []byte) (*models.TestGenerationResponse, error) {
	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("no content in Anthropic response")
	}

	// Clean the content - remove markdown code blocks if present
	content := anthropicResp.Content[0].Text
	content = tg.cleanJSONResponse(content)

	// Parse the JSON content
	var response models.TestGenerationResponse
	if err := json.Unmarshal([]byte(content), &response); err != nil {
		// Log the actual content for debugging
		fmt.Printf("DEBUG: Failed to parse JSON. Content: %s\n", content)
		return nil, fmt.Errorf("failed to parse test generation response: %w", err)
	}

	return &response, nil
}

// cleanJSONResponse removes markdown formatting from AI responses
func (tg *TestGenerator) cleanJSONResponse(content string) string {
	// Remove markdown code blocks
	content = strings.TrimSpace(content)

	// Remove ```json and ``` markers
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}

	// Find the first { and last } to extract just the JSON
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		content = content[start : end+1]
	}

	return strings.TrimSpace(content)
}

// writeTestFile writes tests to a file
func (tg *TestGenerator) writeTestFile(sourceFile string, functions []models.FunctionInfo, tests []models.GeneratedTest) error {
	testFilePath := tg.config.GetTestOutputPath(sourceFile)

	// Check if we should overwrite
	if _, err := os.Stat(testFilePath); err == nil && !tg.config.Output.Overwrite {
		return fmt.Errorf("test file %s already exists (use overwrite: true to replace)", testFilePath)
	}

	// Backup existing file if configured
	if tg.config.Output.BackupExisting {
		if err := tg.backupFile(testFilePath); err != nil {
			return fmt.Errorf("failed to backup existing file: %w", err)
		}
	}

	// Build complete test file content
	content, err := tg.buildTestFileContent(sourceFile, functions, tests)
	if err != nil {
		return fmt.Errorf("failed to build test content: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(testFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create test directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(testFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	fmt.Printf("Generated tests: %s\n", testFilePath)
	return nil
}

// buildTestFileContent creates the complete test file content
func (tg *TestGenerator) buildTestFileContent(sourceFile string, functions []models.FunctionInfo, tests []models.GeneratedTest) (string, error) {
	var content strings.Builder

	// Get package name
	packageName := "main"
	if len(functions) > 0 {
		packageName = functions[0].Package
	}

	// Package declaration
	content.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	// Standard test imports
	content.WriteString("import (\n")
	content.WriteString("\t\"testing\"\n")

	// Add additional imports based on test content
	importSet := make(map[string]bool)
	for _, test := range tests {
		if strings.Contains(test.Code, "reflect.") {
			importSet["reflect"] = true
		}
		if strings.Contains(test.Code, "errors.") {
			importSet["errors"] = true
		}
		if strings.Contains(test.Code, "fmt.") {
			importSet["fmt"] = true
		}
		if strings.Contains(test.Code, "strings.") {
			importSet["strings"] = true
		}
		if strings.Contains(test.Code, "time.") {
			importSet["time"] = true
		}
	}

	// Add detected imports
	for imp := range importSet {
		content.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
	}

	content.WriteString(")\n\n")

	// Generated tests comment
	content.WriteString("// Tests generated by testgen\n\n")

	// Add each test
	for _, test := range tests {
		content.WriteString(fmt.Sprintf("// %s\n", test.Description))
		content.WriteString(test.Code)
		content.WriteString("\n\n")
	}

	return content.String(), nil
}

// backupFile creates a backup of an existing file
func (tg *TestGenerator) backupFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	backupPath := filePath + ".backup"

	// Read original file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("Created backup: %s\n", backupPath)
	return nil
}
