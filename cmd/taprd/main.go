package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bh107/tapr"
	"github.com/bh107/tapr/api"
	"github.com/bh107/tapr/config"
	"github.com/bh107/tapr/server"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config

	showVersion bool
	debug       bool
	runAudit    bool
	mock        bool
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

	rootCmd.PersistentFlags().BoolVar(&mock,
		"mock", false, "enable mocking",
	)

	rootCmd.Flags().BoolVarP(&runAudit,
		"audit", "A", false, "run initial inventory audit",
	)
}

func initConfig() {
	if showVersion {
		fmt.Printf("taprd v%s\n", tapr.Version)
		os.Exit(1)
	}

	if debug {
		fmt.Println("[+] debug enabled")
		log.SetFlags(log.Lshortfile | log.Ltime)
	}

	if mock {
		fmt.Println("[+] mocking enabled")
	}

	if cfgFile == "" {
		cfgFile = "./tapr.conf"
	}

	f, err := os.Open(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to open config file")
		os.Exit(1)
	}

	defer f.Close()

	cfg, err = config.Parse(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse config file:", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	srv, err := server.New(cfg, debug, runAudit, mock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c

		srv.Shutdown()
		os.Exit(0)
	}()

	api.Start(srv)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
