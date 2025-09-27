// internal/analyzer/integration.go
package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Eranmonnie/testgen/internal/git"
	"github.com/Eranmonnie/testgen/internal/parser"
	"github.com/Eranmonnie/testgen/pkg/models"
)

// AnalysisResult combines git diff and AST analysis
type AnalysisResult struct {
	ChangedFiles      []ChangedFileAnalysis
	TotalFunctions    int
	ModifiedFunctions int
	GenerationTargets []models.FunctionInfo
}

// ChangedFileAnalysis represents analysis of a single changed file
type ChangedFileAnalysis struct {
	FilePath          string
	ModifiedFunctions []string
	FunctionDetails   []models.FunctionInfo
	FileAnalysis      *parser.FileAnalysis
}

// AnalyzeChanges performs complete analysis of git changes
func AnalyzeChanges(fromRef, toRef string) (*AnalysisResult, error) {
	// Step 1: Get git diff
	diffResult, err := git.GetDiff(fromRef, toRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	// Filter to only Go files
	goFiles := diffResult.FilterGoFiles()

	result := &AnalysisResult{
		ChangedFiles: make([]ChangedFileAnalysis, 0, len(goFiles.Files)),
	}

	// Step 2: Analyze each changed Go file
	for _, fileDiff := range goFiles.Files {
		fileAnalysis, err := analyzeChangedFile(fileDiff)
		if err != nil {
			// Log error but continue with other files
			fmt.Printf("Warning: failed to analyze %s: %v\n", fileDiff.NewPath, err)
			continue
		}

		if fileAnalysis != nil {
			result.ChangedFiles = append(result.ChangedFiles, *fileAnalysis)
			result.TotalFunctions += len(fileAnalysis.FunctionDetails)
			result.ModifiedFunctions += len(fileAnalysis.ModifiedFunctions)
		}
	}

	// Step 3: Build generation targets
	result.GenerationTargets = buildGenerationTargets(result.ChangedFiles)

	return result, nil
}

// analyzeChangedFile analyzes a single file from git diff
func analyzeChangedFile(fileDiff git.FileDiff) (*ChangedFileAnalysis, error) {
	// Skip if file was deleted
	if fileDiff.NewPath == "" {
		return nil, nil
	}

	// Parse the Go file using AST
	fileAnalysis, err := parser.ParseFile(fileDiff.NewPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	// Get functions that were actually modified (not just context)
	modifiedFunctionNames := fileDiff.GetModifiedFunctions()

	if len(modifiedFunctionNames) == 0 {
		// No functions were modified in this file
		return nil, nil
	}

	// Filter AST analysis to only modified functions
	modifiedFunctions := fileAnalysis.FilterFunctions(modifiedFunctionNames)

	// Convert to our models format
	var functionDetails []models.FunctionInfo
	for _, fn := range modifiedFunctions {
		modelFunc := convertToModelFunction(fn, fileAnalysis)
		functionDetails = append(functionDetails, modelFunc)
	}

	return &ChangedFileAnalysis{
		FilePath:          fileDiff.NewPath,
		ModifiedFunctions: modifiedFunctionNames,
		FunctionDetails:   functionDetails,
		FileAnalysis:      fileAnalysis,
	}, nil
}

// convertToModelFunction converts parser.FunctionInfo to models.FunctionInfo
func convertToModelFunction(fn parser.FunctionInfo, fileAnalysis *parser.FileAnalysis) models.FunctionInfo {
	modelFunc := models.FunctionInfo{
		Name:      fn.Name,
		Package:   fn.Package,
		File:      fn.File,
		Signature: fn.Signature,
		IsMethod:  fn.IsMethod,
		Comments:  fn.Comments,
	}

	// Convert parameters
	for _, param := range fn.Parameters {
		modelFunc.Parameters = append(modelFunc.Parameters, models.ParameterInfo{
			Name: param.Name,
			Type: param.Type,
		})
	}

	// Convert returns
	for _, ret := range fn.Returns {
		modelFunc.Returns = append(modelFunc.Returns, models.ReturnInfo{
			Name: ret.Name,
			Type: ret.Type,
		})
	}

	// Convert receiver if method
	if fn.IsMethod && fn.Receiver != nil {
		modelFunc.Receiver = &models.ReceiverInfo{
			Name: fn.Receiver.Name,
			Type: fn.Receiver.Type,
		}
	}

	// Convert complexity info
	modelFunc.Complexity = models.ComplexityInfo{
		HasErrors:            fn.Complexity.HasErrors,
		HasPointers:          fn.Complexity.HasPointers,
		HasInterfaces:        fn.Complexity.HasInterfaces,
		HasChannels:          fn.Complexity.HasChannels,
		HasGoroutines:        fn.Complexity.HasGoroutines,
		Dependencies:         fn.Complexity.Dependencies,
		CyclomaticComplexity: fn.Complexity.CyclomaticComplexity,
	}

	return modelFunc
}

// buildGenerationTargets creates the list of functions to generate tests for
func buildGenerationTargets(changedFiles []ChangedFileAnalysis) []models.FunctionInfo {
	var targets []models.FunctionInfo

	for _, file := range changedFiles {
		for _, fn := range file.FunctionDetails {
			if shouldGenerateTest(fn) {
				targets = append(targets, fn)
			}
		}
	}

	return targets
}

// shouldGenerateTest determines if we should generate a test for this function
func shouldGenerateTest(fn models.FunctionInfo) bool {
	// Skip main functions
	if fn.Name == "main" {
		return false
	}

	// Skip init functions
	if fn.Name == "init" {
		return false
	}

	// Skip existing test functions (we don't generate tests for tests)
	if isTestFunction(fn.Name) {
		return false
	}

	// Only include exported functions by default (this is our main filter now)
	if !isExported(fn.Name) {
		return false
	}

	// Skip functions that are too complex (could be configurable)
	if fn.Complexity.CyclomaticComplexity > 15 {
		return false
	}

	// Skip functions with no parameters and no return values (usually not worth testing)
	if len(fn.Parameters) == 0 && len(fn.Returns) == 0 {
		return false
	}

	return true
}

// isTestFunction checks if function name indicates it's a test
func isTestFunction(name string) bool {
	return len(name) > 4 && (name[:4] == "Test" ||
		name[:9] == "Benchmark" ||
		name[:7] == "Example" ||
		name[:4] == "Fuzz")
}

// isExported checks if function is exported (starts with capital letter)
func isExported(name string) bool {
	if name == "" {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// GetProjectContext extracts context information for the entire project
func GetProjectContext(analysisResult *AnalysisResult) models.RequestContext {
	context := models.RequestContext{
		ProjectName: getProjectName(),
		GitContext:  getGitContext(),
	}

	// Aggregate imports and constants across all files
	importSet := make(map[string]bool)
	allConstants := make(map[string]string)

	for _, file := range analysisResult.ChangedFiles {
		if file.FileAnalysis != nil {
			// Collect unique imports
			for _, imp := range file.FileAnalysis.Imports {
				importSet[imp.Path] = true
			}

			// Collect constants
			for name, value := range file.FileAnalysis.Constants {
				allConstants[name] = value
			}

			// Set package name from first file
			if context.PackageName == "" {
				context.PackageName = file.FileAnalysis.PackageName
			}
		}
	}

	// Convert import set to slice
	for imp := range importSet {
		context.Imports = append(context.Imports, imp)
	}
	context.Constants = allConstants

	return context
}

// getProjectName tries to determine project name from go.mod or directory
func getProjectName() string {
	// Try to read go.mod first
	if content, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					// Get last part of module path
					modulePath := parts[1]
					return filepath.Base(modulePath)
				}
			}
		}
	}

	// Fallback to current directory name
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}

	return "unknown"
}

