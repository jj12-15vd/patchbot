package main

import (
	"context"
	"os"
	"os/signal"

	"patchbot/internal/cli"
	"patchbot/internal/scan"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, scan.New().Scan)
	stop() // os.Exit skips deferred functions, so clean up explicitly.
	os.Exit(code)
}
