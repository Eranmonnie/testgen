package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// FileAnalysis contains all parsed information from a Go file
type FileAnalysis struct {
	PackageName string
	Imports     []ImportInfo
	Functions   []FunctionInfo
	Constants   map[string]string
	Variables   map[string]string
	Types       []TypeInfo
}

// ImportInfo represents an import statement
type ImportInfo struct {
	Name string // alias name (if any)
	Path string // import path
}

// TypeInfo represents type definitions in the file
type TypeInfo struct {
	Name   string
	Kind   string // struct, interface, etc.
	Fields []string
}

// FunctionInfo represents detailed function analysis
type FunctionInfo struct {
	Name       string
	Package    string
	File       string
	StartLine  int
	EndLine    int
	Signature  string
	Parameters []ParameterInfo
	Returns    []ReturnInfo
	IsMethod   bool
	Receiver   *ReceiverInfo
	Comments   []string
	Complexity ComplexityInfo
	Body       string // function body for context
}

type ParameterInfo struct {
	Name string
	Type string
}

type ReturnInfo struct {
	Name string
	Type string
}

type ReceiverInfo struct {
	Name string
	Type string
}

type ComplexityInfo struct {
	HasErrors            bool
	HasPointers          bool
	HasInterfaces        bool
	HasChannels          bool
	HasGoroutines        bool
	HasDefers            bool
	HasPanic             bool
	Dependencies         []string
	CyclomaticComplexity int
	ControlFlowCount     int // if, for, switch, select statements
}

// ParseFile analyzes a Go source file and extracts function information
func ParseFile(filePath string) (*FileAnalysis, error) {
	fset := token.NewFileSet()

	// Parse the file
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	analysis := &FileAnalysis{
		PackageName: node.Name.Name,
		Constants:   make(map[string]string),
	}

	// Extract imports
	for _, imp := range node.Imports {
		importInfo := ImportInfo{
			Path: strings.Trim(imp.Path.Value, `"`),
		}
		if imp.Name != nil {
			importInfo.Name = imp.Name.Name
		}
		analysis.Imports = append(analysis.Imports, importInfo)
	}

	// Walk the AST and extract information
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Include all functions, not just exported ones
			// We'll filter later based on requirements
			funcInfo := analyzeFunctionDecl(x, fset, filePath)
			analysis.Functions = append(analysis.Functions, funcInfo)
		case *ast.GenDecl:
			// Handle constants and type declarations
			analyzeGenDecl(x, analysis)
		}
		return true
	})

	return analysis, nil
}

// analyzeFunctionDecl extracts detailed information from a function declaration
func analyzeFunctionDecl(funcDecl *ast.FuncDecl, fset *token.FileSet, filePath string) FunctionInfo {
	funcInfo := FunctionInfo{
		Name:    funcDecl.Name.Name,
		Package: filepath.Base(filepath.Dir(filePath)),
		File:    filePath,
	}

	// Get line numbers
	startPos := fset.Position(funcDecl.Pos())
	endPos := fset.Position(funcDecl.End())
	funcInfo.StartLine = startPos.Line
	funcInfo.EndLine = endPos.Line

	// Check if it's a method (has receiver)
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		funcInfo.IsMethod = true
		receiver := funcDecl.Recv.List[0]
		funcInfo.Receiver = &ReceiverInfo{
			Type: extractTypeString(receiver.Type),
		}
		if len(receiver.Names) > 0 {
			funcInfo.Receiver.Name = receiver.Names[0].Name
		}
	}

	// Extract parameters
	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			typeStr := extractTypeString(param.Type)
			if len(param.Names) > 0 {
				// Named parameters
				for _, name := range param.Names {
					funcInfo.Parameters = append(funcInfo.Parameters, ParameterInfo{
						Name: name.Name,
						Type: typeStr,
					})
				}
			} else {
				// Unnamed parameter (interface{}, etc.)
				funcInfo.Parameters = append(funcInfo.Parameters, ParameterInfo{
					Name: "",
					Type: typeStr,
				})
			}
		}
	}

	// Extract return types
	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			typeStr := extractTypeString(result.Type)
			if len(result.Names) > 0 {
				// Named returns
				for _, name := range result.Names {
					funcInfo.Returns = append(funcInfo.Returns, ReturnInfo{
						Name: name.Name,
						Type: typeStr,
					})
				}
			} else {
				// Unnamed return
				funcInfo.Returns = append(funcInfo.Returns, ReturnInfo{
					Type: typeStr,
				})
			}
		}
	}

	// Extract comments
	if funcDecl.Doc != nil {
		for _, comment := range funcDecl.Doc.List {
			funcInfo.Comments = append(funcInfo.Comments, strings.TrimPrefix(comment.Text, "//"))
		}
	}

	// Build signature string
	funcInfo.Signature = buildSignatureString(funcInfo)

	// Analyze complexity
	if funcDecl.Body != nil {
		funcInfo.Complexity = analyzeComplexity(funcDecl.Body)
		funcInfo.Body = extractBodyString(funcDecl.Body, fset)
	}

	// Additional complexity analysis from signature
	// Check for error returns
	for _, ret := range funcInfo.Returns {
		if ret.Type == "error" {
			funcInfo.Complexity.HasErrors = true
		}
	}

	// Check for pointer parameters
	for _, param := range funcInfo.Parameters {
		if strings.HasPrefix(param.Type, "*") {
			funcInfo.Complexity.HasPointers = true
		}
	}

	// Check for pointer receiver
	if funcInfo.IsMethod && funcInfo.Receiver != nil && strings.HasPrefix(funcInfo.Receiver.Type, "*") {
		funcInfo.Complexity.HasPointers = true
	}

	return funcInfo
}

