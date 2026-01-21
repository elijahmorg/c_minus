package main

import (
	"context"
	"log"
	"os"

	"github.com/elijahmorgan/c_minus/internal/lsp"
)

func main() {
	if err := lsp.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		// LSP servers typically log to stderr.
		log.Printf("c_minus_lsp failed: %v", err)
		os.Exit(1)
	}
}
