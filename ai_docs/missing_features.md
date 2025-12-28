# Missing Features for Porting C Projects to C-minus

This document identifies features needed to **port** existing C projects to C-minus. Porting means rewriting C code in C-minus syntax, not automatic conversion or parsing existing C verbatim.

## Key Insight: Porting vs. Automatic Conversion

C-minus is designed to **eliminate** C's worst features (preprocessor, headers, include hell). When porting:

- We rewrite code in C-minus syntax
- We replace preprocessor patterns with C-minus equivalents
- We do NOT need to parse arbitrary C code

This significantly reduces what's actually necessary.

---

## Critical Missing Features

### 1. Build Tags (File-Based Conditional Compilation)

**Status**: IMPLEMENTED (commit 238cdc6)

**Purpose**: Replace `#ifdef` for platform-specific and feature-specific code

**Design** (Go-style):

```c
// +build linux darwin
// +build amd64

module "platform"

pub func get_page_size() int {
    return 4096;
}
```

**Rules**:
- Tags appear at top of file, before `module` declaration
- Comma within a tag = OR: `// +build linux,darwin` means linux OR darwin
- Multiple lines = AND: two `// +build` lines both must match
- Special tag `!` for negation: `// +build !windows`

**Built-in Tags**:
- OS: `linux`, `darwin`, `windows`, `freebsd`
- Arch: `amd64`, `arm64`, `arm`, `386`
- Build mode: `debug`, `release`

**Custom Tags**:
```bash
c_minus build -tags=feature_x,experimental
```

**Example - Platform Abstraction**:

```
platform/
    platform.cm         # Common interface (always compiled)
    platform_linux.cm   # // +build linux
    platform_darwin.cm  # // +build darwin
    platform_windows.cm # // +build windows
```

**Impact**: Critical - Replaces #ifdef patterns cleanly

---

### 2. Global Variables

**Status**: IMPLEMENTED (commit ba2b379)

**Syntax** (C-style with `pub` modifier):
```c
module "state"

// Public global
pub int error_count = 0;

// Module-private global
int last_error_code = 0;

// Uninitialized
pub char* buffer;

// With type qualifiers
pub const char* version = "1.0.0";
```

**Generated C**:
```c
// In state.h
extern int state_error_count;
extern char* state_buffer;
extern const char* state_version;

// In state.c
int state_error_count = 0;
static int state_last_error_code = 0;
char* state_buffer;
const char* state_version = "1.0.0";
```

**Impact**: Critical - Most C programs have module-level state

---

### 3. Constants (#define)

**Status**: IMPLEMENTED (commit e216a38)

Simple `#define` constants are fine and should be supported. The problem with C preprocessor isn't `#define PI 3.14` - it's macro abuse, include hell, and conditional compilation spaghetti.

**What's Supported**:
- Simple value constants: `#define MAX_SIZE 1024`
- String constants: `#define VERSION "1.0.0"`
- Expression constants: `#define TIMEOUT (30 * 1000)`

**What's NOT Supported**:
- Function-like macros: `#define MIN(a,b) ...` - use actual functions
- Multi-line macros with backslash continuation
- Token pasting: `##`
- Stringification: `#`
- Conditional compilation: `#ifdef` - use build tags

**Syntax**:
```c
module "fileio"

// Public - exported to header, name-mangled, qualified access required
pub #define MAX_PATH 4096
pub #define BUFFER_SIZE 1024
pub #define DEFAULT_MODE "rb"

// Module-private - not exported, not mangled
#define INTERNAL_CHUNK 512
```

**Usage** (qualified like everything else):
```c
module "main"

import "fileio"

func main() int {
    char path[fileio.MAX_PATH];       // Qualified access
    char buf[fileio.BUFFER_SIZE];
    return 0;
}
```

**Generated C**:
```c
// In fileio.h
#define fileio_MAX_PATH 4096
#define fileio_BUFFER_SIZE 1024
#define fileio_DEFAULT_MODE "rb"

// In fileio.c (private defines stay local, no mangling)
#define INTERNAL_CHUNK 512
```

**Transform**: `fileio.MAX_PATH` becomes `fileio_MAX_PATH` - same transform as functions and types.

