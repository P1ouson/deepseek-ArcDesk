package control

import (
	"strings"

	"arcdesk/internal/hook"
	"arcdesk/internal/mcpcmd"
)

func (c *Controller) handleHooksSlash(trimmed string) {
	args := mcpcmd.TokenizeArgs(trimmed)
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch sub {
	case "", "list", "ls":
		c.notice(c.hookListText())
	case "trust":
		if err := hook.Trust(c.cpRoot, ""); err != nil {
			c.notice("hooks trust: " + err.Error())
			return
		}
		c.notice("trusted this project's hooks — they load on the next /new or restart")
	default:
		c.notice("unknown /hooks subcommand " + args[1] + " — try: /hooks, /hooks trust")
	}
}
