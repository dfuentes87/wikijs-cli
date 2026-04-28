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

	if err := cli.NewRootCommand().ExecuteContext(ctx); err != nil {
		cli.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
