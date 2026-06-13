package callgraph

import "errors"

const indexNotReadyMsg = "Callgraph index not ready — wait for background indexing to finish or retry after callgraph_status shows nodes."

func toolResultOrNotReady(out string, err error) (string, error) {
	if err == nil {
		return out, nil
	}
	if errors.Is(err, ErrIndexNotReady) {
		return indexNotReadyMsg, nil
	}
	return "", err
}
