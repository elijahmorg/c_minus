package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/elijahmorgan/c_minus/internal/paths"
	"github.com/elijahmorgan/c_minus/internal/project"
)

type server struct {
	conn *jsonrpcConn

	rootURI  string
	rootPath string
	buildDir string

	clangd *clangdProxy

	mu          sync.Mutex
	openDocs    map[string]string // absolute path -> full text
	openedCDocs map[string]int    // c file absolute path -> version

	lineMapsMu sync.Mutex
	lineMaps   map[string]*lineMapper // c file absolute path -> mapper
}

func Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s := &server{
		conn:        newJSONRPCConn(in, out),
		openDocs:    make(map[string]string),
		openedCDocs: make(map[string]int),
		lineMaps:    make(map[string]*lineMapper),
	}

	for {
		msg, err := s.conn.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if msg.Method == "" {
			// We currently only expect requests/notifications from the client.
			continue
		}

		if len(msg.ID) > 0 {
			if err := s.handleRequest(ctx, msg); err != nil {
				return err
			}
			continue
		}

		if err := s.handleNotification(ctx, msg); err != nil {
			return err
		}
	}
}

func (s *server) handleRequest(ctx context.Context, msg jsonrpcMessage) error {
	key := jsonrpcIDKey(msg.ID)
	_ = key

	switch msg.Method {
	case "initialize":
		var params struct {
			RootURI string `json:"rootUri"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		if params.RootURI == "" {
			return s.writeError(msg.ID, -32602, "missing rootUri")
		}
		rootPath, err := filePathFromURI(params.RootURI)
		if err != nil {
			return s.writeError(msg.ID, -32602, fmt.Sprintf("invalid rootUri: %v", err))
		}

		s.rootURI = params.RootURI
		s.rootPath = rootPath

		buildDir := filepath.Join(rootPath, ".c_minus")
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return s.writeError(msg.ID, -32002, fmt.Sprintf("failed to create build dir: %v", err))
		}
		s.buildDir = buildDir

		s.clangd = newClangdProxy(rootPath, buildDir)
		s.clangd.onNotification = s.onClangdNotification
		if err := s.clangd.start(ctx); err != nil {
			return s.writeError(msg.ID, -32002, fmt.Sprintf("failed to start clangd: %v", err))
		}
		if err := s.clangd.initialize(ctx, s.rootURI); err != nil {
			return s.writeError(msg.ID, -32002, fmt.Sprintf("failed to initialize clangd: %v", err))
		}

		result := map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": map[string]any{
					"openClose": true,
					"change":    1, // Full
				},
				"hoverProvider":           true,
				"definitionProvider":      true,
				"referencesProvider":      true,
				"renameProvider":          map[string]any{"prepareProvider": true},
				"documentSymbolProvider":  true,
				"workspaceSymbolProvider": true,
				"completionProvider": map[string]any{
					"resolveProvider":   false,
					"triggerCharacters": []string{".", ">", ":", "\""},
				},
			},
		}
		return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: mustJSON(result)})

	case "shutdown":
		if s.clangd != nil {
			_ = s.clangd.notify("shutdown", nil)
			_ = s.clangd.stop()
		}
		return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")})

	case "textDocument/hover":
		return s.forwardHover(ctx, msg)
	case "textDocument/definition":
		return s.forwardDefinition(ctx, msg)
	case "textDocument/references":
		return s.forwardReferences(ctx, msg)
	case "textDocument/completion":
		return s.forwardCompletion(ctx, msg)
	case "textDocument/documentSymbol":
		return s.documentSymbols(ctx, msg)
	case "workspace/symbol":
		return s.workspaceSymbols(ctx, msg)
	case "textDocument/prepareRename":
		return s.prepareRename(ctx, msg)
	case "textDocument/rename":
		return s.rename(ctx, msg)
	default:
		// Method not supported yet.
		return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: msg.ID, Error: &jsonrpcError{Code: -32601, Message: "method not found"}})
	}
}

func (s *server) handleNotification(ctx context.Context, msg jsonrpcMessage) error {
	switch msg.Method {
	case "exit":
		return io.EOF

	case "initialized":
		return nil

	case "textDocument/didOpen":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Text    string `json:"text"`
				Version int    `json:"version"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		cmPath, err := filePathFromURI(params.TextDocument.URI)
		if err != nil {
			return err
		}
		cmPath, err = filepath.Abs(cmPath)
		if err != nil {
			return err
		}

		s.mu.Lock()
		s.openDocs[cmPath] = params.TextDocument.Text
		s.mu.Unlock()

		return s.refreshFile(ctx, cmPath)

	case "textDocument/didChange":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		if len(params.ContentChanges) == 0 {
			return nil
		}
		cmPath, err := filePathFromURI(params.TextDocument.URI)
		if err != nil {
			return err
		}
		cmPath, err = filepath.Abs(cmPath)
		if err != nil {
			return err
		}

		s.mu.Lock()
		s.openDocs[cmPath] = params.ContentChanges[len(params.ContentChanges)-1].Text
		s.mu.Unlock()

		return s.refreshFile(ctx, cmPath)

	case "textDocument/didClose":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		cmPath, err := filePathFromURI(params.TextDocument.URI)
		if err != nil {
			return err
		}
		cmPath, err = filepath.Abs(cmPath)
		if err != nil {
			return err
		}

		s.mu.Lock()
		delete(s.openDocs, cmPath)
		s.mu.Unlock()

		// Best-effort: clear diagnostics for closed file.
		_ = s.publishDiagnostics(cmPath, nil)
		return nil
	}

	return nil
}

func (s *server) refreshFile(ctx context.Context, cmPath string) error {
	proj, err := project.Discover(filepath.Dir(cmPath))
	if err != nil {
		return s.publishParserError(cmPath, err)
	}

	s.mu.Lock()
	openDocsCopy := make(map[string]string, len(s.openDocs))
	for k, v := range s.openDocs {
		openDocsCopy[k] = v
	}
	s.mu.Unlock()

	buildDir, err := transpileWorkspace(proj, openDocsCopy)
	if err != nil {
		return s.publishParserError(cmPath, err)
	}
	s.buildDir = buildDir

	// Open/update the generated C file in clangd with the generated content.
	modPath, err := projectModuleImportPath(proj, cmPath)
	if err != nil {
		return s.publishParserError(cmPath, err)
	}
	cPath := generatedCPath(proj.RootPath, modPath, filepath.Base(cmPath))

	// Invalidate any cached line map for this generated file.
	s.lineMapsMu.Lock()
	delete(s.lineMaps, cPath)
	s.lineMapsMu.Unlock()

	cURI, err := fileURIFromPath(cPath)
	if err != nil {
		return err
	}
	cText, err := os.ReadFile(cPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	ver, alreadyOpen := s.openedCDocs[cPath]
	if alreadyOpen {
		ver++
		s.openedCDocs[cPath] = ver
	}
	s.mu.Unlock()

	if !alreadyOpen {
		s.mu.Lock()
		s.openedCDocs[cPath] = 1
		s.mu.Unlock()

		_ = s.clangd.notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri":        cURI,
				"languageId": "c",
				"version":    1,
				"text":       string(cText),
			},
		})

		// Clear any previous diagnostics for this .cm file.
		_ = s.publishDiagnostics(cmPath, nil)
		return nil
	}

	// Clear any previous diagnostics for this .cm file.
	_ = s.publishDiagnostics(cmPath, nil)

	return s.clangd.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     cURI,
			"version": ver,
		},
		"contentChanges": []any{map[string]any{"text": string(cText)}},
	})
}

