package reporag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"arcdesk/internal/tool"
)

var errToolUnavailable = errors.New("tool unavailable")

func registryCall(reg *tool.Registry, ctx context.Context, names []string, args json.RawMessage) (string, string, error) {
	if reg == nil {
		return "", "", errToolUnavailable
	}
	for _, name := range names {
		t, ok := reg.Get(name)
		if !ok {
			continue
		}
		out, err := t.Execute(ctx, args)
		return out, name, err
	}
	return "", "", errToolUnavailable
}

func codegraphCall(reg *tool.Registry, ctx context.Context, toolName string, payload any) (string, error) {
	args, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	names := []string{
		"mcp__codegraph__" + toolName,
		toolName,
	}
	out, used, err := registryCall(reg, ctx, names, args)
	if err != nil || used == "" {
		return "", errToolUnavailable
	}
	return out, nil
}

func formatUnavailable(layer string) string {
	return fmt.Sprintf("%s unavailable — use grep/glob or install the required index.", layer)
}
