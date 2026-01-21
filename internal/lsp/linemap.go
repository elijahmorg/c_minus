package lsp

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

type lineMapSegment struct {
	outStartLine  int    // 1-based generated line where mapping starts
	origStartLine int    // 1-based original line where mapping starts
	origFile      string // original file path
}

type lineMapper struct {
	segments []lineMapSegment
}

func (lm *lineMapper) mapToGeneratedLine(origFile string, origLine1Based int) (int, bool) {
	if lm == nil || len(lm.segments) == 0 {
		return 0, false
	}

	for i := 0; i < len(lm.segments); i++ {
		seg := lm.segments[i]
		if seg.origFile != origFile {
			continue
		}

		endOut := int(^uint(0) >> 1) // max int
		if i+1 < len(lm.segments) {
			endOut = lm.segments[i+1].outStartLine - 1
		}

		if origLine1Based < seg.origStartLine {
			continue
		}
		maxOrig := seg.origStartLine + (endOut - seg.outStartLine)
		if origLine1Based > maxOrig {
			continue
		}

		out := seg.outStartLine + (origLine1Based - seg.origStartLine)
		return out, true
	}

	return 0, false
}

func newLineMapperFromC(r io.Reader) (*lineMapper, error) {
	lm := &lineMapper{}

	// Default segment maps to generated file itself; we keep origFile empty and treat it as "no mapping".
	// A #line directive overrides it.
	lm.segments = append(lm.segments, lineMapSegment{outStartLine: 1, origStartLine: 1, origFile: ""})

	scanner := bufio.NewScanner(r)
	outLine := 0
	for scanner.Scan() {
		outLine++
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#line ") {
			continue
		}

		// Format: #line <number> "<path>"
		rest := strings.TrimSpace(strings.TrimPrefix(line, "#line "))
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		// Extract quoted path: fields[1] includes quotes (possibly with spaces not supported by Fields).
		quoted := strings.TrimSpace(rest[len(fields[0]):])
		quoted = strings.TrimSpace(quoted)
		if !strings.HasPrefix(quoted, "\"") {
			continue
		}
		end := strings.LastIndex(quoted, "\"")
		if end <= 0 {
			continue
		}
		path := quoted[1:end]

		// #line applies to the next output line.
		lm.segments = append(lm.segments, lineMapSegment{
			outStartLine:  outLine + 1,
			origStartLine: n,
			origFile:      path,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lm, nil
}

func (lm *lineMapper) mapLine(outLine1Based int) (origFile string, origLine1Based int) {
	if lm == nil || len(lm.segments) == 0 {
		return "", outLine1Based
	}

	// Find the last segment starting at or before outLine.
	seg := lm.segments[0]
	for _, s := range lm.segments {
		if s.outStartLine <= outLine1Based {
			seg = s
		}
	}

	if seg.origFile == "" {
		return "", outLine1Based
	}

	delta := outLine1Based - seg.outStartLine
	if delta < 0 {
		delta = 0
	}
	return seg.origFile, seg.origStartLine + delta
}
