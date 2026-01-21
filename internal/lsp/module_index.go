package lsp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/project"
)

type symbolKind string

const (
	symbolKindFunc    symbolKind = "func"
	symbolKindStruct  symbolKind = "struct"
	symbolKindUnion   symbolKind = "union"
	symbolKindEnum    symbolKind = "enum"
	symbolKindTypedef symbolKind = "typedef"
	symbolKindGlobal  symbolKind = "global"
	symbolKindDefine  symbolKind = "define"
)

type cmSymbol struct {
	Name      string
	Kind      symbolKind
	File      string
	Line1     int // 1-based
	Char0     int // 0-based best-effort
	Public    bool
	Doc       string
	Signature string
}

type moduleIndex struct {
	Modules map[string][]cmSymbol // importPath -> symbols
}

func buildModuleIndex(proj *project.Project, openDocs map[string]string) (*moduleIndex, error) {
	idx := &moduleIndex{Modules: make(map[string][]cmSymbol)}

	for importPath, mod := range proj.Modules {
		for _, fpath := range mod.Files {
			content, ok := openDocs[fpath]
			var pf *parser.File
			var err error
			if ok {
				pf, err = parser.ParseSource(content, fpath)
			} else {
				pf, err = parser.ParseFile(fpath)
			}
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", fpath, err)
			}

			syms, err := symbolsFromParsedFile(pf, fpath, content)
			if err != nil {
				return nil, err
			}
			idx.Modules[importPath] = append(idx.Modules[importPath], syms...)
		}
	}

	return idx, nil
}

func symbolsFromParsedFile(pf *parser.File, filePath string, inMemory string) ([]cmSymbol, error) {
	var src string
	if inMemory != "" {
		src = inMemory
	} else {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		src = string(b)
	}

	lines := splitLinesPreserve(src)

	findLineChar := func(line1 int, needle string) (int, int) {
		if line1 <= 0 || line1 > len(lines) {
			return line1, 0
		}
		idx := indexOfIdentifier(lines[line1-1], needle)
		if idx < 0 {
			return line1, 0
		}
		return line1, idx
	}

	var out []cmSymbol
	for _, d := range pf.Decls {
		switch {
		case d.Function != nil:
			line1, ch0 := findLineChar(d.Function.Line, d.Function.Name)
			sig := formatFuncSignature(d.Function)
			out = append(out, cmSymbol{Name: d.Function.Name, Kind: symbolKindFunc, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Function.Public, Doc: d.Function.DocComment, Signature: sig})
		case d.Struct != nil:
			line1, ch0 := findDeclLineChar(lines, "struct", d.Struct.Name)
			out = append(out, cmSymbol{Name: d.Struct.Name, Kind: symbolKindStruct, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Struct.Public, Doc: d.Struct.DocComment, Signature: "struct " + d.Struct.Name})
		case d.Union != nil:
			line1, ch0 := findDeclLineChar(lines, "union", d.Union.Name)
			out = append(out, cmSymbol{Name: d.Union.Name, Kind: symbolKindUnion, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Union.Public, Doc: d.Union.DocComment, Signature: "union " + d.Union.Name})
		case d.Enum != nil:
			line1, ch0 := findDeclLineChar(lines, "enum", d.Enum.Name)
			out = append(out, cmSymbol{Name: d.Enum.Name, Kind: symbolKindEnum, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Enum.Public, Doc: d.Enum.DocComment, Signature: "enum " + d.Enum.Name})
		case d.Typedef != nil:
			// Best-effort: find the typedef name by scanning for "typedef" and taking the last identifier.
			name, line1, ch0 := findTypedefName(lines)
			if name != "" {
				out = append(out, cmSymbol{Name: name, Kind: symbolKindTypedef, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Typedef.Public, Doc: d.Typedef.DocComment, Signature: "typedef " + name})
			}
		case d.Global != nil:
			line1, ch0 := findLineChar(d.Global.Line, d.Global.Name)
			out = append(out, cmSymbol{Name: d.Global.Name, Kind: symbolKindGlobal, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Global.Public, Doc: d.Global.DocComment, Signature: d.Global.Type + " " + d.Global.Name})
		case d.Define != nil:
			line1, ch0 := findDeclLineChar(lines, "#define", d.Define.Name)
			out = append(out, cmSymbol{Name: d.Define.Name, Kind: symbolKindDefine, File: filepath.Clean(filePath), Line1: line1, Char0: ch0, Public: d.Define.Public, Doc: d.Define.DocComment, Signature: "#define " + d.Define.Name})
		}
	}

	return out, nil
}

func splitLinesPreserve(s string) []string {
	// strings.Split drops final empty line, but that's OK for our use.
	// Use a tiny helper to avoid importing strings in many files.
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, trimCR(s[start:]))
	}
	return lines
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func findDeclLineChar(lines []string, keyword, name string) (line1 int, ch0 int) {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// very basic match
		if indexOfSubstring(line, keyword) >= 0 && indexOfIdentifier(line, name) >= 0 {
			return i + 1, indexOfIdentifier(line, name)
		}
	}
	return 1, 0
}

func findTypedefName(lines []string) (name string, line1 int, ch0 int) {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if indexOfSubstring(line, "typedef") < 0 {
			continue
		}
		// Grab last identifier on the line.
		name, pos := lastIdentifier(line)
		if name == "" {
			continue
		}
		return name, i + 1, pos
	}
	return "", 1, 0
}

func indexOfSubstring(haystack, needle string) int {
	// naive
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func indexOfIdentifier(line, ident string) int {
	if ident == "" {
		return -1
	}
	for i := 0; i+len(ident) <= len(line); i++ {
		if line[i:i+len(ident)] != ident {
			continue
		}
		beforeOK := i == 0 || !isIdentChar(line[i-1])
		afterOK := i+len(ident) == len(line) || !isIdentChar(line[i+len(ident)])
		if beforeOK && afterOK {
			return i
		}
	}
	return -1
}

func lastIdentifier(line string) (string, int) {
	end := -1
	for i := len(line) - 1; i >= 0; i-- {
		if isIdentChar(line[i]) {
			end = i
			break
		}
	}
	if end < 0 {
		return "", -1
	}
	start := end
	for start >= 0 && isIdentChar(line[start]) {
		start--
	}
	start++
	return line[start : end+1], start
}
