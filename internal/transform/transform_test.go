package transform

import (
	"testing"

	"github.com/elijahmorgan/c_minus/internal/parser"
)

func TestBuildCImportMap(t *testing.T) {
	cimports := []*parser.CImport{
		{Path: "stdio.h"},
		{Path: "stdlib.h"},
		{Path: "string.h"},
	}

	cimportMap, err := BuildCImportMap(cimports)
	if err != nil {
		t.Fatalf("BuildCImportMap failed: %v", err)
	}

	if len(cimportMap) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(cimportMap))
	}

	// Check prefixes
	if cimportMap["stdio"] != "stdio.h" {
		t.Errorf("expected stdio -> stdio.h, got %s", cimportMap["stdio"])
	}

	if cimportMap["stdlib"] != "stdlib.h" {
		t.Errorf("expected stdlib -> stdlib.h, got %s", cimportMap["stdlib"])
	}

	if cimportMap["string"] != "string.h" {
		t.Errorf("expected string -> string.h, got %s", cimportMap["string"])
	}
}

func TestGetCImportPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"stdio.h", "stdio"},
		{"stdlib.h", "stdlib"},
		{"string.h", "string"},
		{"sys/types.h", "types"},
		{"arpa/inet.h", "inet"},
		{"math", "math"}, // No .h extension
	}

	for _, tt := range tests {
		result := getCImportPrefix(tt.input)
		if result != tt.expected {
			t.Errorf("getCImportPrefix(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestTransformFunctionBodyFull_CImports(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		cimportMap CImportMap
		expected   string
	}{
		{
			name:       "simple printf",
			body:       `{ stdio.printf("hello\n"); }`,
			cimportMap: CImportMap{"stdio": "stdio.h"},
			expected:   `{ printf("hello\n"); }`,
		},
		{
			name:       "printf with args",
			body:       `{ stdio.printf("value: %d\n", x); }`,
			cimportMap: CImportMap{"stdio": "stdio.h"},
			expected:   `{ printf("value: %d\n", x); }`,
		},
		{
			name:       "multiple C calls",
			body:       `{ char* p = stdlib.malloc(100); stdio.printf("allocated\n"); stdlib.free(p); }`,
			cimportMap: CImportMap{"stdio": "stdio.h", "stdlib": "stdlib.h"},
			expected:   `{ char* p = malloc(100); printf("allocated\n"); free(p); }`,
		},
		{
			name:       "mixed with struct access",
			body:       `{ stdio.printf("id: %d\n", t.id); }`,
			cimportMap: CImportMap{"stdio": "stdio.h"},
			expected:   `{ printf("id: %d\n", t.id); }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TransformFunctionBodyFull(tt.body, nil, tt.cimportMap, nil)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTransformFunctionBodyFull_MixedImports(t *testing.T) {
	// Test both c_minus imports and C imports together
	body := `{ stdio.printf("calling module\n"); ticket.create(&t); }`

	importMap := ImportMap{"ticket": "ticket"}
	cimportMap := CImportMap{"stdio": "stdio.h"}

	result := TransformFunctionBodyFull(body, importMap, cimportMap, nil)
	expected := `{ printf("calling module\n"); ticket_create(&t); }`

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
