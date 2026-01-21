package lsp_integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareRenameRejectsStringsAndComments(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	mainCM := "module \"main\"\n\nfunc main() int {\n    // hello comment\n    char* s = \"hello\";\n    return 0;\n}\n"
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

	// Position inside the comment (on the "hello") should return null.
	pr1 := client.request("textDocument/prepareRename", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 3, "character": 7},
	})
	if pr1.Error != nil {
		t.Fatalf("prepareRename error: %s", pr1.Error.Message)
	}
	if string(pr1.Result) != "null" {
		t.Fatalf("expected null prepareRename in comment, got %s", string(pr1.Result))
	}

	// Position inside the string literal should return null.
	pr2 := client.request("textDocument/prepareRename", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 4, "character": 17},
	})
	if pr2.Error != nil {
		t.Fatalf("prepareRename error: %s", pr2.Error.Message)
	}
	if string(pr2.Result) != "null" {
		t.Fatalf("expected null prepareRename in string, got %s", string(pr2.Result))
	}
}
