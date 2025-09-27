package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DiffChange represents a single change in a diff
type DiffChange struct {
	Type     ChangeType // Added, Removed, Modified
	Line     string
	LineNum  int
	Function string // Function this change belongs to
}

// FileDiff represents all changes in a single file
type FileDiff struct {
	OldPath   string
	NewPath   string
	Changes   []DiffChange
	Functions []string // Functions that were modified
}

// DiffResult represents the complete diff analysis
type DiffResult struct {
	Files []FileDiff
}

type ChangeType int

const (
	Added ChangeType = iota
	Removed
	Modified
	Context
)

// GetDiff gets the diff between two git references
func GetDiff(from, to string) (*DiffResult, error) {
	// Get the raw diff with function context
	cmd := exec.Command("git", "diff", "--function-context", from, to)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	return parseDiff(string(output))
}

// GetChangedFiles returns just the list of changed file paths
func GetChangedFiles(from, to string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", from, to)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Add this helper method to better detect function modifications
func (fd *FileDiff) addFunctionIfModified(functionName string) {
	if functionName == "" {
		return
	}

	// Check if function already exists
	for _, existing := range fd.Functions {
		if existing == functionName {
			return
		}
	}

	// Add the function
	fd.Functions = append(fd.Functions, functionName)
}

// Update the parseDiff function to better handle function detection
func parseDiff(diffText string) (*DiffResult, error) {
	result := &DiffResult{}
	scanner := bufio.NewScanner(strings.NewReader(diffText))

	var currentFile *FileDiff
	var currentFunction string
	var lineNum int

	// Regex patterns for parsing
	fileHeaderRegex := regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)
	hunkHeaderRegex := regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@ ?(.*)$`)

	for scanner.Scan() {
		line := scanner.Text()

		// New file diff
		if matches := fileHeaderRegex.FindStringSubmatch(line); matches != nil {
			if currentFile != nil {
				result.Files = append(result.Files, *currentFile)
			}
			currentFile = &FileDiff{
				OldPath: matches[1],
				NewPath: matches[2],
			}
			lineNum = 0
			currentFunction = ""
			continue
		}

		// Hunk header (contains function context)
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			if len(matches) > 5 && matches[5] != "" {
				// Extract function name from context
				funcContext := matches[5]
				if extractedFunc := extractFunctionName(funcContext); extractedFunc != "" {
					currentFunction = extractedFunc
					if currentFile != nil {
						currentFile.addFunctionIfModified(currentFunction)
					}
				}
			}
			lineNum = 0
			continue
		}

		// Skip file metadata lines
		if strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Parse actual diff content
		if currentFile != nil {
			change := parseDiffLine(line, currentFunction)
			if change != nil {
				change.LineNum = lineNum
				currentFile.Changes = append(currentFile.Changes, *change)

				// If this line defines a new function, update our tracking
				if (change.Type == Added || change.Type == Context) && strings.Contains(change.Line, "func ") {
					if funcName := extractFunctionName(change.Line); funcName != "" {
						currentFile.addFunctionIfModified(funcName)
						currentFunction = funcName
					}
				}
			}
			lineNum++
		}
	}

	// Don't forget the last file
	if currentFile != nil {
		result.Files = append(result.Files, *currentFile)
	}

	return result, nil
}

// GetModifiedFunctions extracts function names that were actually modified
func (fd FileDiff) GetModifiedFunctions() []string {
	// Track which functions have actual changes (not just context)
	functionsWithChanges := make(map[string]bool)

	for _, change := range fd.Changes {
		// Only count functions that have additions or removals
		if change.Type == Added || change.Type == Removed {
			if change.Function != "" {
				functionsWithChanges[change.Function] = true
			}
		}
	}

	// Convert map to slice
	var result []string
	for funcName := range functionsWithChanges {
		result = append(result, funcName)
	}

	return result
}

// extractFunctionName extracts function name from a function declaration line or context
func extractFunctionName(line string) string {
	// Clean up the line
	line = strings.TrimSpace(line)

	// Handle context lines that might have extra characters
	if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") {
		line = strings.TrimSpace(line[1:])
	}

	// Must start with "func " to be a function declaration
	if !strings.HasPrefix(line, "func ") {
		return ""
	}

	// Remove "func " prefix
	line = strings.TrimPrefix(line, "func ")
	line = strings.TrimSpace(line)

	// Handle method declarations: (receiver) FunctionName(
	if strings.HasPrefix(line, "(") {
		// Find the closing parenthesis for receiver
		closeParen := strings.Index(line, ") ")
		if closeParen != -1 {
			// Skip the receiver part: ") FunctionName("
			line = strings.TrimSpace(line[closeParen+2:])
		}
	}

	// Now we should have: FunctionName(params...)
	// Find the opening parenthesis
	parenIndex := strings.Index(line, "(")
	if parenIndex == -1 {
		return ""
	}

	// Extract function name (everything before the '(')
	funcName := strings.TrimSpace(line[:parenIndex])

	// Remove any remaining special characters
	funcName = strings.Trim(funcName, " \t*&[]")

	return funcName
}

// parseDiffLine parses a single line from the diff
func parseDiffLine(line, currentFunction string) *DiffChange {
	if len(line) == 0 {
		return nil
	}

	change := &DiffChange{
		Function: currentFunction,
	}

	switch line[0] {
	case '+':
		change.Type = Added
		change.Line = line[1:]
	case '-':
		change.Type = Removed
		change.Line = line[1:]
	case ' ':
		change.Type = Context
		change.Line = line[1:]
	default:
		return nil // Skip unrecognized lines
	}

	return change
}

// FilterGoFiles filters the diff to only include Go files
func (dr *DiffResult) FilterGoFiles() *DiffResult {
	filtered := &DiffResult{}
	for _, file := range dr.Files {
		if strings.HasSuffix(file.NewPath, ".go") && !strings.HasSuffix(file.NewPath, "_test.go") {
			filtered.Files = append(filtered.Files, file)
		}
	}
	return filtered
}

// ParseDiff is the exported version of parseDiff
func ParseDiff(diffText string) (*DiffResult, error) {
	return parseDiff(diffText)
}