// getGitContext extracts git-related context
func getGitContext() models.GitContext {
	context := models.GitContext{}

	// Get current branch
	if cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			context.Branch = strings.TrimSpace(string(output))
		}
	}

	// Get last commit message
	if cmd := exec.Command("git", "log", "-1", "--pretty=format:%s"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			context.CommitMessage = strings.TrimSpace(string(output))
		}
	}

	// Get author of last commit
	if cmd := exec.Command("git", "log", "-1", "--pretty=format:%an"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			context.Author = strings.TrimSpace(string(output))
		}
	}

	return context
}

// AnalyzeSpecificFunctions analyzes only specific functions in specific files
func AnalyzeSpecificFunctions(filePaths []string, functionNames []string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		ChangedFiles: make([]ChangedFileAnalysis, 0, len(filePaths)),
	}

	functionSet := make(map[string]bool)
	for _, name := range functionNames {
		functionSet[name] = true
	}

	for _, filePath := range filePaths {
		// Skip non-Go files
		if !strings.HasSuffix(filePath, ".go") || strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		// Parse the file
		fileAnalysis, err := parser.ParseFile(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to analyze %s: %v\n", filePath, err)
			continue
		}

		// Filter to requested functions
		var filteredFunctions []parser.FunctionInfo
		var matchedNames []string

		for _, fn := range fileAnalysis.Functions {
			if len(functionNames) == 0 || functionSet[fn.Name] {
				filteredFunctions = append(filteredFunctions, fn)
				matchedNames = append(matchedNames, fn.Name)
			}
		}

		if len(filteredFunctions) == 0 {
			continue
		}

		// Convert to model functions
		var functionDetails []models.FunctionInfo
		for _, fn := range filteredFunctions {
			modelFunc := convertToModelFunction(fn, fileAnalysis)
			functionDetails = append(functionDetails, modelFunc)
		}

		fileAnalysisResult := ChangedFileAnalysis{
			FilePath:          filePath,
			ModifiedFunctions: matchedNames,
			FunctionDetails:   functionDetails,
			FileAnalysis:      fileAnalysis,
		}

		result.ChangedFiles = append(result.ChangedFiles, fileAnalysisResult)
		result.TotalFunctions += len(functionDetails)
		result.ModifiedFunctions += len(matchedNames)
	}

	result.GenerationTargets = buildGenerationTargets(result.ChangedFiles)
	return result, nil
}

