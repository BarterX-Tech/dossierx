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

	"github.com/BarterX-Tech/dossierx/internal/cliout"
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
				return serveFailure(cmd, err)
			}
			v, _, _ := resolveVersionInfo()

			srv := serve.New(cfg, v)
			if err := srv.Listen(port); err != nil {
				return serveFailure(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "serving: %s\n", srv.URL())

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return serveFailure(cmd, srv.Serve(ctx))
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "port to listen on (0 = a random high port)")
	return cmd
}

// serveFailure gives "serve" the one thing its text-only exemption cost it: a
// machine-readable answer when it does not start.
//
// serve is exempt from the one-envelope-per-invocation contract for a real
// reason — its useful output (the URL) has to appear BEFORE it blocks, which one
// envelope cannot express (see output.go's annotationTextOnly). But the
// exemption was applied to the FAILURE path too, and there it bought nothing:
// a serve that cannot bind its port, or cannot find a config, is an ordinary
// request/response failure with an ordinary answer. What actually happened was
// that "dossierx serve" on a taken port wrote NOTHING to stdout at all and
// exited non-zero, which is the one outcome the machine contract exists to
// prevent — an agent supervising the human's viewer could not tell a crash from
// a config error from a port collision without parsing stderr prose.
//
// Under --format text nothing changes: the error is handed back and runCLI
// prints cobra's own "Error: <msg>" line, byte for byte as before. Under JSON it
// writes one failure envelope to stdout and marks the error as already-rendered
// so runCLI does not report it twice.
func serveFailure(cmd *cobra.Command, err error) error {
	if err == nil || !jsonOutput() {
		return err
	}
	env := cliout.Failure(commandPath(cmd), errorForCLI(err))
	if writeErr := cliout.Write(cmd.OutOrStdout(), env); writeErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "dossierx: could not write the output envelope: %v\n", writeErr)
	}
	return &emittedErr{err: err}
}
