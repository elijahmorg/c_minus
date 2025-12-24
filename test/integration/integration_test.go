package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEndToEnd creates a complete C-minus project and builds it
func TestEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/project"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create math module directory
	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("failed to create math dir: %v", err)
	}

	// Create math/vector.cm
	vectorCM := `module "math"

pub func add(a int, b int) int {
    return a + b;
}

func helper() int {
    return 42;
}
`
	if err := os.WriteFile(filepath.Join(mathDir, "vector.cm"), []byte(vectorCM), 0644); err != nil {
		t.Fatalf("failed to create vector.cm: %v", err)
	}

	// Create math/matrix.cm
	matrixCM := `module "math"

pub func multiply(a int, b int) int {
    return a * b;
}
`
	if err := os.WriteFile(filepath.Join(mathDir, "matrix.cm"), []byte(matrixCM), 0644); err != nil {
		t.Fatalf("failed to create matrix.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

import "math"

func main() int {
    int sum = math.add(1, 2);
    int prod = math.multiply(3, 4);
    return sum + prod;
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.cm"), []byte(mainCM), 0644); err != nil {
		t.Fatalf("failed to create main.cm: %v", err)
	}

	// Find c_minus binary
	cMinusBinary := findCMinusBinary(t)

	// Run c_minus build
	cmd := exec.Command(cMinusBinary, "build")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("c_minus build failed: %v\nOutput: %s", err, output)
	}

	// Verify .c_minus directory was created
	buildDir := filepath.Join(tmpDir, ".c_minus")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		t.Error(".c_minus directory was not created")
	}

	// Verify generated files exist in .c_minus
	expectedFiles := []string{
		"math.h",
		"math_internal.h",
		"math_vector.c",
		"math_matrix.c",
		"main_main.c",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(buildDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", file)
		}
	}

	// Verify binary was created at project root
	binaryPath := filepath.Join(tmpDir, filepath.Base(tmpDir))
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Error("binary was not created at project root")
	}

	// Read and verify math.h (public header)
	mathH, err := os.ReadFile(filepath.Join(buildDir, "math.h"))
	if err != nil {
		t.Fatalf("failed to read math.h: %v", err)
	}

	mathHContent := string(mathH)

	// Should contain public functions (with name mangling)
	if !containsAll(mathHContent, "int math_add(int a, int b);", "int math_multiply(int a, int b);") {
		t.Error("math.h missing public function declarations")
	}

	// Should NOT contain helper (it's private)
	if contains(mathHContent, "math_helper") {
		t.Error("math.h should not contain private function helper")
	}

	// Read and verify math_internal.h
	mathInternal, err := os.ReadFile(filepath.Join(buildDir, "math_internal.h"))
	if err != nil {
		t.Fatalf("failed to read math_internal.h: %v", err)
	}

	mathInternalContent := string(mathInternal)

	// Should include public header
	if !contains(mathInternalContent, `#include "math.h"`) {
		t.Error("math_internal.h should include math.h")
	}

	// Should contain private helper (with name mangling)
	if !contains(mathInternalContent, "int math_helper()") {
		t.Error("math_internal.h should contain helper declaration")
	}
}

// TestCircularDependency verifies cycle detection works
func TestCircularDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/circular"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create circular dependency: a -> b -> a
	aDir := filepath.Join(tmpDir, "a")
	bDir := filepath.Join(tmpDir, "b")
	os.MkdirAll(aDir, 0755)
	os.MkdirAll(bDir, 0755)

	aCM := `module "a"

import "b"

func aFunc() int {
    return 1;
}
`
	bCM := `module "b"

import "a"

func bFunc() int {
    return 2;
}
`

	os.WriteFile(filepath.Join(aDir, "a.cm"), []byte(aCM), 0644)
	os.WriteFile(filepath.Join(bDir, "b.cm"), []byte(bCM), 0644)

	// Find c_minus binary
	cMinusBinary := findCMinusBinary(t)

	// Run c_minus build - should fail
	cmd := exec.Command(cMinusBinary, "build")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected build to fail due to circular dependency")
	}

	// Should mention circular dependency
	if !contains(string(output), "circular") {
		t.Errorf("error message should mention circular dependency, got: %s", output)
	}
}

// TestModuleMismatch verifies module validation works
func TestModuleMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/mismatch"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create math directory with mismatched module declarations
	mathDir := filepath.Join(tmpDir, "math")
	os.MkdirAll(mathDir, 0755)

	vector := `module "math"

func vectorFunc() int {
    return 1;
}
`
	matrix := `module "wrongname"

func matrixFunc() int {
    return 2;
}
`

	os.WriteFile(filepath.Join(mathDir, "vector.cm"), []byte(vector), 0644)
	os.WriteFile(filepath.Join(mathDir, "matrix.cm"), []byte(matrix), 0644)

	// Find c_minus binary
	cMinusBinary := findCMinusBinary(t)

	// Run c_minus build - should fail
	cmd := exec.Command(cMinusBinary, "build")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected build to fail due to module mismatch")
	}

	// Should mention mismatch
	if !contains(string(output), "mismatch") {
		t.Errorf("error message should mention mismatch, got: %s", output)
	}
}

// Helper functions

func findCMinusBinary(t *testing.T) string {
	// Try to find the binary in the project
	candidates := []string{
		"../../c_minus",
		"../../../c_minus",
		"../../../../c_minus",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(candidate)
			return abs
		}
	}

	// Try to build it
	buildCmd := exec.Command("go", "build", "-o", "c_minus", "./cmd/c_minus")
	buildCmd.Dir = filepath.Join("..", "..")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build c_minus: %v", err)
	}

	abs, _ := filepath.Abs(filepath.Join("..", "..", "c_minus"))
	return abs
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !contains(s, substr) {
			return false
		}
	}
	return true
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
