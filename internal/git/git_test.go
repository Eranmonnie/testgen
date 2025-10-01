package git

import (
	"testing"
)

func TestParseDiff(t *testing.T) {
	diffOutput := `diff --git a/user.go b/user.go
index 1234567..abcdefg 100644
--- a/user.go
+++ b/user.go
@@ -10,6 +10,10 @@ func ValidateUser(user *User) error {
 )
 
 func ValidateUser(user *User) error {
+	if user == nil {
+		return errors.New("user is nil")
+	}
+	if user.Name == "" {
+		return errors.New("name required")
+	}
     return nil
 }
+
+func CreateUser(name, email string) *User {
+	return &User{
+		Name:  name,
+		Email: email,
+	}
+}
@@ -30,7 +40,7 @@ func GetUser(id int) (*User, error) {
     // This function appears in diff but has no actual changes
     return findUser(id)
`
	result, err := ParseDiff(diffOutput)
	if err != nil {
		t.Fatalf("ParseDiff failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	file := result.Files[0]
	functions := file.GetModifiedFunctions()

	// Debug: print what we actually found
	t.Logf("Found functions: %v", functions)
	t.Logf("File changes count: %d", len(file.Changes))
	for i, change := range file.Changes {
		if i < 5 { // Print first 5 changes for debugging
			t.Logf("Change %d: Type=%v, Line=%q, Function=%q", i, change.Type, change.Line, change.Function)
		}
	}

	// Should detect both ValidateUser (modified) and CreateUser (added)
	expectedFunctions := []string{"ValidateUser", "CreateUser"}
	if len(functions) != len(expectedFunctions) {
		t.Errorf("expected %d functions, got %d: %v", len(expectedFunctions), len(functions), functions)
	}

	// Check that both expected functions are found
	for _, expected := range expectedFunctions {
		found := false
		for _, actual := range functions {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected function %s not found in %v", expected, functions)
		}
	}
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"func ValidateUser(user *User) error {", "ValidateUser"},
		{"func CreateUser(name, email string) *User {", "CreateUser"},
		{"func main() {", "main"},
		{"func (u *User) GetName() string {", "GetName"},
		{"+func NewUser() *User {", "NewUser"},
		{" func helper() {", "helper"},
		{"not a function", ""},
		{"", ""},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := extractFunctionName(test.input)
			if result != test.expected {
				t.Errorf("extractFunctionName(%q) = %q, expected %q", test.input, result, test.expected)
			}
		})
	}
}

func TestFilterGoFiles(t *testing.T) {
	result := &DiffResult{
		Files: []FileDiff{
			{NewPath: "user.go"},
			{NewPath: "user_test.go"}, // should be filtered out
			{NewPath: "README.md"},    // should be filtered out
			{NewPath: "handler.go"},
		},
	}

	filtered := result.FilterGoFiles()

	if len(filtered.Files) != 2 {
		t.Errorf("expected 2 Go files, got %d", len(filtered.Files))
	}

	expectedFiles := []string{"user.go", "handler.go"}
	for i, file := range filtered.Files {
		if file.NewPath != expectedFiles[i] {
			t.Errorf("expected %s, got %s", expectedFiles[i], file.NewPath)
		}
	}
}
