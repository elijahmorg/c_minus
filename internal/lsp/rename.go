package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) rename(ctx context.Context, msg jsonrpcMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
		NewName string `json:"newName"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.writeError(msg.ID, -32602, fmt.Sprintf("invalid params: %v", err))
	}

	if params.NewName == "" {
		return s.writeError(msg.ID, -32602, "newName is required")
	}
	for i := 0; i < len(params.NewName); i++ {
		b := params.NewName[i]
		if !(isIdentChar(b) || (i == 0 && (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_')) {
			return s.writeError(msg.ID, -32602, "newName must be an identifier")
		}
	}

	cmPath, err := filePathFromURI(params.TextDocument.URI)
	if err != nil {
		return s.writeError(msg.ID, -32602, fmt.Sprintf("invalid uri: %v", err))
	}
	cmPath, err = filepath.Abs(cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32602, fmt.Sprintf("invalid path: %v", err))
	}

	s.mu.Lock()
	cmText, ok := s.openDocs[cmPath]
	s.mu.Unlock()
	if !ok {
		b, err := os.ReadFile(cmPath)
		if err != nil {
			return s.writeError(msg.ID, -32002, err.Error())
		}
		cmText = string(b)
	}

	lines := splitLinesPreserve(cmText)
	if params.Position.Line < 0 || params.Position.Line >= len(lines) {
		return s.writeError(msg.ID, -32602, "position out of range")
	}
	line := lines[params.Position.Line]
	if params.Position.Character < 0 {
		params.Position.Character = 0
	}
	if params.Position.Character > len(line) {
		params.Position.Character = len(line)
	}

	if isInStringOrComment(cmText, params.Position.Line, params.Position.Character) {
		return s.writeError(msg.ID, -32602, "rename not valid in strings/comments")
	}

	oldIdent, qualifier := identifierAt(line, params.Position.Character)
	if oldIdent == "" {
		return s.writeError(msg.ID, -32602, "no identifier at position")
	}

	proj, err := project.Discover(filepath.Dir(cmPath))
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	currentModule, err := projectModuleImportPath(proj, cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	targetModule := currentModule
	if qualifier != "" {
		targetModule = qualifier
	}

	s.mu.Lock()
	openDocsCopy := make(map[string]string, len(s.openDocs))
	for k, v := range s.openDocs {
		openDocsCopy[k] = v
	}
	s.mu.Unlock()

	idx, err := buildModuleIndex(proj, openDocsCopy)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	// Determine whether this is a public symbol.
	isPublic := false
	for _, sym := range idx.Modules[targetModule] {
		if sym.Name == oldIdent {
			isPublic = sym.Public
			break
		}
	}

	changes := make(map[string][]any)

	// Rename inside defining module: unqualified usages.
	for _, sym := range idx.Modules[targetModule] {
		if sym.Kind == "" {
			continue
		}
		// Use each file once.
		_ = sym
	}

	for _, fpath := range proj.Modules[targetModule].Files {
		text := openDocsCopy[fpath]
		if text == "" {
			b, err := os.ReadFile(fpath)
			if err != nil {
				continue
			}
			text = string(b)
		}
		edits := findRenameEdits(text, oldIdent, params.NewName, false, "")
		if len(edits) == 0 {
			continue
		}
		uri, err := fileURIFromPath(fpath)
		if err != nil {
			continue
		}
		changes[uri] = append(changes[uri], edits...)
	}

	// Rename qualified usages in other modules if public.
	if isPublic {
		for importPath, mod := range proj.Modules {
			if importPath == targetModule {
				continue
			}
			for _, fpath := range mod.Files {
				text := openDocsCopy[fpath]
				if text == "" {
					b, err := os.ReadFile(fpath)
					if err != nil {
						continue
					}
					text = string(b)
				}
				edits := findRenameEdits(text, oldIdent, params.NewName, true, targetModule)
				if len(edits) == 0 {
					continue
				}
				uri, err := fileURIFromPath(fpath)
				if err != nil {
					continue
				}
				changes[uri] = append(changes[uri], edits...)
			}
		}
	}

	workspaceEdit := map[string]any{"changes": changes}
	b, _ := json.Marshal(workspaceEdit)
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: b})
}

func identifierAt(line string, char0 int) (ident string, qualifier string) {
	// Expand left/right for identifier chars.
	start := char0
	if start > 0 && start == len(line) {
		start = len(line) - 1
	}
	for start > 0 && isIdentChar(line[start-1]) {
		start--
	}
	end := char0
	for end < len(line) && isIdentChar(line[end]) {
		end++
	}
	if start < 0 || end <= start {
		return "", ""
	}
	ident = line[start:end]

	// Check for qualifier like "mod.ident".
	if start > 0 && line[start-1] == '.' {
		qEnd := start - 1
		qStart := qEnd
		for qStart > 0 && isIdentChar(line[qStart-1]) {
			qStart--
		}
		if qStart < qEnd {
			qualifier = line[qStart:qEnd]
		}
	}
	return ident, qualifier
}

func findRenameEdits(text, oldName, newName string, qualified bool, module string) []any {
	lines := splitLinesPreserve(text)
	var out []any
	for i, line := range lines {
		search := line
		needle := oldName
		if qualified {
			needle = module + "." + oldName
		}

		pos := 0
		for {
			idx := indexOfSubstring(search[pos:], needle)
			if idx < 0 {
				break
			}
			abs := pos + idx

			// boundary checks
			beforeOK := abs == 0 || !isIdentChar(line[abs-1])
			afterIdx := abs + len(needle)
			afterOK := afterIdx >= len(line) || !isIdentChar(line[afterIdx])
			if !qualified {
				beforeOK = abs == 0 || !isIdentChar(line[abs-1])
				afterOK = afterIdx >= len(line) || !isIdentChar(line[afterIdx])
			}

			if beforeOK && afterOK {
				// Avoid renaming inside comments/strings.
				if isInStringOrComment(text, i, abs) {
					pos = abs + len(needle)
					if pos >= len(line) {
						break
					}
					continue
				}
				repl := newName
				if qualified {
					repl = module + "." + newName
				}
				out = append(out, map[string]any{
					"range": map[string]any{
						"start": map[string]any{"line": i, "character": abs},
						"end":   map[string]any{"line": i, "character": abs + len(needle)},
					},
					"newText": repl,
				})
			}

			pos = abs + len(needle)
			if pos >= len(line) {
				break
			}
		}
	}
	return out
}
