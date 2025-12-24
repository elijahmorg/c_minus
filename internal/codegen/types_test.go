package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/project"
)

func TestGenerateModuleWithTypes(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "math",
		Files:      []string{"vector.cm"},
	}

	files := []*parser.File{
		{
			Module:  &parser.ModuleDecl{Path: "math"},
			Imports: []*parser.Import{},
			Decls: []*parser.Decl{
				{
					Struct: &parser.StructDecl{
						Public: true,
						Name:   "Vec3",
						Body:   "{\n    float x;\n    float y;\n    float z;\n}",
						Semi:   true,
					},
				},
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
		},
	}

	err := GenerateModule(mod, files, tmpDir)
	if err != nil {
		t.Fatalf("GenerateModule failed: %v", err)
	}

	// Check public header
	headerPath := filepath.Join(tmpDir, "math.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read math.h: %v", err)
	}

	headerContent := string(content)

	// Should contain the type definition with mangling
	if !strings.Contains(headerContent, "typedef struct math_Vec3") {
		t.Errorf("header missing typedef struct math_Vec3, got:\n%s", headerContent)
	}

	// Should contain mangled function name
	if !strings.Contains(headerContent, "float math_dot") {
		t.Errorf("header missing mangled function name math_dot, got:\n%s", headerContent)
	}

	// Check implementation file
	cPath := filepath.Join(tmpDir, "math_vector.c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("failed to read math_vector.c: %v", err)
	}

	cStr := string(cContent)

	// Should have mangled function name
	if !strings.Contains(cStr, "float math_dot") {
		t.Errorf("C file missing mangled function name, got:\n%s", cStr)
	}
}

func TestGenerateWithQualifiedAccess(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "main",
		Files:      []string{"main.cm"},
	}

	files := []*parser.File{
		{
			Module: &parser.ModuleDecl{Path: "main"},
			Imports: []*parser.Import{
				{Path: "math"},
			},
			Decls: []*parser.Decl{
				{
					Function: &parser.FuncDecl{
						Public:     false,
						Name:       "main",
						ReturnType: "int",
						Params:     []*parser.Param{},
						Body:       "{\n    math.Vec3 a;\n    float result = math.dot(a, a);\n    return 0;\n}",
					},
				},
			},
		},
	}

	err := GenerateModule(mod, files, tmpDir)
	if err != nil {
		t.Fatalf("GenerateModule failed: %v", err)
	}

	// Check implementation file
	cPath := filepath.Join(tmpDir, "main_main.c")
	cContent, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("failed to read main_main.c: %v", err)
	}

	cStr := string(cContent)

	// Should have transformed math.Vec3 to math_Vec3
	if !strings.Contains(cStr, "math_Vec3 a") {
		t.Errorf("C file missing transformed type reference math_Vec3, got:\n%s", cStr)
	}

	// Should have transformed math.dot to math_dot
	if !strings.Contains(cStr, "math_dot(a, a)") {
		t.Errorf("C file missing transformed function call math_dot, got:\n%s", cStr)
	}

	// Should NOT have qualified access syntax in output
	if strings.Contains(cStr, "math.Vec3") {
		t.Errorf("C file should not contain math.Vec3 syntax, got:\n%s", cStr)
	}
	if strings.Contains(cStr, "math.dot") {
		t.Errorf("C file should not contain math.dot syntax, got:\n%s", cStr)
	}
}
