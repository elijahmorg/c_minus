package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

func filePathFromURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported uri scheme %q", u.Scheme)
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		// file:///C:/path => /C:/path
		path = strings.TrimPrefix(path, "/")
		path = strings.ReplaceAll(path, "/", "\\")
	}
	return filepath.Clean(path), nil
}

func fileURIFromPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.ToSlash(abs)

	u := url.URL{Scheme: "file", Path: abs}
	// url.URL will handle escaping and platform-specific formatting.
	return u.String(), nil
}
