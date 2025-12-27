package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create cm.mod
	modContent := `module "github.com/test/project"`
	modPath := filepath.Join(tmpDir, "cm.mod")
	if err := os.WriteFile(modPath, []byte(modContent), 0644); err != nil {
		t.Fatalf("failed to create cm.mod: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Test finding from subdirectory
	rootPath, rootModule, err := findProjectRoot(subDir)
	if err != nil {
		t.Fatalf("findProjectRoot failed: %v", err)
	}

	if rootPath != tmpDir {
		t.Errorf("expected root path %s, got %s", tmpDir, rootPath)
	}

	if rootModule != "github.com/test/project" {
		t.Errorf("expected module github.com/test/project, got %s", rootModule)
	}
}

func TestScanModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create module structure
	// main.cm
	mainCM := filepath.Join(tmpDir, "main.cm")
	if err := os.WriteFile(mainCM, []byte(`module "main"`), 0644); err != nil {
		t.Fatalf("failed to create main.cm: %v", err)
	}

	// math/vector.cm
	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("failed to create math dir: %v", err)
	}
	vectorCM := filepath.Join(mathDir, "vector.cm")
	if err := os.WriteFile(vectorCM, []byte(`module "math"`), 0644); err != nil {
		t.Fatalf("failed to create vector.cm: %v", err)
	}

	// Scan modules
	modules, err := scanModules(tmpDir)
	if err != nil {
		t.Fatalf("scanModules failed: %v", err)
	}

	// Check we found both modules
	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	// Check main module
	mainMod, ok := modules["main"]
	if !ok {
		t.Errorf("main module not found")
	} else if len(mainMod.Files) != 1 {
		t.Errorf("expected 1 file in main module, got %d", len(mainMod.Files))
	}

	// Check math module
	mathMod, ok := modules["math"]
	if !ok {
		t.Errorf("math module not found")
	} else if len(mathMod.Files) != 1 {
		t.Errorf("expected 1 file in math module, got %d", len(mathMod.Files))
	}
}

func TestValidateModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with matching module declarations
	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("failed to create math dir: %v", err)
	}

	vector := filepath.Join(mathDir, "vector.cm")
	if err := os.WriteFile(vector, []byte("module \"math\"\n"), 0644); err != nil {
		t.Fatalf("failed to create vector.cm: %v", err)
	}

	matrix := filepath.Join(mathDir, "matrix.cm")
	if err := os.WriteFile(matrix, []byte("module \"math\"\nimport \"io\"\n"), 0644); err != nil {
		t.Fatalf("failed to create matrix.cm: %v", err)
	}

	modules, err := scanModules(tmpDir)
	if err != nil {
		t.Fatalf("scanModules failed: %v", err)
	}

	proj := &Project{
		RootPath: tmpDir,
		Modules:  modules,
	}

	// Validate - should succeed
	if err := validateModules(proj); err != nil {
		t.Errorf("validateModules failed: %v", err)
	}

	// Check imports were collected
	mathMod := proj.Modules["math"]
	if len(mathMod.Imports) != 1 || mathMod.Imports[0] != "io" {
		t.Errorf("expected imports [io], got %v", mathMod.Imports)
	}
}

func TestValidateModulesMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with mismatched module declarations
	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("failed to create math dir: %v", err)
	}

	vector := filepath.Join(mathDir, "vector.cm")
	if err := os.WriteFile(vector, []byte("module \"math\"\n"), 0644); err != nil {
		t.Fatalf("failed to create vector.cm: %v", err)
	}

	matrix := filepath.Join(mathDir, "matrix.cm")
	if err := os.WriteFile(matrix, []byte("module \"wrongname\"\n"), 0644); err != nil {
		t.Fatalf("failed to create matrix.cm: %v", err)
	}

	modules, err := scanModules(tmpDir)
	if err != nil {
		t.Fatalf("scanModules failed: %v", err)
	}

	proj := &Project{
		RootPath: tmpDir,
		Modules:  modules,
	}

	// Validate - should fail
	if err := validateModules(proj); err == nil {
		t.Error("expected validation error for mismatched modules")
	}
}

