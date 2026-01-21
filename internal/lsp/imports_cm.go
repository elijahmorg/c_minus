package lsp

import (
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/project"
)

// importedModulePrefixesFromFile parses import directives in a .cm file.
// It returns a map from prefix (last path segment) to import path.
// Example: import "utils/io" => prefix "io".
func importedModulePrefixesFromFile(filePath string) map[string]string {
	pf, err := parser.ParseFile(filePath)
	if err != nil || pf == nil {
		return map[string]string{}
	}

	out := make(map[string]string)
	for _, imp := range pf.Imports {
		p := imp.Path
		prefix := project.ImportPrefix(p)
		out[prefix] = p
	}
	return out
}

// importedModulePrefixesFromText parses import directives from in-memory content,
// using the actual file path for error messages.
func importedModulePrefixesFromText(filePath, cmText string) map[string]string {
	pf, err := parser.ParseSource(cmText, filePath)
	if err != nil || pf == nil {
		return map[string]string{}
	}

	out := make(map[string]string)
	for _, imp := range pf.Imports {
		p := imp.Path
		prefix := project.ImportPrefix(p)
		out[prefix] = p
	}
	return out
}

// importedModulePrefixes is a best-effort helper used by completion/hover.
// If parsing fails, it falls back to a dumb line-based scan.
func importedModulePrefixes(filePath, cmText string) map[string]string {
	var out map[string]string
	if cmText != "" {
		out = importedModulePrefixesFromText(filePath, cmText)
	}
	if len(out) == 0 {
		out = importedModulePrefixesFromFile(filePath)
	}
	if len(out) == 0 {
		out = importedModulePrefixesFallback(cmText)
	}
	return out
}

func importedModulePrefixesFallback(cmText string) map[string]string {
	out := make(map[string]string)
	lines := splitLinesPreserve(cmText)
	for _, line := range lines {
		line = trimSpaces(line)
		if len(line) < len("import \"x\"") {
			continue
		}
		if !hasPrefix(line, "import \"") {
			continue
		}
		rest := line[len("import \""):]
		end := indexOfSubstring(rest, "\"")
		if end < 0 {
			continue
		}
		path := rest[:end]
		prefix := project.ImportPrefix(filepath.ToSlash(path))
		out[prefix] = filepath.ToSlash(path)
	}
	return out
}

func trimSpaces(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
