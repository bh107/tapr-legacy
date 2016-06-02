package main

import (
	"fmt"
	"os"

	"github.com/bh107/tapr"
	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	config      *tapr.Config
	showVersion bool
	debug       bool
	runAudit    bool
)

var rootCmd = &cobra.Command{
	Use:   "taprd",
	Short: "Run the tapr server",
	Long:  `YMMV.`,
	Run:   run,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&cfgFile, "config", "", "config file",
	)

	rootCmd.PersistentFlags().BoolVarP(&showVersion,
		"version", "v", false, "print version info and exit",
	)

	rootCmd.PersistentFlags().BoolVarP(&debug,
		"debug", "D", false, "enable debug mode",
	)

	rootCmd.PersistentFlags().BoolVarP(&runAudit,
		"audit", "A", false, "run initial inventory audit",
	)
}

func initConfig() {
	if showVersion {
		fmt.Printf("taprd v%s\n", tapr.Version)
		os.Exit(1)
	}

	if cfgFile == "" {
		fmt.Fprintln(os.Stderr, "please specify config file")
		os.Exit(1)
	}

	f, err := os.Open(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open config file")
		os.Exit(1)
	}

	defer f.Close()

	config, err = tapr.ParseConfig(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse config file:", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	fmt.Println("run")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
