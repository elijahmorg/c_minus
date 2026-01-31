package tree_sitter_c_minus_test

import (
	"testing"

	tree_sitter_c_minus "github.com/elijahmorgan/c_plus/treesitter/tree-sitter-cminus/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestCanLoadGrammar(t *testing.T) {
	language := tree_sitter.NewLanguage(tree_sitter_c_minus.Language())
	if language == nil {
		t.Errorf("Error loading C-minus grammar")
	}
}
