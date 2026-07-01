// Command ARCDESK is a config- and plugin-driven coding agent CLI.
package main

import (
	"os"

	"arcdesk/internal/cli"

	// Blank imports wire compile-time built-ins into their registries.
	_ "arcdesk/internal/provider/anthropic"
	_ "arcdesk/internal/provider/ollama"
	_ "arcdesk/internal/provider/openai"
	_ "arcdesk/internal/tool/builtin"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
