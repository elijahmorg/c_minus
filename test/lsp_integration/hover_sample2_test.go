package lsp_integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHoverInSample2OnQualifiedFunctionCall(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	sample2Root := filepath.Join(repoRoot, "sample2")

	mainPath := filepath.Join(sample2Root, "main.cm")
	mainBytes, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read sample2 main.cm: %v", err)
	}
	mainCM := string(mainBytes)

	lspBin := findLSPBinary(t)
	cmd := exec.Command(lspBin)
	cmd.Dir = sample2Root
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
	rootURI := fileURIForPath(t, sample2Root)
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

	// Wait for generated output.
	cPath := filepath.Join(sample2Root, ".c_minus", "main_main.c")
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

	// Hover at the '(' in ticket.create_ticket(...)
	// In sample2/main.cm, this is line 9 (0-based) and the '(' is around column 24.
	hoverResp := client.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 9, "character": 24},
	})
	if hoverResp.Error != nil {
		t.Fatalf("hover error: %s", hoverResp.Error.Message)
	}
	if len(hoverResp.Result) == 0 || string(hoverResp.Result) == "null" {
		t.Fatalf("expected hover result, got %s", string(hoverResp.Result))
	}
}