func TestDetectCycles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create circular dependency: a -> b -> a
	aDir := filepath.Join(tmpDir, "a")
	bDir := filepath.Join(tmpDir, "b")
	os.MkdirAll(aDir, 0755)
	os.MkdirAll(bDir, 0755)

	aFile := filepath.Join(aDir, "a.cm")
	bFile := filepath.Join(bDir, "b.cm")

	os.WriteFile(aFile, []byte("module \"a\"\nimport \"b\"\n"), 0644)
	os.WriteFile(bFile, []byte("module \"b\"\nimport \"a\"\n"), 0644)

	modules, _ := scanModules(tmpDir)
	proj := &Project{
		RootPath: tmpDir,
		Modules:  modules,
	}

	validateModules(proj)

	// Should detect cycle
	if err := detectCycles(proj); err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestDetectNoCycles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid DAG: a -> b, a -> c
	aDir := filepath.Join(tmpDir, "a")
	bDir := filepath.Join(tmpDir, "b")
	cDir := filepath.Join(tmpDir, "c")
	os.MkdirAll(aDir, 0755)
	os.MkdirAll(bDir, 0755)
	os.MkdirAll(cDir, 0755)

	aFile := filepath.Join(aDir, "a.cm")
	bFile := filepath.Join(bDir, "b.cm")
	cFile := filepath.Join(cDir, "c.cm")

	os.WriteFile(aFile, []byte("module \"a\"\nimport \"b\"\nimport \"c\"\n"), 0644)
	os.WriteFile(bFile, []byte("module \"b\"\n"), 0644)
	os.WriteFile(cFile, []byte("module \"c\"\n"), 0644)

	modules, _ := scanModules(tmpDir)
	proj := &Project{
		RootPath: tmpDir,
		Modules:  modules,
	}

	validateModules(proj)

	// Should not detect cycle
	if err := detectCycles(proj); err != nil {
		t.Errorf("unexpected cycle error: %v", err)
	}
}

func TestBuildTagMatching(t *testing.T) {
	tests := []struct {
		name        string
		tags        [][]string
		ctx         *BuildContext
		shouldMatch bool
	}{
		{
			name:        "no tags matches everything",
			tags:        nil,
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "single OS tag match",
			tags:        [][]string{{"linux"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "single OS tag no match",
			tags:        [][]string{{"windows"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: false,
		},
		{
			name:        "OR tags - first matches",
			tags:        [][]string{{"linux", "darwin"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "OR tags - second matches",
			tags:        [][]string{{"linux", "darwin"}},
			ctx:         &BuildContext{OS: "darwin", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "OR tags - none match",
			tags:        [][]string{{"linux", "darwin"}},
			ctx:         &BuildContext{OS: "windows", Arch: "amd64"},
			shouldMatch: false,
		},
		{
			name:        "AND tags - both match",
			tags:        [][]string{{"linux"}, {"amd64"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "AND tags - first doesn't match",
			tags:        [][]string{{"windows"}, {"amd64"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: false,
		},
		{
			name:        "negation - not windows on linux",
			tags:        [][]string{{"!windows"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64"},
			shouldMatch: true,
		},
		{
			name:        "negation - not windows on windows",
			tags:        [][]string{{"!windows"}},
			ctx:         &BuildContext{OS: "windows", Arch: "amd64"},
			shouldMatch: false,
		},
		{
			name:        "custom tag match",
			tags:        [][]string{{"feature_x"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64", Tags: map[string]bool{"feature_x": true}},
			shouldMatch: true,
		},
		{
			name:        "custom tag no match",
			tags:        [][]string{{"feature_x"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64", Tags: map[string]bool{}},
			shouldMatch: false,
		},
		{
			name:        "release mode match",
			tags:        [][]string{{"release"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64", Release: true},
			shouldMatch: true,
		},
		{
			name:        "debug mode match",
			tags:        [][]string{{"debug"}},
			ctx:         &BuildContext{OS: "linux", Arch: "amd64", Release: false},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesBuildTags(tt.tags, tt.ctx)
			if result != tt.shouldMatch {
				t.Errorf("expected %v, got %v", tt.shouldMatch, result)
			}
		})
	}
}

func TestDefaultBuildContext(t *testing.T) {
	ctx := DefaultBuildContext()

	if ctx.OS != runtime.GOOS {
		t.Errorf("expected OS %s, got %s", runtime.GOOS, ctx.OS)
	}

	if ctx.Arch != runtime.GOARCH {
		t.Errorf("expected Arch %s, got %s", runtime.GOARCH, ctx.Arch)
	}

	if ctx.Release {
		t.Error("expected Release to be false by default")
	}

	if ctx.Tags == nil {
		t.Error("expected Tags to be initialized")
	}
}
