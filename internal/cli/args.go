package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// positionalArgs validates the arguments of the root command. At most two may
// come before "--": an agent and its prompt. Arguments after "--" belong to the
// agent, so they are not counted, and they need an agent to receive them.
func positionalArgs(cmd *cobra.Command, args []string) error {
	positional, _ := splitArgs(cmd, args)
	if len(positional) > 2 {
		return fmt.Errorf("accepts at most 2 arg(s), received %d", len(positional))
	}
	if cmd.ArgsLenAtDash() == 0 && len(args) > 0 {
		return errors.New("arguments after -- are passed to an agent; name one first, e.g. caged claude -- --resume")
	}
	return nil
}

// splitArgs separates the arguments before "--" (an agent and a prompt) from
// the arguments after it. Without "--", everything is positional.
func splitArgs(cmd *cobra.Command, args []string) (positional, agentArgs []string) {
	dash := cmd.ArgsLenAtDash()
	if dash < 0 {
		return args, nil
	}
	if dash == len(args) {
		return args[:dash], nil
	}
	return args[:dash], args[dash:]
}

// flagErrorWithHint points users who put an agent's flag before the agent name
// at "--". Cobra reports those as unknown flags.
func flagErrorWithHint(_ *cobra.Command, err error) error {
	message := err.Error()
	if strings.HasPrefix(message, "unknown flag") || strings.HasPrefix(message, "unknown shorthand flag") {
		return fmt.Errorf("%w (caged's flags go before the agent name; pass agent flags after --, e.g. caged claude -- --resume)", err)
	}
	return err
}
