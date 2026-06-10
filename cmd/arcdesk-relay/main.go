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

const defaultRelayListenAddr = "127.0.0.1:8788"

func main() {
	// Default to loopback — public relay deployments must pass -addr explicitly
	// (e.g. -addr 0.0.0.0:8788) behind TLS and network ACLs.
	addr := flag.String("addr", defaultRelayListenAddr, "listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := relay.Run(ctx, *addr); err != nil {
		slog.Error("relay stopped", "err", err)
		os.Exit(1)
	}
}
