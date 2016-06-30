package cli

import "github.com/spf13/cobra"

var taprCmd = &cobra.Command{
	Use:   "tapr",
	Short: "Run the tapr server",
	Long: `
YMMV.
`,
}

var cfgFile string
var flagDebug bool

func init() {
	cobra.EnableCommandSorting = false

	f := taprCmd.PersistentFlags()

	f.StringVar(
		&cfgFile, "config", "", "config file",
	)

	f.BoolVar(&flagDebug,
		"debug", false, "enable debug mode",
	)

	taprCmd.AddCommand(
		startCmd,
		libraryCmd,
		versionCmd,
	)
}

func Run(args []string) error {
	return taprCmd.Execute()
}
