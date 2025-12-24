package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModuleDeclaration(t *testing.T) {
	source := `module "math"

pub func add(a int, b int) int {
    return a + b;
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cm")
	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if file.Module == nil {
		t.Fatal("module declaration not parsed")
	}

	if file.Module.Path != "math" {
		t.Errorf("expected module path 'math', got '%s'", file.Module.Path)
	}
}

func TestParseImports(t *testing.T) {
	source := `module "main"

import "math"
import "io"

func main() int {
    return 0;
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cm")
	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(file.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(file.Imports))
	}

	if file.Imports[0].Path != "math" {
		t.Errorf("expected first import 'math', got '%s'", file.Imports[0].Path)
	}

	if file.Imports[1].Path != "io" {
		t.Errorf("expected second import 'io', got '%s'", file.Imports[1].Path)
	}
}

func TestParsePublicFunction(t *testing.T) {
	source := `module "math"

pub func add(a int, b int) int {
    return a + b;
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cm")
	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	fn := file.Decls[0].Function
	if fn == nil {
		t.Fatal("expected function declaration")
	}

	if !fn.Public {
		t.Error("expected function to be public")
	}

	if fn.Name != "add" {
		t.Errorf("expected function name 'add', got '%s'", fn.Name)
	}

	if fn.ReturnType != "int" {
		t.Errorf("expected return type 'int', got '%s'", fn.ReturnType)
	}

	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(fn.Params))
	}

	if fn.Params[0].Name != "a" || fn.Params[0].Type != "int" {
		t.Errorf("unexpected first parameter: %+v", fn.Params[0])
	}

	if fn.Params[1].Name != "b" || fn.Params[1].Type != "int" {
		t.Errorf("unexpected second parameter: %+v", fn.Params[1])
	}
}

func TestParsePrivateFunction(t *testing.T) {
	source := `module "math"

func helper() int {
    return 42;
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cm")
	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	fn := file.Decls[0].Function
	if fn == nil {
		t.Fatal("expected function declaration")
	}

	if fn.Public {
		t.Error("expected function to be private")
	}

	if fn.Name != "helper" {
		t.Errorf("expected function name 'helper', got '%s'", fn.Name)
	}
}

func TestParseMultipleFunctions(t *testing.T) {
	source := `module "math"

pub func add(a int, b int) int {
    return a + b;
}

func subtract(a int, b int) int {
    return a - b;
}

pub func multiply(a int, b int) int {
    return a * b;
}
`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cm")
	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file, err := ParseFile(testFile)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(file.Decls) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(file.Decls))
	}

	// Check first function
	if file.Decls[0].Function.Name != "add" || !file.Decls[0].Function.Public {
		t.Error("first function incorrect")
	}

	// Check second function
	if file.Decls[1].Function.Name != "subtract" || file.Decls[1].Function.Public {
		t.Error("second function incorrect")
	}

	// Check third function
	if file.Decls[2].Function.Name != "multiply" || !file.Decls[2].Function.Public {
		t.Error("third function incorrect")
	}
}
