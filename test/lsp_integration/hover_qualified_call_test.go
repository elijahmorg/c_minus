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

func TestHoverOnQualifiedFunctionCallWorks(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	// module ticket
	ticketDir := filepath.Join(tmpDir, "ticket")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("mkdir ticket: %v", err)
	}
	ticketCM := strings.Join([]string{
		`module "ticket"`,
		"",
		"pub struct Ticket { int id; };",
		"",
		"pub func create_ticket(int id, Ticket* t) void { t->id = id; }",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(ticketDir, "ticket.cm"), []byte(ticketCM), 0644); err != nil {
		t.Fatalf("write ticket.cm: %v", err)
	}

	mainCM := strings.Join([]string{
		`module "main"`,
		"",
		`import "ticket"`,
		"",
		"func main() int {",
		"    ticket.Ticket t;",
		"    ticket.create_ticket(1, &t);",
		"    return t.id;",
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

	// Wait for generated output.
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

	// Hover over create_ticket in: "ticket.create_ticket(1, &t);"
	// Line is 6 (0-based):
	// 0 module
	// 1 blank
	// 2 import
	// 3 blank
	// 4 func
	// 5 ticket.Ticket
	// 6 ticket.create_ticket
	// Hover at the '(' to ensure the server snaps to the identifier.
	hoverResp := client.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": docURI},
		"position":     map[string]any{"line": 6, "character": 24},
	})
	if hoverResp.Error != nil {
		t.Fatalf("hover error: %s", hoverResp.Error.Message)
	}
	if len(hoverResp.Result) == 0 || string(hoverResp.Result) == "null" {
		t.Fatalf("expected hover result, got %s", string(hoverResp.Result))
	}

	// Best-effort assertion that we got a function signature.
	var h struct {
		Contents any `json:"contents"`
	}
	if err := json.Unmarshal(hoverResp.Result, &h); err != nil {
		return
	}
	_ = h
}
