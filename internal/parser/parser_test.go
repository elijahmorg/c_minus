package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseModuleDeclaration(t *testing.T) {
	source := `module "math"

pub func add(int a, int b) int {
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

pub func add(int a, int b) int {
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

pub func add(int a, int b) int {
    return a + b;
}

func subtract(int a, int b) int {
    return a - b;
}

pub func multiply(int a, int b) int {
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

func TestParseCImports(t *testing.T) {
	source := `module "main"

cimport "stdio.h"
cimport "stdlib.h"

import "math"

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

	// Check cimports
	if len(file.CImports) != 2 {
		t.Fatalf("expected 2 cimports, got %d", len(file.CImports))
	}

	if file.CImports[0].Path != "stdio.h" {
		t.Errorf("expected first cimport 'stdio.h', got '%s'", file.CImports[0].Path)
	}

	if file.CImports[1].Path != "stdlib.h" {
		t.Errorf("expected second cimport 'stdlib.h', got '%s'", file.CImports[1].Path)
	}

	// Check regular imports still work
	if len(file.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(file.Imports))
	}

	if file.Imports[0].Path != "math" {
		t.Errorf("expected import 'math', got '%s'", file.Imports[0].Path)
	}
}

func TestParseDocCommentFunction(t *testing.T) {
	source := `module "math"

// add returns the sum of two integers.
pub func add(int a, int b) int {
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

	if fn.DocComment != "add returns the sum of two integers." {
		t.Errorf("expected doc comment 'add returns the sum of two integers.', got '%s'", fn.DocComment)
	}
}

func TestParseDocCommentMultiLine(t *testing.T) {
	source := `module "math"

// multiply multiplies two integers.
// It returns the product as an int.
pub func multiply(int a, int b) int {
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

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	fn := file.Decls[0].Function
	if fn == nil {
		t.Fatal("expected function declaration")
	}

	expected := "multiply multiplies two integers.\nIt returns the product as an int."
	if fn.DocComment != expected {
		t.Errorf("expected doc comment %q, got %q", expected, fn.DocComment)
	}
}

func TestParseDocCommentWithBlankLine(t *testing.T) {
	source := `module "math"

// This comment is not a doc comment because there's a blank line.

pub func add(int a, int b) int {
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

	// Doc comment should be empty because of the blank line
	if fn.DocComment != "" {
		t.Errorf("expected empty doc comment, got '%s'", fn.DocComment)
	}
}

func TestParseDocCommentStruct(t *testing.T) {
	source := `module "data"

// Point represents a 2D point.
pub struct Point {
    int x;
    int y;
};
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

	s := file.Decls[0].Struct
	if s == nil {
		t.Fatal("expected struct declaration")
	}

	if s.DocComment != "Point represents a 2D point." {
		t.Errorf("expected doc comment 'Point represents a 2D point.', got '%s'", s.DocComment)
	}
}

func TestParseDocCommentEnum(t *testing.T) {
	source := `module "status"

// Status represents the status of an item.
pub enum Status {
    TODO,
    DONE
};
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

	e := file.Decls[0].Enum
	if e == nil {
		t.Fatal("expected enum declaration")
	}

	if e.DocComment != "Status represents the status of an item." {
		t.Errorf("expected doc comment 'Status represents the status of an item.', got '%s'", e.DocComment)
	}
}
