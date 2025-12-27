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

pub func add(int a, int b) int {
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

pub func multiply(int a, int b) int {
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

// TestUnionsAndFunctionPointers tests union types and function pointer parameters
func TestUnionsAndFunctionPointers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/unions"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create types module directory
	typesDir := filepath.Join(tmpDir, "types")
	if err := os.MkdirAll(typesDir, 0755); err != nil {
		t.Fatalf("failed to create types dir: %v", err)
	}

	// Create types/types.cm with union and function pointer typedef
	typesCM := `module "types"

// Value represents a variant type that can hold different values
pub union Value {
    int i;
    float f;
    char* s;
};

// Comparator is a function pointer type for comparison functions
pub typedef int (*Comparator)(void* a, void* b);

// get_int extracts the integer value from a Value union
pub func get_int(Value* v) int {
    return v->i;
}

// set_int sets the integer value in a Value union
pub func set_int(Value* v, int val) void {
    v->i = val;
}
`
	if err := os.WriteFile(filepath.Join(typesDir, "types.cm"), []byte(typesCM), 0644); err != nil {
		t.Fatalf("failed to create types.cm: %v", err)
	}

	// Create callbacks module directory
	callbacksDir := filepath.Join(tmpDir, "callbacks")
	if err := os.MkdirAll(callbacksDir, 0755); err != nil {
		t.Fatalf("failed to create callbacks dir: %v", err)
	}

	// Create callbacks/callbacks.cm with function pointer parameter
	callbacksCM := `module "callbacks"

// apply_func applies a function to a value
pub func apply_func(int val, int (*fn)(int)) int {
    return fn(val);
}

// process_array uses a comparison function pointer
pub func process_array(int* arr, int n, int (*cmp)(int, int)) int {
    return cmp(arr[0], arr[1]);
}
`
	if err := os.WriteFile(filepath.Join(callbacksDir, "callbacks.cm"), []byte(callbacksCM), 0644); err != nil {
		t.Fatalf("failed to create callbacks.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

cimport "stdio.h"

import "types"

func main() int {
    // Test union
    types.Value v;
    types.set_int(&v, 42);
    int result = types.get_int(&v);
    
    stdio.printf("Union value: %d\n", result);
    
    return result - 42;
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

	// Verify types.h contains union definition
	typesH, err := os.ReadFile(filepath.Join(buildDir, "types.h"))
	if err != nil {
		t.Fatalf("failed to read types.h: %v", err)
	}
	typesHContent := string(typesH)

	if !contains(typesHContent, "typedef union types_Value") {
		t.Error("types.h missing union definition")
	}

	if !contains(typesHContent, "typedef int (*Comparator)") {
		t.Error("types.h missing function pointer typedef")
	}

	// Verify callbacks.h contains function with function pointer param
	callbacksH, err := os.ReadFile(filepath.Join(buildDir, "callbacks.h"))
	if err != nil {
		t.Fatalf("failed to read callbacks.h: %v", err)
	}
	callbacksHContent := string(callbacksH)

	if !contains(callbacksHContent, "int (*fn)(int)") {
		t.Error("callbacks.h missing function pointer parameter in apply_func")
	}

	if !contains(callbacksHContent, "int (*cmp)(int, int)") {
		t.Error("callbacks.h missing function pointer parameter in process_array")
	}

	// Verify binary was created
	binaryPath := filepath.Join(tmpDir, filepath.Base(tmpDir))
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Error("binary was not created at project root")
	}

	// Run the binary and verify output
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary execution failed: %v\nOutput: %s", err, runOutput)
	}

	if !contains(string(runOutput), "Union value: 42") {
		t.Errorf("unexpected output, expected to contain 'Union value: 42', got: %s", runOutput)
	}
}

// TestFunctionPointerParameter tests passing function pointers as parameters
func TestFunctionPointerParameter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/funcptr"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create util module directory
	utilDir := filepath.Join(tmpDir, "util")
	if err := os.MkdirAll(utilDir, 0755); err != nil {
		t.Fatalf("failed to create util dir: %v", err)
	}

	// Create util/util.cm with function pointer parameter
	utilCM := `module "util"

// apply applies a function to a value
pub func apply(int val, int (*fn)(int)) int {
    return fn(val);
}

// transform applies a transformation function
pub func transform(int a, int b, int (*op)(int, int)) int {
    return op(a, b);
}
`
	if err := os.WriteFile(filepath.Join(utilDir, "util.cm"), []byte(utilCM), 0644); err != nil {
		t.Fatalf("failed to create util.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

func main() int {
    return 0;
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

	// Verify util.h contains function pointer parameters
	buildDir := filepath.Join(tmpDir, ".c_minus")
	utilH, err := os.ReadFile(filepath.Join(buildDir, "util.h"))
	if err != nil {
		t.Fatalf("failed to read util.h: %v", err)
	}
	utilHContent := string(utilH)

	// Check for correct function pointer syntax in apply
	if !contains(utilHContent, "int util_apply(int val, int (*fn)(int))") {
		t.Errorf("util.h has incorrect apply signature, got:\n%s", utilHContent)
	}

	// Check for correct function pointer syntax in transform
	if !contains(utilHContent, "int util_transform(int a, int b, int (*op)(int, int))") {
		t.Errorf("util.h has incorrect transform signature, got:\n%s", utilHContent)
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

// TestDefineConstants tests #define constant support
func TestDefineConstants(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/defines"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create config module directory
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create config/config.cm with #define constants
	configCM := `module "config"

// Max buffer size
pub #define MAX_BUFFER 1024

// Timeout in ms
pub #define TIMEOUT 5000

// Internal chunk size (private)
#define CHUNK_SIZE 256

pub func get_buffer_size() int {
    return MAX_BUFFER;
}

pub func get_timeout() int {
    return TIMEOUT;
}

pub func get_chunk() int {
    return CHUNK_SIZE;
}
`
	if err := os.WriteFile(filepath.Join(configDir, "config.cm"), []byte(configCM), 0644); err != nil {
		t.Fatalf("failed to create config.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

cimport "stdio.h"

import "config"

func main() int {
    stdio.printf("Buffer size: %d\n", config.MAX_BUFFER);
    stdio.printf("Timeout: %d\n", config.get_timeout());
    return config.MAX_BUFFER + config.TIMEOUT;
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

	// Verify config.h contains #define macros with mangled names
	buildDir := filepath.Join(tmpDir, ".c_minus")
	configH, err := os.ReadFile(filepath.Join(buildDir, "config.h"))
	if err != nil {
		t.Fatalf("failed to read config.h: %v", err)
	}
	configHContent := string(configH)

	if !contains(configHContent, "#define config_MAX_BUFFER 1024") {
		t.Errorf("config.h missing #define for MAX_BUFFER, got:\n%s", configHContent)
	}
	if !contains(configHContent, "#define config_TIMEOUT 5000") {
		t.Errorf("config.h missing #define for TIMEOUT, got:\n%s", configHContent)
	}

	// Verify private defines are NOT in public header
	if contains(configHContent, "CHUNK_SIZE") {
		t.Errorf("config.h should not contain private CHUNK_SIZE, got:\n%s", configHContent)
	}

	// Verify private defines are in internal header
	configInternalH, err := os.ReadFile(filepath.Join(buildDir, "config_internal.h"))
	if err != nil {
		t.Fatalf("failed to read config_internal.h: %v", err)
	}
	configInternalHContent := string(configInternalH)

	if !contains(configInternalHContent, "#define CHUNK_SIZE 256") {
		t.Errorf("config_internal.h missing #define for CHUNK_SIZE, got:\n%s", configInternalHContent)
	}

	// Verify the binary runs correctly
	binaryPath := filepath.Join(tmpDir, filepath.Base(tmpDir))
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	// Exit code 6024 (1024 + 5000) is expected
	if err != nil {
		// That's fine, we just want to check output
	}

	if !contains(string(runOutput), "Buffer size: 1024") {
		t.Errorf("unexpected output, expected 'Buffer size: 1024', got: %s", runOutput)
	}
	if !contains(string(runOutput), "Timeout: 5000") {
		t.Errorf("unexpected output, expected 'Timeout: 5000', got: %s", runOutput)
	}
}

// TestGlobalVariables tests global variable support
func TestGlobalVariables(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/globals"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create state module directory
	stateDir := filepath.Join(tmpDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}

	// Create state/state.cm with global variables
	stateCM := `module "state"

// Public counter
pub int counter = 0;

// Private internal counter
int internal_counter = 100;

// Version string
pub const char* version = "1.0.0";

pub func increment() void {
    counter = counter + 1;
    internal_counter = internal_counter + 1;
}

pub func get_counter() int {
    return counter;
}

pub func get_internal() int {
    return internal_counter;
}
`
	if err := os.WriteFile(filepath.Join(stateDir, "state.cm"), []byte(stateCM), 0644); err != nil {
		t.Fatalf("failed to create state.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

cimport "stdio.h"

import "state"

func main() int {
    stdio.printf("Initial counter: %d\n", state.counter);
    state.increment();
    state.increment();
    stdio.printf("After increment: %d\n", state.get_counter());
    stdio.printf("Version: %s\n", state.version);
    return state.get_counter();
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

	// Verify state.h contains extern declarations
	buildDir := filepath.Join(tmpDir, ".c_minus")
	stateH, err := os.ReadFile(filepath.Join(buildDir, "state.h"))
	if err != nil {
		t.Fatalf("failed to read state.h: %v", err)
	}
	stateHContent := string(stateH)

	if !contains(stateHContent, "extern int state_counter;") {
		t.Errorf("state.h missing extern declaration for counter, got:\n%s", stateHContent)
	}
	if !contains(stateHContent, "extern const char* state_version;") {
		t.Errorf("state.h missing extern declaration for version, got:\n%s", stateHContent)
	}

	// Verify state.c contains definitions
	stateC, err := os.ReadFile(filepath.Join(buildDir, "state_state.c"))
	if err != nil {
		t.Fatalf("failed to read state_state.c: %v", err)
	}
	stateCContent := string(stateC)

	if !contains(stateCContent, "int state_counter = 0;") {
		t.Errorf("state.c missing definition for counter, got:\n%s", stateCContent)
	}
	if !contains(stateCContent, `const char* state_version = "1.0.0";`) {
		t.Errorf("state.c missing definition for version, got:\n%s", stateCContent)
	}

	// Run the binary and verify output
	binaryPath := filepath.Join(tmpDir, filepath.Base(tmpDir))
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		// Exit code 2 is expected (counter after 2 increments)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 2 {
				t.Fatalf("unexpected exit code: %d, output: %s", exitErr.ExitCode(), runOutput)
			}
		} else {
			t.Fatalf("binary execution failed: %v\nOutput: %s", err, runOutput)
		}
	}

	if !contains(string(runOutput), "Initial counter: 0") {
		t.Errorf("unexpected output, expected 'Initial counter: 0', got: %s", runOutput)
	}
	if !contains(string(runOutput), "After increment: 2") {
		t.Errorf("unexpected output, expected 'After increment: 2', got: %s", runOutput)
	}
	if !contains(string(runOutput), "Version: 1.0.0") {
		t.Errorf("unexpected output, expected 'Version: 1.0.0', got: %s", runOutput)
	}
}

// TestVariadicFunctions tests variadic function support
func TestVariadicFunctions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cm.mod
	modFile := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modFile, []byte(`module "test/variadic"`), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create logging module directory
	loggingDir := filepath.Join(tmpDir, "logging")
	if err := os.MkdirAll(loggingDir, 0755); err != nil {
		t.Fatalf("failed to create logging dir: %v", err)
	}

	// Create logging/logging.cm with variadic function
	loggingCM := `module "logging"

cimport "stdarg.h"
cimport "stdio.h"

pub func log(char* fmt, ...) void {
    stdarg.va_list args;
    stdarg.va_start(args, fmt);
    stdio.vprintf(fmt, args);
    stdarg.va_end(args);
}
`
	if err := os.WriteFile(filepath.Join(loggingDir, "logging.cm"), []byte(loggingCM), 0644); err != nil {
		t.Fatalf("failed to create logging.cm: %v", err)
	}

	// Create main.cm
	mainCM := `module "main"

import "logging"

func main() int {
    logging.log("Hello %s, number %d\n", "World", 42);
    return 0;
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

	// Verify logging.h contains variadic function signature
	buildDir := filepath.Join(tmpDir, ".c_minus")
	loggingH, err := os.ReadFile(filepath.Join(buildDir, "logging.h"))
	if err != nil {
		t.Fatalf("failed to read logging.h: %v", err)
	}
	loggingHContent := string(loggingH)

	if !contains(loggingHContent, "void logging_log(char* fmt, ...)") {
		t.Errorf("logging.h missing variadic function signature, got:\n%s", loggingHContent)
	}

	// Run the binary and verify output
	binaryPath := filepath.Join(tmpDir, filepath.Base(tmpDir))
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary execution failed: %v\nOutput: %s", err, runOutput)
	}

	if !contains(string(runOutput), "Hello World, number 42") {
		t.Errorf("unexpected output, expected 'Hello World, number 42', got: %s", runOutput)
	}
}
