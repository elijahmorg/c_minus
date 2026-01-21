package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type clangdProxy struct {
	rootPath string
	buildDir string

	cmd  *exec.Cmd
	conn *jsonrpcConn

	readLoopDone chan struct{}

	mu      sync.Mutex
	nextID  int
	pending map[string]chan jsonrpcMessage

	onNotification func(jsonrpcMessage)
}

func newClangdProxy(rootPath, buildDir string) *clangdProxy {
	return &clangdProxy{
		rootPath: rootPath,
		buildDir: buildDir,
		nextID:   1,
		pending:  make(map[string]chan jsonrpcMessage),
	}
}

func (p *clangdProxy) start(ctx context.Context) error {
	if p.cmd != nil {
		return nil
	}

	p.cmd = exec.CommandContext(ctx, "clangd",
		"--compile-commands-dir="+p.buildDir,
		"--background-index",
		"--log=error",
	)
	p.cmd.Dir = p.rootPath

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.cmd.Stderr = io.Discard

	if err := p.cmd.Start(); err != nil {
		return err
	}

	p.conn = newJSONRPCConn(stdout, stdin)
	p.readLoopDone = make(chan struct{})
	go p.readLoop()

	return nil
}

func (p *clangdProxy) stop() error {
	if p.cmd == nil {
		return nil
	}

	_ = p.cmd.Process.Kill()
	<-p.readLoopDone
	p.cmd = nil
	return nil
}

func (p *clangdProxy) readLoop() {
	defer close(p.readLoopDone)

	for {
		msg, err := p.conn.readMessage()
		if err != nil {
			return
		}

		if msg.Method != "" {
			if p.onNotification != nil {
				p.onNotification(msg)
			}
			continue
		}

		if len(msg.ID) == 0 {
			continue
		}

		key := jsonrpcIDKey(msg.ID)
		p.mu.Lock()
		ch := p.pending[key]
		delete(p.pending, key)
		p.mu.Unlock()

		if ch != nil {
			ch <- msg
		}
	}
}

func (p *clangdProxy) notify(method string, params any) error {
	payload := jsonrpcMessage{JSONRPC: "2.0", Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		payload.Params = b
	}
	return p.conn.writeMessage(payload)
}

func (p *clangdProxy) request(ctx context.Context, method string, params any, out any) error {
	p.mu.Lock()
	id := p.nextID
	p.nextID++
	ch := make(chan jsonrpcMessage, 1)
	p.pending[fmt.Sprintf("%d", id)] = ch
	p.mu.Unlock()

	payload := jsonrpcMessage{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf("%d", id)), Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		payload.Params = b
	}
	if err := p.conn.writeMessage(payload); err != nil {
		return err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("clangd error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(resp.Result, out)
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return fmt.Errorf("clangd request timeout: %s", method)
	}
}

func (p *clangdProxy) initialize(ctx context.Context, rootURI string) error {
	var result any
	initParams := map[string]any{
		"processId":    nil,
		"rootUri":      rootURI,
		"capabilities": map[string]any{},
	}
	if err := p.request(ctx, "initialize", initParams, &result); err != nil {
		return err
	}
	return p.notify("initialized", map[string]any{})
}
