package lsp

import "encoding/json"

func mergeCompletionItems(clangdResult any, extraItems []any) any {
	if len(extraItems) == 0 {
		return clangdResult
	}

	switch vv := clangdResult.(type) {
	case map[string]any:
		// CompletionList
		items, _ := vv["items"].([]any)
		vv["items"] = append(items, extraItems...)
		return vv
	case []any:
		return append(vv, extraItems...)
	default:
		// If clangd returned null/unknown, just return our items.
		return extraItems
	}
}

func mapTextEditToCM(edit map[string]any, lm *lineMapper, cmPath, cmText string, cmLine, cmChar int) map[string]any {
	rawRange, ok := edit["range"]
	if !ok {
		return edit
	}
	b, err := json.Marshal(rawRange)
	if err != nil {
		return forceInsertAt(edit, cmLine, cmChar)
	}
	var r lspRange
	if err := json.Unmarshal(b, &r); err != nil {
		return forceInsertAt(edit, cmLine, cmChar)
	}

	file, mapped, err := mapRangeCToCM(lm, r)
	if err != nil || file != cmPath {
		return forceInsertAt(edit, cmLine, cmChar)
	}

	mapped = clampRangeToLine(mapped, cmText)
	edit["range"] = mapped
	return edit
}

func mapInsertReplaceEditToCM(edit map[string]any, lm *lineMapper, cmPath, cmText string, cmLine, cmChar int) map[string]any {
	ins, ok1 := edit["insert"]
	rep, ok2 := edit["replace"]
	if !ok1 || !ok2 {
		return edit
	}

	b1, err1 := json.Marshal(ins)
	b2, err2 := json.Marshal(rep)
	if err1 != nil || err2 != nil {
		return forceInsertReplaceAt(edit, cmLine, cmChar)
	}
	var r1, r2 lspRange
	if json.Unmarshal(b1, &r1) != nil || json.Unmarshal(b2, &r2) != nil {
		return forceInsertReplaceAt(edit, cmLine, cmChar)
	}

	f1, mr1, err := mapRangeCToCM(lm, r1)
	if err != nil || f1 != cmPath {
		return forceInsertReplaceAt(edit, cmLine, cmChar)
	}
	f2, mr2, err := mapRangeCToCM(lm, r2)
	if err != nil || f2 != cmPath {
		return forceInsertReplaceAt(edit, cmLine, cmChar)
	}

	mr1 = clampRangeToLine(mr1, cmText)
	mr2 = clampRangeToLine(mr2, cmText)
	edit["insert"] = mr1
	edit["replace"] = mr2
	return edit
}

func forceInsertAt(edit map[string]any, line, char int) map[string]any {
	edit["range"] = map[string]any{
		"start": map[string]any{"line": line, "character": char},
		"end":   map[string]any{"line": line, "character": char},
	}
	return edit
}

func forceInsertReplaceAt(edit map[string]any, line, char int) map[string]any {
	r := map[string]any{
		"start": map[string]any{"line": line, "character": char},
		"end":   map[string]any{"line": line, "character": char},
	}
	edit["insert"] = r
	edit["replace"] = r
	return edit
}

func clampRangeToLine(r lspRange, cmText string) lspRange {
	lines := splitLinesPreserve(cmText)
	clamp := func(line, char int) int {
		if line < 0 || line >= len(lines) {
			return char
		}
		if char < 0 {
			return 0
		}
		if char > len(lines[line]) {
			return len(lines[line])
		}
		return char
	}

	r.Start.Character = clamp(r.Start.Line, r.Start.Character)
	r.End.Character = clamp(r.End.Line, r.End.Character)
	return r
}