// extractTypeString converts an ast.Expr to a string representation
func extractTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractTypeString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + extractTypeString(t.Elt)
		}
		return "[...]" + extractTypeString(t.Elt) // simplified
	case *ast.MapType:
		return "map[" + extractTypeString(t.Key) + "]" + extractTypeString(t.Value)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + extractTypeString(t.Value)
		case ast.RECV:
			return "<-chan " + extractTypeString(t.Value)
		default:
			return "chan " + extractTypeString(t.Value)
		}
	case *ast.InterfaceType:
		return "interface{}" // simplified
	case *ast.StructType:
		return "struct{}" // simplified
	case *ast.FuncType:
		return "func(...)" // simplified
	case *ast.SelectorExpr:
		return extractTypeString(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
}

// analyzeComplexity analyzes function body for complexity indicators
func analyzeComplexity(body *ast.BlockStmt) ComplexityInfo {
	complexity := ComplexityInfo{}

	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			// Check for common patterns
			if ident, ok := x.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "panic":
					complexity.HasPanic = true
				}
			}
			// Check for method calls that might indicate error handling
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Error" {
					complexity.HasErrors = true
				}
			}
		case *ast.DeferStmt:
			complexity.HasDefers = true
		case *ast.GoStmt:
			complexity.HasGoroutines = true
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			complexity.ControlFlowCount++
		case *ast.StarExpr:
			complexity.HasPointers = true
		case *ast.ChanType:
			complexity.HasChannels = true
		case *ast.InterfaceType:
			complexity.HasInterfaces = true
		case *ast.Ident:
			// Check for error type usage
			if x.Name == "error" {
				complexity.HasErrors = true
			}
		}
		return true
	})

	// Also check function signature for error returns and pointer params
	// This will be set by the calling function

	// Simple cyclomatic complexity approximation
	complexity.CyclomaticComplexity = complexity.ControlFlowCount + 1

	return complexity
}

// buildSignatureString creates a human-readable function signature
func buildSignatureString(funcInfo FunctionInfo) string {
	var sig strings.Builder

	sig.WriteString("func ")

	// Add receiver if it's a method
	if funcInfo.IsMethod && funcInfo.Receiver != nil {
		sig.WriteString("(")
		if funcInfo.Receiver.Name != "" {
			sig.WriteString(funcInfo.Receiver.Name + " ")
		}
		sig.WriteString(funcInfo.Receiver.Type)
		sig.WriteString(") ")
	}

	sig.WriteString(funcInfo.Name)
	sig.WriteString("(")

	// Add parameters
	for i, param := range funcInfo.Parameters {
		if i > 0 {
			sig.WriteString(", ")
		}
		if param.Name != "" {
			sig.WriteString(param.Name + " ")
		}
		sig.WriteString(param.Type)
	}

	sig.WriteString(")")

	// Add return types
	if len(funcInfo.Returns) > 0 {
		sig.WriteString(" ")
		if len(funcInfo.Returns) > 1 {
			sig.WriteString("(")
		}
		for i, ret := range funcInfo.Returns {
			if i > 0 {
				sig.WriteString(", ")
			}
			if ret.Name != "" {
				sig.WriteString(ret.Name + " ")
			}
			sig.WriteString(ret.Type)
		}
		if len(funcInfo.Returns) > 1 {
			sig.WriteString(")")
		}
	}

	return sig.String()
}

// extractBodyString extracts a simplified version of the function body
func extractBodyString(body *ast.BlockStmt, fset *token.FileSet) string {
	start := fset.Position(body.Pos())
	end := fset.Position(body.End())
	return fmt.Sprintf("// Function body from line %d to %d", start.Line, end.Line)
}

// analyzeGenDecl handles const and type declarations
func analyzeGenDecl(decl *ast.GenDecl, analysis *FileAnalysis) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.ValueSpec:
			// Constants and variables
			if decl.Tok == token.CONST {
				for i, name := range s.Names {
					if len(s.Values) > i {
						// Simplified constant value extraction
						analysis.Constants[name.Name] = extractValue(s.Values[i])
					}
				}
			} else if decl.Tok == token.VAR {
				// add variable handling
				for i, name := range s.Names {
					if len(s.Values) > i {
						// Simplified variable value extraction
						analysis.Variables[name.Name] = extractValue(s.Values[i])
					}
				}
			}

		case *ast.TypeSpec:
			// Type definitions
			typeInfo := TypeInfo{
				Name: s.Name.Name,
				Kind: extractTypeString(s.Type),
			}
			analysis.Types = append(analysis.Types, typeInfo)
		}
	}
}
func extractValue(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	}
	return "unknown"
}

// FilterFunctions filters functions by names (from git diff analysis)
func (fa *FileAnalysis) FilterFunctions(functionNames []string) []FunctionInfo {
	nameSet := make(map[string]bool)
	for _, name := range functionNames {
		nameSet[name] = true
	}

	var filtered []FunctionInfo
	for _, fn := range fa.Functions {
		if nameSet[fn.Name] {
			filtered = append(filtered, fn)
		}
	}
	return filtered
}
