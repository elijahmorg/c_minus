package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elijahmorgan/c_minus/internal/parser"
	"github.com/elijahmorgan/c_minus/internal/paths"
	"github.com/elijahmorgan/c_minus/internal/project"
	"github.com/elijahmorgan/c_minus/internal/transform"
)

// GenerateModule generates .h and .c files for a module
func GenerateModule(mod *project.ModuleInfo, files []*parser.File, buildDir string) error {
	moduleName := paths.SanitizeModuleName(mod.ImportPath)

	// First pass: collect all type names in this module for later qualification
	typeNames := make(map[string]bool)
	// Also collect enum values for function body transformation
	enumValues := make(transform.EnumValueMap)
	// Also collect global variable names for function body transformation
	globalVars := make(transform.GlobalVarMap)
	// Also collect #define constant names for function body transformation
	defines := make(transform.DefineMap)
	for _, file := range files {
		for _, decl := range file.Decls {
			if decl.Struct != nil {
				typeNames[decl.Struct.Name] = true
			} else if decl.Union != nil {
				typeNames[decl.Union.Name] = true
			} else if decl.Enum != nil {
				typeNames[decl.Enum.Name] = true
				// Extract enum values from the body
				extractEnumValues(decl.Enum.Body, decl.Enum.Name, moduleName, enumValues)
			} else if decl.Global != nil && !decl.Global.Static {
				// Map non-static global variable name to mangled name
				// Static globals are file-local and not mangled
				globalVars[decl.Global.Name] = moduleName + "_" + decl.Global.Name
			} else if decl.Define != nil && decl.Define.Public {
				// Only public defines get mangled; private ones keep their original names
				defines[decl.Define.Name] = moduleName + "_" + decl.Define.Name
			}
		}
	}

	// Collect all public and private declarations
	publicFuncDecls := []*funcDeclInfo{}
	privateFuncDecls := []*funcDeclInfo{}
	publicTypeDecls := []*typeDecl{}
	privateTypeDecls := []*typeDecl{}
	publicGlobalDecls := []*globalDecl{}
	privateGlobalDecls := []*globalDecl{}
	publicDefineDecls := []*defineDecl{}
	privateDefineDecls := []*defineDecl{}

	for _, file := range files {
		for _, decl := range file.Decls {
			if decl.Function != nil {
				funcSig := generateFunctionSignature(decl.Function, moduleName)
				funcInfo := &funcDeclInfo{
					signature:  funcSig,
					docComment: decl.Function.DocComment,
				}
				if decl.Function.Public {
					publicFuncDecls = append(publicFuncDecls, funcInfo)
				} else {
					privateFuncDecls = append(privateFuncDecls, funcInfo)
				}
			} else if decl.Struct != nil {
				// Transform the struct body to qualify type references
				transformedBody := transformTypeBody(decl.Struct.Body, typeNames, moduleName)
				typeDecl := &typeDecl{
					kind:       "struct",
					name:       decl.Struct.Name,
					body:       transformedBody,
					public:     decl.Struct.Public,
					docComment: decl.Struct.DocComment,
				}
				if decl.Struct.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Union != nil {
				// Transform the union body to qualify type references
				transformedBody := transformTypeBody(decl.Union.Body, typeNames, moduleName)
				typeDecl := &typeDecl{
					kind:       "union",
					name:       decl.Union.Name,
					body:       transformedBody,
					public:     decl.Union.Public,
					docComment: decl.Union.DocComment,
				}
				if decl.Union.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Enum != nil {
				// Transform enum body to qualify enum values
				transformedBody := transformEnumBody(decl.Enum.Body, decl.Enum.Name, moduleName)
				typeDecl := &typeDecl{
					kind:       "enum",
					name:       decl.Enum.Name,
					body:       transformedBody,
					public:     decl.Enum.Public,
					docComment: decl.Enum.DocComment,
				}
				if decl.Enum.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Typedef != nil {
				typeDecl := &typeDecl{
					kind:       "typedef",
					body:       decl.Typedef.Body,
					public:     decl.Typedef.Public,
					docComment: decl.Typedef.DocComment,
				}
				if decl.Typedef.Public {
					publicTypeDecls = append(publicTypeDecls, typeDecl)
				} else {
					privateTypeDecls = append(privateTypeDecls, typeDecl)
				}
			} else if decl.Global != nil {
				gd := &globalDecl{
					typeName:   decl.Global.Type,
					name:       decl.Global.Name,
					value:      decl.Global.Value,
					public:     decl.Global.Public,
					static:     decl.Global.Static,
					docComment: decl.Global.DocComment,
				}
				// Static globals are file-local, don't add to header lists
				if decl.Global.Static {
					// Static globals are handled per-file in generateCFile
				} else if decl.Global.Public {
					publicGlobalDecls = append(publicGlobalDecls, gd)
				} else {
					privateGlobalDecls = append(privateGlobalDecls, gd)
				}
			} else if decl.Define != nil {
				dd := &defineDecl{
					name:       decl.Define.Name,
					value:      decl.Define.Value,
					public:     decl.Define.Public,
					docComment: decl.Define.DocComment,
				}
				if decl.Define.Public {
					publicDefineDecls = append(publicDefineDecls, dd)
				} else {
					privateDefineDecls = append(privateDefineDecls, dd)
				}
			}
		}
	}

	// Collect all imports from all files in the module
	allImports := make(map[string]bool)
	for _, file := range files {
		for _, imp := range file.Imports {
			allImports[imp.Path] = true
		}
	}

	// Generate public header
	if err := generatePublicHeader(mod, publicTypeDecls, publicFuncDecls, publicGlobalDecls, publicDefineDecls, allImports, buildDir); err != nil {
		return err
	}

	// Generate internal header (always, even if empty - C files include it)
	if err := generateInternalHeader(mod, privateTypeDecls, privateFuncDecls, privateGlobalDecls, privateDefineDecls, buildDir); err != nil {
		return err
	}

	// Generate .c files for each source file
	for i, file := range files {
		if err := generateCFile(mod, file, mod.Files[i], buildDir, enumValues, globalVars, defines); err != nil {
			return err
		}
	}

	return nil
}

// typeDecl represents a type declaration for code generation
type typeDecl struct {
	kind       string // "struct", "union", "enum", or "typedef"
	name       string // type name (for struct/union/enum)
	body       string // opaque body content
	public     bool
	docComment string // Go-style doc comment
}

// globalDecl represents a global variable declaration for code generation
type globalDecl struct {
	typeName   string // e.g., "int", "char*", "const char*"
	name       string
	value      string // Initial value (optional)
	public     bool
	static     bool // File-private (static keyword in C)
	docComment string
}

// defineDecl represents a #define constant for code generation
type defineDecl struct {
	name       string
	value      string
	public     bool
	docComment string
}

// funcDeclInfo represents a function declaration for code generation
type funcDeclInfo struct {
	signature  string // The C function signature
	docComment string // Go-style doc comment
}

// generatePublicHeader generates the public .h file for a module
func generatePublicHeader(mod *project.ModuleInfo, publicTypes []*typeDecl, publicFuncs []*funcDeclInfo, publicGlobals []*globalDecl, publicDefines []*defineDecl, imports map[string]bool, buildDir string) error {
	moduleName := paths.SanitizeModuleName(mod.ImportPath)
	guardName := strings.ToUpper(moduleName) + "_H"

	var sb strings.Builder

	// Include guard
	sb.WriteString(fmt.Sprintf("#ifndef %s\n", guardName))
	sb.WriteString(fmt.Sprintf("#define %s\n\n", guardName))

	// Include headers for imported modules (needed for types used in function signatures)
	for imp := range imports {
		importName := paths.SanitizeModuleName(imp)
		sb.WriteString(fmt.Sprintf("#include \"%s.h\"\n", importName))
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}

	// Public #define constants (mangled with module prefix)
	for _, dd := range publicDefines {
		if dd.docComment != "" {
			sb.WriteString(formatDocComment(dd.docComment))
		}
		sb.WriteString(fmt.Sprintf("#define %s_%s %s\n", moduleName, dd.name, dd.value))
	}
	if len(publicDefines) > 0 {
		sb.WriteString("\n")
	}

	// Forward declarations for all structs and unions (to handle dependencies)
	for _, td := range publicTypes {
		if td.kind == "struct" && td.body != "" {
			sb.WriteString(fmt.Sprintf("struct %s_%s;\n", moduleName, td.name))
		} else if td.kind == "union" && td.body != "" {
			sb.WriteString(fmt.Sprintf("union %s_%s;\n", moduleName, td.name))
		}
	}
	if len(publicTypes) > 0 {
		sb.WriteString("\n")
	}

	// Public type declarations
	for _, td := range publicTypes {
		sb.WriteString(generateTypeDeclaration(td, moduleName))
		sb.WriteString("\n\n")
	}

	// Public global variable declarations (extern)
	for _, gd := range publicGlobals {
		if gd.docComment != "" {
			sb.WriteString(formatDocComment(gd.docComment))
		}
		// In header, emit as extern declaration
		sb.WriteString(fmt.Sprintf("extern %s %s_%s;\n\n", gd.typeName, moduleName, gd.name))
	}

	// Public function declarations
	for _, decl := range publicFuncs {
		if decl.docComment != "" {
			sb.WriteString(formatDocComment(decl.docComment))
		}
		sb.WriteString(decl.signature)
		sb.WriteString(";\n\n")
	}

	sb.WriteString("#endif\n")

	// Write to file
	headerPath := filepath.Join(buildDir, moduleName+".h")
	if err := os.WriteFile(headerPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", headerPath, err)
	}

	return nil
}

// generateInternalHeader generates the internal _internal.h file for a module
func generateInternalHeader(mod *project.ModuleInfo, privateTypes []*typeDecl, privateFuncs []*funcDeclInfo, privateGlobals []*globalDecl, privateDefines []*defineDecl, buildDir string) error {
	moduleName := paths.SanitizeModuleName(mod.ImportPath)
	guardName := strings.ToUpper(moduleName) + "_INTERNAL_H"

	var sb strings.Builder

	// Include guard
	sb.WriteString(fmt.Sprintf("#ifndef %s\n", guardName))
	sb.WriteString(fmt.Sprintf("#define %s\n\n", guardName))

	// Include public header
	sb.WriteString(fmt.Sprintf("#include \"%s.h\"\n\n", moduleName))

	// Private #define constants (not mangled - module-internal only)
	for _, dd := range privateDefines {
		if dd.docComment != "" {
			sb.WriteString(formatDocComment(dd.docComment))
		}
		sb.WriteString(fmt.Sprintf("#define %s %s\n", dd.name, dd.value))
	}
	if len(privateDefines) > 0 {
		sb.WriteString("\n")
	}

	// Forward declarations for private structs and unions
	for _, td := range privateTypes {
		if td.kind == "struct" && td.body != "" {
			sb.WriteString(fmt.Sprintf("struct %s_%s;\n", moduleName, td.name))
		} else if td.kind == "union" && td.body != "" {
			sb.WriteString(fmt.Sprintf("union %s_%s;\n", moduleName, td.name))
		}
	}
	if len(privateTypes) > 0 {
		sb.WriteString("\n")
	}

	// Private type declarations
	for _, td := range privateTypes {
		sb.WriteString(generateTypeDeclaration(td, moduleName))
		sb.WriteString("\n\n")
	}

	// Private global variable declarations (extern for internal header)
	for _, gd := range privateGlobals {
		if gd.docComment != "" {
			sb.WriteString(formatDocComment(gd.docComment))
		}
		// In internal header, emit as extern (definition is in .c file)
		sb.WriteString(fmt.Sprintf("extern %s %s_%s;\n\n", gd.typeName, moduleName, gd.name))
	}

	// Private function declarations
	for _, decl := range privateFuncs {
		if decl.docComment != "" {
			sb.WriteString(formatDocComment(decl.docComment))
		}
		sb.WriteString(decl.signature)
		sb.WriteString(";\n\n")
	}

	sb.WriteString("#endif\n")

	// Write to file
	headerPath := filepath.Join(buildDir, moduleName+"_internal.h")
	if err := os.WriteFile(headerPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", headerPath, err)
	}

	return nil
}

// generateCFile generates a .c implementation file
func generateCFile(mod *project.ModuleInfo, file *parser.File, srcPath string, buildDir string, enumValues transform.EnumValueMap, globalVars transform.GlobalVarMap, defines transform.DefineMap) error {
	moduleName := paths.SanitizeModuleName(mod.ImportPath)
	baseName := filepath.Base(srcPath)
	baseName = baseName[:len(baseName)-3] // Remove .cm extension

	// Build import map for qualified access transformation
	importMap, err := transform.BuildImportMap(file.Imports)
	if err != nil {
		return fmt.Errorf("failed to build import map for %s: %w", srcPath, err)
	}

	// Build C import map for C header access transformation
	cimportMap, err := transform.BuildCImportMap(file.CImports)
	if err != nil {
		return fmt.Errorf("failed to build cimport map for %s: %w", srcPath, err)
	}

	var sb strings.Builder

	// Include internal header (which includes public header)
	sb.WriteString(fmt.Sprintf("#include \"%s_internal.h\"\n", moduleName))

	// Include C standard library headers (cimports)
	for _, cimp := range file.CImports {
		sb.WriteString(fmt.Sprintf("#include <%s>\n", cimp.Path))
	}

	// Include c_minus dependency headers
	for _, imp := range file.Imports {
		importName := paths.SanitizeModuleName(imp.Path)
		sb.WriteString(fmt.Sprintf("#include \"%s.h\"\n", importName))
	}

	sb.WriteString("\n")

	// Emit global variable definitions
	for _, decl := range file.Decls {
		if decl.Global != nil {
			// Add #line directive for source mapping
			if decl.Global.Line > 0 {
				sb.WriteString(fmt.Sprintf("#line %d \"%s\"\n", decl.Global.Line, srcPath))
			}
			globalDef := generateGlobalDefinition(decl.Global, moduleName)
			sb.WriteString(globalDef)
			sb.WriteString("\n\n")
		}
	}

	// Emit function implementations
	for _, decl := range file.Decls {
		if decl.Function != nil {
			funcImpl := generateFunctionImplementation(decl.Function, moduleName, importMap, cimportMap, enumValues, globalVars, defines, srcPath)
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

// generateGlobalDefinition generates a global variable definition for a .c file
func generateGlobalDefinition(g *parser.GlobalDecl, moduleName string) string {
	var sb strings.Builder

	// Static globals: use static keyword, no name mangling
	if g.Static {
		sb.WriteString("static ")
		sb.WriteString(g.Type)
		sb.WriteString(" ")
		sb.WriteString(g.Name)
	} else {
		// Type and mangled name
		sb.WriteString(g.Type)
		sb.WriteString(" ")
		sb.WriteString(moduleName)
		sb.WriteString("_")
		sb.WriteString(g.Name)
	}

	// Optional initializer
	if g.Value != "" {
		sb.WriteString(" = ")
		sb.WriteString(g.Value)
	}

	sb.WriteString(";")

	return sb.String()
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

		// Handle variadic parameter
		if param.Type == "..." {
			sb.WriteString("...")
			continue
		}

		// Transform parameter type: mangle non-primitive types with module prefix
		paramType := mangleTypeInSignature(param.Type, moduleName)

		// Check if this is a function pointer type (contains "(*)")
		// For function pointers, the name goes inside: "int (*name)(args)"
		if strings.Contains(paramType, "(*)") {
			// Insert the name after (*
			paramStr := strings.Replace(paramType, "(*)", "(*"+param.Name+")", 1)
			sb.WriteString(paramStr)
		} else {
			sb.WriteString(paramType)
			sb.WriteString(" ")
			sb.WriteString(param.Name)
		}
	}
	sb.WriteString(")")

	return sb.String()
}

// mangleTypeInSignature mangles custom type names in function signatures
// Primitive C types are left unchanged
// Handles qualified types like "module.Type" -> "module_Type"
func mangleTypeInSignature(typeName string, moduleName string) string {
	// Common primitive types - don't mangle these
	primitives := map[string]bool{
		"void":      true,
		"char":      true,
		"short":     true,
		"int":       true,
		"long":      true,
		"float":     true,
		"double":    true,
		"unsigned":  true,
		"signed":    true,
		"size_t":    true,
		"ssize_t":   true,
		"int8_t":    true,
		"int16_t":   true,
		"int32_t":   true,
		"int64_t":   true,
		"uint8_t":   true,
		"uint16_t":  true,
		"uint32_t":  true,
		"uint64_t":  true,
		"intptr_t":  true,
		"uintptr_t": true,
		"ptrdiff_t": true,
		// Also handle pointer types
	}

	// Check for pointers
	if strings.HasSuffix(typeName, "*") {
		// Strip pointer, mangle base type, re-add pointer
		baseType := strings.TrimRight(typeName, "*")
		asterisks := typeName[len(baseType):]
		return mangleTypeInSignature(baseType, moduleName) + asterisks
	}

	// Check for struct/union/enum keywords
	if strings.HasPrefix(typeName, "struct ") {
		return typeName // Already has struct keyword
	}
	if strings.HasPrefix(typeName, "union ") {
		return typeName // Already has union keyword
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

	// Check for qualified type (e.g., "module.Type")
	if strings.Contains(typeName, ".") {
		// Split on dot: "ticket.Ticket" -> ["ticket", "Ticket"]
		dotParts := strings.SplitN(typeName, ".", 2)
		if len(dotParts) == 2 {
			// Return qualified module_Type format
			return dotParts[0] + "_" + dotParts[1]
		}
	}

	// Custom type - mangle it with current module prefix
	return moduleName + "_" + typeName
}

// generateTypeDeclaration generates a type declaration with name mangling
func generateTypeDeclaration(td *typeDecl, moduleName string) string {
	var sb strings.Builder

	// Add doc comment if present
	if td.docComment != "" {
		sb.WriteString(formatDocComment(td.docComment))
	}

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
	case "union":
		if td.body == "" {
			// Forward declaration
			sb.WriteString(fmt.Sprintf("union %s_%s;", moduleName, td.name))
		} else {
			// Full union definition with typedef
			sb.WriteString(fmt.Sprintf("typedef union %s_%s %s", moduleName, td.name, td.body))
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
func generateFunctionImplementation(fn *parser.FuncDecl, moduleName string, importMap transform.ImportMap, cimportMap transform.CImportMap, enumValues transform.EnumValueMap, globalVars transform.GlobalVarMap, defines transform.DefineMap, srcPath string) string {
	var sb strings.Builder

	// Add #line directive for source mapping (maps C errors back to .cm file)
	if fn.Line > 0 && srcPath != "" {
		sb.WriteString(fmt.Sprintf("#line %d \"%s\"\n", fn.Line, srcPath))
	}

	// Function signature
	sb.WriteString(generateFunctionSignature(fn, moduleName))
	sb.WriteString(" ")

	// Transform function body to replace qualified access with mangled names
	// Also transform C imports (stdio.printf -> printf), enum values, global variables, and defines
	transformedBody := transform.TransformFunctionBodyFull(fn.Body, importMap, cimportMap, enumValues, globalVars, defines)
	sb.WriteString(transformedBody)

	return sb.String()
}

// extractEnumValues extracts enum value names from an enum body and adds them to the map
// For enum body like "{ TODO, IN_PROGRESS, DONE }", it adds entries like:
// "TODO" -> "module_EnumName_TODO"
func extractEnumValues(body, enumName, moduleName string, enumValues transform.EnumValueMap) {
	// Find the opening and closing braces
	startBrace := strings.Index(body, "{")
	endBrace := strings.LastIndex(body, "}")
	if startBrace == -1 || endBrace == -1 || startBrace >= endBrace {
		return
	}

	prefix := moduleName + "_" + enumName + "_"
	inner := body[startBrace+1 : endBrace]

	// Split on commas and extract each value name
	values := strings.Split(inner, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		// Handle values with explicit assignments like "FOO = 1"
		if eqIdx := strings.Index(v, "="); eqIdx != -1 {
			v = strings.TrimSpace(v[:eqIdx])
		}
		if v != "" {
			enumValues[v] = prefix + v
		}
	}
}

// transformTypeBody transforms type references within a struct body
// Qualifies references to module-local types (enums, structs) with the module prefix
func transformTypeBody(body string, typeNames map[string]bool, moduleName string) string {
	if len(typeNames) == 0 {
		return body
	}

	result := body
	for typeName := range typeNames {
		// Look for the type name as a standalone identifier (not part of another identifier)
		// Match patterns like "Type " or "Type;" at field type positions
		result = replaceTypeInBody(result, typeName, moduleName+"_"+typeName)
	}
	return result
}

// replaceTypeInBody replaces type references in a struct body with qualified names
// Handles patterns like "TypeName fieldname;" where TypeName is a type reference
func replaceTypeInBody(body, typeName, replacement string) string {
	var result strings.Builder
	i := 0

	for i < len(body) {
		// Check if we're at the start of the type name
		if i+len(typeName) <= len(body) && body[i:i+len(typeName)] == typeName {
			// Check that this is a standalone identifier:
			// - character before is not alphanumeric or underscore (or we're at start)
			// - character after is not alphanumeric or underscore
			before := i == 0 || !isIdentChar(rune(body[i-1]))
			after := i+len(typeName) >= len(body) || !isIdentChar(rune(body[i+len(typeName)]))

			if before && after {
				result.WriteString(replacement)
				i += len(typeName)
				continue
			}
		}
		result.WriteByte(body[i])
		i++
	}

	return result.String()
}

// isIdentChar returns true if the character can be part of an identifier
func isIdentChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// transformEnumBody transforms enum values to have the module_EnumName_ prefix
func transformEnumBody(body, enumName, moduleName string) string {
	// Parse enum body like "{ TODO, IN_PROGRESS, DONE }"
	// Transform to "{ module_EnumName_TODO, module_EnumName_IN_PROGRESS, module_EnumName_DONE }"

	// Find the opening and closing braces
	startBrace := strings.Index(body, "{")
	endBrace := strings.LastIndex(body, "}")
	if startBrace == -1 || endBrace == -1 || startBrace >= endBrace {
		return body
	}

	prefix := moduleName + "_" + enumName + "_"
	inner := body[startBrace+1 : endBrace]

	// Split on commas and transform each value
	values := strings.Split(inner, ",")
	var transformed []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		// Handle values with explicit assignments like "FOO = 1"
		if eqIdx := strings.Index(v, "="); eqIdx != -1 {
			name := strings.TrimSpace(v[:eqIdx])
			rest := v[eqIdx:]
			transformed = append(transformed, prefix+name+rest)
		} else {
			transformed = append(transformed, prefix+v)
		}
	}

	return "{\n    " + strings.Join(transformed, ",\n    ") + "\n}"
}

// formatDocComment formats a doc comment for C output.
// It converts the internal representation (newline-separated lines)
// into a C-style comment block.
func formatDocComment(comment string) string {
	if comment == "" {
		return ""
	}

	lines := strings.Split(comment, "\n")
	var sb strings.Builder

	// Use C-style block comments for multi-line, // for single-line
	if len(lines) == 1 {
		sb.WriteString("// ")
		sb.WriteString(lines[0])
		sb.WriteString("\n")
	} else {
		sb.WriteString("/*\n")
		for _, line := range lines {
			sb.WriteString(" * ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString(" */\n")
	}

	return sb.String()
}
