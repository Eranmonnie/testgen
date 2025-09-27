package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Eranmonnie/testgen/internal/parser"
	"github.com/Eranmonnie/testgen/pkg/models"
)

func TestShouldGenerateTest(t *testing.T) {
	tests := []struct {
		name     string
		function models.FunctionInfo
		expected bool
	}{
		{
			name: "exported function with parameters and returns",
			function: models.FunctionInfo{
				Name: "ValidateUser",
				Parameters: []models.ParameterInfo{
					{Name: "user", Type: "*User"},
				},
				Returns: []models.ReturnInfo{
					{Type: "error"},
				},
				Complexity: models.ComplexityInfo{
					CyclomaticComplexity: 3,
				},
			},
			expected: true,
		},
		{
			name: "main function should be skipped",
			function: models.FunctionInfo{
				Name:       "main",
				Parameters: []models.ParameterInfo{},
				Returns:    []models.ReturnInfo{},
			},
			expected: false,
		},
		{
			name: "init function should be skipped",
			function: models.FunctionInfo{
				Name:       "init",
				Parameters: []models.ParameterInfo{},
				Returns:    []models.ReturnInfo{},
			},
			expected: false,
		},
		{
			name: "test function should be skipped",
			function: models.FunctionInfo{
				Name: "TestValidateUser",
				Parameters: []models.ParameterInfo{
					{Name: "t", Type: "*testing.T"},
				},
				Returns: []models.ReturnInfo{},
			},
			expected: false,
		},
		{
			name: "unexported function should be skipped",
			function: models.FunctionInfo{
				Name: "validateUser",
				Parameters: []models.ParameterInfo{
					{Name: "user", Type: "*User"},
				},
				Returns: []models.ReturnInfo{
					{Type: "error"},
				},
			},
			expected: false,
		},
		{
			name: "function with no params and no returns should be skipped",
			function: models.FunctionInfo{
				Name:       "DoNothing",
				Parameters: []models.ParameterInfo{},
				Returns:    []models.ReturnInfo{},
				Complexity: models.ComplexityInfo{
					CyclomaticComplexity: 1,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldGenerateTest(tt.function)
			if result != tt.expected {
				t.Errorf("shouldGenerateTest() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsTestFunction(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		expected bool
	}{
		{"regular function", "ValidateUser", false},
		{"test function", "TestValidateUser", true},
		{"benchmark function", "BenchmarkProcess", true},
		{"example function", "ExampleValidate", true},
		{"fuzz function", "FuzzValidate", true},
		{"short name", "Test", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTestFunction(tt.funcName)
			if result != tt.expected {
				t.Errorf("isTestFunction(%q) = %v, expected %v", tt.funcName, result, tt.expected)
			}
		})
	}
}

func TestIsExported(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		expected bool
	}{
		{"exported function", "ValidateUser", true},
		{"unexported function", "validateUser", false},
		{"single uppercase letter", "A", true},
		{"single lowercase letter", "a", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExported(tt.funcName)
			if result != tt.expected {
				t.Errorf("isExported(%q) = %v, expected %v", tt.funcName, result, tt.expected)
			}
		})
	}
}

func TestBuildGenerationTargets(t *testing.T) {
	changedFiles := []ChangedFileAnalysis{
		{
			FilePath: "user.go",
			FunctionDetails: []models.FunctionInfo{
				{
					Name: "ValidateUser",
					Parameters: []models.ParameterInfo{
						{Name: "user", Type: "*User"},
					},
					Returns: []models.ReturnInfo{
						{Type: "error"},
					},
					Complexity: models.ComplexityInfo{
						CyclomaticComplexity: 3,
					},
				},
				{
					Name:       "main",
					Parameters: []models.ParameterInfo{},
					Returns:    []models.ReturnInfo{},
				},
			},
		},
	}

	targets := buildGenerationTargets(changedFiles)

	if len(targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(targets))
	}

	if targets[0].Name != "ValidateUser" {
		t.Errorf("Expected target 'ValidateUser', got %q", targets[0].Name)
	}
}

func TestConvertToModelFunction(t *testing.T) {
	parserFunc := parser.FunctionInfo{
		Name:    "ValidateUser",
		Package: "user",
		File:    "user.go",
		Parameters: []parser.ParameterInfo{
			{Name: "user", Type: "*User"},
		},
		Returns: []parser.ReturnInfo{
			{Type: "error"},
		},
		IsMethod: false,
		Complexity: parser.ComplexityInfo{
			HasErrors:            true,
			HasPointers:          true,
			CyclomaticComplexity: 3,
		},
	}

	fileAnalysis := &parser.FileAnalysis{
		PackageName: "user",
	}

	modelFunc := convertToModelFunction(parserFunc, fileAnalysis)

	if modelFunc.Name != "ValidateUser" {
		t.Errorf("Expected name 'ValidateUser', got %q", modelFunc.Name)
	}

	if modelFunc.Package != "user" {
		t.Errorf("Expected package 'user', got %q", modelFunc.Package)
	}

	if len(modelFunc.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(modelFunc.Parameters))
	}

	if !modelFunc.Complexity.HasErrors {
		t.Error("Expected HasErrors to be true")
	}
}

func TestGetProjectName(t *testing.T) {
	originalDir, _ := os.Getwd()
	tmpDir := t.TempDir()

	goModContent := `module github.com/user/testproject

go 1.22.2
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	projectName := getProjectName()
	expected := "testproject"
	if projectName != expected {
		t.Errorf("Expected project name %q, got %q", expected, projectName)
	}
}

func TestAnalyzeSpecificFunctions(t *testing.T) {
	testCode := `package main

import "fmt"

func ValidateUser(user string) error {
    if user == "" {
        return fmt.Errorf("user cannot be empty")
    }
    return nil
}

func processData(data []byte) error {
    return nil
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result, err := AnalyzeSpecificFunctions([]string{testFile}, []string{"ValidateUser"})
	if err != nil {
		t.Fatalf("AnalyzeSpecificFunctions failed: %v", err)
	}

	if len(result.ChangedFiles) != 1 {
		t.Errorf("Expected 1 changed file, got %d", len(result.ChangedFiles))
	}

	file := result.ChangedFiles[0]
	if file.FilePath != testFile {
		t.Errorf("Expected file path %q, got %q", testFile, file.FilePath)
	}

	found := false
	for _, funcName := range file.ModifiedFunctions {
		if funcName == "ValidateUser" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ValidateUser not found in modified functions")
	}
}
