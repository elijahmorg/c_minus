package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *server) prepareRename(ctx context.Context, msg jsonrpcMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
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
		// Per LSP spec, return null if rename not valid.
		return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")})
	}

	ident, _ := identifierAt(line, params.Position.Character)
	if ident == "" {
		return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")})
	}

	start := indexOfIdentifier(line, ident)
	if start < 0 {
		start = params.Position.Character
	}
	end := start + len(ident)

	res := map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": params.Position.Line, "character": start},
			"end":   map[string]any{"line": params.Position.Line, "character": end},
		},
		"placeholder": ident,
	}

	b, _ := json.Marshal(res)
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: b})
}
