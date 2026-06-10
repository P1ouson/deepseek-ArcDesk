package main

import "arcdesk/internal/control"

// enableDesktopInteractive wires interactive tool/`ask` approval and desktop-only
// subagent gate inheritance. CLI/TUI callers use EnableInteractiveApproval alone.
func enableDesktopInteractive(ctrl *control.Controller) {
	if ctrl == nil {
		return
	}
	ctrl.EnableInteractiveApproval()
	ctrl.EnableDesktopSubagentGate()
}
