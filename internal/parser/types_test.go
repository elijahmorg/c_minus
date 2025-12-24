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
