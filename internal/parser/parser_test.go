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

func TestParseBitFields(t *testing.T) {
	source := `module "hardware"

pub struct StatusRegister {
    unsigned int ready : 1;
    unsigned int error : 1;
    unsigned int mode : 3;
    unsigned int reserved : 27;
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

	if s.Name != "StatusRegister" {
		t.Errorf("expected struct name 'StatusRegister', got '%s'", s.Name)
	}

	// Verify the body contains bit field syntax
	if !contains(s.Body, ": 1") {
		t.Errorf("expected bit field syntax in body, got '%s'", s.Body)
	}

	if !contains(s.Body, ": 3") {
		t.Errorf("expected bit field syntax ': 3' in body, got '%s'", s.Body)
	}

	if !contains(s.Body, ": 27") {
		t.Errorf("expected bit field syntax ': 27' in body, got '%s'", s.Body)
	}
}

func TestParseVariadicFunction(t *testing.T) {
	source := `module "logging"

cimport "stdarg.h"
cimport "stdio.h"

pub func log(char* fmt, ...) void {
    // variadic implementation
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

	if fn.Name != "log" {
		t.Errorf("expected function name 'log', got '%s'", fn.Name)
	}

	// Should have 2 parameters: "char* fmt" and "..."
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 parameters (fmt and ...), got %d", len(fn.Params))
	}

	// First param should be char* fmt
	if fn.Params[0].Type != "char*" || fn.Params[0].Name != "fmt" {
		t.Errorf("expected first param 'char* fmt', got type='%s' name='%s'",
			fn.Params[0].Type, fn.Params[0].Name)
	}

	// Second param should be the variadic marker
	if fn.Params[1].Type != "..." || fn.Params[1].Name != "" {
		t.Errorf("expected second param to be variadic '...', got type='%s' name='%s'",
			fn.Params[1].Type, fn.Params[1].Name)
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseBuildTags(t *testing.T) {
	source := `// +build linux darwin
// +build amd64

module "platform"

pub func get_page_size() int {
    return 4096;
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

	// Should have 2 build tag groups
	if len(file.BuildTags) != 2 {
		t.Fatalf("expected 2 build tag groups, got %d", len(file.BuildTags))
	}

	// First group: linux, darwin (OR)
	if len(file.BuildTags[0]) != 2 {
		t.Fatalf("expected 2 tags in first group, got %d", len(file.BuildTags[0]))
	}
	if file.BuildTags[0][0] != "linux" {
		t.Errorf("expected first tag 'linux', got '%s'", file.BuildTags[0][0])
	}
	if file.BuildTags[0][1] != "darwin" {
		t.Errorf("expected second tag 'darwin', got '%s'", file.BuildTags[0][1])
	}

	// Second group: amd64
	if len(file.BuildTags[1]) != 1 {
		t.Fatalf("expected 1 tag in second group, got %d", len(file.BuildTags[1]))
	}
	if file.BuildTags[1][0] != "amd64" {
		t.Errorf("expected tag 'amd64', got '%s'", file.BuildTags[1][0])
	}
}

func TestParseBuildTagNegation(t *testing.T) {
	source := `// +build !windows

module "unix"

pub func get_null_device() char* {
    return "/dev/null";
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

	// Should have 1 build tag group
	if len(file.BuildTags) != 1 {
		t.Fatalf("expected 1 build tag group, got %d", len(file.BuildTags))
	}

	// First group: !windows
	if len(file.BuildTags[0]) != 1 {
		t.Fatalf("expected 1 tag in group, got %d", len(file.BuildTags[0]))
	}
	if file.BuildTags[0][0] != "!windows" {
		t.Errorf("expected tag '!windows', got '%s'", file.BuildTags[0][0])
	}
}

func TestParseDefineConstant(t *testing.T) {
	source := `module "fileio"

// Max path length
pub #define MAX_PATH 4096

// Buffer size for IO
pub #define BUFFER_SIZE 1024

// Internal chunk size (private)
#define INTERNAL_CHUNK 512

// Version string
pub #define VERSION "1.0.0"
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

	if len(file.Decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(file.Decls))
	}

	// Check first define: pub #define MAX_PATH 4096
	d1 := file.Decls[0].Define
	if d1 == nil {
		t.Fatal("expected first declaration to be a define")
	}
	if !d1.Public {
		t.Error("expected MAX_PATH to be public")
	}
	if d1.Name != "MAX_PATH" {
		t.Errorf("expected name 'MAX_PATH', got '%s'", d1.Name)
	}
	if d1.Value != "4096" {
		t.Errorf("expected value '4096', got '%s'", d1.Value)
	}

	// Check second define: pub #define BUFFER_SIZE 1024
	d2 := file.Decls[1].Define
	if d2 == nil {
		t.Fatal("expected second declaration to be a define")
	}
	if d2.Name != "BUFFER_SIZE" {
		t.Errorf("expected name 'BUFFER_SIZE', got '%s'", d2.Name)
	}

	// Check third define: #define INTERNAL_CHUNK 512 (private)
	d3 := file.Decls[2].Define
	if d3 == nil {
		t.Fatal("expected third declaration to be a define")
	}
	if d3.Public {
		t.Error("expected INTERNAL_CHUNK to be private")
	}
	if d3.Name != "INTERNAL_CHUNK" {
		t.Errorf("expected name 'INTERNAL_CHUNK', got '%s'", d3.Name)
	}

	// Check fourth define: pub #define VERSION "1.0.0"
	d4 := file.Decls[3].Define
	if d4 == nil {
		t.Fatal("expected fourth declaration to be a define")
	}
	if d4.Value != `"1.0.0"` {
		t.Errorf("expected value '\"1.0.0\"', got '%s'", d4.Value)
	}
}

func TestParseStaticGlobal(t *testing.T) {
	source := `module "singleton"

// File-private static global
static int initialized = 0;

// Static with const
static const char* internal_name = "secret";
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

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check first static: static int initialized = 0
	g1 := file.Decls[0].Global
	if g1 == nil {
		t.Fatal("expected first declaration to be a global")
	}
	if !g1.Static {
		t.Error("expected initialized to be static")
	}
	if g1.Public {
		t.Error("expected initialized to not be public")
	}
	if g1.Type != "int" {
		t.Errorf("expected type 'int', got '%s'", g1.Type)
	}
	if g1.Name != "initialized" {
		t.Errorf("expected name 'initialized', got '%s'", g1.Name)
	}
	if g1.Value != "0" {
		t.Errorf("expected value '0', got '%s'", g1.Value)
	}

	// Check second static: static const char* internal_name
	g2 := file.Decls[1].Global
	if g2 == nil {
		t.Fatal("expected second declaration to be a global")
	}
	if !g2.Static {
		t.Error("expected internal_name to be static")
	}
	if g2.Type != "const char*" {
		t.Errorf("expected type 'const char*', got '%s'", g2.Type)
	}
	if g2.Name != "internal_name" {
		t.Errorf("expected name 'internal_name', got '%s'", g2.Name)
	}
}

func TestParseGlobalVariable(t *testing.T) {
	source := `module "state"

// Public global with initializer
pub int error_count = 0;

// Private global
int last_error_code = 0;

// Uninitialized global
pub char* buffer;

// Const global
pub const char* version = "1.0.0";
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

	if len(file.Decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(file.Decls))
	}

	// Check first global: pub int error_count = 0
	g1 := file.Decls[0].Global
	if g1 == nil {
		t.Fatal("expected first declaration to be a global")
	}
	if !g1.Public {
		t.Error("expected error_count to be public")
	}
	if g1.Type != "int" {
		t.Errorf("expected type 'int', got '%s'", g1.Type)
	}
	if g1.Name != "error_count" {
		t.Errorf("expected name 'error_count', got '%s'", g1.Name)
	}
	if g1.Value != "0" {
		t.Errorf("expected value '0', got '%s'", g1.Value)
	}

	// Check second global: int last_error_code = 0
	g2 := file.Decls[1].Global
	if g2 == nil {
		t.Fatal("expected second declaration to be a global")
	}
	if g2.Public {
		t.Error("expected last_error_code to be private")
	}
	if g2.Name != "last_error_code" {
		t.Errorf("expected name 'last_error_code', got '%s'", g2.Name)
	}

	// Check third global: pub char* buffer (uninitialized)
	g3 := file.Decls[2].Global
	if g3 == nil {
		t.Fatal("expected third declaration to be a global")
	}
	if g3.Type != "char*" {
		t.Errorf("expected type 'char*', got '%s'", g3.Type)
	}
	if g3.Name != "buffer" {
		t.Errorf("expected name 'buffer', got '%s'", g3.Name)
	}
	if g3.Value != "" {
		t.Errorf("expected empty value, got '%s'", g3.Value)
	}

	// Check fourth global: pub const char* version = "1.0.0"
	g4 := file.Decls[3].Global
	if g4 == nil {
		t.Fatal("expected fourth declaration to be a global")
	}
	if g4.Type != "const char*" {
		t.Errorf("expected type 'const char*', got '%s'", g4.Type)
	}
	if g4.Name != "version" {
		t.Errorf("expected name 'version', got '%s'", g4.Name)
	}
	if g4.Value != `"1.0.0"` {
		t.Errorf("expected value '\"1.0.0\"', got '%s'", g4.Value)
	}
}

func TestParseCGoDirectives(t *testing.T) {
	source := `module "http"

#cgo CFLAGS: -I/usr/local/include/curl
#cgo LDFLAGS: -lcurl
#cgo linux LDFLAGS: -lpthread
#cgo darwin LDFLAGS: -framework Security

cimport "curl/curl.h"

pub func fetch(char* url) int {
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

	if len(file.CGoFlags) != 4 {
		t.Fatalf("expected 4 #cgo directives, got %d", len(file.CGoFlags))
	}

	// Check first: CFLAGS without platform
	f1 := file.CGoFlags[0]
	if f1.Platform != "" {
		t.Errorf("expected empty platform for first directive, got '%s'", f1.Platform)
	}
	if f1.Type != "CFLAGS" {
		t.Errorf("expected type 'CFLAGS', got '%s'", f1.Type)
	}
	if f1.Flags != "-I/usr/local/include/curl" {
		t.Errorf("expected flags '-I/usr/local/include/curl', got '%s'", f1.Flags)
	}

	// Check second: LDFLAGS without platform
	f2 := file.CGoFlags[1]
	if f2.Platform != "" {
		t.Errorf("expected empty platform for second directive, got '%s'", f2.Platform)
	}
	if f2.Type != "LDFLAGS" {
		t.Errorf("expected type 'LDFLAGS', got '%s'", f2.Type)
	}
	if f2.Flags != "-lcurl" {
		t.Errorf("expected flags '-lcurl', got '%s'", f2.Flags)
	}

	// Check third: linux LDFLAGS
	f3 := file.CGoFlags[2]
	if f3.Platform != "linux" {
		t.Errorf("expected platform 'linux', got '%s'", f3.Platform)
	}
	if f3.Type != "LDFLAGS" {
		t.Errorf("expected type 'LDFLAGS', got '%s'", f3.Type)
	}
	if f3.Flags != "-lpthread" {
		t.Errorf("expected flags '-lpthread', got '%s'", f3.Flags)
	}

	// Check fourth: darwin LDFLAGS
	f4 := file.CGoFlags[3]
	if f4.Platform != "darwin" {
		t.Errorf("expected platform 'darwin', got '%s'", f4.Platform)
	}
	if f4.Type != "LDFLAGS" {
		t.Errorf("expected type 'LDFLAGS', got '%s'", f4.Type)
	}
	if f4.Flags != "-framework Security" {
		t.Errorf("expected flags '-framework Security', got '%s'", f4.Flags)
	}
}
