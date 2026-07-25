// serve.go wires internal/serve into the CLI as "dossierx serve": a local HTTP
// server that renders the claims viewer from memory and exposes the same-origin
// comment write API, so a human can review claims in a browser alongside the
// agent CLI. Like the other command files it is a package-main newServeCmd()
// constructor registered in newRootCmd()'s AddCommand list; the server itself
// (binding, admission middleware, render pipeline, JSON API) lives in
// internal/serve so it can be tested without the cobra shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/serve"
)

// newServeCmd is the "dossierx serve" command. It binds 127.0.0.1 on a random
// high port (override with --port), prints the absolute URL to open, and runs
// until interrupted. SIGINT/SIGTERM cancel the context so Serve drains
// in-flight requests — letting any comment op mid-write release the claims lock
// its defer holds — before the process exits; a bare signal death would skip
// that release and could leave the sentinel lock file behind.
func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the claims viewer with a localhost comment API",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			v, _, _ := resolveVersionInfo()

			srv := serve.New(cfg, v)
			if err := srv.Listen(port); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "serving: %s\n", srv.URL())

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return srv.Serve(ctx)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "port to listen on (0 = a random high port)")
	return cmd
}
