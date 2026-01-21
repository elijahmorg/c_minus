package lsp

import (
	"strings"
	"testing"
)

func TestLineMapper_MapsLineDirectives(t *testing.T) {
	c := strings.Join([]string{
		"#include <stdio.h>",
		"#line 10 \"/tmp/main.cm\"",
		"int main() {",
		"  return does_not_exist;",
		"}",
	}, "\n") + "\n"

	lm, err := newLineMapperFromC(strings.NewReader(c))
	if err != nil {
		t.Fatalf("newLineMapperFromC: %v", err)
	}

	// The 'int main() {' line is output line 3, and should map to original line 10.
	file, line := lm.mapLine(3)
	if file != "/tmp/main.cm" || line != 10 {
		t.Fatalf("expected /tmp/main.cm:10, got %s:%d", file, line)
	}

	// The return statement is output line 4, and should map to original line 11.
	file, line = lm.mapLine(4)
	if file != "/tmp/main.cm" || line != 11 {
		t.Fatalf("expected /tmp/main.cm:11, got %s:%d", file, line)
	}
}
