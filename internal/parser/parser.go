package parser

import (
	"fmt"
	"os"
	"strings"
)

// File represents a parsed .cm file
type File struct {
	Module   *ModuleDecl
	Imports  []*Import
	CImports []*CImport
	Decls    []*Decl
}

// ModuleDecl represents a module declaration
type ModuleDecl struct {
	Path string
}

// Import represents an import statement for c_minus modules
type Import struct {
	Path string
}

// CImport represents a C header import statement
type CImport struct {
	Path string // e.g., "stdio.h"
}

// Decl represents a top-level declaration (function, type, etc.)
type Decl struct {
	Function *FuncDecl
	Struct   *StructDecl
	Union    *UnionDecl
	Enum     *EnumDecl
	Typedef  *TypedefDecl
	Global   *GlobalDecl
}

// GlobalDecl represents a global variable declaration
type GlobalDecl struct {
	Public     bool
	Type       string // e.g., "int", "char*", "const char*"
	Name       string
	Value      string // Initial value (optional, empty if uninitialized)
	DocComment string
}

// FuncDecl represents a function declaration
type FuncDecl struct {
	Public     bool
	ReturnType string
	Name       string
	Params     []*Param
	Body       string
	DocComment string // Go-style doc comment (comments immediately preceding the declaration)
}

// Param represents a function parameter
type Param struct {
	Name string
	Type string
}

// StructDecl represents a struct type declaration
type StructDecl struct {
	Public     bool
	Name       string
	Body       string // Opaque body: everything between { and }
	Semi       bool
	DocComment string // Go-style doc comment (comments immediately preceding the declaration)
}

// UnionDecl represents a union type declaration
type UnionDecl struct {
	Public     bool
	Name       string
	Body       string // Opaque body: everything between { and }
	Semi       bool
	DocComment string // Go-style doc comment (comments immediately preceding the declaration)
}

// EnumDecl represents an enum type declaration
type EnumDecl struct {
	Public     bool
	Name       string
	Body       string // Opaque body: everything between { and }
	Semi       bool
	DocComment string // Go-style doc comment (comments immediately preceding the declaration)
}

// TypedefDecl represents a typedef declaration
type TypedefDecl struct {
	Public     bool
	Body       string // Everything from typedef to ;
	Semi       bool
	DocComment string // Go-style doc comment (comments immediately preceding the declaration)
}

// Manual parser implementation - no Participle code generation needed

// ParseFile parses a .cm file
func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// For now, use a simpler manual parser until we refine Participle grammar
	return manualParse(string(data), path)
}

// manualParse is a simple manual parser for initial implementation
func manualParse(source string, path string) (*File, error) {
	file := &File{
		Imports:  []*Import{},
		CImports: []*CImport{},
		Decls:    []*Decl{},
	}

	lines := strings.Split(source, "\n")

	// Phase 1: Extract module, imports, and cimports
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "module") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				file.Module = &ModuleDecl{
					Path: strings.Trim(parts[1], `"`),
				}
			}
		}

		// Check for cimport before import (since "import" is a prefix of "cimport" when checking HasPrefix)
		if strings.HasPrefix(line, "cimport") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				file.CImports = append(file.CImports, &CImport{
					Path: strings.Trim(parts[1], `"`),
				})
			}
		} else if strings.HasPrefix(line, "import") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				file.Imports = append(file.Imports, &Import{
					Path: strings.Trim(parts[1], `"`),
				})
			}
		}
	}

	if file.Module == nil {
		return nil, fmt.Errorf("%s: no module declaration found", path)
	}

	// Phase 2: Extract declarations (functions and types)
	i := 0
	var pendingDocComment []string // Collects consecutive comment lines
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Handle empty lines - they break doc comment association
		if line == "" {
			pendingDocComment = nil // Reset pending doc comments on blank line
			i++
			continue
		}

		// Handle comments - collect them as potential doc comments
		if strings.HasPrefix(line, "//") {
			pendingDocComment = append(pendingDocComment, line)
			i++
			continue
		}

		// Get the doc comment string (if any)
		docComment := buildDocComment(pendingDocComment)
		pendingDocComment = nil // Reset after use

		// Check for function declaration
		if strings.Contains(line, "func") {
			funcDecl, consumed, err := parseFunction(lines, i, source)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			funcDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Function: funcDecl})
			i += consumed
		} else if strings.Contains(line, "struct") {
			structDecl, consumed, err := parseStruct(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			structDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Struct: structDecl})
			i += consumed
		} else if strings.Contains(line, "union") {
			unionDecl, consumed, err := parseUnion(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			unionDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Union: unionDecl})
			i += consumed
		} else if strings.Contains(line, "enum") {
			enumDecl, consumed, err := parseEnum(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			enumDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Enum: enumDecl})
			i += consumed
		} else if strings.Contains(line, "typedef") {
			typedefDecl, consumed, err := parseTypedef(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			typedefDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Typedef: typedefDecl})
			i += consumed
		} else if isGlobalVariableDecl(line) {
			globalDecl, consumed, err := parseGlobal(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			globalDecl.DocComment = docComment
			file.Decls = append(file.Decls, &Decl{Global: globalDecl})
			i += consumed
		} else {
			i++
		}
	}

	return file, nil
}

