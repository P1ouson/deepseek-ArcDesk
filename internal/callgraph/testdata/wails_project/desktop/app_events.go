package main

import "context"

func (a *App) Notify(msg string) error {
	runtime.EventsEmit(context.Background(), "agent:event", msg)
	return nil
}

func (a *App) MultiEmit() error {
	runtime.EventsEmit(context.Background(), "agent:ready", nil)
	runtime.EventsEmit(context.Background(), "terminal:output", "x")
	return nil
}
