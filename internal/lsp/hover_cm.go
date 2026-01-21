package lsp

import (
	"encoding/json"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) tryCMHover(proj *project.Project, cmPath, cmText string, line0, char0 int) (json.RawMessage, bool) {
	lines := splitLinesPreserve(cmText)
	if line0 < 0 || line0 >= len(lines) {
		return nil, false
	}
	line := lines[line0]

	// If cursor is not on an identifier, snap within the CM line.
	snapped, ok := snapCharToIdentifier(line, char0)
	if ok {
		char0 = snapped
	}

	ident, qualifier := identifierAt(line, char0)
	if ident == "" {
		// If not directly on an identifier (common when cursor is on '('), try snapping.
		if snapped, ok := snapCharToIdentifier(line, char0); ok {
			ident, qualifier = identifierAt(line, snapped)
			char0 = snapped
		}
	}
	if ident == "" {
		return nil, false
	}
	if qualifier == "" {
		// If identifierAt didn't detect a qualifier (it only looks for "." immediately
		// before the identifier), check for qualified access where the dot is AFTER the identifier.
		if char0+len(ident) < len(line) && line[char0+len(ident)] == '.' {
			qualifier = ident
			ident = ""
		}
	}
	if qualifier == "" {
		// Support hover when the cursor is on the module prefix (e.g. "ticket" in "ticket.create_ticket").
		if char0 < len(line) && line[char0] == '.' {
			name, _ := lastIdentifier(line[:char0])
			qualifier = name
			ident = ""
		} else if char0+1 < len(line) && line[char0+1] == '.' {
			name, _ := lastIdentifier(line[:char0+1])
			qualifier = name
			ident = ""
		} else {
			// Also support hovering on the member name in "mod.member".
			for i := char0; i >= 0 && i < len(line); i-- {
				if line[i] == '.' {
					name, _ := lastIdentifier(line[:i])
					if name != "" {
						qualifier = name
						break
					}
					break
				}
			}
			if qualifier == "" {
				return nil, false
			}
		}
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
	if ident != "" {
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
	}

	// Range on the CM token.
	start := 0
	end := 0
	var value string
	if ident == "" {
		// Hover on module prefix.
		start = indexOfIdentifier(line, qualifier)
		if start < 0 {
			start = char0
		}
		end = start + len(qualifier)
		value = "```c\nmodule \"" + importPath + "\"\n```"
	} else {
		start = indexOfIdentifier(line, ident)
		if start < 0 {
			start = char0
		}
		end = start + len(ident)

		value = "```c\n" + sym.Signature + "\n```"
		if sym.Doc != "" {
			value += "\n\n" + sym.Doc
		}
	}

	hover := map[string]any{
		"contents": map[string]any{
			"kind":  "markdown",
			"value": value,
		},
		"range": map[string]any{
			"start": map[string]any{"line": line0, "character": start},
			"end":   map[string]any{"line": line0, "character": end},
		},
	}

	b, _ := json.Marshal(hover)
	_ = filepath.Clean(cmPath)
	return b, true
}