// parseFunction parses a function declaration starting at the given line
func parseFunction(lines []string, startIdx int, fullSource string) (*FuncDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	funcDecl := &FuncDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		funcDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Parse "func name(params) returnType"
	if !strings.HasPrefix(line, "func ") {
		return nil, 0, fmt.Errorf("expected 'func' keyword")
	}

	line = strings.TrimPrefix(line, "func ")

	// Find function name (word before '(')
	parenIdx := strings.Index(line, "(")
	if parenIdx == -1 {
		return nil, 0, fmt.Errorf("expected '(' after function name")
	}

	nameParts := strings.Fields(line[:parenIdx])
	if len(nameParts) < 1 {
		return nil, 0, fmt.Errorf("missing function name")
	}

	funcDecl.Name = nameParts[0]

	// Find matching closing parenthesis (respecting nested parens for function pointers)
	closeParenIdx := findMatchingParen(line, parenIdx)
	if closeParenIdx == -1 {
		return nil, 0, fmt.Errorf("expected ')' after parameters")
	}

	// Parse parameters
	paramStr := line[parenIdx+1 : closeParenIdx]
	funcDecl.Params = parseParams(paramStr)

	// Parse return type
	afterParen := strings.TrimSpace(line[closeParenIdx+1:])
	retTypeParts := strings.Fields(afterParen)
	if len(retTypeParts) > 0 {
		// Remove '{' if present
		funcDecl.ReturnType = strings.TrimSuffix(retTypeParts[0], "{")
	}

	// Extract function body (brace-balanced)
	body, consumed := extractBraceBlock(lines, startIdx)
	funcDecl.Body = body

	return funcDecl, consumed, nil
}

// findMatchingParen finds the index of the closing ')' that matches the opening '(' at startIdx
func findMatchingParen(s string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseParams parses function parameters from string like "int a, float b" (C-style)
// Also handles function pointer parameters like "int (*cmp)(void*, void*)"
// and variadic parameters "..."
func parseParams(paramStr string) []*Param {
	params := []*Param{}

	if strings.TrimSpace(paramStr) == "" {
		return params
	}

	// Split on commas, but respect parentheses (for function pointer params)
	parts := splitParamsRespectingParens(paramStr)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle variadic marker "..."
		if part == "..." {
			params = append(params, &Param{
				Name: "",
				Type: "...",
			})
			continue
		}

		// Check if this is a function pointer parameter: contains "(*" pattern
		if strings.Contains(part, "(*") {
			param := parseFunctionPointerParam(part)
			if param != nil {
				params = append(params, param)
			}
			continue
		}

		// Normal parameter: C-style where type comes first, name is last token
		fields := strings.Fields(part)
		if len(fields) >= 2 {
			// This handles cases like "int a", "ticket.Ticket* t", "unsigned int x"
			name := fields[len(fields)-1]
			typeParts := fields[:len(fields)-1]
			paramType := strings.Join(typeParts, " ")
			params = append(params, &Param{
				Name: name,
				Type: paramType,
			})
		}
	}

	return params
}

