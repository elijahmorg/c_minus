package project

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultBuildContext returns a BuildContext based on the current runtime
func DefaultBuildContext() *BuildContext {
	return &BuildContext{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Tags:    make(map[string]bool),
		Release: false,
	}
}

// NewBuildContext creates a BuildContext with custom tags
func NewBuildContext(customTags []string, release bool) *BuildContext {
	ctx := DefaultBuildContext()
	ctx.Release = release
	for _, tag := range customTags {
		ctx.Tags[tag] = true
	}
	return ctx
}

// ImportPrefix returns the last path segment used as the import prefix.
// Example: "utils/io" -> "io".
func ImportPrefix(importPath string) string {
	last := importPath
	for i := len(importPath) - 1; i >= 0; i-- {
		if importPath[i] == '/' {
			last = importPath[i+1:]
			break
		}
	}
	if last == "" {
		return importPath
	}
	return last
}

// Project represents a C-minus project with all its modules
type Project struct {
	RootPath   string                 // Filesystem path to project root (where cm.mod is)
	RootModule string                 // Module path from cm.mod (e.g., "github.com/user/myproject")
	Modules    map[string]*ModuleInfo // Import path -> module info
}

// ModuleInfo represents a single module (directory with .cm files)
type ModuleInfo struct {
	ImportPath string   // Import path (e.g., "math")
	DirPath    string   // Filesystem path to module directory
	Files      []string // All .cm files in this module (absolute paths)
	Imports    []string // Dependencies (other module import paths)
	External   bool     // True if external dependency (future)
}

// BuildContext contains the current build configuration for tag matching
type BuildContext struct {
	OS      string          // Current OS (linux, darwin, windows, etc.)
	Arch    string          // Current architecture (amd64, arm64, etc.)
	Tags    map[string]bool // Custom build tags from command line
	Release bool            // True if building in release mode
}

// Discover finds the project root by locating cm.mod and scans all modules
func Discover(startDir string) (*Project, error) {
	return DiscoverWithContext(startDir, nil)
}

// DiscoverWithContext finds the project root and scans modules, filtering by build context
func DiscoverWithContext(startDir string, ctx *BuildContext) (*Project, error) {
	// Find project root by walking up directories
	rootPath, rootModule, err := findProjectRoot(startDir)
	if err != nil {
		return nil, err
	}

	// Scan for all modules in the project
	modules, err := scanModulesWithContext(rootPath, ctx)
	if err != nil {
		return nil, err
	}

	proj := &Project{
		RootPath:   rootPath,
		RootModule: rootModule,
		Modules:    modules,
	}

	// Validate module declarations and build dependency graph
	if err := validateModules(proj); err != nil {
		return nil, err
	}

	// Detect circular dependencies
	if err := detectCycles(proj); err != nil {
		return nil, err
	}

	return proj, nil
}

// findProjectRoot walks up from startDir to find cm.mod
func findProjectRoot(startDir string) (string, string, error) {
	absPath, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	current := absPath
	for {
		modPath := filepath.Join(current, "cm.mod")
		if _, err := os.Stat(modPath); err == nil {
			// Found cm.mod, parse it
			moduleName, err := parseModFile(modPath)
			if err != nil {
				return "", "", err
			}
			return current, moduleName, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return "", "", fmt.Errorf("no cm.mod found (searched up from %s)", absPath)
		}
		current = parent
	}
}

// parseModFile parses cm.mod to extract the module declaration
func parseModFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read cm.mod: %w", err)
	}

	// Simple parsing: look for module "name"
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module") {
			// Extract quoted string
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return "", fmt.Errorf("invalid module declaration in cm.mod: %s", line)
			}
			moduleName := strings.Trim(parts[1], `"`)
			return moduleName, nil
		}
	}

	return "", fmt.Errorf("no module declaration found in cm.mod")
}

// scanModules recursively finds all .cm files and groups them by directory
func scanModules(rootPath string) (map[string]*ModuleInfo, error) {
	return scanModulesWithContext(rootPath, nil)
}

// scanModulesWithContext recursively finds all .cm files, filtering by build context
func scanModulesWithContext(rootPath string, ctx *BuildContext) (map[string]*ModuleInfo, error) {
	modules := make(map[string]*ModuleInfo)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .c_minus directory
		if info.IsDir() && info.Name() == ".c_minus" {
			return filepath.SkipDir
		}

		// Skip non-.cm files
		if !strings.HasSuffix(path, ".cm") {
			return nil
		}

		// Check build tags if we have a context
		if ctx != nil {
			buildTags, err := extractBuildTags(path)
			if err != nil {
				return err
			}
			if !matchesBuildTags(buildTags, ctx) {
				// File doesn't match build tags, skip it
				return nil
			}
		}

		// Get directory containing this .cm file
		dir := filepath.Dir(path)

		// Compute import path (relative to root)
		relDir, err := filepath.Rel(rootPath, dir)
		if err != nil {
			return fmt.Errorf("failed to compute relative path: %w", err)
		}

		// Normalize import path (use forward slashes)
		importPath := filepath.ToSlash(relDir)
		if importPath == "." {
			importPath = "main"
		}

		// Add to modules map
		if modules[importPath] == nil {
			modules[importPath] = &ModuleInfo{
				ImportPath: importPath,
				DirPath:    dir,
				Files:      []string{},
			}
		}
		modules[importPath].Files = append(modules[importPath].Files, path)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan modules: %w", err)
	}

	return modules, nil
}

