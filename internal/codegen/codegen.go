package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/project"
	"github.com/elijahmorgan/c_minus/internal/transform"
)

// GenerateModule generates .h and .c files for a module
func GenerateModule(mod *project.ModuleInfo, files []*parser.File, buildDir string) error {
	moduleName := sanitizeModuleName(mod.ImportPath)

	// Collect all public and private declarations
	publicFuncDecls := []string{}
	privateFuncDecls := []string{}
	publicTypeDecls := []*typeDecl{}
	privateTypeDecls := []*typeDecl{}

	for _, file := range files {
		for _, decl := range file.Decls {
			if decl.Function != nil {
				funcSig := generateFunctionSignature(decl.Function, moduleName)
				if decl.Function.Public {
					publicFuncDecls = append(publicFuncDecls, funcSig)
				} else {
					privateFuncDecls = append(privateFuncDecls, funcSig)
				}
			} else if decl.Struct != nil {
				typeDecl := &typeDecl{
					kind:   "struct",
					name:   decl.Struct.Name,
					body:   decl.Struct.Body,
					public: decl.Struct.Public,
				}
				if decl.Struct.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Enum != nil {
				typeDecl := &typeDecl{
					kind:   "enum",
					name:   decl.Enum.Name,
					body:   decl.Enum.Body,
					public: decl.Enum.Public,
				}
				if decl.Enum.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Typedef != nil {
				typeDecl := &typeDecl{
					kind:   "typedef",
					body:   decl.Typedef.Body,
					public: decl.Typedef.Public,
				}
				if decl.Typedef.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			}
		}
	}

	// Generate public header
	if err := generatePublicHeader(mod, publicTypeDecls, publicFuncDecls, buildDir); err != nil {
		return err
	}

	// Generate internal header (if there are private declarations)
	if len(privateFuncDecls) > 0 || len(privateTypeDecls) > 0 {
		if err := generateInternalHeader(mod, privateTypeDecls, privateFuncDecls, buildDir); err != nil {
			return err
		}
	}

	// Generate .c files for each source file
	for i, file := range files {
		if err := generateCFile(mod, file, mod.Files[i], buildDir); err != nil {
			return err
		}
	}

	return nil
}

// typeDecl represents a type declaration for code generation
type typeDecl struct {
	kind   string // "struct", "enum", or "typedef"
	name   string // type name (for struct/enum)
	body   string // opaque body content
	public bool
}

// generatePublicHeader generates the public .h file for a module
func generatePublicHeader(mod *project.ModuleInfo, publicTypes []*typeDecl, publicFuncs []string, buildDir string) error {
	moduleName := sanitizeModuleName(mod.ImportPath)
	guardName := strings.ToUpper(moduleName) + "_H"

	var sb strings.Builder

	// Include guard
	sb.WriteString(fmt.Sprintf("#ifndef %s\n", guardName))
	sb.WriteString(fmt.Sprintf("#define %s\n\n", guardName))

	// Forward declarations for all structs (to handle dependencies)
	for _, td := range publicTypes {
		if td.kind == "struct" && td.body != "" {
			sb.WriteString(fmt.Sprintf("struct %s_%s;\n", moduleName, td.name))
		}
	}
	if len(publicTypes) > 0 {
		sb.WriteString("\n")
	}

	// Public type declarations
	for _, td := range publicTypes {
		sb.WriteString(generateTypeDeclaration(td, moduleName))
		sb.WriteString("\n")
	}

	// Public function declarations
	for _, decl := range publicFuncs {
		sb.WriteString(decl)
		sb.WriteString(";\n")
	}

	sb.WriteString("\n#endif\n")

	// Write to file
	headerPath := filepath.Join(buildDir, moduleName+".h")
	if err := os.WriteFile(headerPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", headerPath, err)
	}

	return nil
}

// generateInternalHeader generates the internal _internal.h file for a module
func generateInternalHeader(mod *project.ModuleInfo, privateTypes []*typeDecl, privateFuncs []string, buildDir string) error {
	moduleName := sanitizeModuleName(mod.ImportPath)
	guardName := strings.ToUpper(moduleName) + "_INTERNAL_H"

	var sb strings.Builder

	// Include guard
	sb.WriteString(fmt.Sprintf("#ifndef %s\n", guardName))
	sb.WriteString(fmt.Sprintf("#define %s\n\n", guardName))

	// Include public header
	sb.WriteString(fmt.Sprintf("#include \"%s.h\"\n\n", moduleName))

	// Forward declarations for private structs
	for _, td := range privateTypes {
		if td.kind == "struct" && td.body != "" {
			sb.WriteString(fmt.Sprintf("struct %s_%s;\n", moduleName, td.name))
		}
	}
	if len(privateTypes) > 0 {
		sb.WriteString("\n")
	}

	// Private type declarations
	for _, td := range privateTypes {
		sb.WriteString(generateTypeDeclaration(td, moduleName))
		sb.WriteString("\n")
	}

	// Private function declarations
	for _, decl := range privateFuncs {
		sb.WriteString(decl)
		sb.WriteString(";\n")
	}

	sb.WriteString("\n#endif\n")

	// Write to file
	headerPath := filepath.Join(buildDir, moduleName+"_internal.h")
	if err := os.WriteFile(headerPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", headerPath, err)
	}

	return nil
}

// generateCFile generates a .c implementation file
func generateCFile(mod *project.ModuleInfo, file *parser.File, srcPath string, buildDir string) error {
	moduleName := sanitizeModuleName(mod.ImportPath)
	baseName := filepath.Base(srcPath)
	baseName = baseName[:len(baseName)-3] // Remove .cm extension

	// Build import map for qualified access transformation
	importMap, err := transform.BuildImportMap(file.Imports)
	if err != nil {
		return fmt.Errorf("failed to build import map for %s: %w", srcPath, err)
	}

	var sb strings.Builder

	// Include internal header (which includes public header)
	sb.WriteString(fmt.Sprintf("#include \"%s_internal.h\"\n", moduleName))

	// Include dependency headers
	for _, imp := range file.Imports {
		importName := sanitizeModuleName(imp.Path)
		sb.WriteString(fmt.Sprintf("#include \"%s.h\"\n", importName))
	}

	sb.WriteString("\n")

	// Emit function implementations
	for _, decl := range file.Decls {
		if decl.Function != nil {
			funcImpl := generateFunctionImplementation(decl.Function, moduleName, importMap, srcPath)
			sb.WriteString(funcImpl)
			sb.WriteString("\n\n")
		}
	}

	// Write to file
	cPath := filepath.Join(buildDir, moduleName+"_"+baseName+".c")
	if err := os.WriteFile(cPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cPath, err)
	}

	return nil
}

// generateFunctionSignature generates a C function signature with name mangling
func generateFunctionSignature(fn *parser.FuncDecl, moduleName string) string {
	var sb strings.Builder

	// Return type (mangle if it's a custom type)
	returnType := fn.ReturnType
	if returnType == "" {
		returnType = "void"
	}
	// Transform return type: mangle non-primitive types with module prefix
	returnType = mangleTypeInSignature(returnType, moduleName)
	sb.WriteString(returnType)
	sb.WriteString(" ")

	// Function name (mangled with module prefix, except for main)
	if fn.Name != "main" {
		sb.WriteString(moduleName)
		sb.WriteString("_")
	}
	sb.WriteString(fn.Name)

	// Parameters
	sb.WriteString("(")
	for i, param := range fn.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		// Transform parameter type: mangle non-primitive types with module prefix
		paramType := mangleTypeInSignature(param.Type, moduleName)
		sb.WriteString(paramType)
		sb.WriteString(" ")
		sb.WriteString(param.Name)
	}
	sb.WriteString(")")

	return sb.String()
}

