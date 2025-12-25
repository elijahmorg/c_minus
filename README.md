# C-minus

*C minus all the pain.*

## Overview

A source-to-source transpiler that compiles C-minus (`.cm` files) into standard C. The goal is to fix C's worst developer experience problems—header files, dependency management, macro chaos—without changing the runtime model. Output remains standard C, preserving compatibility with existing toolchains, debuggers, and optimizing compilers (GCC, Clang).

---

## Current Features

### Module System

Go-style module system replacing the header/include model. Each source file declares its module and explicitly imports dependencies. The transpiler generates .h/.c pairs automatically—developers never write headers.

```
module "math"

import "utils"

pub func add(int a, int b) int {
    return a + b;
}
```

### No Header Files

Public declarations are marked with the `pub` keyword. The transpiler extracts these and emits headers. Implementation details stay private by default.

### No Circular Dependencies

Module imports form a DAG enforced at compile time. The transpiler builds a dependency graph, topologically sorts it, and rejects cycles.

### C Standard Library Imports

Import C standard library headers with qualified access to symbols:

```
module "main"

cimport "stdio.h"

func main() int {
    stdio.printf("Hello from C-minus!\n");
    return 0;
}
```

The `stdio.printf()` call transpiles to just `printf()` in the generated C.

### Type Declarations

Structs, enums, and typedefs are supported:

```
pub struct Vec3 {
    float x;
    float y;
    float z;
};

pub enum Status {
    TODO,
    IN_PROGRESS,
    DONE
};
```

### C-Style Function Parameters

Function parameters use C-style syntax (`type name`):

```
pub func dot(Vec3 a, Vec3 b) float {
    return a.x * b.x + a.y * b.y + a.z * b.z;
}

pub func process(Ticket* t) void {
    t->status = DONE;
}
```

### Build System

- Incremental builds (only recompiles changed files)
- Parallel compilation with `-j` flag
- Binary output at project root (Go convention)
- Intermediate files in `.c_minus/` directory

---

## Quick Start

### Project Structure

```
myproject/
├── cm.mod              # Project manifest
├── main.cm             # Main module
└── math/
    └── vector.cm       # "math" module
```

### cm.mod

```
module "github.com/user/myproject"
```

### main.cm

```
module "main"

import "math"

func main() int {
    math.Vec3 v;
    v.x = 1.0;
    v.y = 2.0;
    v.z = 3.0;
    return 0;
}
```

### Build and Run

```bash
c_minus build
./myproject
```

---

## Generated Output

Each module transpiles to a header/implementation pair:

```
math/vector.cm  →  .c_minus/math.h + .c_minus/math_vector.c
```

Generated headers contain:
- Include guards
- Public type definitions
- Public function declarations

Generated implementation files contain:
- Include of own header
- Includes of dependency headers
- Function implementations

---

## Future Features

### Multiple Return Values

Syntactic sugar for returning multiple values from functions:

```
(int result, bool ok) parse(char* s) {
    if (s == NULL) {
        return (0, false);
    }
    return (atoi(s), true);
}
```

Transpiles to out-parameters:

```c
void parse(char* s, int* result, bool* ok) {
    if (s == NULL) {
        *result = 0;
        *ok = false;
        return;
    }
    *result = atoi(s);
    *ok = true;
    return;
}
```

### Macro Restrictions

Macros cannot be exported from modules. Imported modules expose no macro definitions. This keeps preprocessing predictable and debuggable.

### Source Mapping

Emit `#line` directives in generated C so compiler errors and debugger stepping refer back to original source locations.

### External Dependencies

Support for external module dependencies via `cm.mod`:

```
module "github.com/user/myproject"

require (
    "github.com/other/lib" v1.2.0
)
```

### Better Error Diagnostics

Error messages with file:line:col context and multiple error collection before failing.

---

## Implementation

Written in Go. Uses a manual recursive descent parser with brace-balanced extraction for function bodies.

### Package Structure

```
cmd/c_minus/          # CLI
internal/
├── project/          # Project discovery and validation
├── parser/           # C-minus parser
├── codegen/          # Code generator
├── transform/        # Body transformations (qualified names, etc.)
└── build/            # Build orchestration
```

---

## Design Considerations

### Forward Declarations

Pointer-to-struct across modules isn't a circular dependency. The cycle detector distinguishes structural cycles from pointer-only references.

### Module Visibility

- `pub` - Exported, visible to importers (public header)
- No modifier - Module-private, visible within the same module (internal header)
