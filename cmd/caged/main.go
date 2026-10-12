// Command caged is the command-line entry point for caged.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/JeremyOrr4/caged/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}

		os.Exit(1)
	}
}
