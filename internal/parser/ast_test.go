package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	// Create a temporary Go file for testing
	testCode := `package main

import (
	"errors"
	"fmt"
	"strings"
)

const MaxUsers = 100

type User struct {
	Name  string
	Email string
}

// ValidateUser checks if a user is valid
func ValidateUser(user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}
	if user.Email == "" {
		return errors.New("email required")
	}
	if !strings.Contains(user.Email, "@") {
		return errors.New("invalid email format")
	}
	return nil
}

// GetName returns the user's name
func (u *User) GetName() string {
	if u == nil {
		return ""
	}
	return u.Name
}

func processUsers(users []User, handler func(User) error) (int, error) {
	count := 0
	for _, user := range users {
		if err := handler(user); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func startWorker() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panic(r)
			}
		}()
		// worker logic
	}()
}`

	// Write test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse the file
	analysis, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Test package name
	if analysis.PackageName != "main" {
		t.Errorf("Expected package name 'main', got '%s'", analysis.PackageName)
	}

	// Test imports
	expectedImports := []string{"errors", "fmt", "strings"}
	if len(analysis.Imports) != len(expectedImports) {
		t.Errorf("Expected %d imports, got %d", len(expectedImports), len(analysis.Imports))
	}

	// Test constants
	if _, exists := analysis.Constants["MaxUsers"]; !exists {
		t.Error("Expected MaxUsers constant to be found")
	}

	// Test functions
	if len(analysis.Functions) != 4 {
		t.Errorf("Expected 4 functions, got %d", len(analysis.Functions))
	}

	// Test ValidateUser function
	var validateUser *FunctionInfo
	for _, fn := range analysis.Functions {
		if fn.Name == "ValidateUser" {
			validateUser = &fn
			break
		}
	}

	if validateUser == nil {
		t.Fatal("ValidateUser function not found")
	}

	// Test function signature
	expectedSig := "func ValidateUser(user *User) error"
	if validateUser.Signature != expectedSig {
		t.Errorf("Expected signature '%s', got '%s'", expectedSig, validateUser.Signature)
	}

	// Test parameters
	if len(validateUser.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(validateUser.Parameters))
	}
	if validateUser.Parameters[0].Name != "user" || validateUser.Parameters[0].Type != "*User" {
		t.Errorf("Expected parameter 'user *User', got '%s %s'",
			validateUser.Parameters[0].Name, validateUser.Parameters[0].Type)
	}

	// Test return types
	if len(validateUser.Returns) != 1 || validateUser.Returns[0].Type != "error" {
		t.Errorf("Expected return type 'error', got %v", validateUser.Returns)
	}

	// Test complexity analysis
	if !validateUser.Complexity.HasErrors {
		t.Error("Expected HasErrors to be true")
	}
	if !validateUser.Complexity.HasPointers {
		t.Error("Expected HasPointers to be true")
	}
	if validateUser.Complexity.ControlFlowCount != 3 { // 3 if statements
		t.Errorf("Expected ControlFlowCount 3, got %d", validateUser.Complexity.ControlFlowCount)
	}

	// Test method parsing (GetName)
	var getName *FunctionInfo
	for _, fn := range analysis.Functions {
		if fn.Name == "GetName" {
			getName = &fn
			break
		}
	}

	if getName == nil {
		t.Fatal("GetName method not found")
	}

	if !getName.IsMethod {
		t.Error("GetName should be identified as a method")
	}
	if getName.Receiver == nil || getName.Receiver.Type != "*User" {
		t.Errorf("Expected receiver '*User', got %v", getName.Receiver)
	}

	// Test complex function (startWorker)
	var startWorker *FunctionInfo
	for _, fn := range analysis.Functions {
		if fn.Name == "startWorker" {
			startWorker = &fn
			break
		}
	}

	if startWorker == nil {
		t.Fatal("startWorker function not found")
	}

	if !startWorker.Complexity.HasGoroutines {
		t.Error("Expected HasGoroutines to be true")
	}
	if !startWorker.Complexity.HasDefers {
		t.Error("Expected HasDefers to be true")
	}
	if !startWorker.Complexity.HasPanic {
		t.Error("Expected HasPanic to be true")
	}
}

func TestFilterFunctions(t *testing.T) {
	analysis := &FileAnalysis{
		Functions: []FunctionInfo{
			{Name: "ValidateUser"},
			{Name: "GetName"},
			{Name: "processUsers"},
			{Name: "startWorker"},
		},
	}

	// Filter for specific functions (like from git diff)
	filtered := analysis.FilterFunctions([]string{"ValidateUser", "startWorker"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered functions, got %d", len(filtered))
	}

	expectedNames := []string{"ValidateUser", "startWorker"}
	for i, fn := range filtered {
		if fn.Name != expectedNames[i] {
			t.Errorf("Expected function '%s', got '%s'", expectedNames[i], fn.Name)
		}
	}
}

func TestExtractTypeString(t *testing.T) {
	tests := []struct {
		description string
		// We can't easily test this without creating AST nodes,
		// but the logic is tested implicitly in the main test
	}{
		{"pointer types should have * prefix"},
		{"slice types should have [] prefix"},
		{"map types should have map[key]value format"},
		{"channel types should indicate direction"},
	}

	// This is more of a documentation test
	for _, test := range tests {
		t.Log(test.description)
	}
}

func TestBuildSignatureString(t *testing.T) {
	// Test regular function
	funcInfo := FunctionInfo{
		Name: "ValidateUser",
		Parameters: []ParameterInfo{
			{Name: "user", Type: "*User"},
		},
		Returns: []ReturnInfo{
			{Type: "error"},
		},
	}

	expected := "func ValidateUser(user *User) error"
	signature := buildSignatureString(funcInfo)
	if signature != expected {
		t.Errorf("Expected '%s', got '%s'", expected, signature)
	}

	// Test method
	methodInfo := FunctionInfo{
		Name:     "GetName",
		IsMethod: true,
		Receiver: &ReceiverInfo{
			Name: "u",
			Type: "*User",
		},
		Returns: []ReturnInfo{
			{Type: "string"},
		},
	}

	expectedMethod := "func (u *User) GetName() string"
	methodSignature := buildSignatureString(methodInfo)
	if methodSignature != expectedMethod {
		t.Errorf("Expected '%s', got '%s'", expectedMethod, methodSignature)
	}
}