// PrintAnalysisSummary prints a summary of the analysis results
func PrintAnalysisSummary(result *AnalysisResult) {
	fmt.Printf("Analysis Summary:\n")
	fmt.Printf("================\n")
	fmt.Printf("Files analyzed: %d\n", len(result.ChangedFiles))
	fmt.Printf("Total functions found: %d\n", result.TotalFunctions)
	fmt.Printf("Modified functions: %d\n", result.ModifiedFunctions)
	fmt.Printf("Test generation targets: %d\n", len(result.GenerationTargets))
	fmt.Printf("\n")

	for _, file := range result.ChangedFiles {
		fmt.Printf("File: %s\n", file.FilePath)
		fmt.Printf("  Modified functions: %v\n", file.ModifiedFunctions)
		fmt.Printf("  Package: %s\n", file.FileAnalysis.PackageName)
		fmt.Printf("  Imports: %d\n", len(file.FileAnalysis.Imports))

		for _, fn := range file.FunctionDetails {
			fmt.Printf("    - %s (complexity: %d, params: %d, returns: %d)\n",
				fn.Name, fn.Complexity.CyclomaticComplexity,
				len(fn.Parameters), len(fn.Returns))

			if fn.Complexity.HasErrors {
				fmt.Printf("      [handles errors]")
			}
			if fn.Complexity.HasGoroutines {
				fmt.Printf("      [uses goroutines]")
			}
			if fn.Complexity.HasPointers {
				fmt.Printf("      [uses pointers]")
			}
			if fn.IsMethod {
				fmt.Printf("      [method]")
			}
			fmt.Printf("\n")
		}
		fmt.Printf("\n")
	}
}