// validateModules ensures all files in a directory declare the same module
func validateModules(proj *Project) error {
	for importPath, modInfo := range proj.Modules {
		// Fast scan each file to extract module and import declarations
		var declaredModule string
		imports := make(map[string]bool)

		for _, filePath := range modInfo.Files {
			mod, fileImports, err := fastScanFile(filePath)
			if err != nil {
				return err
			}

			// Validate module declaration
			if declaredModule == "" {
				declaredModule = mod
			} else if declaredModule != mod {
				return fmt.Errorf("module mismatch in %s: expected %q, got %q",
					filePath, declaredModule, mod)
			}

			// Validate module path matches directory
			if mod != importPath {
				return fmt.Errorf("module path mismatch in %s: module declares %q but directory is %q",
					filePath, mod, importPath)
			}

			// Collect imports
			for _, imp := range fileImports {
				imports[imp] = true
			}
		}

		// Store imports
		modInfo.Imports = make([]string, 0, len(imports))
		for imp := range imports {
			modInfo.Imports = append(modInfo.Imports, imp)
		}
	}

	return nil
}

// fastScanFile quickly scans a file for module and import declarations
func fastScanFile(path string) (module string, imports []string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse module declaration
		if strings.HasPrefix(line, "module") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				module = strings.Trim(parts[1], `"`)
			}
		}

		// Parse import declaration
		if strings.HasPrefix(line, "import") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				imp := strings.Trim(parts[1], `"`)
				imports = append(imports, imp)
			}
		}
	}

	if module == "" {
		return "", nil, fmt.Errorf("no module declaration in %s", path)
	}

	return module, imports, nil
}

// extractBuildTags reads a file and extracts build tags
func extractBuildTags(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var buildTags [][]string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "// +build ") {
			tagLine := strings.TrimPrefix(line, "// +build ")
			// Split by spaces - each tag in the line is OR'd together
			tags := strings.Fields(tagLine)
			if len(tags) > 0 {
				buildTags = append(buildTags, tags)
			}
		} else if strings.HasPrefix(line, "module") {
			// Stop looking for build tags once we hit the module declaration
			break
		} else if line != "" && !strings.HasPrefix(line, "//") {
			// Non-comment, non-empty line before module - stop looking
			break
		}
	}

	return buildTags, nil
}

// matchesBuildTags checks if the given build tags match the current context
func matchesBuildTags(buildTags [][]string, ctx *BuildContext) bool {
	// No build tags means always include
	if len(buildTags) == 0 {
		return true
	}

	// Each group (line) must have at least one matching tag (AND between lines)
	for _, orGroup := range buildTags {
		if !matchesOrGroup(orGroup, ctx) {
			return false
		}
	}

	return true
}

// matchesOrGroup checks if any tag in the group matches (OR logic)
func matchesOrGroup(tags []string, ctx *BuildContext) bool {
	for _, tag := range tags {
		if matchesTag(tag, ctx) {
			return true
		}
	}
	return false
}

// matchesTag checks if a single tag matches the current context
func matchesTag(tag string, ctx *BuildContext) bool {
	// Handle negation
	if strings.HasPrefix(tag, "!") {
		return !matchesTag(tag[1:], ctx)
	}

	// Check built-in OS tags
	switch tag {
	case "linux", "darwin", "windows", "freebsd", "openbsd", "netbsd":
		return ctx.OS == tag
	}

	// Check built-in arch tags
	switch tag {
	case "amd64", "arm64", "arm", "386", "mips", "mips64", "ppc64", "s390x":
		return ctx.Arch == tag
	}

	// Check build mode tags
	switch tag {
	case "debug":
		return !ctx.Release
	case "release":
		return ctx.Release
	}

	// Check custom tags
	return ctx.Tags[tag]
}

// detectCycles performs topological sort to detect circular dependencies
func detectCycles(proj *Project) error {
	// Build adjacency list
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for path, mod := range proj.Modules {
		if _, exists := inDegree[path]; !exists {
			inDegree[path] = 0
		}
		graph[path] = mod.Imports
		for _, imp := range mod.Imports {
			inDegree[imp]++
		}
	}

	// Kahn's algorithm for topological sort
	queue := []string{}
	for path, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, path)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++

		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If we didn't process all modules, there's a cycle
	if processed != len(proj.Modules) {
		return fmt.Errorf("circular dependency detected among modules")
	}

	return nil
}