// splitParamsRespectingParens splits a parameter string on commas,
// but respects parentheses so function pointer params stay together.
// E.g., "int a, void (*cb)(int, int), float b" -> ["int a", "void (*cb)(int, int)", "float b"]
func splitParamsRespectingParens(paramStr string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range paramStr {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Don't forget the last part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseFunctionPointerParam parses a function pointer parameter.
// Input format: "returnType (*name)(paramTypes)" or "returnType (*name)(paramTypes)"
// Returns a Param with the name extracted and the type as the full signature minus the name.
func parseFunctionPointerParam(part string) *Param {
	// Find the (*name) part
	// Pattern: returnType (*name)(paramTypes)
	startParen := strings.Index(part, "(*")
	if startParen == -1 {
		return nil
	}

	// Find the closing paren after the name
	endParen := strings.Index(part[startParen:], ")")
	if endParen == -1 {
		return nil
	}
	endParen += startParen // Adjust to absolute position

	// Extract the name (between "(* and )")
	name := strings.TrimSpace(part[startParen+2 : endParen])

	// Build the type by replacing the name with empty
	// E.g., "int (*cmp)(void*, void*)" -> "int (*)(void*, void*)"
	typeStr := part[:startParen+2] + part[endParen:]

	return &Param{
		Name: name,
		Type: typeStr,
	}
}

// extractBraceBlock extracts a brace-balanced block starting from a line
func extractBraceBlock(lines []string, startIdx int) (string, int) {
	var result strings.Builder
	braceCount := 0
	foundStart := false
	consumed := 0
	inBody := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		consumed++

		// Process character by character to extract only from { to }
		for j, ch := range line {
			if ch == '{' {
				if !foundStart {
					foundStart = true
					inBody = true
					result.WriteRune('{')
				} else {
					result.WriteRune(ch)
				}
				braceCount++
			} else if ch == '}' {
				braceCount--
				result.WriteRune(ch)
				if braceCount == 0 && foundStart {
					return result.String(), consumed
				}
			} else if inBody {
				result.WriteRune(ch)
			}

			// Preserve everything after the opening brace on the same line
			if foundStart && braceCount > 0 && j < len(line)-1 {
				// Already handled by rune iteration
			}
		}

		// Add newline if we're in the body and not at the end
		if inBody && braceCount > 0 {
			result.WriteRune('\n')
		}
	}

	return result.String(), consumed
}

// parseStruct parses a struct declaration starting at the given line
func parseStruct(lines []string, startIdx int) (*StructDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	structDecl := &StructDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		structDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Parse "struct Name"
	if !strings.HasPrefix(line, "struct ") {
		return nil, 0, fmt.Errorf("expected 'struct' keyword")
	}

	line = strings.TrimPrefix(line, "struct ")
	line = strings.TrimSpace(line)

	// Extract struct name (word before '{' or ';')
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == '{' || r == ';'
	})
	if len(parts) < 1 {
		return nil, 0, fmt.Errorf("missing struct name")
	}

	structDecl.Name = strings.TrimSpace(parts[0])

	// Check if this is a forward declaration (ends with ;)
	if strings.Contains(line, ";") && !strings.Contains(line, "{") {
		structDecl.Body = ""
		structDecl.Semi = true
		return structDecl, 1, nil
	}

	// Extract struct body (brace-balanced)
	body, consumed := extractBraceBlock(lines, startIdx)
	structDecl.Body = body

	// Check for semicolon after body
	lastLine := strings.TrimSpace(lines[startIdx+consumed-1])
	if strings.HasSuffix(lastLine, ";") || (startIdx+consumed < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[startIdx+consumed]), ";")) {
		structDecl.Semi = true
		if startIdx+consumed < len(lines) && strings.TrimSpace(lines[startIdx+consumed]) == ";" {
			consumed++
		}
	}

	return structDecl, consumed, nil
}

