package prefixruntime

import (
	"context"
	"strings"

	"arcdesk/internal/provider"
)

const prewarmUserTag = "[prefix-prewarm]"

// PrewarmUserContent is the minimal tail appended during prefix warmup.
const PrewarmUserContent = prewarmUserTag + " Repository context loaded. Reply with OK only."

// IsPrewarmUser reports whether a user message is an internal warmup tail.
func IsPrewarmUser(content string) bool {
	return strings.HasPrefix(strings.TrimSpace(content), prewarmUserTag)
}

// PrewarmPrefix sends one lightweight completion so the provider can persist the
// frozen system+tools prefix before the user's first real task.
func PrewarmPrefix(ctx context.Context, prov provider.Provider, systemPrompt string, tools []provider.ToolSchema) error {
	if prov == nil || strings.TrimSpace(systemPrompt) == "" {
		return nil
	}
	msgs := ForProviderRequest([]provider.Message{
		{Role: provider.RoleSystem, Content: systemPrompt},
		{Role: provider.RoleUser, Content: PrewarmUserContent},
	})
	ch, err := prov.Stream(ctx, provider.Request{
		Messages:    msgs,
		Tools:       tools,
		Temperature: 0,
	})
	if err != nil {
		return err
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError && chunk.Err != nil {
			return chunk.Err
		}
	}
	return nil
}
