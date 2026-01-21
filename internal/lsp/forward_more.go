package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/project"
)

func (s *server) forwardReferences(ctx context.Context, msg jsonrpcMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
		Context struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
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
		cLine1 = params.Position.Line + 1
	}

	forwardParams := map[string]any{
		"textDocument": map[string]any{"uri": cURI},
		"position": map[string]any{
			"line":      cLine1 - 1,
			"character": params.Position.Character,
		},
		"context": map[string]any{"includeDeclaration": params.Context.IncludeDeclaration},
	}

	var raw json.RawMessage
	if err := s.clangd.request(ctx, "textDocument/references", forwardParams, &raw); err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	mapped, err := mapDefinitionResultToCM(lm, raw)
	if err != nil {
		mapped = raw
	}
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: mapped})
}

func (s *server) forwardCompletion(ctx context.Context, msg jsonrpcMessage) error {
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
		cLine1 = params.Position.Line + 1
	}

	forwardParams := map[string]any{
		"textDocument": map[string]any{"uri": cURI},
		"position": map[string]any{
			"line":      cLine1 - 1,
			"character": params.Position.Character,
		},
	}

	// We decode into an interface{} so we can rewrite the edit ranges to .cm coordinates.
	var result any
	if err := s.clangd.request(ctx, "textDocument/completion", forwardParams, &result); err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}

	// Merge in C-minus specific completions.
	s.mu.Lock()
	cmText, ok := s.openDocs[cmPath]
	s.mu.Unlock()
	if !ok {
		cmText = ""
	}

	s.mu.Lock()
	openDocsCopy := make(map[string]string, len(s.openDocs))
	for k, v := range s.openDocs {
		openDocsCopy[k] = v
	}
	s.mu.Unlock()

	idx, _ := buildModuleIndex(proj, openDocsCopy)
	var cmItems []any
	if idx != nil && cmText != "" {
		cmItems = cmCompletions(proj, idx, cmPath, cmText, params.Position.Line, params.Position.Character)
	}

	mapped := mapCompletionResultToCM(result, lm, cmPath, cmText, params.Position.Line, params.Position.Character)
	mapped = mergeCompletionItems(mapped, cmItems)
	out, err := json.Marshal(mapped)
	if err != nil {
		return s.writeError(msg.ID, -32002, err.Error())
	}
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: out})
}

func mapCompletionResultToCM(v any, lm *lineMapper, cmPath, cmText string, cmLine, cmChar int) any {
	switch vv := v.(type) {
	case map[string]any:
		// CompletionList: {isIncomplete, items}
		if items, ok := vv["items"].([]any); ok {
			for i := range items {
				items[i] = mapCompletionItemToCM(items[i], lm, cmPath, cmText, cmLine, cmChar)
			}
			vv["items"] = items
		}
		return vv
	case []any:
		for i := range vv {
			vv[i] = mapCompletionItemToCM(vv[i], lm, cmPath, cmText, cmLine, cmChar)
		}
		return vv
	default:
		return v
	}
}

func mapCompletionItemToCM(item any, lm *lineMapper, cmPath, cmText string, cmLine, cmChar int) any {
	m, ok := item.(map[string]any)
	if !ok {
		return item
	}

	// If clangd supplies a textEdit, map its range back to .cm.
	if te, ok := m["textEdit"].(map[string]any); ok {
		m["textEdit"] = mapTextEditToCM(te, lm, cmPath, cmText, cmLine, cmChar)
	}
	if te, ok := m["textEdit"].(map[string]any); ok {
		// InsertReplaceEdit: {newText, insert, replace}
		if _, hasInsert := te["insert"]; hasInsert {
			m["textEdit"] = mapInsertReplaceEditToCM(te, lm, cmPath, cmText, cmLine, cmChar)
		}
	}

	if ates, ok := m["additionalTextEdits"].([]any); ok {
		for i := range ates {
			edit, ok := ates[i].(map[string]any)
			if !ok {
				continue
			}
			ates[i] = mapTextEditToCM(edit, lm, cmPath, cmText, cmLine, cmChar)
		}
		m["additionalTextEdits"] = ates
	}

	return m
}
