// Package runtime captures live session observation for the coding agent:
// browser console output, shell/Go stderr, frontend network requests, and
// lightweight runtime state snapshots. Records are held in an in-memory ring
// buffer scoped to one controller session and exposed through runtime_* tools.
//
// Desktop feeds the hub via a Wails binding and frontend hooks; CLI sessions
// still collect bash tool output and notices through CaptureSink.
package runtime
