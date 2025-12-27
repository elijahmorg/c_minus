package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/paths"
	"github.com/elijahmorgan/c_minus/internal/project"
	"github.com/elijahmorgan/c_minus/internal/transform"
)

func TestGeneratePublicHeader(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "math",
	}

	publicTypes := []*typeDecl{}
	publicFuncs := []*funcDeclInfo{
		{signature: "int math_add(int a, int b)"},
		{signature: "int math_multiply(int a, int b)"},
	}

	imports := make(map[string]bool)
	err := generatePublicHeader(mod, publicTypes, publicFuncs, imports, tmpDir)
	if err != nil {
		t.Fatalf("generatePublicHeader failed: %v", err)
	}

	// Read generated file
	headerPath := filepath.Join(tmpDir, "math.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read generated header: %v", err)
	}

	contentStr := string(content)

	// Check include guard
	if !strings.Contains(contentStr, "#ifndef MATH_H") {
		t.Error("missing include guard start")
	}
	if !strings.Contains(contentStr, "#define MATH_H") {
		t.Error("missing include guard define")
	}
	if !strings.Contains(contentStr, "#endif") {
		t.Error("missing include guard end")
	}

	// Check declarations (with name mangling)
	if !strings.Contains(contentStr, "int math_add(int a, int b);") {
		t.Error("missing add declaration")
	}
	if !strings.Contains(contentStr, "int math_multiply(int a, int b);") {
		t.Error("missing multiply declaration")
	}
}

func TestGenerateInternalHeader(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "math",
	}

	privateTypes := []*typeDecl{}
	privateFuncs := []*funcDeclInfo{
		{signature: "int math_helper(int x)"},
	}

	err := generateInternalHeader(mod, privateTypes, privateFuncs, tmpDir)
	if err != nil {
		t.Fatalf("generateInternalHeader failed: %v", err)
	}

	// Read generated file
	headerPath := filepath.Join(tmpDir, "math_internal.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read generated header: %v", err)
	}

	contentStr := string(content)

	// Check include guard
	if !strings.Contains(contentStr, "#ifndef MATH_INTERNAL_H") {
		t.Error("missing include guard start")
	}

	// Check includes public header
	if !strings.Contains(contentStr, "#include \"math.h\"") {
		t.Error("missing include of public header")
	}

	// Check private declaration (with name mangling)
	if !strings.Contains(contentStr, "int math_helper(int x);") {
		t.Error("missing helper declaration")
	}
}

func TestGenerateCFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a source file
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	srcFile := filepath.Join(srcDir, "vector.cm")

	mod := &project.ModuleInfo{
		ImportPath: "math",
		Files:      []string{srcFile},
	}

	file := &parser.File{
		Module: &parser.ModuleDecl{Path: "math"},
		Imports: []*parser.Import{
			{Path: "io"},
		},
		Decls: []*parser.Decl{
			{
				Function: &parser.FuncDecl{
					Public:     true,
					Name:       "dot",
					ReturnType: "float",
					Params: []*parser.Param{
						{Name: "a", Type: "Vec3"},
						{Name: "b", Type: "Vec3"},
					},
					Body: "{\n    return a.x * b.x + a.y * b.y + a.z * b.z;\n}",
				},
			},
		},
	}

	buildDir := filepath.Join(tmpDir, "build")
	os.MkdirAll(buildDir, 0755)

	enumValues := make(transform.EnumValueMap)
	err := generateCFile(mod, file, srcFile, buildDir, enumValues)
	if err != nil {
		t.Fatalf("generateCFile failed: %v", err)
	}

	// Read generated file
	cPath := filepath.Join(buildDir, "math_vector.c")
	content, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("failed to read generated C file: %v", err)
	}

	contentStr := string(content)

	// Check includes internal header
	if !strings.Contains(contentStr, "#include \"math_internal.h\"") {
		t.Error("missing include of internal header")
	}

	// Check includes dependency
	if !strings.Contains(contentStr, "#include \"io.h\"") {
		t.Error("missing include of io.h")
	}

	// Check function implementation (with name mangling and type mangling)
	if !strings.Contains(contentStr, "float math_dot(math_Vec3 a, math_Vec3 b)") {
		t.Error("missing function signature")
	}

	if !strings.Contains(contentStr, "return a.x * b.x + a.y * b.y + a.z * b.z;") {
		t.Error("missing function body")
	}
}

