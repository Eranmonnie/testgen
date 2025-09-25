package git

import (
	"testing"
)

func TestParseDiff(t *testing.T) {
	// Sample git diff output
	diffText := `diff --git a/user.go b/user.go
index 1234567..abcdefg 100644
--- a/user.go
+++ b/user.go
@@ -10,6 +10,12 @@ func ValidateUser(user *User) error {
     if user.Email == "" {
         return errors.New("email required")
     }
+    if !strings.Contains(user.Email, "@") {
+        return errors.New("invalid email format")
+    }
     return nil
 }
 
@@ -25,3 +31,7 @@ func CreateUser(name, email string) *User {
         Email: email,
     }
 }
+
+func DeleteUser(id int) error {
+    return database.Delete("users", id)
+}`

	result, err := parseDiff(diffText)
	if err != nil {
		t.Fatalf("parseDiff failed: %v", err)
	}

	// Should have one file
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	file := result.Files[0]

	// Check file paths
	if file.OldPath != "user.go" || file.NewPath != "user.go" {
		t.Errorf("expected user.go -> user.go, got %s -> %s", file.OldPath, file.NewPath)
	}

	// Check functions found
	expectedFunctions := []string{"ValidateUser", "CreateUser", "DeleteUser"}
	if len(file.Functions) != len(expectedFunctions) {
		t.Errorf("expected %d functions, got %d: %v", len(expectedFunctions), len(file.Functions), file.Functions)
	}

	// Check modified functions (only those with + or - changes)
	modifiedFunctions := file.GetModifiedFunctions()
	expectedModified := []string{"ValidateUser", "DeleteUser"}

	if len(modifiedFunctions) != len(expectedModified) {
		t.Errorf("expected %d modified functions, got %d: %v", len(expectedModified), len(modifiedFunctions), modifiedFunctions)
	}

	// Check that we have some added lines
	addedCount := 0
	for _, change := range file.Changes {
		if change.Type == Added {
			addedCount++
		}
	}

	if addedCount == 0 {
		t.Error("expected some added lines, got none")
	}
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		context  string
		expected string
	}{
		{"func ValidateUser(user *User) error {", "ValidateUser"},
		{"func (u *User) GetName() string {", "GetName"},
		{"func CreateUser(name, email string) *User {", "CreateUser"},
		{"func main() {", "main"},
		{"    if user.Email == \"\" {", ""}, // not a function line
	}

	for _, test := range tests {
		result := extractFunctionName(test.context)
		if result != test.expected {
			t.Errorf("extractFunctionName(%q) = %q, expected %q", test.context, result, test.expected)
		}
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