// parseUnion parses a union declaration starting at the given line
func parseUnion(lines []string, startIdx int) (*UnionDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	unionDecl := &UnionDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		unionDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Parse "union Name"
	if !strings.HasPrefix(line, "union ") {
		return nil, 0, fmt.Errorf("expected 'union' keyword")
	}

	line = strings.TrimPrefix(line, "union ")
	line = strings.TrimSpace(line)

	// Extract union name (word before '{' or ';')
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == '{' || r == ';'
	})
	if len(parts) < 1 {
		return nil, 0, fmt.Errorf("missing union name")
	}

	unionDecl.Name = strings.TrimSpace(parts[0])

	// Check if this is a forward declaration (ends with ;)
	if strings.Contains(line, ";") && !strings.Contains(line, "{") {
		unionDecl.Body = ""
		unionDecl.Semi = true
		return unionDecl, 1, nil
	}

	// Extract union body (brace-balanced)
	body, consumed := extractBraceBlock(lines, startIdx)
	unionDecl.Body = body

	// Check for semicolon after body
	lastLine := strings.TrimSpace(lines[startIdx+consumed-1])
	if strings.HasSuffix(lastLine, ";") || (startIdx+consumed < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[startIdx+consumed]), ";")) {
		unionDecl.Semi = true
		if startIdx+consumed < len(lines) && strings.TrimSpace(lines[startIdx+consumed]) == ";" {
			consumed++
		}
	}

	return unionDecl, consumed, nil
}

// parseEnum parses an enum declaration starting at the given line
func parseEnum(lines []string, startIdx int) (*EnumDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	enumDecl := &EnumDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		enumDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Parse "enum Name"
	if !strings.HasPrefix(line, "enum ") {
		return nil, 0, fmt.Errorf("expected 'enum' keyword")
	}

	line = strings.TrimPrefix(line, "enum ")
	line = strings.TrimSpace(line)

	// Extract enum name (word before '{')
	parts := strings.FieldsFunc(line, func(r rune) bool {
		return r == '{'
	})
	if len(parts) < 1 {
		return nil, 0, fmt.Errorf("missing enum name")
	}

	enumDecl.Name = strings.TrimSpace(parts[0])

	// Extract enum body (brace-balanced)
	body, consumed := extractBraceBlock(lines, startIdx)
	enumDecl.Body = body

	// Check for semicolon after body
	lastLine := strings.TrimSpace(lines[startIdx+consumed-1])
	if strings.HasSuffix(lastLine, ";") || (startIdx+consumed < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[startIdx+consumed]), ";")) {
		enumDecl.Semi = true
		if startIdx+consumed < len(lines) && strings.TrimSpace(lines[startIdx+consumed]) == ";" {
			consumed++
		}
	}

	return enumDecl, consumed, nil
}

// parseTypedef parses a typedef declaration starting at the given line
func parseTypedef(lines []string, startIdx int) (*TypedefDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	typedefDecl := &TypedefDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		typedefDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Parse typedef - capture everything from typedef to semicolon
	if !strings.HasPrefix(line, "typedef ") {
		return nil, 0, fmt.Errorf("expected 'typedef' keyword")
	}

	// Extract full typedef body (everything after "typedef" up to ";")
	consumed := 1
	var bodyBuilder strings.Builder

	// Start from the current line after "typedef"
	bodyBuilder.WriteString(strings.TrimPrefix(line, "typedef "))

	// If the line doesn't end with semicolon, continue reading
	for !strings.HasSuffix(strings.TrimSpace(lines[startIdx+consumed-1]), ";") {
		if startIdx+consumed >= len(lines) {
			return nil, 0, fmt.Errorf("typedef missing semicolon")
		}
		bodyBuilder.WriteString("\n")
		bodyBuilder.WriteString(lines[startIdx+consumed])
		consumed++
	}

	typedefDecl.Body = strings.TrimSpace(strings.TrimSuffix(bodyBuilder.String(), ";"))
	typedefDecl.Semi = true

	return typedefDecl, consumed, nil
}

