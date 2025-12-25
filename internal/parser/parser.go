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
	Enum     *EnumDecl
	Typedef  *TypedefDecl
}

// FuncDecl represents a function declaration
type FuncDecl struct {
	Public     bool
	ReturnType string
	Name       string
	Params     []*Param
	Body       string
}

// Param represents a function parameter
type Param struct {
	Name string
	Type string
}

// StructDecl represents a struct type declaration
type StructDecl struct {
	Public bool
	Name   string
	Body   string // Opaque body: everything between { and }
	Semi   bool
}

// EnumDecl represents an enum type declaration
type EnumDecl struct {
	Public bool
	Name   string
	Body   string // Opaque body: everything between { and }
	Semi   bool
}

// TypedefDecl represents a typedef declaration
type TypedefDecl struct {
	Public bool
	Body   string // Everything from typedef to ;
	Semi   bool
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
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			i++
			continue
		}

		// Check for function declaration
		if strings.Contains(line, "func") {
			funcDecl, consumed, err := parseFunction(lines, i, source)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			file.Decls = append(file.Decls, &Decl{Function: funcDecl})
			i += consumed
		} else if strings.Contains(line, "struct") {
			structDecl, consumed, err := parseStruct(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			file.Decls = append(file.Decls, &Decl{Struct: structDecl})
			i += consumed
		} else if strings.Contains(line, "enum") {
			enumDecl, consumed, err := parseEnum(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			file.Decls = append(file.Decls, &Decl{Enum: enumDecl})
			i += consumed
		} else if strings.Contains(line, "typedef") {
			typedefDecl, consumed, err := parseTypedef(lines, i)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
			}
			file.Decls = append(file.Decls, &Decl{Typedef: typedefDecl})
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

	// Find return type (word after ')')
	closeParenIdx := strings.Index(line, ")")
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

// parseParams parses function parameters from string like "int a, float b" (C-style)
func parseParams(paramStr string) []*Param {
	params := []*Param{}

	if strings.TrimSpace(paramStr) == "" {
		return params
	}

	parts := strings.Split(paramStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) >= 2 {
			// C-style: type comes first, name is the last token
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
