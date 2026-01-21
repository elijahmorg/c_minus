package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) documentSymbols(ctx context.Context, msg jsonrpcMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.writeError(msg.ID, -32602, fmt.Sprintf("invalid params: %v", err))
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
		return s.writeError(msg.ID, -32002, "document not open")
	}

	proj, err := project.Discover(filepath.Dir(cmPath))
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	idx, err := buildModuleIndex(proj, map[string]string{cmPath: cmText})
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	modPath, err := projectModuleImportPath(proj, cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	syms := idx.Modules[modPath]

	uri, err := fileURIFromPath(cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	var out []any
	for _, sym := range syms {
		if filepath.Clean(sym.File) != filepath.Clean(cmPath) {
			continue
		}

		kind := 13 // Enum
		switch sym.Kind {
		case symbolKindFunc:
			kind = 12 // Function
		case symbolKindStruct, symbolKindUnion:
			kind = 23 // Struct
		case symbolKindEnum:
			kind = 10 // Enum
		case symbolKindTypedef:
			kind = 23
		case symbolKindGlobal:
			kind = 13 // Variable
		case symbolKindDefine:
			kind = 14 // Constant
		}

		startLine0 := sym.Line1 - 1
		if startLine0 < 0 {
			startLine0 = 0
		}
		startChar0 := sym.Char0
		if startChar0 < 0 {
			startChar0 = 0
		}

		r := map[string]any{
			"start": map[string]any{"line": startLine0, "character": startChar0},
			"end":   map[string]any{"line": startLine0, "character": startChar0 + len(sym.Name)},
		}

		out = append(out, map[string]any{
			"name":           sym.Name,
			"kind":           kind,
			"range":          r,
			"selectionRange": r,
		})
	}

	// DocumentSymbol[] response
	b, _ := json.Marshal(out)
	_ = uri
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: b})
}

func (s *server) workspaceSymbols(ctx context.Context, msg jsonrpcMessage) error {
	var params struct {
		Query string `json:"query"`
	}
	_ = json.Unmarshal(msg.Params, &params)

	if s.rootPath == "" {
		return s.writeError(msg.ID, -32002, "server not initialized")
	}

	proj, err := project.Discover(s.rootPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
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

	var out []any
	for _, syms := range idx.Modules {
		for _, sym := range syms {
			if params.Query != "" && indexOfSubstring(sym.Name, params.Query) < 0 {
				continue
			}

			uri, err := fileURIFromPath(sym.File)
			if err != nil {
				continue
			}

			startLine0 := sym.Line1 - 1
			if startLine0 < 0 {
				startLine0 = 0
			}
			startChar0 := sym.Char0
			if startChar0 < 0 {
				startChar0 = 0
			}

			kind := 12 // Function
			switch sym.Kind {
			case symbolKindFunc:
				kind = 12
			case symbolKindStruct, symbolKindUnion:
				kind = 23
			case symbolKindEnum:
				kind = 10
			case symbolKindTypedef:
				kind = 23
			case symbolKindGlobal:
				kind = 13
			case symbolKindDefine:
				kind = 14
			}

			out = append(out, map[string]any{
				"name": sym.Name,
				"kind": kind,
				"location": map[string]any{
					"uri": uri,
					"range": map[string]any{
						"start": map[string]any{"line": startLine0, "character": startChar0},
						"end":   map[string]any{"line": startLine0, "character": startChar0 + len(sym.Name)},
					},
				},
			})
		}
	}

	b, _ := json.Marshal(out)
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: b})
}
