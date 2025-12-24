# C-minus

*C minus all the pain.*

## Overview

A source-to-source transpiler that compiles C-minus (`.cm` files) into standard C. The goal is to fix C's worst developer experience problems—header files, dependency management, macro chaos—without changing the runtime model. Output remains standard C, preserving compatibility with existing toolchains, debuggers, and optimizing compilers (GCC, Clang).

---

## Language Design

### Modules

Go-style module system replacing the header/include model. Each source file declares its module and explicitly imports dependencies. The transpiler generates .h/.c pairs automatically—developers never write headers.

### No Header Files

Public declarations are marked in source (e.g., `pub` keyword). The transpiler extracts these and emits headers. Implementation details stay private by default.

### No Circular Dependencies

Module imports form a DAG enforced at compile time. The transpiler builds a dependency graph, topologically sorts it, and rejects cycles. This eliminates the class of bugs caused by tangled include hierarchies.

### Macro Restrictions

Macros cannot be exported from modules. Imported modules expose no macro definitions. Deep macro nesting (macros expanding to macros) is restricted or prohibited. This keeps preprocessing predictable and debuggable.

### Multiple Return Values

Syntactic sugar for returning multiple values from functions:

```
(int result, bool ok) parse(str s)
```

Transpiles to out-parameters:

```c
void parse(str s, int *result, bool *ok);
```

Return statements in the body are expanded to pointer assignments:

```
(int result, bool ok) parse(str s) {
    if (s.len == 0) {
        return (0, false);
    }
    return (atoi(s.data), true);
}
```

Becomes:

```c
void parse(str s, int *result, bool *ok) {
    if (s.len == 0) {
        *result = 0;
        *ok = false;
        return;
    }
    *result = atoi(s.data);
    *ok = true;
    return;
}
```

All return values remain on the caller's stack—no heap allocation, no ownership transfer, no cleanup required. This is pure syntactic sugar over the standard C out-parameter convention.

---

## Implementation: Go + Participle

### Why Go

- Fast compilation and execution (within 10-20% of C for this workload)
- Excellent string handling and memory safety—no manual allocation during AST manipulation
- Rapid iteration during language design phase
- Strong standard library for file I/O, text processing, and tooling

### Why Participle

Participle is a Go parser generator that derives parsers from struct definitions via reflection. You define your AST as Go types with tagged fields; Participle builds a recursive descent parser at runtime.

**Key characteristics:**

- No code generation step—parser constructed at startup via reflection
- Grammar defined inline using struct tags with EBNF-like syntax
- Alternation via Go interfaces (parser tries each implementing type)
- Custom lexer support for token definitions
- Fast iteration: change struct, rerun program

**Example AST definition:**

```go
type File struct {
    Module  *ModuleDecl  `@@`
    Imports []*Import    `@@*`
    Decls   []Decl       `@@*`
}

type ModuleDecl struct {
    Name string `"module" @Ident`
}

type Import struct {
    Path string `"import" @String`
}
```

### Parsing Strategy

The language extensions live primarily at the file and signature level—module declarations, imports, function signatures with multiple returns. Most of the function body remains standard C, but the transpiler must parse return statements in functions with multiple return values to expand tuple returns into pointer assignments. This requires:

1. Identifying `return (expr1, expr2, ...);` statements
2. Parsing the tuple expressions
3. Rewriting to `*out1 = expr1; *out2 = expr2; return;`

This is not full C expression parsing—only return statements in multi-return functions need transformation. The rest of the body can still be captured via brace-balanced extraction and emitted verbatim.

---

## Transpiler Architecture

```
Source Files (.cm)
       │
       ▼
┌─────────────────────────────────────────────────┐
│  Pass 1: Scan imports, build dependency graph   │
│          Detect and reject circular deps        │
└─────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────┐
│  Pass 2: Parse each file                        │
│          Module decls, imports, signatures      │
│          Extract function bodies as raw text    │
└─────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────┐
│  Pass 3: Resolve types across modules           │
│          Validate macro restrictions            │
│          Type-check public interfaces           │
└─────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────┐
│  Pass 4: Emit .h/.c pairs                       │
│          Generate include guards                │
│          Synthesize return-value structs        │
│          Output in dependency order             │
└─────────────────────────────────────────────────┘
       │
       ▼
   Standard C (.h + .c)
       │
       ▼
   gcc / clang
```

---

## Output Structure

Each module transpiles to a header/implementation pair:

```
math/vector.cm  →  math_vector.h + math_vector.c
```

Generated headers contain:
- Include guards
- Forward declarations as needed
- Public type definitions
- Public function declarations
- Transformed multi-return function signatures (out-parameters)

Generated implementation files contain:
- Include of own header
- Includes of dependency headers
- Private declarations
- Function implementations (bodies emitted verbatim)

---

## Module System

Following Go's module model.

### Project Structure

A `cm.mod` file marks the project root and declares the module path:

```
module "github.com/user/myproject"
```

A **module is a directory**. All `.cm` files in a directory belong to the same module and share a namespace (like Go packages):

```
myproject/
├── cm.mod                  # project manifest (no code)
├── main.cm                 # module "main"
└── math/
    ├── vector.cm           # module "math"
    └── matrix.cm           # module "math" (same module)
```

### Import Resolution

Imports resolve to directories relative to the project root:

```
import "math"
```

Resolves to `<project_root>/math/*.cm`

### Dependency Graph

The transpiler scans all `.cm` files for `module` and `import` statements (fast regex/minimal parse), builds a dependency graph, topologically sorts it, and rejects cycles before full parsing begins.

```go
type ModuleGraph struct {
    RootPath   string                  // filesystem path to project root
    RootModule string                  // "github.com/user/myproject"
    Modules    map[string]*ModuleInfo  // import path -> info
}

type ModuleInfo struct {
    ImportPath string   // "math"
    DirPath    string   // filesystem path to module directory
    Files      []string // all .cm files in the module
    Imports    []string
    External   bool     // false for local, true for external deps
}
```

### External Dependencies (Future)

The initial implementation only supports local imports. The design accommodates future external dependency support via a `require` block in `cm.mod`:

```
module "github.com/user/myproject"

require (
    "github.com/other/lib" v1.2.0
)
```

External imports would resolve from a module cache (similar to Go's `GOMODCACHE`).

---

## Design Considerations

### Forward Declarations

Pointer-to-struct across modules isn't a circular dependency (no implementation coupling), but requires careful header ordering or forward declarations. The cycle detector should distinguish structural cycles from pointer-only references.

### Source Mapping

Emit `#line` directives in generated C so compiler errors and debugger stepping refer back to original source locations.

### Error Diagnostics

Participle provides token positions. Wrap errors with file:line:col context. Add a semantic validation pass that collects multiple errors with source locations before failing.
