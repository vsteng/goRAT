package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Printf("Minimal client started!\n")
	fmt.Printf("OS: %s, Arch: %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Args: %v\n", os.Args)
	fmt.Println("If you see this, the Go runtime works fine.")
	fmt.Println("Press Enter to exit...")
	var input string
	fmt.Scanln(&input)
}
