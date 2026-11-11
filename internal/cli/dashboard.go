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
// (spec/1.8.0.md §1).
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
	cmd.Flags().IntVar(&dashboardPort, "port", dashboardDefaultPort, "the loopback port to bind")
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
	addr := net.JoinHostPort(dashboard.Host, strconv.Itoa(dashboardPort))
	ln, err := (&net.ListenConfig{}).Listen(cmd.Context(), "tcp", addr)
	if err != nil {
		output.Error(ExitValidation, fmt.Sprintf("port %d is taken: %v", dashboardPort, err),
			fmt.Sprintf("stop the process holding port %d, or pass --port naming a free one", dashboardPort))
		os.Exit(ExitValidation)
	}
	url := "http://" + addr + "/"
	if output.IsJSON() {
		printResultOrExit(map[string]string{"url": url})
	} else {
		fmt.Println(url)
	}
	return serveDashboard(cmd.Context(), ln)
}

// serveDashboard serves the dashboard on ln until the process is interrupted
// or terminated, then shuts the server down.
func serveDashboard(parent context.Context, ln net.Listener) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Handler: dashboard.Handler(ln.Addr().(*net.TCPAddr).Port, dashboardAPI{}), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
