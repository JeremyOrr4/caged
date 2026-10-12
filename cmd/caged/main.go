// Command caged is the command-line entry point for caged.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("caged: hello (args: %v)\n", os.Args[1:])
}
