package lsp

import (
	"encoding/json"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) tryCMDefinition(proj *project.Project, cmPath, cmText string, line0, char0 int) (json.RawMessage, bool) {
	lines := splitLinesPreserve(cmText)
	if line0 < 0 || line0 >= len(lines) {
		return nil, false
	}
	line := lines[line0]

	// Snap within the CM line so requests on '(' still work.
	if snapped, ok := snapCharToIdentifier(line, char0); ok {
		char0 = snapped
	}

	ident, qualifier := identifierAt(line, char0)
	if ident == "" {
		return nil, false
	}
	if qualifier == "" {
		// Find qualifier to the left of the dot.
		for i := char0; i >= 0 && i < len(line); i-- {
			if line[i] == '.' {
				name, _ := lastIdentifier(line[:i])
				qualifier = name
				break
			}
		}
	}
	if qualifier == "" {
		return nil, false
	}

	imports := importedModulePrefixes(cmPath, cmText)
	importPath, ok := imports[qualifier]
	if !ok {
		return nil, false
	}

	s.mu.Lock()
	openDocsCopy := make(map[string]string, len(s.openDocs))
	for k, v := range s.openDocs {
		openDocsCopy[k] = v
	}
	s.mu.Unlock()

	idx, err := buildModuleIndex(proj, openDocsCopy)
	if err != nil {
		return nil, false
	}

	syms := idx.Modules[importPath]
	var sym *cmSymbol
	for i := range syms {
		if syms[i].Name == ident {
			if !syms[i].Public {
				continue
			}
			sym = &syms[i]
			break
		}
	}
	if sym == nil {
		return nil, false
	}

	uri, err := fileURIFromPath(sym.File)
	if err != nil {
		return nil, false
	}

	startLine0 := sym.Line1 - 1
	if startLine0 < 0 {
		startLine0 = 0
	}
	startChar0 := sym.Char0
	if startChar0 < 0 {
		startChar0 = 0
	}

	// Ensure range end is at least name-length.
	endChar0 := startChar0 + len(sym.Name)

	loc := map[string]any{
		"uri": uri,
		"range": map[string]any{
			"start": map[string]any{"line": startLine0, "character": startChar0},
			"end":   map[string]any{"line": startLine0, "character": endChar0},
		},
	}

	_ = filepath.Clean(cmPath)
	b, _ := json.Marshal([]any{loc})
	return b, true
}
