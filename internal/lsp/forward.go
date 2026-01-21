package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) forwardHover(ctx context.Context, msg jsonrpcMessage) error {
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

	proj, err := project.Discover(filepath.Dir(cmPath))
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	// If we have the current buffer text, try to provide a C-minus native hover.
	s.mu.Lock()
	cmText, hasText := s.openDocs[cmPath]
	s.mu.Unlock()
	if hasText {
		if cmHover, ok := s.tryCMHover(proj, cmPath, cmText, params.Position.Line, params.Position.Character); ok {
			return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: cmHover})
		}
	}

	modPath, err := projectModuleImportPath(proj, cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	cPath := generatedCPath(proj.RootPath, modPath, filepath.Base(cmPath))
	cURI, err := fileURIFromPath(cPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	lm, err := s.getLineMapperForCFile(cPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	cLine1, ok := lm.mapToGeneratedLine(cmPath, params.Position.Line+1)
	if !ok {
		// If we can't map, fall back to same line number.
		cLine1 = params.Position.Line + 1
	}

	cChar := params.Position.Character

	// clangd is often position-sensitive (e.g. hovering on '(' yields null). If the direct
	// hover returns no result, we try to snap the character to a nearby identifier.
	forwardParams := func(char int) map[string]any {
		return map[string]any{
			"textDocument": map[string]any{"uri": cURI},
			"position": map[string]any{
				"line":      cLine1 - 1,
				"character": char,
			},
		}
	}

	var raw json.RawMessage
	if err := s.clangd.request(ctx, "textDocument/hover", forwardParams(cChar), &raw); err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	if len(raw) == 0 || string(raw) == "null" {
		// Best-effort: adjust char based on the generated C line.
		if snapped, ok := snapCharToIdentifierInCFile(cPath, cLine1, cChar); ok {
			_ = s.clangd.request(ctx, "textDocument/hover", forwardParams(snapped), &raw)
		}
	}

	mapped, _, err := mapHoverResultToCM(lm, raw)
	if err != nil {
		// Best-effort: return clangd's response unmodified.
		mapped = raw
	}

	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: mapped})
}

func (s *server) forwardDefinition(ctx context.Context, msg jsonrpcMessage) error {
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

	proj, err := project.Discover(filepath.Dir(cmPath))
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	// If we have the current buffer text, try to provide a C-minus native definition
	// for qualified symbols (e.g. ticket.create_ticket).
	s.mu.Lock()
	cmText, hasText := s.openDocs[cmPath]
	s.mu.Unlock()
	if hasText {
		if cmDef, ok := s.tryCMDefinition(proj, cmPath, cmText, params.Position.Line, params.Position.Character); ok {
			return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: cmDef})
		}
	}

	modPath, err := projectModuleImportPath(proj, cmPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	cPath := generatedCPath(proj.RootPath, modPath, filepath.Base(cmPath))
	cURI, err := fileURIFromPath(cPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	// Ensure mapper is based on latest generated C.
	if _, err := os.Stat(cPath); err != nil {
		return s.writeError(msg.ID, -32002, fmt.Sprintf("generated file missing: %v", err))
	}

	lm, err := s.getLineMapperForCFile(cPath)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	cLine1, ok := lm.mapToGeneratedLine(cmPath, params.Position.Line+1)
	if !ok {
		cLine1 = params.Position.Line + 1
	}

	cChar := params.Position.Character
	forwardParams := func(char int) map[string]any {
		return map[string]any{
			"textDocument": map[string]any{"uri": cURI},
			"position": map[string]any{
				"line":      cLine1 - 1,
				"character": char,
			},
		}
	}

	var raw json.RawMessage
	if err := s.clangd.request(ctx, "textDocument/definition", forwardParams(cChar), &raw); err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	if len(raw) == 0 || string(raw) == "null" {
		if snapped, ok := snapCharToIdentifierInCFile(cPath, cLine1, cChar); ok {
			_ = s.clangd.request(ctx, "textDocument/definition", forwardParams(snapped), &raw)
		}
	}

	mapped, err := mapDefinitionResultToCM(lm, raw)
	if err != nil {
		mapped = raw
	}

	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: mapped})
}
