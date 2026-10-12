package container

import (
	"fmt"
	"os"
	"os/exec"
)

// Run starts an interactive, disposable Docker container.
func Run(image string, command []string) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	cmd := exec.Command(docker, buildArgs(image, command)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildArgs(image string, command []string) []string {
	args := []string{"run", "-it", "--rm", image}
	return append(args, command...)
}
