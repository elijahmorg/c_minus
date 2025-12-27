package transform

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/paths"
)

// ImportMap maps import prefixes to full module paths
// Example: {"io": "utils/io", "math": "math"}
type ImportMap map[string]string

// BuildImportMap creates a map from module prefix to full path for all imports
func BuildImportMap(imports []*parser.Import) (ImportMap, error) {
	importMap := make(ImportMap)

	for _, imp := range imports {
		// Get the last segment of the import path as the prefix
		prefix := getModulePrefix(imp.Path)

		// Check for collisions
		if existing, exists := importMap[prefix]; exists {
			if existing != imp.Path {
				return nil, fmt.Errorf("import prefix collision: both %q and %q would use prefix %q",
					existing, imp.Path, prefix)
			}
		}

		importMap[prefix] = imp.Path
	}

	return importMap, nil
}

// getModulePrefix extracts the last segment of a module path as the prefix
// Example: "utils/io" -> "io", "math" -> "math"
func getModulePrefix(modulePath string) string {
	// Use the last segment after the last slash
	parts := strings.Split(modulePath, "/")
	return parts[len(parts)-1]
}

// CImportMap maps C header prefixes to their header paths
// Example: {"stdio": "stdio.h", "stdlib": "stdlib.h"}
// Used to transform stdio.printf -> printf (strip prefix, no mangling)
type CImportMap map[string]string

// BuildCImportMap creates a map from header prefix to header path
// For "stdio.h", the prefix is "stdio"
func BuildCImportMap(cimports []*parser.CImport) (CImportMap, error) {
	cimportMap := make(CImportMap)

	for _, cimp := range cimports {
		// Get prefix by stripping .h extension
		prefix := getCImportPrefix(cimp.Path)

		// Check for collisions
		if existing, exists := cimportMap[prefix]; exists {
			if existing != cimp.Path {
				return nil, fmt.Errorf("cimport prefix collision: both %q and %q would use prefix %q",
					existing, cimp.Path, prefix)
			}
		}

		cimportMap[prefix] = cimp.Path
	}

	return cimportMap, nil
}

