package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var libraryCmd = &cobra.Command{
	Use:   "library",
	Short: "manage libraries",
	Long: `
Add or modify libraries.
`,
}

var libraryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add a library",
	Long: `
Add a library.
`,
	Example: `  tapr library add --name primary`,
	RunE:    runLibraryAdd,
}

var libraryModifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "modify a library",
	Long: `
Modify a library.
`,
	RunE: runLibraryModify,
}

var libraryName string

func init() {
	libraryCmd.AddCommand(
		libraryAddCmd,
		libraryModifyCmd,
	)

	f := libraryAddCmd.Flags()

	f.StringVar(
		&libraryName, "name", "", "library name",
	)
}

func runLibraryAdd(cmd *cobra.Command, args []string) error {
	fmt.Println(args)
	return nil
}

func runLibraryModify(cmd *cobra.Command, args []string) error {
	fmt.Println(args)
	return nil
}
