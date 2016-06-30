package cli

import (
	"fmt"

	"github.com/bh107/tapr/build"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "output version information",
	Long: `
Output build version information."
`,
	Run: func(cmd *cobra.Command, args []string) {
		info := build.GetInfo()
		fmt.Printf("Build version: %s\n", info.Tag)
		fmt.Printf("Build time:    %s\n", info.Time)
		fmt.Printf("Platform:      %s\n", info.Platform)
		fmt.Printf("Go version:    %s\n", info.GoVersion)
	},
}
