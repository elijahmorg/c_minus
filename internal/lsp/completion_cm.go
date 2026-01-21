package lsp

import (
	"github.com/elijahmorgan/c_minus/internal/project"
)

type cmCompletionContext struct {
	InImportString bool
	MemberModule   string // if completing after `mod.`
}

func completionContext(cmText string, line0, char0 int) cmCompletionContext {
	lines := splitLinesPreserve(cmText)
	if line0 < 0 || line0 >= len(lines) {
		return cmCompletionContext{}
	}
	line := lines[line0]
	if char0 < 0 {
		char0 = 0
	}
	if char0 > len(line) {
		char0 = len(line)
	}
	prefix := line[:char0]

	// import "...  (trigger includes the quote)
	if idx := indexOfSubstring(prefix, "import \""); idx >= 0 {
		// If there isn't a closing quote after idx, we're in the import string.
		after := prefix[idx+len("import \""):]
		if indexOfSubstring(after, "\"") < 0 {
			return cmCompletionContext{InImportString: true}
		}
	}

	// member completion: <ident>.
	if len(prefix) > 0 && prefix[len(prefix)-1] == '.' {
		name, _ := lastIdentifier(prefix[:len(prefix)-1])
		if name != "" {
			return cmCompletionContext{MemberModule: name}
		}
	}

	return cmCompletionContext{}
}

func cmCompletions(proj *project.Project, idx *moduleIndex, cmPath, cmText string, line0, char0 int) []any {
	ctx := completionContext(cmText, line0, char0)
	if ctx.InImportString {
		items := make([]any, 0, len(proj.Modules))
		for importPath := range proj.Modules {
			if importPath == "main" {
				continue
			}
			items = append(items, map[string]any{
				"label":      importPath,
				"kind":       9, // Module
				"insertText": importPath,
			})
		}
		return items
	}

	if ctx.MemberModule != "" {
		modPrefix := ctx.MemberModule

		imports := importedModulePrefixes(cmPath, cmText)
		targetImportPath, ok := imports[modPrefix]
		if !ok {
			// Not imported in this file; don't suggest module members.
			return nil
		}

		syms := idx.Modules[targetImportPath]
		items := make([]any, 0, len(syms))
		for _, s := range syms {
			if !s.Public {
				continue
			}
			kind := 6 // Variable
			switch s.Kind {
			case symbolKindFunc:
				kind = 3
			case symbolKindStruct, symbolKindUnion:
				kind = 22
			case symbolKindEnum:
				kind = 13
			case symbolKindTypedef:
				kind = 22
			case symbolKindDefine:
				kind = 21
			case symbolKindGlobal:
				kind = 6
			}
			items = append(items, map[string]any{
				"label":      s.Name,
				"kind":       kind,
				"insertText": s.Name,
			})
		}
		return items
	}

	return nil
}
