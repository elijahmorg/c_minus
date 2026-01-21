package lsp

// This file provides a tiny lexer used by rename/completion helpers.
// It is intentionally minimal and only supports enough correctness
// to avoid editing inside comments/strings.

func isInStringOrComment(src string, line0, char0 int) bool {
	lines := splitLinesPreserve(src)
	if line0 < 0 || line0 >= len(lines) {
		return false
	}

	// Convert (line,char) to absolute offset.
	off := 0
	for i := 0; i < line0; i++ {
		off += len(lines[i]) + 1 // +1 for '\n'
	}
	if char0 < 0 {
		char0 = 0
	}
	if char0 > len(lines[line0]) {
		char0 = len(lines[line0])
	}
	off += char0

	inLineComment := false
	inBlockComment := false
	inString := false
	inChar := false
	escaped := false

	for i := 0; i < len(src) && i < off; i++ {
		c := src[i]
		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '\'' {
				inChar = false
			}
			continue
		}

		if c == '/' && i+1 < len(src) {
			n := src[i+1]
			if n == '/' {
				inLineComment = true
				i++
				continue
			}
			if n == '*' {
				inBlockComment = true
				i++
				continue
			}
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '\'' {
			inChar = true
			continue
		}
	}

	return inLineComment || inBlockComment || inString || inChar
}

func isInStringOrCommentAt(line string, char0 int) bool {
	return isInStringOrComment(line, 0, char0)
}
