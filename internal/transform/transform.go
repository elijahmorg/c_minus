package transform

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/elijahmorgan/c_minus/internal/parser"
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

// TransformFunctionBody transforms qualified symbol access in a function body
// Converts "module.symbol" to "full_module_path_symbol" using the import map
func TransformFunctionBody(body string, importMap ImportMap) string {
	if len(importMap) == 0 {
		return body
	}

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

			// Check if this is an imported module prefix
			if fullPath, ok := importMap[prefix]; ok {
				// This is a qualified access - transform it
				mangledPrefix := strings.ReplaceAll(fullPath, "/", "_")

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

	for i, ch := range body {
		if ch == '.' {
			flushIdent()
			flushOther()
			tokens = append(tokens, token{kind: tokenDot, value: "."})
			inIdent = false
		} else if isIdentStart(ch) || (inIdent && isIdentContinue(ch)) {
			if !inIdent {
				flushOther()
				inIdent = true
			}
			current.WriteRune(ch)
		} else {
			if inIdent {
				flushIdent()
				inIdent = false
			}
			current.WriteRune(ch)

			// Special handling for string literals and comments to avoid false matches
			if ch == '"' {
				// Consume string literal
				i++
				for i < len(body) && body[i] != '"' {
					if body[i] == '\\' && i+1 < len(body) {
						current.WriteRune(rune(body[i]))
						i++
					}
					if i < len(body) {
						current.WriteRune(rune(body[i]))
						i++
					}
				}
				if i < len(body) {
					current.WriteRune(rune(body[i]))
				}
			}
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
	mangledPrefix := strings.ReplaceAll(modulePath, "/", "_")
	return mangledPrefix + "_" + typeName
}

// MangleFunctionName mangles a function name with the module prefix
func MangleFunctionName(funcName string, modulePath string) string {
	mangledPrefix := strings.ReplaceAll(modulePath, "/", "_")
	return mangledPrefix + "_" + funcName
}

// TransformTypeBody transforms type references within a type body
// This handles types that reference other types from the same or other modules
func TransformTypeBody(body string, importMap ImportMap, currentModule string) string {
	// For now, we'll keep type bodies opaque and only transform local references
	// Full type reference transformation is deferred to future phases
	return body
}
