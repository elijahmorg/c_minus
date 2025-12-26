package paths

import (
	"path/filepath"
	"testing"
)

func TestSanitizeModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"math", "math"},
		{"util/strings", "util_strings"},
		{"a/b/c", "a_b_c"},
		{"fileio/ticketio", "fileio_ticketio"},
	}

	for _, tt := range tests {
		result := SanitizeModuleName(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeModuleName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestModuleHeaderPath(t *testing.T) {
	buildDir := "/build"
	tests := []struct {
		importPath string
		expected   string
	}{
		{"math", filepath.Join("/build", "math.h")},
		{"fileio/ticketio", filepath.Join("/build", "fileio_ticketio.h")},
	}

	for _, tt := range tests {
		result := ModuleHeaderPath(buildDir, tt.importPath)
		if result != tt.expected {
			t.Errorf("ModuleHeaderPath(%q, %q) = %q, expected %q", buildDir, tt.importPath, result, tt.expected)
		}
	}
}

func TestModuleInternalHeaderPath(t *testing.T) {
	buildDir := "/build"
	tests := []struct {
		importPath string
		expected   string
	}{
		{"math", filepath.Join("/build", "math_internal.h")},
		{"fileio/ticketio", filepath.Join("/build", "fileio_ticketio_internal.h")},
	}

	for _, tt := range tests {
		result := ModuleInternalHeaderPath(buildDir, tt.importPath)
		if result != tt.expected {
			t.Errorf("ModuleInternalHeaderPath(%q, %q) = %q, expected %q", buildDir, tt.importPath, result, tt.expected)
		}
	}
}

func TestModuleCFilePath(t *testing.T) {
	buildDir := "/build"
	tests := []struct {
		importPath string
		cmFileName string
		expected   string
	}{
		{"math", "vector.cm", filepath.Join("/build", "math_vector.c")},
		{"fileio/ticketio", "ticketio.cm", filepath.Join("/build", "fileio_ticketio_ticketio.c")},
		{"main", "main.cm", filepath.Join("/build", "main_main.c")},
	}

	for _, tt := range tests {
		result := ModuleCFilePath(buildDir, tt.importPath, tt.cmFileName)
		if result != tt.expected {
			t.Errorf("ModuleCFilePath(%q, %q, %q) = %q, expected %q", buildDir, tt.importPath, tt.cmFileName, result, tt.expected)
		}
	}
}

func TestModuleOFilePath(t *testing.T) {
	buildDir := "/build"
	tests := []struct {
		importPath string
		cmFileName string
		expected   string
	}{
		{"math", "vector.cm", filepath.Join("/build", "math_vector.o")},
		{"fileio/ticketio", "ticketio.cm", filepath.Join("/build", "fileio_ticketio_ticketio.o")},
	}

	for _, tt := range tests {
		result := ModuleOFilePath(buildDir, tt.importPath, tt.cmFileName)
		if result != tt.expected {
			t.Errorf("ModuleOFilePath(%q, %q, %q) = %q, expected %q", buildDir, tt.importPath, tt.cmFileName, result, tt.expected)
		}
	}
}
