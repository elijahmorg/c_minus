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

func TestGenerateModuleWithUnion(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "types",
		Files:      []string{"types.cm"},
	}

	files := []*parser.File{
		{
			Module:  &parser.ModuleDecl{Path: "types"},
			Imports: []*parser.Import{},
			Decls: []*parser.Decl{
				{
					Union: &parser.UnionDecl{
						Public: true,
						Name:   "Value",
						Body:   "{\n    int i;\n    float f;\n    char* s;\n}",
						Semi:   true,
					},
				},
				{
					Function: &parser.FuncDecl{
						Public:     true,
						Name:       "get_int",
						ReturnType: "int",
						Params: []*parser.Param{
							{Name: "v", Type: "Value*"},
						},
						Body: "{\n    return v->i;\n}",
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
	headerPath := filepath.Join(tmpDir, "types.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read types.h: %v", err)
	}

	headerContent := string(content)

	// Should contain the forward declaration
	if !strings.Contains(headerContent, "union types_Value;") {
		t.Errorf("header missing forward declaration union types_Value;, got:\n%s", headerContent)
	}

	// Should contain the typedef union
	if !strings.Contains(headerContent, "typedef union types_Value") {
		t.Errorf("header missing typedef union types_Value, got:\n%s", headerContent)
	}

	// Should contain the type alias
	if !strings.Contains(headerContent, "} types_Value;") {
		t.Errorf("header missing type alias types_Value, got:\n%s", headerContent)
	}

	// Should contain mangled function signature
	if !strings.Contains(headerContent, "int types_get_int(types_Value* v)") {
		t.Errorf("header missing mangled function signature, got:\n%s", headerContent)
	}
}

func TestGenerateModuleWithPrivateUnion(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "internal",
		Files:      []string{"internal.cm"},
	}

	files := []*parser.File{
		{
			Module:  &parser.ModuleDecl{Path: "internal"},
			Imports: []*parser.Import{},
			Decls: []*parser.Decl{
				{
					Union: &parser.UnionDecl{
						Public: false,
						Name:   "PrivateData",
						Body:   "{\n    int raw;\n    char bytes[4];\n}",
						Semi:   true,
					},
				},
			},
		},
	}

	err := GenerateModule(mod, files, tmpDir)
	if err != nil {
		t.Fatalf("GenerateModule failed: %v", err)
	}

	// Check that public header does NOT contain the private union
	publicHeader, err := os.ReadFile(filepath.Join(tmpDir, "internal.h"))
	if err != nil {
		t.Fatalf("failed to read internal.h: %v", err)
	}
	if strings.Contains(string(publicHeader), "PrivateData") {
		t.Errorf("public header should not contain private union, got:\n%s", string(publicHeader))
	}

	// Check internal header contains the union
	internalHeader, err := os.ReadFile(filepath.Join(tmpDir, "internal_internal.h"))
	if err != nil {
		t.Fatalf("failed to read internal_internal.h: %v", err)
	}
	if !strings.Contains(string(internalHeader), "typedef union internal_PrivateData") {
		t.Errorf("internal header missing union definition, got:\n%s", string(internalHeader))
	}
}

func TestGenerateFunctionWithFunctionPointerParam(t *testing.T) {
	tmpDir := t.TempDir()

	mod := &project.ModuleInfo{
		ImportPath: "callbacks",
		Files:      []string{"callbacks.cm"},
	}

	files := []*parser.File{
		{
			Module:  &parser.ModuleDecl{Path: "callbacks"},
			Imports: []*parser.Import{},
			Decls: []*parser.Decl{
				{
					Function: &parser.FuncDecl{
						Public:     true,
						Name:       "qsort",
						ReturnType: "void",
						Params: []*parser.Param{
							{Name: "base", Type: "void*"},
							{Name: "n", Type: "size_t"},
							{Name: "cmp", Type: "int (*)(void*, void*)"},
						},
						Body: "{\n    // implementation\n}",
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
	headerPath := filepath.Join(tmpDir, "callbacks.h")
	content, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("failed to read callbacks.h: %v", err)
	}

	headerContent := string(content)

	// Should contain the function signature with function pointer parameter
	// Note: size_t gets mangled to callbacks_size_t since it's not in our primitive list
	// (in real usage, size_t would come from a cimport which wouldn't be mangled)
	if !strings.Contains(headerContent, "void callbacks_qsort") {
		t.Errorf("header missing callbacks_qsort function, got:\n%s", headerContent)
	}
	// Function pointer params should have the name inside: int (*cmp)(void*, void*)
	if !strings.Contains(headerContent, "int (*cmp)(void*, void*)") {
		t.Errorf("header missing correctly formatted function pointer parameter, got:\n%s", headerContent)
	}
}
