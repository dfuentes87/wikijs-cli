package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dfuentes87/wikijs-cli/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := cli.NewRootCommand()
	if err := cmd.ExecuteContext(ctx); err != nil {
		cli.PrintErrorColor(os.Stderr, err, cli.CommandColorEnabled(cmd))
		os.Exit(1)
	}
}