// mangleTypeInSignature mangles custom type names in function signatures
// Primitive C types are left unchanged
func mangleTypeInSignature(typeName string, moduleName string) string {
	// Common primitive types - don't mangle these
	primitives := map[string]bool{
		"void":     true,
		"char":     true,
		"short":    true,
		"int":      true,
		"long":     true,
		"float":    true,
		"double":   true,
		"unsigned": true,
		"signed":   true,
		// Also handle pointer types
	}

	// Check for pointers
	if strings.HasSuffix(typeName, "*") {
		// Strip pointer, mangle base type, re-add pointer
		baseType := strings.TrimRight(typeName, "*")
		asterisks := typeName[len(baseType):]
		return mangleTypeInSignature(baseType, moduleName) + asterisks
	}

	// Check for struct/enum keywords
	if strings.HasPrefix(typeName, "struct ") {
		return typeName // Already has struct keyword
	}
	if strings.HasPrefix(typeName, "enum ") {
		return typeName // Already has enum keyword
	}

	// Split on spaces to handle complex types
	parts := strings.Fields(typeName)
	if len(parts) == 0 {
		return typeName
	}

	// Check if first word is a primitive
	if primitives[parts[0]] {
		return typeName
	}

	// Custom type - mangle it
	return moduleName + "_" + typeName
}

// generateTypeDeclaration generates a type declaration with name mangling
func generateTypeDeclaration(td *typeDecl, moduleName string) string {
	var sb strings.Builder

	switch td.kind {
	case "struct":
		if td.body == "" {
			// Forward declaration
			sb.WriteString(fmt.Sprintf("struct %s_%s;", moduleName, td.name))
		} else {
			// Full struct definition with typedef
			sb.WriteString(fmt.Sprintf("typedef struct %s_%s %s", moduleName, td.name, td.body))
			sb.WriteString(fmt.Sprintf(" %s_%s;", moduleName, td.name))
		}
	case "enum":
		// Enum definition with typedef
		sb.WriteString(fmt.Sprintf("typedef enum %s_%s %s", moduleName, td.name, td.body))
		sb.WriteString(fmt.Sprintf(" %s_%s;", moduleName, td.name))
	case "typedef":
		// Typedef - we need to parse out the name and mangle it
		sb.WriteString(fmt.Sprintf("typedef %s;", td.body))
	}

	return sb.String()
}

// generateFunctionImplementation generates a complete C function implementation
func generateFunctionImplementation(fn *parser.FuncDecl, moduleName string, importMap transform.ImportMap, srcPath string) string {
	var sb strings.Builder

	// Add line directive for source mapping
	// Note: We'd need to track actual line numbers from parser
	// For now, just emit the function

	// Function signature
	sb.WriteString(generateFunctionSignature(fn, moduleName))
	sb.WriteString(" ")

	// Transform function body to replace qualified access with mangled names
	transformedBody := transform.TransformFunctionBody(fn.Body, importMap)
	sb.WriteString(transformedBody)

	return sb.String()
}

// sanitizeModuleName converts import path to safe filename
func sanitizeModuleName(importPath string) string {
	// Replace slashes with underscores
	return strings.ReplaceAll(importPath, "/", "_")
}