**Impact**: Critical - Simple, familiar way to define constants

---

### 4. Static Storage Class

**Status**: IMPLEMENTED (commit 5049d9f)

**Purpose**: File-private globals and persistent local state

**Syntax**:
```c
module "singleton"

// File-private global (not exported, not visible to other files in module)
static int initialized = 0;

pub func get_instance() Instance* {
    static Instance inst;  // Persistent local
    if (!initialized) {
        init_instance(&inst);
        initialized = 1;
    }
    return &inst;
}
```

**Note**: Module-private (`var` without `pub`) is visible within the module. `static` is file-private only.

**Impact**: High - Common C pattern

---

### 5. Compiler and Linker Flags (#cgo Directives)

**Status**: IMPLEMENTED (commit 238cdc6)

**Purpose**: Pass flags to the C compiler and linker for external library integration.

**Design**: Follows Go's cgo approach - flags are declared in source files next to the code that needs them.

#### Per-File CFLAGS

CFLAGS are declared per-file and apply only to that file's compilation:

```c
// file: http/curl_wrapper.cm
module "http"

#cgo CFLAGS: -I/usr/local/include/curl
#cgo LDFLAGS: -lcurl

cimport "curl/curl.h"

pub func fetch(char* url) int {
    curl.CURL* handle = curl.curl_easy_init();
    // ...
}
```

```c
// file: http/ssl.cm
module "http"

#cgo CFLAGS: -I/usr/local/include/openssl
#cgo LDFLAGS: -lssl -lcrypto

cimport "openssl/ssl.h"
// ...
```

- `curl_wrapper.c` compiles with `-I/usr/local/include/curl`
- `ssl.c` compiles with `-I/usr/local/include/openssl`
- Each file only gets its own CFLAGS

#### LDFLAGS Aggregation

All LDFLAGS from all files across all modules are collected, deduplicated, and passed to the final link step:

```
gcc ... -o binary -lcurl -lssl -lcrypto
```

#### Platform-Specific Flags

Two styles supported (like Go):

**Inline style:**
```c
#cgo CFLAGS: -I/usr/local/include
#cgo linux LDFLAGS: -lpthread -ldl
#cgo darwin LDFLAGS: -framework Security
#cgo windows LDFLAGS: -lws2_32
```

**File-level with build tags:**
```c
// +build linux

// file: platform_linux.cm
module "platform"

#cgo LDFLAGS: -lpthread -ldl
```

#### Default Compiler Flags

C-minus provides sensible defaults:
- Warnings as errors (strict)
- Many warnings enabled
- Middle-ground optimization (fast compile, decent performance)

Defaults are configurable (exact flags TBD).

#### Build Modes

```bash
c_minus build           # Debug (default)
c_minus build --release # Optimized release build
```

#### Project-Level Overrides (cm.mod)

Base flags can be set in `cm.mod`. These append to defaults:

```
module "myproject"

cflags = "-Wno-unused-parameter"
ldflags = "-lm"
```

To replace defaults entirely (syntax TBD):
```
module "myproject"

cflags! = "-Wall -O2"  # Replaces defaults instead of appending
```

#### Build System Integration

In `internal/build/build.go`:

1. **Compile step** (`compileModule`): Each file gets its own CFLAGS
   ```go
   args := []string{"-c", cFile, "-o", oFile, "-I", buildDir}
   args = append(args, defaultCFlags...)
   args = append(args, fileCFlags...)  // From #cgo CFLAGS in this file
   ```

2. **Link step** (`linkBinary`): All LDFLAGS aggregated and deduplicated
   ```go
   args := oFiles
   args = append(args, "-o", outputPath)
   args = append(args, defaultLDFlags...)
   args = append(args, allCollectedLDFlags...)  // Deduplicated
   ```

**Impact**: Critical - Cannot use third-party C libraries without this

---

### 6. Unions

**Status**: IMPLEMENTED

**Syntax**:
```c
module "types"

pub union Value {
    int i;
    float f;
    char* s;
    char bytes[8];
};

pub union Register {
    uint32_t full;
    struct {
        uint16_t low;
        uint16_t high;
    } parts;
};
```