func TestGenerateFunctionSignature(t *testing.T) {
	tests := []struct {
		name     string
		fn       *parser.FuncDecl
		expected string
	}{
		{
			name: "simple function",
			fn: &parser.FuncDecl{
				Name:       "add",
				ReturnType: "int",
				Params: []*parser.Param{
					{Name: "a", Type: "int"},
					{Name: "b", Type: "int"},
				},
			},
			expected: "int math_add(int a, int b)",
		},
		{
			name: "void function",
			fn: &parser.FuncDecl{
				Name:       "print",
				ReturnType: "",
				Params: []*parser.Param{
					{Name: "msg", Type: "char*"},
				},
			},
			expected: "void math_print(char* msg)",
		},
		{
			name: "no parameters",
			fn: &parser.FuncDecl{
				Name:       "getNumber",
				ReturnType: "int",
				Params:     []*parser.Param{},
			},
			expected: "int math_getNumber()",
		},
		{
			name: "variadic function",
			fn: &parser.FuncDecl{
				Name:       "log",
				ReturnType: "void",
				Params: []*parser.Param{
					{Name: "fmt", Type: "char*"},
					{Name: "", Type: "..."},
				},
			},
			expected: "void math_log(char* fmt, ...)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := generateFunctionSignature(tt.fn, "math")
			if sig != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, sig)
			}
		})
	}
}

func TestSanitizeModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"math", "math"},
		{"util/strings", "util_strings"},
		{"a/b/c", "a_b_c"},
	}

	for _, tt := range tests {
		result := paths.SanitizeModuleName(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeModuleName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDocComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty comment",
			input:    "",
			expected: "",
		},
		{
			name:     "single line comment",
			input:    "This is a comment.",
			expected: "// This is a comment.\n",
		},
		{
			name:     "multi line comment",
			input:    "Line one.\nLine two.",
			expected: "/*\n * Line one.\n * Line two.\n */\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDocComment(tt.input)
			if result != tt.expected {
				t.Errorf("formatDocComment(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGeneratePublicHeaderWithDocComments(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "math",
	}

	publicTypes := []*typeDecl{
		{
			kind:       "struct",
			name:       "Point",
			body:       "{\n    int x;\n    int y;\n}",
			public:     true,
			docComment: "Point represents a 2D point.",
		},
	}
	publicFuncs := []*funcDeclInfo{
		{
			signature:  "int math_add(int a, int b)",
			docComment: "add returns the sum of two integers.",
		},
		{
			signature:  "int math_multiply(int a, int b)",
			docComment: "multiply multiplies two integers.\nReturns the product.",
		},
	}

	imports := make(map[string]bool)
	err := generatePublicHeader(mod, publicTypes, publicFuncs, imports, tmpDir)
	if err != nil {
		t.Fatalf("generatePublicHeader failed: %v", err)
	}

	// Read generated file
	headerPath := filepath.Join(tmpDir, "math.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read generated header: %v", err)
	}

	contentStr := string(content)

	// Check single-line doc comment for struct
	if !strings.Contains(contentStr, "// Point represents a 2D point.") {
		t.Error("missing struct doc comment")
	}

	// Check single-line doc comment for function
	if !strings.Contains(contentStr, "// add returns the sum of two integers.") {
		t.Error("missing single-line function doc comment")
	}

	// Check multi-line doc comment for function (block style)
	if !strings.Contains(contentStr, "/*\n * multiply multiplies two integers.") {
		t.Error("missing multi-line function doc comment")
	}
}
