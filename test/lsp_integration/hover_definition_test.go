package lsp_integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHoverAndDefinitionAreSourceMappedToCM(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	mainCM := strings.Join([]string{
		`module "main"`,
		"",
		"func main() int {",
		"    int x = 1;",
		"    return x;",
		"}",
		"",
	}, "\n")
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

	// Wait for the generated C file to exist so clangd has something to index.
	// clangd doesn't always emit diagnostics for clean code, so don't wait on publishDiagnostics here.
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

	// Hover over the 'x' in 'return x;'
	hoverResp := client.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 4, "character": 11},
	})
	if hoverResp.Error != nil {
		t.Fatalf("hover error: %s", hoverResp.Error.Message)
	}
	if len(hoverResp.Result) == 0 || string(hoverResp.Result) == "null" {
		t.Fatalf("expected hover result")
	}

	// Definition on 'x' should point back to the declaration line.
	defResp := client.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 4, "character": 11},
	})
	if defResp.Error != nil {
		t.Fatalf("definition error: %s", defResp.Error.Message)
	}

	var locs []struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(defResp.Result, &locs); err != nil {
		t.Fatalf("unmarshal definition result: %v; raw=%s", err, string(defResp.Result))
	}
	if len(locs) == 0 {
		t.Fatalf("expected at least one definition location")
	}

	if locs[0].URI != docURI {
		t.Fatalf("expected definition uri %s, got %s", docURI, locs[0].URI)
	}

	// Declaration is on line 3 (0-based) for 'int x = 1;' (line 4 in file).
	if locs[0].Range.Start.Line != 3 {
		t.Fatalf("expected definition at line 3 (0-based), got %d", locs[0].Range.Start.Line)
	}
}
