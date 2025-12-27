package parser

import (
	"testing"
)

func TestParseStruct(t *testing.T) {
	source := `module "math"

pub struct Vec3 {
    float x;
    float y;
    float z;
};

struct PrivateData {
    int count;
};
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check public struct
	if file.Decls[0].Struct == nil {
		t.Fatal("expected struct declaration")
	}
	pubStruct := file.Decls[0].Struct
	if !pubStruct.Public {
		t.Error("expected public struct")
	}
	if pubStruct.Name != "Vec3" {
		t.Errorf("expected struct name Vec3, got %s", pubStruct.Name)
	}
	if pubStruct.Body == "" {
		t.Error("expected struct body")
	}

	// Check private struct
	if file.Decls[1].Struct == nil {
		t.Fatal("expected struct declaration")
	}
	privStruct := file.Decls[1].Struct
	if privStruct.Public {
		t.Error("expected private struct")
	}
	if privStruct.Name != "PrivateData" {
		t.Errorf("expected struct name PrivateData, got %s", privStruct.Name)
	}
}

func TestParseEnum(t *testing.T) {
	source := `module "state"

pub enum State {
    IDLE,
    RUNNING,
    STOPPED
};
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	if file.Decls[0].Enum == nil {
		t.Fatal("expected enum declaration")
	}

	enum := file.Decls[0].Enum
	if !enum.Public {
		t.Error("expected public enum")
	}
	if enum.Name != "State" {
		t.Errorf("expected enum name State, got %s", enum.Name)
	}
	if enum.Body == "" {
		t.Error("expected enum body")
	}
}

func TestParseTypedef(t *testing.T) {
	source := `module "types"

pub typedef int Counter;

typedef void (*Callback)(int);
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check public typedef
	if file.Decls[0].Typedef == nil {
		t.Fatal("expected typedef declaration")
	}
	pubTypedef := file.Decls[0].Typedef
	if !pubTypedef.Public {
		t.Error("expected public typedef")
	}

	// Check private typedef
	if file.Decls[1].Typedef == nil {
		t.Fatal("expected typedef declaration")
	}
	privTypedef := file.Decls[1].Typedef
	if privTypedef.Public {
		t.Error("expected private typedef")
	}
}

func TestParseForwardDeclaration(t *testing.T) {
	source := `module "types"

pub struct Node;

pub struct List {
    struct Node* head;
};
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check forward declaration
	if file.Decls[0].Struct == nil {
		t.Fatal("expected struct declaration")
	}
	fwdDecl := file.Decls[0].Struct
	if fwdDecl.Name != "Node" {
		t.Errorf("expected struct name Node, got %s", fwdDecl.Name)
	}
	if fwdDecl.Body != "" {
		t.Error("expected empty body for forward declaration")
	}

	// Check full struct
	if file.Decls[1].Struct == nil {
		t.Fatal("expected struct declaration")
	}
	fullStruct := file.Decls[1].Struct
	if fullStruct.Name != "List" {
		t.Errorf("expected struct name List, got %s", fullStruct.Name)
	}
	if fullStruct.Body == "" {
		t.Error("expected struct body")
	}
}

func TestParseMixedDeclarations(t *testing.T) {
	source := `module "mixed"

pub struct Point {
    int x;
    int y;
};

pub func distance(p1 Point, p2 Point) float {
    float dx = p2.x - p1.x;
    float dy = p2.y - p1.y;
    return sqrt(dx*dx + dy*dy);
}

pub enum Color {
    RED,
    GREEN,
    BLUE
};

func helper() int {
    return 42;
}
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(file.Decls))
	}

	// Verify struct
	if file.Decls[0].Struct == nil {
		t.Error("expected struct as first declaration")
	}

	// Verify function
	if file.Decls[1].Function == nil {
		t.Error("expected function as second declaration")
	}

	// Verify enum
	if file.Decls[2].Enum == nil {
		t.Error("expected enum as third declaration")
	}

	// Verify private function
	if file.Decls[3].Function == nil {
		t.Error("expected function as fourth declaration")
	}
	if file.Decls[3].Function.Public {
		t.Error("expected private function")
	}
}

func TestParseUnion(t *testing.T) {
	source := `module "types"

pub union Value {
    int i;
    float f;
    char* s;
};

union PrivateData {
    int raw;
    char bytes[4];
};
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check public union
	if file.Decls[0].Union == nil {
		t.Fatal("expected union declaration")
	}
	pubUnion := file.Decls[0].Union
	if !pubUnion.Public {
		t.Error("expected public union")
	}
	if pubUnion.Name != "Value" {
		t.Errorf("expected union name Value, got %s", pubUnion.Name)
	}
	if pubUnion.Body == "" {
		t.Error("expected union body")
	}

	// Check private union
	if file.Decls[1].Union == nil {
		t.Fatal("expected union declaration")
	}
	privUnion := file.Decls[1].Union
	if privUnion.Public {
		t.Error("expected private union")
	}
	if privUnion.Name != "PrivateData" {
		t.Errorf("expected union name PrivateData, got %s", privUnion.Name)
	}
}