**Generated C**:
```c
typedef union types_Value {
    int i;
    float f;
    char* s;
    char bytes[8];
} types_Value;
```

**Impact**: High - Common for type punning, variant types, hardware registers

---

### 7. Function Pointers

**Status**: IMPLEMENTED

**In Typedefs**:
```c
module "callbacks"

pub typedef int (*CompareFunc)(void* a, void* b);
pub typedef void (*EventHandler)(int event_id, void* data);
```

**In Function Parameters**:
```c
pub func qsort(void* base, size_t n, size_t size, int (*cmp)(void*, void*)) void {
    // ...
}
```

**In Structs**:
```c
pub struct VTable {
    void (*init)(void* self);
    void (*destroy)(void* self);
    int (*process)(void* self, char* data);
};
```

**Impact**: High - Essential for callbacks, vtables, plugin architectures

---

## Should Have Features

### 8. Variadic Functions

**Status**: IMPLEMENTED (commit 6bada10)

**Syntax**:
```c
module "logging"

cimport "stdarg.h"
cimport "stdio.h"

pub func log(char* fmt, ...) void {
    stdarg.va_list args;
    stdarg.va_start(args, fmt);
    stdio.vprintf(fmt, args);
    stdarg.va_end(args);
}
```

**Impact**: Medium - Needed for printf-style APIs

#### Implementation Plan

**1. Parser Changes** (`internal/parser/parser.go`):

Update `parseParams()` to handle the `...` token:

```go
// In parseParams(), after splitting on commas:
for _, part := range parts {
    part = strings.TrimSpace(part)
    if part == "" {
        continue
    }
    
    // Handle variadic marker
    if part == "..." {
        params = append(params, &Param{
            Name: "",
            Type: "...",
        })
        continue
    }
    
    // ... existing parameter parsing ...
}
```

**2. FuncDecl Changes** (`internal/parser/parser.go`):

Option A (simple): Use special Param with `Type: "..."` and `Name: ""`
Option B (explicit): Add `IsVariadic bool` field to FuncDecl

Recommend Option A - simpler, no struct changes needed.

**3. Codegen Changes** (`internal/codegen/codegen.go`):

Update `generateFunctionSignature()` to handle variadic params:

```go
func generateFunctionSignature(fn *parser.FuncDecl, moduleName string) string {
    // ... existing code ...
    
    var params []string
    for _, param := range fn.Params {
        if param.Type == "..." {
            params = append(params, "...")
        } else {
            params = append(params, fmt.Sprintf("%s %s", transformedType, param.Name))
        }
    }
    
    // ... rest of function ...
}
```

**4. Test Cases**:

- Parse `func log(char* fmt, ...) void` - verify `...` captured as param
- Generate C: `void module_log(char* fmt, ...)` - verify output correct
- Full integration: variadic function that calls vprintf

**Files to Modify**:
- `internal/parser/parser.go` - `parseParams()` function
- `internal/codegen/codegen.go` - `generateFunctionSignature()` function
- `internal/parser/parser_test.go` - add variadic test cases
- `internal/codegen/codegen_test.go` - add variadic test cases

---

### 9. Bit Fields

**Status**: LIKELY WORKS (needs verification)

Since struct bodies are treated as opaque and passed through to C unchanged, bit field syntax should work automatically.

**Syntax**:
```c
module "hardware"

pub struct StatusRegister {
    unsigned int ready : 1;
    unsigned int error : 1;
    unsigned int mode : 3;
    unsigned int reserved : 27;
};
```

**Impact**: Medium - Common in embedded/systems code

---

### 10. Source Mapping (#line Directives)

**Status**: IMPLEMENTED (commit 3cb734a)

**Purpose**: Make compiler errors and debugger reference original .cm files

**Generated C**:
```c
#line 15 "math/vector.cm"
float math_dot(math_Vec3 a, math_Vec3 b) {
#line 16 "math/vector.cm"
    return a.x * b.x + a.y * b.y + a.z * b.z;
}
```

**Impact**: Medium - Improves debugging experience

---

## Features NOT Needed for Porting

These were in the original list but are unnecessary when porting:

