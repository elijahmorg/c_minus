package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// jsonrpcMessage is a generic JSON-RPC 2.0 envelope.
//
// It supports requests, responses, and notifications.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type jsonrpcConn struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
}

func newJSONRPCConn(r io.Reader, w io.Writer) *jsonrpcConn {
	return &jsonrpcConn{
		r: bufio.NewReader(r),
		w: w,
	}
}

func (c *jsonrpcConn) readMessage() (jsonrpcMessage, error) {
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
				return msg, fmt.Errorf("invalid Content-Length %q: %w", val, err)
			}
			contentLen = n
		}
	}

	if contentLen < 0 {
		return msg, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLen)
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return msg, err
	}

	if err := json.Unmarshal(payload, &msg); err != nil {
		return msg, fmt.Errorf("invalid json-rpc payload: %w", err)
	}
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	return msg, nil
}

func (c *jsonrpcConn) writeMessage(msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(b)))
	buf.Write(b)
	_, err = c.w.Write(buf.Bytes())
	return err
}

func jsonrpcIDKey(id json.RawMessage) string {
	return string(bytes.TrimSpace(id))
}
