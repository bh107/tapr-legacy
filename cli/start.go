package cli

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/bh107/tapr/api"
	"github.com/bh107/tapr/config"
	"github.com/bh107/tapr/server"
	"github.com/spf13/cobra"
)

var (
	cfg *config.Config

	runAudit bool
	mock     bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start the server",
	Long: `
Start the tapr tape management server.
`,
	Example: `  tapr start`,
	RunE:    runStart,
}

func init() {
	cobra.OnInitialize(initConfig)

	f := startCmd.Flags()

	f.BoolVar(&mock,
		"mock", false, "enable mocking",
	)

	f.BoolVar(&runAudit,
		"audit", false, "run initial inventory audit",
	)
}

func initConfig() {
	if flagDebug {
		log.SetFlags(log.Lshortfile | log.Ltime)
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

func runStart(cmd *cobra.Command, args []string) error {
	srv, err := server.New(cfg, flagDebug, runAudit, mock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if flagDebug {
		log.Print("starting debug http server at :6060")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c

		srv.Shutdown()
		os.Exit(0)
	}()

	return api.Start(srv)
}
