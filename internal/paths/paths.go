// Package paths provides common path handling utilities for C-minus.
package paths

import (
	"path/filepath"
	"strings"
)

// SanitizeModuleName converts an import path to a safe C identifier prefix.
// For example, "fileio/ticketio" becomes "fileio_ticketio".
func SanitizeModuleName(importPath string) string {
	return strings.ReplaceAll(importPath, "/", "_")
}

// ModuleHeaderPath returns the path to a module's public header file.
func ModuleHeaderPath(buildDir, importPath string) string {
	return filepath.Join(buildDir, SanitizeModuleName(importPath)+".h")
}

// ModuleInternalHeaderPath returns the path to a module's internal header file.
func ModuleInternalHeaderPath(buildDir, importPath string) string {
	return filepath.Join(buildDir, SanitizeModuleName(importPath)+"_internal.h")
}

// ModuleCFilePath returns the path to a module's C source file for a given .cm file.
func ModuleCFilePath(buildDir, importPath, cmFileName string) string {
	// Remove .cm extension
	name := cmFileName
	if strings.HasSuffix(name, ".cm") {
		name = name[:len(name)-3]
	}
	return filepath.Join(buildDir, SanitizeModuleName(importPath)+"_"+name+".c")
}

// ModuleOFilePath returns the path to a module's object file for a given .cm file.
func ModuleOFilePath(buildDir, importPath, cmFileName string) string {
	cPath := ModuleCFilePath(buildDir, importPath, cmFileName)
	return cPath[:len(cPath)-2] + ".o"
}
