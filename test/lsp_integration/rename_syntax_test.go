package lsp_integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenameDoesNotEditStringsOrComments(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	mainCM := strings.Join([]string{
		`module "main"`,
		"",
		"pub func hello() int {",
		"    return 1;",
		"}",
		"",
		"func main() int {",
		"    // hello should not be changed here",
		"    char* s = \"hello\";",
		"    return hello();",
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

	// Wait for generated output to exist.
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

	// Rename on the call site hello() (line 9)
	rn := client.request("textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 9, "character": 12},
		"newName":      "hi",
	})
	if rn.Error != nil {
		t.Fatalf("rename error: %s", rn.Error.Message)
	}

	// Ensure changes don't include edits where the newText contains a replacement inside comment/string.
	raw := string(rn.Result)
	if strings.Contains(raw, "hello should not") && strings.Contains(raw, "hi") {
		t.Fatalf("unexpected rename edit inside comment: %s", raw)
	}
	if strings.Contains(raw, "\\\"hi\\\"") {
		t.Fatalf("unexpected rename edit inside string literal: %s", raw)
	}
}
