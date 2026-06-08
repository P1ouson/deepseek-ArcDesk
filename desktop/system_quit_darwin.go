//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void installARCDESKSystemQuitHook(void);
*/
import "C"

import "sync"

var installSystemQuitHookOnce sync.Once

func installSystemQuitHook() {
	installSystemQuitHookOnce.Do(func() {
		C.installARCDESKSystemQuitHook()
	})
}

//export ARCDESKMarkSystemQuit
func ARCDESKMarkSystemQuit() {
	markSystemQuitRequested()
}
