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

// parseDiff parses the raw git diff output
func parseDiff(diffText string) (*DiffResult, error) {
	result := &DiffResult{}
	scanner := bufio.NewScanner(strings.NewReader(diffText))
	
	var currentFile *FileDiff
	var currentFunction string
	
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
			continue
		}
		
		// Hunk header (contains function context)
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			if len(matches) > 5 && matches[5] != "" {
				// Extract function name from context
				funcContext := matches[5]
				currentFunction = extractFunctionName(funcContext)
				if currentFunction != "" && currentFile != nil {
					// Add to functions list if not already there
					found := false
					for _, f := range currentFile.Functions {
						if f == currentFunction {
							found = true
							break
						}
					}
					if !found {
						currentFile.Functions = append(currentFile.Functions, currentFunction)
					}
				}
			}
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
				currentFile.Changes = append(currentFile.Changes, *change)
			}
		}
	}
	
	// Don't forget the last file
	if currentFile != nil {
		result.Files = append(result.Files, *currentFile)
	}
	
	return result, nil
}

// extractFunctionName extracts function name from hunk context
func extractFunctionName(context string) string {
	// Go function pattern: "func FunctionName(" or "func (receiver) FunctionName("
	funcRegex := regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?(\w+)\s*\(`)
	matches := funcRegex.FindStringSubmatch(context)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
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

// GetModifiedFunctions returns functions that have actual code changes (not just context)
func (fd *FileDiff) GetModifiedFunctions() []string {
	functionSet := make(map[string]bool)
	
	for _, change := range fd.Changes {
		if change.Type == Added || change.Type == Removed {
			if change.Function != "" {
				functionSet[change.Function] = true
			}
		}
	}
	
	var functions []string
	for fn := range functionSet {
		functions = append(functions, fn)
	}
	return functions
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