| Feature | Why Not Needed |
|---------|----------------|
| **Function-like macros** | Use actual functions instead |
| **Macro token pasting/stringify** | Abuse that C-minus eliminates |
| **#ifdef/#ifndef** | Use build tags instead - cleaner, file-based |
| **Parser robustness for C** | Not parsing C, developers write new .cm files |
| **Multi-line comments** | `//` comments work fine |
| **Designated initializers** | Use explicit field assignment |
| **VLAs** | Use malloc/dynamic allocation |
| **_Generic** | Use explicit function names (add_int, add_float) |
| **_Atomic** | Use cimport for C11 atomics if needed |
| **inline keyword** | Compilers inline automatically |
| **CMake/Makefile integration** | Build standalone, link resulting .o files |
| **Module aliasing** | Convenience only, not blocking |
| **Documentation generation** | Nice to have, not blocking |
| **Extern declarations** | Accessing symbols not in public headers breaks encapsulation; use cimport for declared symbols |

---

## Porting Patterns

### #define Constants (Kept, With Module Scoping)

**Original C**:
```c
#define MAX_BUFFER 1024
#define PI 3.14159
```

**C-minus** (same syntax, just add `pub` for export):
```c
pub #define MAX_BUFFER 1024
pub #define PI 3.14159
```

**Usage from another module**:
```c
import "config"

func example() int {
    return config.MAX_BUFFER;  // Qualified access
}
```

### Replacing Function-like Macros (Use Real Functions)

**Original C**:
```c
#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define MAX(a, b) ((a) > (b) ? (a) : (b))
```

**C-minus** (function-like macros not supported - use functions):
```c
pub func min_int(int a, int b) int {
    return a < b ? a : b;
}

pub func max_int(int a, int b) int {
    return a > b ? a : b;
}
```

Modern compilers will inline these automatically.

### Replacing #ifdef Platform Code

**Original C**:
```c
#ifdef _WIN32
#include <windows.h>
void sleep_ms(int ms) { Sleep(ms); }
#else
#include <unistd.h>
void sleep_ms(int ms) { usleep(ms * 1000); }
#endif
```

**C-minus** (two files):

`time_windows.cm`:
```c
// +build windows

module "time"

cimport "windows.h"

pub func sleep_ms(int ms) void {
    windows.Sleep(ms);
}
```

`time_unix.cm`:
```c
// +build linux darwin freebsd

module "time"

cimport "unistd.h"

pub func sleep_ms(int ms) void {
    unistd.usleep(ms * 1000);
}
```

### Replacing Include Guards

**Original C**:
```c
#ifndef MYMODULE_H
#define MYMODULE_H
// ...
#endif
```

**C-minus**: Not needed - headers are auto-generated with guards.

---

## Implementation Priority

### Phase 2A: Essential for Any Real Project

1. **Build tags** - File-based conditional compilation
2. **Global variables** - Module-level state
3. **Constants** - Replace #define constants
4. **Static storage class** - File-private globals
5. **External linking** - cflags/ldflags in cm.mod

### Phase 2B: Common Patterns

6. **Unions** - Type punning, variants (IMPLEMENTED)
7. **Function pointers** - Callbacks, vtables (IMPLEMENTED)

### Phase 3: Polish

8. **Variadic functions** - Printf-style APIs
9. **Bit fields** - Embedded/hardware (likely already works)
10. **#line directives** - Better debugging

---

## Conclusion

**ALL FEATURES ARE NOW IMPLEMENTED.**

For **porting** C projects to C-minus, the essential features are:

1. **Build tags** - Clean replacement for #ifdef - DONE
2. **Global variables and constants** - Module-level state - DONE
3. **Static storage** - File-private globals - DONE
4. **External linking** - Third-party library support - DONE
5. **Unions and function pointers** - Common C patterns - DONE
6. **Variadic functions** - Printf-style APIs - DONE
7. **#line directives** - Source mapping for debugging - DONE

The preprocessor is intentionally NOT supported. Porting requires rewriting platform-specific code using build tags and replacing macros with constants/functions. This is the point of C-minus: eliminating preprocessor complexity while keeping C's runtime model.