func TestParseUnionForwardDeclaration(t *testing.T) {
	source := `module "types"

pub union Data;

pub struct Container {
    union Data* data;
};
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check forward declaration
	if file.Decls[0].Union == nil {
		t.Fatal("expected union declaration")
	}
	fwdDecl := file.Decls[0].Union
	if fwdDecl.Name != "Data" {
		t.Errorf("expected union name Data, got %s", fwdDecl.Name)
	}
	if fwdDecl.Body != "" {
		t.Error("expected empty body for forward declaration")
	}
}

func TestParseFunctionPointerParam(t *testing.T) {
	source := `module "callbacks"

pub func qsort(void* base, size_t n, int (*cmp)(void*, void*)) void {
    // implementation
}
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	fn := file.Decls[0].Function
	if fn == nil {
		t.Fatal("expected function declaration")
	}

	if len(fn.Params) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(fn.Params))
	}

	// Check function pointer parameter
	cmpParam := fn.Params[2]
	if cmpParam.Name != "cmp" {
		t.Errorf("expected parameter name 'cmp', got '%s'", cmpParam.Name)
	}
	if cmpParam.Type != "int (*)(void*, void*)" {
		t.Errorf("expected parameter type 'int (*)(void*, void*)', got '%s'", cmpParam.Type)
	}
}

func TestParseFunctionPointerParamComplex(t *testing.T) {
	source := `module "events"

pub func register(int id, void (*handler)(int, char*), void* ctx) int {
    return 0;
}
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	fn := file.Decls[0].Function
	if fn == nil {
		t.Fatal("expected function declaration")
	}

	if len(fn.Params) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(fn.Params))
	}

	// Check first param
	if fn.Params[0].Name != "id" || fn.Params[0].Type != "int" {
		t.Errorf("unexpected first parameter: %+v", fn.Params[0])
	}

	// Check function pointer parameter
	handler := fn.Params[1]
	if handler.Name != "handler" {
		t.Errorf("expected parameter name 'handler', got '%s'", handler.Name)
	}
	if handler.Type != "void (*)(int, char*)" {
		t.Errorf("expected parameter type 'void (*)(int, char*)', got '%s'", handler.Type)
	}

	// Check third param
	if fn.Params[2].Name != "ctx" || fn.Params[2].Type != "void*" {
		t.Errorf("unexpected third parameter: %+v", fn.Params[2])
	}
}

func TestParseFunctionPointerTypedef(t *testing.T) {
	source := `module "callbacks"

pub typedef int (*CompareFunc)(void* a, void* b);
pub typedef void (*EventHandler)(int event_id, void* data);
`

	file, err := manualParse(source, "test.cm")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}

	// Check first typedef
	if file.Decls[0].Typedef == nil {
		t.Fatal("expected typedef declaration")
	}
	td1 := file.Decls[0].Typedef
	if !td1.Public {
		t.Error("expected public typedef")
	}
	if td1.Body != "int (*CompareFunc)(void* a, void* b)" {
		t.Errorf("unexpected typedef body: %s", td1.Body)
	}

	// Check second typedef
	if file.Decls[1].Typedef == nil {
		t.Fatal("expected typedef declaration")
	}
	td2 := file.Decls[1].Typedef
	if td2.Body != "void (*EventHandler)(int event_id, void* data)" {
		t.Errorf("unexpected typedef body: %s", td2.Body)
	}
}
