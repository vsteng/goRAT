package main

// Ultra-minimal program: empty main.
// If this binary still crashes with Exception 0xc0000005 at PC=0x0
// before reaching main(), the problem is NOT in your application
// code. It indicates either:
// 1. Toolchain cross-compilation bug (Go 1.25.4 preview?)
// 2. Incompatible Windows version (older than target baseline)
// 3. Security/AV hooking causing null function pointer
// 4. Corrupted transfer (binary truncated)
// It intentionally produces no output.
func main() {}
