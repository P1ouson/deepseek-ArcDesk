package main

import "arcdesk/internal/control"

func registerDesktopSessionTools(app *App, ctrl *control.Controller) {
	if app == nil || ctrl == nil {
		return
	}
	ctrl.RegisterSessionTool(openWebPreviewTool{opener: app})
}
