package lsp_integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDocumentSymbolsWorkspaceSymbolsAndRename(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	mainCM := "module \"main\"\n\n// hello adds\npub func hello(int a) int {\n    return a;\n}\n\nfunc main() int {\n    return hello(1);\n}\n"
	mainPath := filepath.Join(tmpDir, "main.cm")
	if err := os.WriteFile(mainPath, []byte(mainCM), 0644); err != nil {
		t.Fatalf("write main.cm: %v", err)
	}

	lspBin := findLSPBinary(t)
	cmd := exec.Command(lspBin)
	cmd.Dir = tmpDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start c_minus_lsp: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := newLSPClient(t, stdout, stdin)
	rootURI := fileURIForPath(t, tmpDir)
	initResp := client.request("initialize", map[string]any{"rootUri": rootURI, "capabilities": map[string]any{}})
	if initResp.Error != nil {
		t.Fatalf("initialize error: %s", initResp.Error.Message)
	}
	client.notify("initialized", map[string]any{})

	docURI := fileURIForPath(t, mainPath)
	client.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        docURI,
			"languageId": "cminus",
			"version":    1,
			"text":       mainCM,
		},
	})

	// Wait for generated file.
	cPath := filepath.Join(tmpDir, ".c_minus", "main_main.c")
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(cPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for generated file %s", cPath)
		}
		time.Sleep(25 * time.Millisecond)
	}

	// documentSymbol
	dsResp := client.request("textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
	})
	if dsResp.Error != nil {
		t.Fatalf("documentSymbol error: %s", dsResp.Error.Message)
	}
	var docSyms []map[string]any
	if err := json.Unmarshal(dsResp.Result, &docSyms); err != nil {
		t.Fatalf("unmarshal document symbols: %v", err)
	}
	if len(docSyms) == 0 {
		t.Fatalf("expected at least one document symbol")
	}

	// workspace/symbol
	wsResp := client.request("workspace/symbol", map[string]any{"query": "hello"})
	if wsResp.Error != nil {
		t.Fatalf("workspace/symbol error: %s", wsResp.Error.Message)
	}
	var wsSyms []map[string]any
	if err := json.Unmarshal(wsResp.Result, &wsSyms); err != nil {
		t.Fatalf("unmarshal workspace symbols: %v", err)
	}
	if len(wsSyms) == 0 {
		t.Fatalf("expected at least one workspace symbol")
	}

	// rename: rename hello -> hi
	rnResp := client.request("textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 7, "character": 12}, // on hello(1)
		"newName":      "hi",
	})
	if rnResp.Error != nil {
		t.Fatalf("rename error: %s", rnResp.Error.Message)
	}

	var edit struct {
		Changes map[string][]struct {
			NewText string `json:"newText"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(rnResp.Result, &edit); err != nil {
		t.Fatalf("unmarshal rename result: %v", err)
	}
	if len(edit.Changes) == 0 {
		t.Fatalf("expected rename changes")
	}
	if _, ok := edit.Changes[docURI]; !ok {
		t.Fatalf("expected rename changes for %s", docURI)
	}
}
