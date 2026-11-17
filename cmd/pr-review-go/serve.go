package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"pr-review-go/internal/server"
)

var (
	servePort int
	serveHost string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP webhook daemon for GitHub and Bitbucket",
	Long: `serve starts a high-performance HTTP webhook daemon listening for GitHub and Bitbucket
PR events (opened, modified, commented) and processes reviews asynchronously with an internal worker pool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if servePort > 0 {
			appConfig.Server.Port = servePort
		}
		if serveHost != "" {
			appConfig.Server.Host = serveHost
		}

		srv := server.NewServer(appConfig, llmClient)
		addr := fmt.Sprintf("%s:%d", appConfig.Server.Host, appConfig.Server.Port)

		httpServer := &http.Server{
			Addr:         addr,
			Handler:      srv,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		}

		fmt.Printf("Starting pr-review-go webhook service on %s...\n", addr)
		fmt.Printf("Listening for webhooks at:\n")
		fmt.Printf(" - http://%s/webhooks/github\n", addr)
		fmt.Printf(" - http://%s/webhooks/bitbucket\n", addr)
		fmt.Printf(" - http://%s/healthz\n", addr)

		return httpServer.ListenAndServe()
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "0.0.0.0", "Host/interface to bind")
	rootCmd.AddCommand(serveCmd)
}
