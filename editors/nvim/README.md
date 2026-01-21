Neovim C-minus LSP

This directory contains a minimal Neovim config snippet for using the C-minus LSP.

Binary path is hard-coded to the repo-local build output:

- /Users/elijahmorgan/LocalDocs/projects/c_plus/c_minus_lsp

Usage

1) Build the LSP binary (from repo root):

   go build -o c_minus_lsp ./cmd/c_minus_lsp

2) Source the config from your Neovim config:

   dofile("/Users/elijahmorgan/LocalDocs/projects/c_plus/editors/nvim/init.lua")

Notes

- The server requires clangd to be installed.
- The server uses cm.mod as the project root marker.
