package lsp

import (
	"strings"

	"github.com/elijahmorgan/c_minus/internal/parser"
)

func formatFuncSignature(fn *parser.FuncDecl) string {
	if fn == nil {
		return ""
	}

	var b strings.Builder
	if fn.ReturnType != "" {
		b.WriteString(fn.ReturnType)
		b.WriteByte(' ')
	}
	b.WriteString(fn.Name)
	b.WriteByte('(')
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		// C-minus stores params as {Type, Name}
		if p.Type != "" {
			b.WriteString(p.Type)
		}
		if p.Name != "" {
			if p.Type != "" {
				b.WriteByte(' ')
			}
			b.WriteString(p.Name)
		}
	}
	b.WriteByte(')')
	return b.String()
}
