package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/dashboard"
	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// dashboardDefaultPort is the port tp dashboard binds unless --port names
// another: a fixed port keeps a bookmark and a shared link working
// (spec/1.8.0.md §1). When it is taken and --port was not passed, the
// dashboard serves on a free port instead (listenDashboard).
const dashboardDefaultPort = 7377

// dashboardNoSourceHint answers a repository holding neither a graph nor a
// task file.
const dashboardNoSourceHint = "run tp dashboard inside a repository holding .tp/epics.json or a task file — create one with tp epic set or tp init <spec.md>"

var dashboardPort int

func newDashboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Serve the read-only dashboard on 127.0.0.1 until interrupted",
		Args:  cobra.NoArgs,
		RunE:  runDashboard,
	}
	cmd.Flags().IntVar(&dashboardPort, "port", dashboardDefaultPort, "the loopback port to bind; unset, a taken default falls back to a free port")
	return cmd
}

// runDashboard refuses under TP_UNATTENDED, needs a graph or a task file,
// binds 127.0.0.1 on the port, prints the URL and serves until interrupted.
// It writes nothing to the repository.
func runDashboard(cmd *cobra.Command, _ []string) error {
	if engine.Unattended() {
		output.Error(ExitUsage, "tp dashboard is a long-running server, which is not a unit's job, and is refused under TP_UNATTENDED",
			"run tp dashboard attended, outside tp run")
		os.Exit(ExitUsage)
	}
	if dashboardPort < 1 || dashboardPort > 65535 {
		output.Error(ExitUsage, fmt.Sprintf("--port %d is not a port", dashboardPort), "pass --port a number from 1 to 65535")
		os.Exit(ExitUsage)
	}
	if hasGraph, hasTaskFile := engine.DashboardSources("."); !hasGraph && !hasTaskFile {
		output.Error(ExitFile, "nothing to show: no epic graph and no task file in this repository", dashboardNoSourceHint)
		os.Exit(ExitFile)
	}
	ln, fellBack, err := listenDashboard(cmd.Context(), dashboardPort, cmd.Flags().Changed("port"))
	if err != nil {
		msg, hint := dashboard.ListenFailure(dashboardPort, err)
		output.Error(ExitValidation, msg, hint)
		os.Exit(ExitValidation)
	}
	if fellBack {
		output.Notice(fmt.Sprintf("port %d is taken; serving on a free port instead (--port pins one)", dashboardPort))
	}
	url := "http://" + ln.Addr().String() + "/"
	if output.IsJSON() {
		printResultOrExit(map[string]string{"url": url})
	} else {
		fmt.Println(url)
	}
	return serveDashboard(cmd.Context(), ln)
}

// listenDashboard binds 127.0.0.1 on port. A port the caller did not choose
// that is taken falls back to a free port the system picks, and fellBack says
// so; a chosen port is bound as asked or fails as itself.
func listenDashboard(ctx context.Context, port int, chosen bool) (ln net.Listener, fellBack bool, err error) {
	lc := &net.ListenConfig{}
	ln, err = lc.Listen(ctx, "tcp", net.JoinHostPort(dashboard.Host, strconv.Itoa(port)))
	if err == nil || chosen || !errors.Is(err, syscall.EADDRINUSE) {
		return ln, false, err
	}
	ln, err = lc.Listen(ctx, "tcp", net.JoinHostPort(dashboard.Host, "0"))
	return ln, err == nil, err
}

// serveDashboard serves the dashboard on ln until the process is interrupted
// or terminated, then ends every open event stream and awaits the server's
// shutdown, returning its error.
func serveDashboard(parent context.Context, ln net.Listener) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Handler:           dashboard.Handler(ln.Addr().(*net.TCPAddr).Port, engine.ProjectName("."), dashboardAPI{}),
		ReadHeaderTimeout: 10 * time.Second,
		// Every request's context ends with the signal, so an open event
		// stream returns and its response completes before Shutdown drains.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	shutdownErr := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		shutdownErr <- srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving the dashboard on %s: %w", ln.Addr(), err)
	}
	// Serve returns as soon as Shutdown starts; the drain is awaited here.
	if err := <-shutdownErr; err != nil {
		return fmt.Errorf("shutting the dashboard down: %w", err)
	}
	return nil
}
