package main

import "testing"

func TestDefaultRelayListenAddr(t *testing.T) {
	if defaultRelayListenAddr != "127.0.0.1:8788" {
		t.Fatalf("defaultRelayListenAddr = %q", defaultRelayListenAddr)
	}
}
