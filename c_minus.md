# C-minus Language Reference

C-minus transpiles `.cm` files to standard C. It provides Go-style modules while generating clean C that works with GCC/Clang.

## Project Structure

```
myproject/
├── cm.mod              # module "myproject"
├── main.cm             # module "main"
└── math/
    ├── vector.cm       # module "math"
    └── matrix.cm       # module "math"
```

**Rule**: All `.cm` files in a directory must declare the same module.

## Syntax

### Module and Imports

```c
module "main"
import "math"
import "utils/io"
```

Import prefix = last path segment: `"utils/io"` → `io`

### Functions

```c
pub func add(a int, b int) int {    // Public (exported)
    return a + b;
}

func helper() int {                  // Private (module-only)
    return 42;
}
```

### Types

```c
pub struct Vec3 {
    float x, y, z;
};

struct Cache { int count; };         // Private

pub enum State { IDLE, RUNNING };

pub typedef int Counter;
```

## Qualified Access

**All imported symbols must be prefixed with module name.**

```c
module "main"
import "math"

func main() int {
    math.Vec3 a = {1.0, 2.0, 3.0};   // Type: math.Vec3
    math.Vec3 b = {4.0, 5.0, 6.0};
    float r = math.dot(a, b);         // Function: math.dot()
    return (int)r;
}
```

Local symbols (same module) have no prefix:
```c
module "math"

pub struct Vec3 { float x, y, z; };

pub func dot(a Vec3, b Vec3) float {  // Vec3 is local
    return a.x * b.x + a.y * b.y + a.z * b.z;
}
```

Enum values:
```c
import "state"

state.State s = state.State.IDLE;
if (s == state.State.RUNNING) { }
```

## Generated Code

### Name Mangling

| C-minus | Generated C |
|---------|-------------|
| `math.Vec3` | `math_Vec3` |
| `math.dot()` | `math_dot()` |
| `io.read()` from `utils/io` | `utils_io_read()` |
| `state.State.IDLE` | `state_State_IDLE` |

Exception: `main()` is never mangled.

### Headers

**math.h** (public):
```c
#ifndef MATH_H
#define MATH_H

typedef struct math_Vec3 {
    float x, y, z;
} math_Vec3;

float math_dot(math_Vec3 a, math_Vec3 b);

#endif
```

**math_internal.h** (private):
```c
#ifndef MATH_INTERNAL_H
#define MATH_INTERNAL_H

#include "math.h"

int math_helper();

#endif
```

### Build Output

```
.c_minus/               # Generated files
  ├── math.h
  ├── math_internal.h
  ├── math_vector.c
  └── math.o
myproject               # Binary at root
```

## Function Bodies

C syntax, with transformations:

```c
// Source
func example() int {
    math.Vec3 v;
    return math.dot(v, v);
}

// Generated
int main_example() {
    math_Vec3 v;
    return math_dot(v, v);
}
```

All standard C works: `printf()`, pointers, casts, control flow, etc.

## Build

```bash
c_minus build           # Default
c_minus build -j 8      # Parallel jobs
c_minus build -o bin    # Custom output
```

## Complete Example

**cm.mod**:
```
module "myproject"
```

**math/vector.cm**:
```c
module "math"

pub struct Vec3 {
    float x, y, z;
};

pub func dot(a Vec3, b Vec3) float {
    return a.x * b.x + a.y * b.y + a.z * b.z;
}
```

**main.cm**:
```c
module "main"
import "math"

func main() int {
    math.Vec3 a = {1.0, 2.0, 3.0};
    math.Vec3 b = {4.0, 5.0, 6.0};
    float result = math.dot(a, b);
    return (int)result;  // 32
}
```

**Build**:
```bash
c_minus build
./myproject  # Exit code: 32
```

## Rules

1. **Qualified imports**: `math.dot()` not `dot()`
2. **Local symbols unqualified**: Types/functions in same module need no prefix
3. **No circular imports**: Enforced DAG
4. **Module = directory**: All files in dir must declare same module
5. **Opaque bodies**: Function/type bodies are C syntax, passed through verbatim

## Common Errors

```
use of undeclared identifier 'Vec3'
→ Use math.Vec3 (qualify imports)

call to undeclared function 'dot'
→ Use math.dot() (qualify imports)

circular dependency detected
→ Remove import cycle

no cm.mod found
→ Create cm.mod at project root
```

## What Works

- ✅ Struct, enum, typedef declarations
- ✅ Public/private visibility
- ✅ Qualified imports (Go-style)
- ✅ Automatic name mangling
- ✅ Automatic header generation
- ✅ All C syntax in function bodies
- ✅ Forward declarations
- ✅ Multi-file modules

## What Doesn't Work

- ❌ Import aliasing (`import m "math"`)
- ❌ Multiple return values
- ❌ Unqualified imports
- ❌ will never be supported - Circular dependencies
- ❌ Symbol validation in transpiler (fails at C compile time)

## For LLMs

When generating C-minus code:

1. Always qualify imports: `math.Vec3`, `math.dot()`
2. Local symbols need no prefix: `Vec3` within `module "math"`
3. Function bodies are standard C
4. Directory structure = module structure
5. `main()` is special (not mangled)
