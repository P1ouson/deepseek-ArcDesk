package boot

import (
	"context"
	"testing"
)

func TestPluginSessionCtxPrefersOverride(t *testing.T) {
	buildCtx, buildCancel := context.WithCancel(context.Background())
	t.Cleanup(buildCancel)
	pluginCtx := context.WithValue(context.Background(), struct{}{}, "plugin")

	got := pluginSessionCtx(buildCtx, pluginCtx)
	if got != pluginCtx {
		t.Fatal("expected override plugin ctx")
	}
	if pluginSessionCtx(buildCtx, nil) != buildCtx {
		t.Fatal("expected build ctx when override nil")
	}
}
