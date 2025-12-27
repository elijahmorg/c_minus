package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elijahmorgan/c_minus/internal/codegen"
	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/paths"
	"github.com/elijahmorgan/c_minus/internal/project"
)

// Options contains build configuration
type Options struct {
	Jobs       int    // Number of parallel compile jobs
	OutputPath string // Output binary path (empty = default)
}

// FileFlags stores per-file compiler flags
type FileFlags struct {
	CFlags  []string // CFLAGS for this file
	LDFlags []string // LDFLAGS from this file (aggregated for linking)
}

// Build orchestrates the entire build process
func Build(proj *project.Project, opts Options) error {
	// Create .c_minus directory for intermediate files
	buildDir := filepath.Join(proj.RootPath, ".c_minus")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create .c_minus directory: %w", err)
	}

	// Transpile all modules and collect flags
	fileFlags, err := transpileModules(proj, buildDir)
	if err != nil {
		return fmt.Errorf("transpilation failed: %w", err)
	}

	// Compile .c files to .o files (parallel)
	if err := compileModules(proj, buildDir, opts.Jobs, fileFlags); err != nil {
		return fmt.Errorf("compilation failed: %w", err)
	}

	// Link into final binary at project root
	outputPath := opts.OutputPath
	if outputPath == "" {
		// Default to project root with project name
		outputPath = filepath.Join(proj.RootPath, filepath.Base(proj.RootPath))
	}

	// Collect all LDFLAGS
	allLDFlags := collectLDFlags(fileFlags)

	if err := linkBinary(proj, buildDir, outputPath, allLDFlags); err != nil {
		return fmt.Errorf("linking failed: %w", err)
	}

	return nil
}

// transpileModules converts all .cm files to .h/.c files and returns per-file flags
func transpileModules(proj *project.Project, buildDir string) (map[string]*FileFlags, error) {
	fileFlags := make(map[string]*FileFlags)

	for _, mod := range proj.Modules {
		// Parse all files in this module
		parsedFiles := make([]*parser.File, 0, len(mod.Files))
		for _, filePath := range mod.Files {
			file, err := parser.ParseFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
			}
			parsedFiles = append(parsedFiles, file)

			// Extract and filter CGo flags for this file
			flags := extractFileFlags(file.CGoFlags)
			cFilePath := paths.ModuleCFilePath(buildDir, mod.ImportPath, filepath.Base(filePath))
			fileFlags[cFilePath] = flags
		}

		// Generate code for this module
		if err := codegen.GenerateModule(mod, parsedFiles, buildDir); err != nil {
			return nil, fmt.Errorf("failed to generate code for module %s: %w", mod.ImportPath, err)
		}
	}

	return fileFlags, nil
}

// extractFileFlags extracts and filters CGo flags based on current platform
func extractFileFlags(cgoFlags []*parser.CGoFlag) *FileFlags {
	flags := &FileFlags{
		CFlags:  []string{},
		LDFlags: []string{},
	}

	currentOS := runtime.GOOS

	for _, cgoFlag := range cgoFlags {
		// Filter by platform
		if cgoFlag.Platform != "" && cgoFlag.Platform != currentOS {
			continue
		}

		// Parse the flags string into individual flags
		flagParts := parseFlags(cgoFlag.Flags)

		switch cgoFlag.Type {
		case "CFLAGS":
			flags.CFlags = append(flags.CFlags, flagParts...)
		case "LDFLAGS":
			flags.LDFlags = append(flags.LDFlags, flagParts...)
		}
	}

	return flags
}

// parseFlags splits a flags string into individual flags, preserving quoted values
func parseFlags(flagsStr string) []string {
	var flags []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range flagsStr {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			}
			current.WriteRune(r)
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				flags = append(flags, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		flags = append(flags, current.String())
	}

	return flags
}

// collectLDFlags aggregates and deduplicates all LDFLAGS
func collectLDFlags(fileFlags map[string]*FileFlags) []string {
	seen := make(map[string]bool)
	var ldFlags []string

	for _, flags := range fileFlags {
		for _, flag := range flags.LDFlags {
			if !seen[flag] {
				seen[flag] = true
				ldFlags = append(ldFlags, flag)
			}
		}
	}

	return ldFlags
}

// compileModules compiles all .c files to .o files in parallel
func compileModules(proj *project.Project, buildDir string, jobs int, fileFlags map[string]*FileFlags) error {
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	errChan := make(chan error, len(proj.Modules))

	for _, mod := range proj.Modules {
		if !needsRecompile(mod, buildDir) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(m *project.ModuleInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := compileModule(m, buildDir, fileFlags); err != nil {
				errChan <- err
			}
		}(mod)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

// needsRecompile checks if module needs recompilation
func needsRecompile(mod *project.ModuleInfo, buildDir string) bool {
	// Check each .c file against its corresponding .o file
	for _, srcFile := range mod.Files {
		cFile := paths.ModuleCFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))
		oFile := paths.ModuleOFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))

		oInfo, err := os.Stat(oFile)
		if err != nil {
			// .o doesn't exist, need to compile
			return true
		}

		cInfo, err := os.Stat(cFile)
		if err != nil || cInfo.ModTime().After(oInfo.ModTime()) {
			return true
		}
	}

	return false
}

// compileModule compiles all .c files for a module
// Each .c file is compiled to a .o file, which are collected for linking
func compileModule(mod *project.ModuleInfo, buildDir string, fileFlags map[string]*FileFlags) error {
	// Compile each .c file to its own .o file
	for _, srcFile := range mod.Files {
		cFile := paths.ModuleCFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))
		oFile := paths.ModuleOFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))

		// Build gcc command for this single file
		args := []string{"-c", cFile, "-o", oFile, "-I", buildDir}

		// Add per-file CFLAGS if present
		if flags, ok := fileFlags[cFile]; ok && len(flags.CFlags) > 0 {
			args = append(args, flags.CFlags...)
		}

		cmd := exec.Command("gcc", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gcc failed for %s: %w", cFile, err)
		}
	}

	return nil
}

// linkBinary links all .o files into final executable
func linkBinary(proj *project.Project, buildDir string, outputPath string, ldFlags []string) error {
	// Check if relinking is needed
	if !needsRelink(proj, buildDir, outputPath) {
		return nil
	}

	// Collect all .o files from all source files in all modules
	oFiles := []string{}
	for _, mod := range proj.Modules {
		for _, srcFile := range mod.Files {
			oFile := paths.ModuleOFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))
			oFiles = append(oFiles, oFile)
		}
	}

	// Build gcc command
	args := oFiles
	args = append(args, "-o", outputPath)

	// Add aggregated LDFLAGS
	if len(ldFlags) > 0 {
		args = append(args, ldFlags...)
	}

	cmd := exec.Command("gcc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("linking failed: %w", err)
	}

	return nil
}

// needsRelink checks if relinking is necessary
func needsRelink(proj *project.Project, buildDir string, outputPath string) bool {
	binInfo, err := os.Stat(outputPath)
	if err != nil {
		// Binary doesn't exist, need to link
		return true
	}

	// Check if any .o file is newer than binary
	for _, mod := range proj.Modules {
		for _, srcFile := range mod.Files {
			oFile := paths.ModuleOFilePath(buildDir, mod.ImportPath, filepath.Base(srcFile))
			oInfo, err := os.Stat(oFile)
			if err != nil || oInfo.ModTime().After(binInfo.ModTime()) {
				return true
			}
		}
	}

	return false
}

// Helper to check file modification time
func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
