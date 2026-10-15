package cli

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/spf13/cobra"
)

func newDoctorCmd(service DoctorService) *cobra.Command {
	var opts app.DoctorOptions
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check Docker, the images, and the project directory",
		Long: "Check that Docker is installed and running, that the runtime image is current, that the\n" +
			"config file is valid (config), that symlinked agent config is allowed (links), that the image\n" +
			"matches Docker's architecture, that the project directory can be mounted, and which CLI\n" +
			"versions the image contains. Exits 1 when a check fails. It never builds, pulls, or removes\n" +
			"anything, and never prints an environment value.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := service.Diagnose(cmd.Context(), opts)
			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintln(out, report.Header); err != nil {
				return err
			}
			for _, result := range report.Results {
				if _, err := fmt.Fprintf(out, "%-5s %-10s %s\n", result.Status, result.Name, result.Detail); err != nil {
					return err
				}
			}
			if failed := report.Failed(); failed > 0 {
				return &app.ChecksFailedError{Failed: failed}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.WorkDir, "workdir", "w", "", "check that this host directory can be mounted (default: the current directory)")
	return cmd
}
