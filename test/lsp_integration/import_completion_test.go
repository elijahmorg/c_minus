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

func TestModuleMemberCompletionRequiresImport(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("mkdir math: %v", err)
	}
	mathCM := "module \"math\"\n\npub func add(int a, int b) int {\n    return a + b;\n}\n"
	if err := os.WriteFile(filepath.Join(mathDir, "math.cm"), []byte(mathCM), 0644); err != nil {
		t.Fatalf("write math.cm: %v", err)
	}

	mainCM := strings.Join([]string{
		"module \"main\"",
		"",
		"func main() int {",
		"    math.",
		"    return 0;",
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

	// Completion after math.
	compResp := client.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 3, "character": 9},
	})
	if compResp.Error != nil {
		t.Fatalf("completion error: %s", compResp.Error.Message)
	}

	// Decode items and ensure we DO NOT get "add" because math isn't imported.
	items := extractCompletionLabels(t, compResp.Result)
	for _, it := range items {
		if it == "add" {
			t.Fatalf("did not expect module member completion without import")
		}
	}
}

func extractCompletionLabels(t *testing.T, raw json.RawMessage) []string {
	t.Helper()

	// handle CompletionList or []CompletionItem
	var asList struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &asList); err == nil && asList.Items != nil {
		out := make([]string, 0, len(asList.Items))
		for _, it := range asList.Items {
			out = append(out, it.Label)
		}
		return out
	}

	var asArr []struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(raw, &asArr); err == nil {
		out := make([]string, 0, len(asArr))
		for _, it := range asArr {
			out = append(out, it.Label)
		}
		return out
	}

	return nil
}