// getCImportPrefix extracts the prefix from a C header path
// Example: "stdio.h" -> "stdio", "sys/types.h" -> "types"
func getCImportPrefix(headerPath string) string {
	// Strip .h extension if present
	name := headerPath
	if strings.HasSuffix(name, ".h") {
		name = name[:len(name)-2]
	}
	// Use the last segment after any slash (for headers like sys/types.h)
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// EnumValueMap maps enum value names to their qualified replacement
// Example: {"TODO": "ticket_Status_TODO", "IN_PROGRESS": "ticket_Status_IN_PROGRESS"}
type EnumValueMap map[string]string

// GlobalVarMap maps global variable names to their mangled names
// Example: {"counter": "state_counter", "version": "state_version"}
type GlobalVarMap map[string]string

// TransformFunctionBody transforms qualified symbol access in a function body
// Converts "module.symbol" to "full_module_path_symbol" using the import map
func TransformFunctionBody(body string, importMap ImportMap) string {
	return TransformFunctionBodyFull(body, importMap, nil, nil, nil)
}

// TransformFunctionBodyWithEnums transforms qualified symbol access and enum values in a function body
// Converts "module.symbol" to "full_module_path_symbol" using the import map
// Also transforms bare enum values like "TODO" to "module_EnumName_TODO"
func TransformFunctionBodyWithEnums(body string, importMap ImportMap, enumValues EnumValueMap) string {
	return TransformFunctionBodyFull(body, importMap, nil, enumValues, nil)
}

// TransformFunctionBodyFull transforms qualified symbol access, C imports, enum values, and global variables
// - For c_minus imports: "module.symbol" -> "module_symbol" (mangled)
// - For C imports: "stdio.printf" -> "printf" (just strip prefix, no mangling)
// - For enum values: "TODO" -> "module_EnumName_TODO"
// - For global variables: "counter" -> "module_counter"
func TransformFunctionBodyFull(body string, importMap ImportMap, cimportMap CImportMap, enumValues EnumValueMap, globalVars GlobalVarMap) string {
	// Tokenize the body
	tokens := tokenize(body)

	// Transform qualified access patterns
	var result strings.Builder
	i := 0

	for i < len(tokens) {
		tok := tokens[i]

		// Check for Ident.Ident or Ident.Ident.Ident patterns
		if tok.kind == tokenIdent && i+1 < len(tokens) && tokens[i+1].kind == tokenDot {
			prefix := tok.value

			// Check if this is a C import prefix (e.g., stdio.printf -> printf)
			if _, ok := cimportMap[prefix]; ok {
				// This is a C import access - just strip the prefix
				i += 2 // Skip prefix and dot

				// Collect the symbol name (no mangling for C imports)
				if i < len(tokens) && tokens[i].kind == tokenIdent {
					result.WriteString(tokens[i].value)
					i++
				}
			} else if fullPath, ok := importMap[prefix]; ok {
				// This is a c_minus module qualified access - transform with mangling
				mangledPrefix := paths.SanitizeModuleName(fullPath)

				// Skip the module prefix and dot
				i += 2

				// Collect the rest of the qualified name
				var parts []string
				parts = append(parts, mangledPrefix)

				for i < len(tokens) {
					if tokens[i].kind == tokenIdent {
						parts = append(parts, tokens[i].value)
						i++

						// Check if there's another dot (for enum values)
						if i < len(tokens) && tokens[i].kind == tokenDot {
							i++ // Skip the dot
							continue
						} else {
							break
						}
					} else {
						break
					}
				}

				// Emit the mangled name
				result.WriteString(strings.Join(parts, "_"))
			} else {
				// Not an imported module - could be struct field access, emit as-is
				result.WriteString(tok.value)
				i++
			}
		} else if tok.kind == tokenIdent {
			// Check if this is an enum value that needs qualification
			if replacement, ok := enumValues[tok.value]; ok {
				result.WriteString(replacement)
			} else if replacement, ok := globalVars[tok.value]; ok {
				// Check if this is a global variable that needs mangling
				result.WriteString(replacement)
			} else {
				result.WriteString(tok.value)
			}
			i++
		} else {
			// Not a qualified access pattern - emit as-is
			result.WriteString(tok.value)
			i++
		}
	}

	return result.String()
}

// Token types
type tokenKind int

const (
	tokenIdent tokenKind = iota
	tokenDot
	tokenOther
)

type token struct {
	kind  tokenKind
	value string
}

// tokenize breaks the body into tokens (simple tokenization for transformation)
func tokenize(body string) []token {
	var tokens []token
	var current strings.Builder

	flushIdent := func() {
		if current.Len() > 0 {
			tokens = append(tokens, token{kind: tokenIdent, value: current.String()})
			current.Reset()
		}
	}

	flushOther := func() {
		if current.Len() > 0 {
			tokens = append(tokens, token{kind: tokenOther, value: current.String()})
			current.Reset()
		}
	}

	inIdent := false
	i := 0

	for i < len(body) {
		ch := rune(body[i])

		if ch == '.' {
			flushIdent()
			flushOther()
			tokens = append(tokens, token{kind: tokenDot, value: "."})
			inIdent = false
			i++
		} else if ch == '"' {
			// String literal - consume the entire string as an "other" token
			if inIdent {
				flushIdent()
				inIdent = false
			}
			current.WriteByte(body[i])
			i++
			// Consume until closing quote
			for i < len(body) && body[i] != '"' {
				if body[i] == '\\' && i+1 < len(body) {
					// Escape sequence
					current.WriteByte(body[i])
					i++
					if i < len(body) {
						current.WriteByte(body[i])
						i++
					}
				} else {
					current.WriteByte(body[i])
					i++
				}
			}
			// Consume closing quote
			if i < len(body) {
				current.WriteByte(body[i])
				i++
			}
		} else if ch == '\'' {
			// Character literal - consume the entire char literal as an "other" token
			if inIdent {
				flushIdent()
				inIdent = false
			}
			current.WriteByte(body[i])
			i++
			// Consume until closing quote
			for i < len(body) && body[i] != '\'' {
				if body[i] == '\\' && i+1 < len(body) {
					current.WriteByte(body[i])
					i++
					if i < len(body) {
						current.WriteByte(body[i])
						i++
					}
				} else {
					current.WriteByte(body[i])
					i++
				}
			}
			// Consume closing quote
			if i < len(body) {
				current.WriteByte(body[i])
				i++
			}
		} else if isIdentStart(ch) || (inIdent && isIdentContinue(ch)) {
			if !inIdent {
				flushOther()
				inIdent = true
			}
			current.WriteByte(body[i])
			i++
		} else {
			if inIdent {
				flushIdent()
				inIdent = false
			}
			current.WriteByte(body[i])
			i++
		}
	}

	flushIdent()
	flushOther()

	return tokens
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isIdentContinue(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

// MangleTypeName mangles a type name with the module prefix
func MangleTypeName(typeName string, modulePath string) string {
	return paths.SanitizeModuleName(modulePath) + "_" + typeName
}

// MangleFunctionName mangles a function name with the module prefix
func MangleFunctionName(funcName string, modulePath string) string {
	return paths.SanitizeModuleName(modulePath) + "_" + funcName
}

// TransformTypeBody transforms type references within a type body
// This handles types that reference other types from the same or other modules
func TransformTypeBody(body string, importMap ImportMap, currentModule string) string {
	// For now, we'll keep type bodies opaque and only transform local references
	// Full type reference transformation is deferred to future phases
	return body
}
