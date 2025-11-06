# Odin Programming Language: Error Handling Patterns and Best Practices

## Table of Contents
1. [Overview](#overview)
2. [Error Enum Patterns](#error-enum-patterns)
3. [Multiple Return Value Patterns](#multiple-return-value-patterns)
4. [or_return Operator Usage](#or_return-operator-usage)
5. [Error Wrapping and Context Patterns](#error-wrapping-and-context-patterns)
6. [Best Practices for CLI Applications](#best-practices-for-cli-applications)
7. [Common Pitfalls to Avoid](#common-pitfalls-to-avoid)
8. [Real-World Examples](#real-world-examples)
9. [Resources](#resources)

---

## Overview

Odin uses **plain error handling through multiple return values** rather than exceptions. This design decision makes it explicit which procedure an error value comes from, promoting clarity and maintainability.

### Core Philosophy

> "Treat errors like any other piece of code. Handle errors there and then and don't pass them up the stack. You make your mess; you clean it."
>
> — Ginger Bill (Odin creator)

**Key Principles:**
- No traditional software exceptions
- Explicit error handling at the point of occurrence
- Use the type system (unions, enums, distinct types) to your advantage
- Errors are first-class citizens in code

**Official Documentation:** https://odin-lang.org/docs/overview/

---

## Error Enum Patterns

### 1. Simple Error Enum

```odin
Error :: enum {
    None,
    Account_Is_Empty,
    Investment_Lost,
    Permission_Denied,
}

run :: proc() -> (err: Error) {
    // code here
    if some_condition {
        return .Permission_Denied
    }
    return .None
}
```

### 2. Detailed Error Enum (core:os)

The `core:os` package uses a comprehensive enum for common errors:

```odin
General_Error :: enum u32 {
    None,
    Permission_Denied,
    Exist,
    Not_Exist,
    Closed,
    Timeout,
    Broken_Pipe,
    No_Size,
    Invalid_File,
    Invalid_Dir,
    Invalid_Path,
    // ... more error types
}
```

### 3. Error Union Types

Union types allow composing different error categories:

```odin
Error :: union #shared_nil {
    General_Error,
    io.Error,
    runtime.Allocator_Error,
}
```

The `#shared_nil` directive allows different error types to share a `nil` representation, making error checking more convenient.

### 4. CLI-Specific Error Pattern (core:flags)

From the official flags parsing library:

```odin
Error :: union {
    Parse_Error,
    Open_File_Error,
    Help_Request,
    Validation_Error,
}

Parse_Error :: struct {
    reason: Parse_Error_Reason,
    message: string,
}

Parse_Error_Reason :: enum {
    Extra_Positional,
    Bad_Value,
    No_Flag,
    No_Value,
    Missing_Flag,
    Unsupported_Type,
}
```

### 5. Network Error Union (core:net)

```odin
Network_Error :: union #shared_nil {
    Create_Socket_Error,
    Dial_Error,
    Listen_Error,
    Accept_Error,
    Bind_Error,
    TCP_Send_Error,
    UDP_Send_Error,
    TCP_Recv_Error,
    UDP_Recv_Error,
    Shutdown_Error,
    Interfaces_Error,
    Socket_Option_Error,
    Set_Blocking_Error,
    Parse_Endpoint_Error,
    Resolve_Error,
    DNS_Error,
}
```

**Best Practice:** Use enum for simple error sets, union types for composing error categories from different domains.

---

## Multiple Return Value Patterns

### Pattern 1: Boolean Success Indicator

```odin
read_file :: proc(filename: string) -> (string, bool) {
    if filename == "" {
        return "", false  // Error case
    }
    return "file contents", true  // Success case
}

// Usage
content, success := read_file("myfile.txt")
if !success {
    fmt.println("Failed to read file")
    return
}
```

### Pattern 2: Error Enum Return

```odin
foo :: proc() -> (result: int, err: Error) {
    x := some_procedure() or_return
    result = x
    return
}

// Usage
value, err := foo()
if err != .None {
    fmt.eprintln("Error:", err)
    return
}
```

### Pattern 3: Named Return Values

```odin
foo_2 :: proc() -> (n: int, err: Error) {
    x, err = caller_2()
    if err != nil {
        return  // named returns allow early return without specifying values
    }
    n = x * 2
    return
}
```

### Pattern 4: Union Error Returns

```odin
Error :: union {
    ValueError,
    BarError,
    BazError,
}

foo :: proc() -> (Value_Type, Error) {
    // implementation
}

// Usage with type switch
x, err := foo()
switch e in err {
case ValueError:
    // Handle ValueError specifically
case BarError:
    // Handle BarError specifically
case BazError:
    // Handle BazError specifically
}
```

---

## or_return Operator Usage

The `or_return` operator is Odin's primary mechanism for error propagation. It checks the last value in a multi-valued expression and returns early if it's `nil` or `false`.

### Basic Usage

```odin
parse_number :: proc(s: string) -> (int, bool) {
    if s == "42" {
        return 42, true
    }
    return 0, false
}

example_with_error_handling :: proc() -> bool {
    // or_return automatically returns false if the second value is false
    num := parse_number("42") or_return
    fmt.printf("Parsed number: %d\n", num)
    return true
}
```

### With Error Enums

```odin
caller_2 :: proc() -> (int, Error) {
    // some operation
}

foo :: proc() -> (n: int, err: Error) {
    // Without or_return
    n0, err := caller_2()
    if err != nil {
        return 0, err
    }

    // With or_return (equivalent to above)
    n1 := caller_2() or_return

    return n1, nil
}
```

### Named Returns Requirement

When using `or_return` in procedures with multiple return values, you **must** use named return values:

```odin
// CORRECT
foo :: proc() -> (result: int, err: Error) {
    x := some_procedure() or_return
    result = x
    return
}

// INCORRECT - won't compile without named returns
foo :: proc() -> (int, Error) {
    x := some_procedure() or_return  // ERROR: needs named returns
    return x, nil
}
```

### Related Operators

#### or_else
Provides default values for expressions with optional-ok semantics:

```odin
m := map[string]int{"hello" = 123}
i := m["hellope"] or_else 456  // i = 456 (key doesn't exist)
j := m["hello"] or_else 999    // j = 123 (key exists)

// With Maybe types
n := halve(4).? or_else 0
```

#### or_continue
Similar to `or_return`, but continues the loop instead:

```odin
for item in items {
    value := process(item) or_continue  // skip this iteration on error
    fmt.println(value)
}
```

#### or_break
Breaks out of a loop on error:

```odin
for item in items {
    value := process(item) or_break  // exit loop on error
    fmt.println(value)
}
```

---

## Error Wrapping and Context Patterns

### 1. Error Context with Defer

```odin
foo :: proc() -> (err: Error) {
    defer if err != nil {
        fmt.println("Error in", #procedure, ":", err)
    }

    // Function body
    x := risky_operation() or_return
    return .None
}
```

### 2. Custom Error with Context

```odin
Error :: struct {
    message: string,
    code: int,
    location: runtime.Source_Code_Location,
}

make_error :: proc(msg: string, code: int, loc := #caller_location) -> Error {
    return Error{
        message = msg,
        code = code,
        location = loc,
    }
}

// Usage
if something_bad {
    return make_error("Operation failed", 1)
}
```

### 3. Error String Conversion (core:os)

```odin
file, err := os.open("example.txt")
if err != os.ERROR_NONE {
    // Convert error to human-readable string
    error_msg := os.error_string(err)
    fmt.eprintln("Failed to open file:", error_msg)
    return
}
```

### 4. Implicit Context System

Odin has an implicit `context` value in each scope that's passed by pointer to procedures:

```odin
// The context includes allocator, logger, etc.
data, err := os.read_entire_file("file.txt", context.allocator)
if err != .None {
    log.error("Failed to read file", err)
    return
}
```

This context system allows intercepting and modifying behavior of third-party code, such as allocation or logging.

---

## Best Practices for CLI Applications

### 1. User-Friendly Error Messages

```odin
main :: proc() {
    arguments := os.args

    // Validate argument count
    if len(arguments) < 2 {
        fmt.eprintln("Usage: mytool <command> [arguments...]")
        os.exit(1)
    }

    // Parse arguments
    command, remaining, cli_error := cli.parse_arguments_as_type(arguments[1:], Command)
    if cli_error != nil {
        fmt.eprintln("Failed to parse arguments:", cli_error)
        os.exit(1)
    }

    // Execute command
    if err := run_command(command); err != .None {
        fmt.eprintln("Error:", err)
        os.exit(1)
    }
}
```

### 2. Structured Exit Codes

```odin
Exit_Code :: enum {
    Success = 0,
    Invalid_Arguments = 1,
    File_Not_Found = 2,
    Permission_Denied = 3,
    Unknown_Error = 255,
}

exit_with_error :: proc(err: Error) -> ! {
    code := Exit_Code.Unknown_Error

    switch e in err {
    case File_Error:
        code = .File_Not_Found
    case Permission_Error:
        code = .Permission_Denied
    }

    fmt.eprintln("Error:", err)
    os.exit(int(code))
}
```

### 3. Deferred Cleanup for Resources

```odin
process_file :: proc(filename: string) -> Error {
    // Open file
    f, err := os.open(filename)
    if err != os.ERROR_NONE {
        return err
    }
    defer os.close(f)  // Ensures cleanup

    // Read file
    data, read_err := os.read_entire_file(f)
    if read_err != os.ERROR_NONE {
        return read_err
    }
    defer delete(data)  // Cleanup memory

    // Process data
    result := process_data(data) or_return

    return .None
}
```

### 4. Use Logging for Better Error Tracking

```odin
import "core:log"

// Initialize logger
log.create_console_logger()
defer log.destroy_console_logger()

// Use structured logging
if err := operation(); err != .None {
    log.error("Operation failed", err)
    return
}

log.info("Operation completed successfully")
```

Log procedures by severity:
- `log.debug()` - Development/debugging information
- `log.info()` - General information
- `log.warn()` - Warning messages
- `log.error()` - Error messages
- `log.fatal()` - Fatal errors
- `log.panic()` - Logs and shuts down program

All have format string versions: `log.infof()`, `log.errorf()`, etc.

### 5. Error Formatting with core:fmt

```odin
// Source location formatting
fmt.println("Error at:", location)
// Output: file(line:column) - Default style

// Custom error formatting
fmt.printf("Error: %v\n", err)  // Default formatting
fmt.printf("Error: %#v\n", err) // Debug formatting

// Invalid arguments are handled
fmt.printf("%d", "string")  // Will produce error description
```

### 6. Real-World CLI Example (from odin-cli library)

```odin
Command :: struct {
    input_file: string `args:"pos=0,required" usage:"Input file path"`,
    output_dir: string `args:"name=o,short=o" usage:"Output directory"`,
    verbose: bool      `args:"name=verbose,short=v" usage:"Enable verbose output"`,
}

main :: proc() {
    arguments := os.args

    if len(arguments) < 2 {
        fmt.println("Usage: mytool <input_file> [options]")
        os.exit(1)
    }

    command, remaining_arguments, cli_error := cli.parse_arguments_as_type(
        arguments[1:],
        Command,
    )

    if cli_error != nil {
        fmt.eprintln("Failed to parse arguments:", cli_error)
        os.exit(1)
    }

    if command.verbose {
        log.info("Starting processing...")
    }

    if err := process_file(command.input_file); err != .None {
        log.error("Processing failed:", err)
        os.exit(1)
    }
}
```

---

## Common Pitfalls to Avoid

### 1. Not Checking Error Returns

❌ **WRONG:**
```odin
data, err := os.read_entire_file("file.txt")
// Using data without checking err
process(data)
```

✅ **CORRECT:**
```odin
data, err := os.read_entire_file("file.txt")
if err != .None {
    fmt.eprintln("Failed to read file:", err)
    return
}
defer delete(data)
process(data)
```

### 2. Forgetting defer for Cleanup

❌ **WRONG:**
```odin
f, err := os.open("file.txt")
if err != os.ERROR_NONE {
    return err
}
// Forgot to close the file!
data := read_something(f)
```

✅ **CORRECT:**
```odin
f, err := os.open("file.txt")
if err != os.ERROR_NONE {
    return err
}
defer os.close(f)  // Guaranteed cleanup
data := read_something(f)
```

### 3. Using :: Instead of := for Variables

❌ **WRONG:**
```odin
FOO :: [3]f32{1, 2, 3}  // Compile-time constant
x := FOO[0]  // Error if trying to modify
```

✅ **CORRECT:**
```odin
foo := [3]f32{1, 2, 3}  // Runtime variable
x := foo[0]
```

### 4. Incomplete Switch Statements

Odin defaults to requiring all enum cases in switch statements:

❌ **WRONG (will warn):**
```odin
Error :: enum {
    File_Not_Found,
    Permission_Denied,
    Timeout,
}

handle_error :: proc(err: Error) {
    switch err {
    case .File_Not_Found:
        fmt.println("File not found")
    case .Permission_Denied:
        fmt.println("Permission denied")
    // Missing .Timeout case!
    }
}
```

✅ **CORRECT (explicit partial):**
```odin
handle_error :: proc(err: Error) {
    #partial switch err {  // Explicitly mark as partial
    case .File_Not_Found:
        fmt.println("File not found")
    case .Permission_Denied:
        fmt.println("Permission denied")
    }
}
```

### 5. Trying to Modify Named Returns in defer

❌ **WRONG:**
```odin
foo :: proc() -> (result: int, err: Error) {
    defer {
        result = 42  // This doesn't work!
    }
    return 0, .None
}
```

**Note:** `defer` runs after the return values have been set, so you cannot modify them.

### 6. Using panic for Expected Errors

❌ **WRONG:**
```odin
open_file :: proc(path: string) -> []byte {
    f, err := os.open(path)
    if err != os.ERROR_NONE {
        panic("File not found")  // Don't panic for expected errors!
    }
    defer os.close(f)
    return os.read_entire_file(f)
}
```

✅ **CORRECT:**
```odin
open_file :: proc(path: string) -> ([]byte, Error) {
    f, err := os.open(path)
    if err != os.ERROR_NONE {
        return nil, err  // Return the error
    }
    defer os.close(f)

    data, read_err := os.read_entire_file(f)
    if read_err != os.ERROR_NONE {
        return nil, read_err
    }

    return data, .None
}
```

### 7. Silent Error Propagation

❌ **WRONG:**
```odin
// Silently ignoring errors
result, _ := risky_operation()
```

✅ **CORRECT:**
```odin
result, err := risky_operation()
if err != .None {
    // Handle or propagate the error
    log.error("Operation failed:", err)
    return err
}
```

---

## Real-World Examples

### Example 1: File Processing with Multiple Error Points

```odin
package main

import "core:fmt"
import "core:os"
import "core:strings"

Process_Error :: enum {
    None,
    File_Not_Found,
    Invalid_Content,
    Write_Failed,
}

process_file :: proc(input_path: string, output_path: string) -> Process_Error {
    // Read input file
    data, read_ok := os.read_entire_file_from_filename(input_path)
    if !read_ok {
        fmt.eprintln("Failed to read file:", input_path)
        return .File_Not_Found
    }
    defer delete(data)

    // Validate content
    content := string(data)
    if len(content) == 0 {
        fmt.eprintln("File is empty:", input_path)
        return .Invalid_Content
    }

    // Process content
    processed := strings.to_upper(content)

    // Write output
    write_ok := os.write_entire_file(output_path, transmute([]byte)processed)
    if !write_ok {
        fmt.eprintln("Failed to write file:", output_path)
        return .Write_Failed
    }

    fmt.println("Successfully processed:", input_path, "->", output_path)
    return .None
}

main :: proc() {
    if len(os.args) < 3 {
        fmt.eprintln("Usage: process <input> <output>")
        os.exit(1)
    }

    input := os.args[1]
    output := os.args[2]

    if err := process_file(input, output); err != .None {
        os.exit(1)
    }
}
```

### Example 2: JSON Loading with Error Handling

```odin
package main

import "core:encoding/json"
import "core:fmt"
import "core:os"

load_config :: proc(path: string) -> (json.Value, bool) {
    // Read file
    data, ok := os.read_entire_file_from_filename(path)
    if !ok {
        fmt.eprintln("Failed to load the file!")
        return {}, false
    }
    defer delete(data)

    // Parse JSON
    json_data, err := json.parse(data)
    if err != .None {
        fmt.eprintln("Failed to parse JSON file")
        fmt.eprintln("Error:", err)
        return {}, false
    }

    return json_data, true
}

main :: proc() {
    config, ok := load_config("config.json")
    if !ok {
        os.exit(1)
    }
    defer json.destroy_value(config)

    fmt.println("Config loaded successfully")
}
```

### Example 3: Network Operation with Union Errors

```odin
package main

import "core:fmt"
import "core:net"

connect_and_send :: proc(address: string, message: string) -> net.Network_Error {
    // Connect
    endpoint := net.parse_endpoint(address) or_return
    conn := net.dial_tcp(endpoint) or_return
    defer net.close(conn)

    // Send data
    bytes_sent := net.send_tcp(conn, transmute([]byte)message) or_return

    fmt.printf("Sent %d bytes\n", bytes_sent)
    return nil
}

main :: proc() {
    err := connect_and_send("127.0.0.1:8080", "Hello, Odin!")
    if err != nil {
        fmt.eprintln("Network error:", err)
        os.exit(1)
    }
}
```

### Example 4: CLI Tool with Comprehensive Error Handling

```odin
package main

import "core:fmt"
import "core:os"
import "core:strings"
import "core:log"

App_Error :: enum {
    None,
    Invalid_Arguments,
    File_Not_Found,
    Parse_Error,
    Write_Error,
}

Config :: struct {
    input_file: string,
    output_file: string,
    verbose: bool,
}

parse_args :: proc() -> (Config, App_Error) {
    args := os.args[1:]

    if len(args) < 2 {
        fmt.eprintln("Usage: mytool <input> <output> [--verbose]")
        return {}, .Invalid_Arguments
    }

    config := Config{
        input_file = args[0],
        output_file = args[1],
        verbose = len(args) > 2 && args[2] == "--verbose",
    }

    return config, .None
}

run_app :: proc(config: Config) -> App_Error {
    if config.verbose {
        log.info("Reading input file:", config.input_file)
    }

    // Read input
    data, read_ok := os.read_entire_file_from_filename(config.input_file)
    if !read_ok {
        log.error("Failed to read file:", config.input_file)
        return .File_Not_Found
    }
    defer delete(data)

    if config.verbose {
        log.infof("Read %d bytes", len(data))
    }

    // Process (example: convert to uppercase)
    processed := strings.to_upper(string(data))

    // Write output
    write_ok := os.write_entire_file(config.output_file, transmute([]byte)processed)
    if !write_ok {
        log.error("Failed to write file:", config.output_file)
        return .Write_Error
    }

    if config.verbose {
        log.info("Successfully wrote:", config.output_file)
    }

    return .None
}

main :: proc() {
    // Setup logging
    context.logger = log.create_console_logger()
    defer log.destroy_console_logger()

    // Parse arguments
    config, parse_err := parse_args()
    if parse_err != .None {
        os.exit(1)
    }

    // Run application
    if err := run_app(config); err != .None {
        os.exit(int(err))
    }
}
```

---

## Resources

### Official Documentation
- **Odin Language Overview:** https://odin-lang.org/docs/overview/
- **Odin FAQ:** https://odin-lang.org/docs/faq/
- **core:os Package Documentation:** https://pkg.odin-lang.org/core/os/
- **core:fmt Package Documentation:** https://pkg.odin-lang.org/core/fmt/

### Articles and Blog Posts
- **Exceptions - And Why Odin Will Never Have Them:** https://odin.handmade.network/blog/p/3372-exceptions_-_and_why_odin_will_never_have_them
  - Also available at: https://www.gingerbill.org/article/2018/09/05/exceptions-and-why-odin-will-never-have-them/
- **Moving Towards a New "core:os":** https://odin-lang.org/news/moving-towards-a-new-core-os/
- **Learn Odin in Y Minutes:** https://learnxinyminutes.com/odin/
- **Introduction to Odin (Karl Zylinski):** https://zylinski.se/posts/introduction-to-odin/

### GitHub Resources
- **Official Odin Repository:** https://github.com/odin-lang/Odin
- **Odin Examples Repository:** https://github.com/odin-lang/examples
- **Error Handling Discussion #256:** https://github.com/odin-lang/Odin/issues/256
- **Error Handling Discussion #951:** https://github.com/odin-lang/Odin/discussions/951
- **Demo File with Error Examples:** https://github.com/odin-lang/Odin/blob/master/examples/demo/demo.odin
- **Keywords and Operators Wiki:** https://github.com/odin-lang/Odin/wiki/Keywords-and-Operators

### Libraries with Good Error Handling Examples
- **odin-cli (Command Line Parsing):** https://github.com/GoNZooo/odin-cli
- **net.odin (Network Library):** https://github.com/RestartFU/net.odin
- **core:flags (Official Flags Package):** https://github.com/odin-lang/Odin/blob/master/core/flags/errors.odin
- **core:net (Official Network Package):** https://github.com/odin-lang/Odin/blob/master/core/net/common.odin

### Community Resources
- **Odin Forum:** https://forum.odin-lang.org/
- **Handmade Network (Odin Section):** https://odin.handmade.network/

---

## Summary: Quick Reference

### Error Handling Checklist for CLI Applications

✅ **DO:**
- Use multiple return values for error handling
- Return specific error types (enums or unions)
- Check errors immediately after operations
- Use `defer` for resource cleanup
- Use `or_return` for clean error propagation
- Provide helpful error messages to users
- Log errors appropriately
- Exit with meaningful status codes
- Use named return values with `or_return`

❌ **DON'T:**
- Use `panic` for expected errors (only for unrecoverable situations)
- Ignore error return values
- Forget to clean up resources
- Pass errors up the stack without handling
- Use exceptions (they don't exist in Odin)
- Forget `defer` before operations that might fail
- Try to modify named returns in `defer` blocks

### Common Patterns

```odin
// Pattern 1: Simple error check
value, err := operation()
if err != .None {
    return err
}

// Pattern 2: With or_return
value := operation() or_return

// Pattern 3: With defer cleanup
resource := acquire() or_return
defer release(resource)

// Pattern 4: With error logging
defer if err != nil {
    log.error("Operation failed:", err)
}

// Pattern 5: Union error switching
value, err := operation()
switch e in err {
case SpecificError:
    // Handle specifically
case:
    // Handle others
}
```

---

**Report Generated:** 2025-11-06
**Research Focus:** Odin Error Handling for CLI Applications
**Compiler Version:** Latest (as of January 2025)