// buildDocComment joins collected comment lines into a single doc comment string.
// It strips the leading "//" from each line and joins them with newlines.
func buildDocComment(commentLines []string) string {
	if len(commentLines) == 0 {
		return ""
	}

	var parts []string
	for _, line := range commentLines {
		// Strip the "//" prefix and optional single space after it
		text := strings.TrimPrefix(line, "//")
		if strings.HasPrefix(text, " ") {
			text = text[1:]
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n")
}

// isGlobalVariableDecl checks if a line looks like a global variable declaration
// It must:
// - Optionally start with "pub"
// - Followed by type(s) and a variable name
// - End with ";" or "= value;"
// - Not be a function (no "func" keyword, no "(" in declaration)
// - Not be a type definition (no "struct", "union", "enum", "typedef")
func isGlobalVariableDecl(line string) bool {
	// Skip if empty or doesn't contain potential declaration
	if line == "" {
		return false
	}

	// Skip module, import, cimport
	if strings.HasPrefix(line, "module") ||
		strings.HasPrefix(line, "import") ||
		strings.HasPrefix(line, "cimport") {
		return false
	}

	// Skip if it's a function, struct, union, enum, typedef
	if strings.Contains(line, "func ") ||
		strings.Contains(line, "struct ") ||
		strings.Contains(line, "union ") ||
		strings.Contains(line, "enum ") ||
		strings.Contains(line, "typedef ") {
		return false
	}

	// Skip if it has parentheses (function declaration or call)
	if strings.Contains(line, "(") {
		return false
	}

	// Check if line starts with "pub " and strip it
	workLine := line
	if strings.HasPrefix(workLine, "pub ") {
		workLine = strings.TrimPrefix(workLine, "pub ")
		workLine = strings.TrimSpace(workLine)
	}

	// Must end with ";" to be a declaration (may span multiple lines)
	// Simple heuristic: looks like "type name" or "type name = value"
	// The line should have at least 2 tokens
	fields := strings.Fields(workLine)
	if len(fields) < 2 {
		return false
	}

	// The line should contain = or ; to be a declaration
	return strings.Contains(line, ";") || strings.Contains(line, "=")
}

// parseGlobal parses a global variable declaration
func parseGlobal(lines []string, startIdx int) (*GlobalDecl, int, error) {
	line := strings.TrimSpace(lines[startIdx])

	globalDecl := &GlobalDecl{}

	// Check for pub modifier
	if strings.HasPrefix(line, "pub ") {
		globalDecl.Public = true
		line = strings.TrimPrefix(line, "pub ")
		line = strings.TrimSpace(line)
	}

	// Find the complete declaration (may span multiple lines until ;)
	fullDecl := line
	consumed := 1
	for !strings.Contains(fullDecl, ";") && startIdx+consumed < len(lines) {
		fullDecl += " " + strings.TrimSpace(lines[startIdx+consumed])
		consumed++
	}

	// Remove the trailing semicolon
	fullDecl = strings.TrimSuffix(strings.TrimSpace(fullDecl), ";")

	// Check if there's an initializer
	var declPart, valuePart string
	if eqIdx := strings.Index(fullDecl, "="); eqIdx != -1 {
		declPart = strings.TrimSpace(fullDecl[:eqIdx])
		valuePart = strings.TrimSpace(fullDecl[eqIdx+1:])
	} else {
		declPart = fullDecl
	}

	// Parse the type and name from declPart
	// Format: "type name" or "type1 type2 name" (e.g., "const char* version")
	fields := strings.Fields(declPart)
	if len(fields) < 2 {
		return nil, 0, fmt.Errorf("invalid global declaration: %s", fullDecl)
	}

	// Name is the last field, type is everything else
	globalDecl.Name = fields[len(fields)-1]
	globalDecl.Type = strings.Join(fields[:len(fields)-1], " ")
	globalDecl.Value = valuePart

	return globalDecl, consumed, nil
}