func projectModuleImportPath(proj *project.Project, cmPath string) (string, error) {
	rel, err := filepath.Rel(proj.RootPath, filepath.Dir(cmPath))
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "main", nil
	}
	return rel, nil
}

func generatedCPath(rootPath, importPath, cmBase string) string {
	buildDir := filepath.Join(rootPath, ".c_minus")
	return paths.ModuleCFilePath(buildDir, importPath, cmBase)
}

func (s *server) publishDiagnostics(cmPath string, diags []any) error {
	uri, err := fileURIFromPath(cmPath)
	if err != nil {
		return err
	}
	if diags == nil {
		diags = []any{}
	}
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: mustJSON(map[string]any{"uri": uri, "diagnostics": diags})})
}

func (s *server) publishParserError(cmPath string, err error) error {
	diag := map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 0, "character": 1},
		},
		"severity": 1,
		"source":   "c_minus",
		"message":  err.Error(),
	}
	return s.publishDiagnostics(cmPath, []any{diag})
}

func (s *server) writeError(id json.RawMessage, code int, msg string) error {
	return s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Error: &jsonrpcError{Code: code, Message: msg}})
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *server) onClangdNotification(msg jsonrpcMessage) {
	if msg.Method != "textDocument/publishDiagnostics" {
		return
	}

	var params struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
				End struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"end"`
			} `json:"range"`
			Severity int    `json:"severity,omitempty"`
			Source   string `json:"source,omitempty"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	_ = json.Unmarshal(msg.Params, &params)

	cPath, err := filePathFromURI(params.URI)
	if err != nil {
		return
	}
	cPath, err = filepath.Abs(cPath)
	if err != nil {
		return
	}

	lm, err := s.getLineMapperForCFile(cPath)
	if err != nil {
		return
	}

	byURI := make(map[string][]any)
	for _, d := range params.Diagnostics {
		origFile, origLine1 := lm.mapLine(d.Range.Start.Line + 1)
		if origFile == "" {
			continue
		}
		if filepath.Ext(origFile) != ".cm" {
			continue
		}
		cmURI, err := fileURIFromPath(origFile)
		if err != nil {
			continue
		}

		mapped := map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": origLine1 - 1, "character": d.Range.Start.Character},
				"end":   map[string]any{"line": origLine1 - 1, "character": d.Range.End.Character},
			},
			"severity": d.Severity,
			"source":   "clangd",
			"message":  d.Message,
		}
		byURI[cmURI] = append(byURI[cmURI], mapped)
	}

	for uri, diags := range byURI {
		_ = s.conn.writeMessage(jsonrpcMessage{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: mustJSON(map[string]any{"uri": uri, "diagnostics": diags})})
	}
}

func (s *server) getLineMapperForCFile(cPath string) (*lineMapper, error) {
	s.lineMapsMu.Lock()
	defer s.lineMapsMu.Unlock()

	if lm, ok := s.lineMaps[cPath]; ok {
		return lm, nil
	}

	f, err := os.Open(cPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lm, err := newLineMapperFromC(f)
	if err != nil {
		return nil, err
	}
	s.lineMaps[cPath] = lm
	return lm, nil
}
