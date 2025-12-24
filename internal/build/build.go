package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/elijahmorgan/c_minus/internal/codegen"
	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/project"
)

// Options contains build configuration
type Options struct {
	Jobs       int    // Number of parallel compile jobs
	OutputPath string // Output binary path (empty = default)
}

// Build orchestrates the entire build process
func Build(proj *project.Project, opts Options) error {
	// Create .c_minus directory for intermediate files
	buildDir := filepath.Join(proj.RootPath, ".c_minus")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create .c_minus directory: %w", err)
	}

	// Transpile all modules
	if err := transpileModules(proj, buildDir); err != nil {
		return fmt.Errorf("transpilation failed: %w", err)
	}

	// Compile .c files to .o files (parallel)
	if err := compileModules(proj, buildDir, opts.Jobs); err != nil {
		return fmt.Errorf("compilation failed: %w", err)
	}

	// Link into final binary at project root
	outputPath := opts.OutputPath
	if outputPath == "" {
		// Default to project root with project name
		outputPath = filepath.Join(proj.RootPath, filepath.Base(proj.RootPath))
	}

	if err := linkBinary(proj, buildDir, outputPath); err != nil {
		return fmt.Errorf("linking failed: %w", err)
	}

	return nil
}

// transpileModules converts all .cm files to .h/.c files
func transpileModules(proj *project.Project, buildDir string) error {
	for _, mod := range proj.Modules {
		// Parse all files in this module
		parsedFiles := make([]*parser.File, 0, len(mod.Files))
		for _, filePath := range mod.Files {
			file, err := parser.ParseFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}
			parsedFiles = append(parsedFiles, file)
		}

		// Generate code for this module
		if err := codegen.GenerateModule(mod, parsedFiles, buildDir); err != nil {
			return fmt.Errorf("failed to generate code for module %s: %w", mod.ImportPath, err)
		}
	}

	return nil
}

// compileModules compiles all .c files to .o files in parallel
func compileModules(proj *project.Project, buildDir string, jobs int) error {
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

			if err := compileModule(m, buildDir); err != nil {
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
	moduleName := sanitizeModuleName(mod.ImportPath)

	// Check each .c file against its corresponding .o file
	for _, srcFile := range mod.Files {
		cFile := getCFilePath(srcFile, buildDir, moduleName)
		oFile := cFile[:len(cFile)-2] + ".o"

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
func compileModule(mod *project.ModuleInfo, buildDir string) error {
	moduleName := sanitizeModuleName(mod.ImportPath)

	// Compile each .c file to its own .o file
	for _, srcFile := range mod.Files {
		cFile := getCFilePath(srcFile, buildDir, moduleName)

		// Output .o file (same name as .c but with .o extension)
		oFile := cFile[:len(cFile)-2] + ".o"

		// Build gcc command for this single file
		args := []string{"-c", cFile, "-o", oFile, "-I", buildDir}

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
func linkBinary(proj *project.Project, buildDir string, outputPath string) error {
	// Check if relinking is needed
	if !needsRelink(proj, buildDir, outputPath) {
		return nil
	}

	// Collect all .o files from all source files in all modules
	oFiles := []string{}
	for _, mod := range proj.Modules {
		moduleName := sanitizeModuleName(mod.ImportPath)
		for _, srcFile := range mod.Files {
			cFile := getCFilePath(srcFile, buildDir, moduleName)
			oFile := cFile[:len(cFile)-2] + ".o"
			oFiles = append(oFiles, oFile)
		}
	}

	// Build gcc command
	args := oFiles
	args = append(args, "-o", outputPath)

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
		moduleName := sanitizeModuleName(mod.ImportPath)
		for _, srcFile := range mod.Files {
			cFile := getCFilePath(srcFile, buildDir, moduleName)
			oFile := cFile[:len(cFile)-2] + ".o"
			oInfo, err := os.Stat(oFile)
			if err != nil || oInfo.ModTime().After(binInfo.ModTime()) {
				return true
			}
		}
	}

	return false
}

// getCFilePath gets the .c file path for a source .cm file
func getCFilePath(srcFile string, buildDir string, moduleName string) string {
	base := filepath.Base(srcFile)
	name := base[:len(base)-3] // Remove .cm
	// Match codegen naming: module_file.c
	return filepath.Join(buildDir, moduleName+"_"+name+".c")
}

// sanitizeModuleName converts import path to safe filename
func sanitizeModuleName(importPath string) string {
	// Replace slashes with underscores
	return filepath.ToSlash(importPath)
}

// Modified getCFilePath to match codegen naming
func getModuleOFile(mod *project.ModuleInfo, buildDir string) string {
	return filepath.Join(buildDir, sanitizeModuleName(mod.ImportPath)+".o")
}

// Helper to check file modification time
func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
