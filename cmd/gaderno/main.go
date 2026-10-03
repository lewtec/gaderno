package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lewtec/lewkit/x/entry"
	"github.com/lucasew/gaderno/internal/cli"
)

func init() {
	// Android loads this package as a library and calls entry.RunBound.
	entry.Bind(cli.RunHost)
}

func main() {
	// lewkit release run executes this binary with no arguments.
	// Packaged hosts set ELETROCROMO_NO_UI and exec the same binary.
	if cli.HostLaunch(os.Args[1:]) {
		entry.Main(cli.RunHost)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Execute(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
