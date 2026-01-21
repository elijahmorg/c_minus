package lsp_integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type lspClient struct {
	t      *testing.T
	r      *bufio.Reader
	w      io.Writer
	mu     sync.Mutex
	nextID int

	pendingMu sync.Mutex
	pending   map[string]chan jsonrpcMessage

	notifs chan jsonrpcMessage
}

func newLSPClient(t *testing.T, r io.Reader, w io.Writer) *lspClient {
	c := &lspClient{
		t:       t,
		r:       bufio.NewReader(r),
		w:       w,
		nextID:  1,
		pending: make(map[string]chan jsonrpcMessage),
		notifs:  make(chan jsonrpcMessage, 64),
	}
	go c.readLoop()
	return c
}

func (c *lspClient) readLoop() {
	for {
		msg, err := c.readMessage()
		if err != nil {
			return
		}

		if msg.Method != "" && len(msg.ID) == 0 {
			c.notifs <- msg
			continue
		}

		if len(msg.ID) > 0 {
			key := strings.TrimSpace(string(msg.ID))
			c.pendingMu.Lock()
			ch := c.pending[key]
			delete(c.pending, key)
			c.pendingMu.Unlock()
			if ch != nil {
				ch <- msg
			}
		}
	}
}

func (c *lspClient) readMessage() (jsonrpcMessage, error) {
	var msg jsonrpcMessage

	contentLen := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return msg, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if strings.EqualFold(key, "Content-Length") {
			n, err := strconv.Atoi(val)
			if err != nil {
				return msg, err
			}
			contentLen = n
		}
	}

	if contentLen < 0 {
		return msg, fmt.Errorf("missing Content-Length")
	}

	payload := make([]byte, contentLen)
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return msg, err
	}

	if err := json.Unmarshal(payload, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func (c *lspClient) writeMessage(msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("marshal json: %v", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(b)))
	buf.Write(b)
	if _, err := c.w.Write(buf.Bytes()); err != nil {
		c.t.Fatalf("write json-rpc: %v", err)
	}
}

func (c *lspClient) request(method string, params any) jsonrpcMessage {
	c.pendingMu.Lock()
	id := c.nextID
	c.nextID++
	key := fmt.Sprintf("%d", id)
	ch := make(chan jsonrpcMessage, 1)
	c.pending[key] = ch
	c.pendingMu.Unlock()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	c.writeMessage(payload)

	select {
	case resp := <-ch:
		return resp
	case <-time.After(10 * time.Second):
		c.t.Fatalf("timeout waiting for response to %s", method)
		return jsonrpcMessage{}
	}
}

func (c *lspClient) notify(method string, params any) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	c.writeMessage(payload)
}

func (c *lspClient) waitForNotification(method string, timeout time.Duration) jsonrpcMessage {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case msg := <-c.notifs:
			if msg.Method == method {
				return msg
			}
		case <-deadline.C:
			c.t.Fatalf("timeout waiting for notification %s", method)
			return jsonrpcMessage{}
		}
	}
}

func (c *lspClient) waitForDiagnostics(uri string, contains string, timeout time.Duration) jsonrpcMessage {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case msg := <-c.notifs:
			if msg.Method != "textDocument/publishDiagnostics" {
				continue
			}

			var params struct {
				URI         string `json:"uri"`
				Diagnostics []struct {
					Message string `json:"message"`
				} `json:"diagnostics"`
			}
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				continue
			}
			if params.URI != uri {
				continue
			}
			if contains == "" {
				return msg
			}
			for _, d := range params.Diagnostics {
				if strings.Contains(d.Message, contains) {
					return msg
				}
			}
		case <-deadline.C:
			c.t.Fatalf("timeout waiting for diagnostics on %s containing %q", uri, contains)
			return jsonrpcMessage{}
		}
	}
}

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("clangd"); err != nil {
		fmt.Fprintf(os.Stderr, "clangd is required for LSP integration tests: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func findLSPBinary(t *testing.T) string {
	t.Helper()

	// Always build: the repo may contain a stale c_minus_lsp binary.
	projectRoot := filepath.Join("..", "..")
	outPath := filepath.Join(t.TempDir(), "c_minus_lsp")

	buildCmd := exec.Command("go", "build", "-o", outPath, "./cmd/c_minus_lsp")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build c_minus_lsp: %v\nOutput: %s", err, output)
	}

	abs, _ := filepath.Abs(outPath)
	return abs
}

func fileURIForPath(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	abs = filepath.ToSlash(abs)
	return (&url.URL{Scheme: "file", Path: abs}).String()
}

func TestDiagnosticsAreSourceMappedToCM(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "cm.mod"), []byte(`module "test/lsp"`), 0644); err != nil {
		t.Fatalf("write cm.mod: %v", err)
	}

	// Create a file that will generate a C-level error inside a function body.
	mainCM := "module \"main\"\n\nfunc main() int {\n    return does_not_exist;\n}\n"
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

	n := client.waitForDiagnostics(docURI, "does_not_exist", 20*time.Second)
	var params struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(n.Params, &params); err != nil {
		t.Fatalf("unmarshal diagnostics params: %v", err)
	}

	if params.URI != docURI {
		t.Fatalf("expected diagnostics for %s, got %s", docURI, params.URI)
	}

	found := false
	for _, d := range params.Diagnostics {
		if strings.Contains(d.Message, "does_not_exist") {
			// The identifier is on line 4 (1-based) in the .cm; LSP uses 0-based lines.
			if d.Range.Start.Line != 3 {
				t.Fatalf("expected diagnostic line 3 (0-based), got %d; message: %s", d.Range.Start.Line, d.Message)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic mentioning does_not_exist; got %d diagnostics", len(params.Diagnostics))
	}
}
