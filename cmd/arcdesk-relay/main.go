package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"arcdesk/internal/relay"
)

func main() {
	addr := flag.String("addr", ":8788", "listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := relay.Run(ctx, *addr); err != nil {
		slog.Error("relay stopped", "err", err)
		os.Exit(1)
	}
}
