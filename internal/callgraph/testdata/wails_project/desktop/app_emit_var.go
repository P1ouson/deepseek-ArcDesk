package main

import "context"

func (a *App) EmitVar(ch string) {
	runtime.EventsEmit(context.Background(), ch, nil)
}

func emitHelper() {
	ch := "agent:event"
	runtime.EventsEmit(context.Background(), ch, nil)
}
