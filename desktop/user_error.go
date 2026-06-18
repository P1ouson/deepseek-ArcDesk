package main

import "arcdesk/internal/control"

func userFacingErr(err error) error {
	return control.ExplainError(err)
}

func userFacingMsg(err error) string {
	return control.UserFacingMessage(err)
}
