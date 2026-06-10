package agent

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// httptest servers (SSE provider probes) can leave net/http idle conns on
	// Windows long enough to trip goleak even after srv.Close(); ignore the
	// server-side serve loop — client-side leaks still fail the check.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.(*conn).serve"),
		goleak.IgnoreTopFunction("net/http.(*conn).closeWriteAndWait"),
		goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
	)
}
