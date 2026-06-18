package control

import (
	"errors"
	"strings"
	"testing"

	"arcdesk/internal/agent"
	"arcdesk/internal/i18n"
	"arcdesk/internal/provider"
)

func TestExplainError(t *testing.T) {
	if ExplainError(nil) != nil {
		t.Error("nil should stay nil")
	}

	empty := ExplainError(agent.ErrEmptyModelResponse)
	if empty == nil || empty.Error() != i18n.M.AgentEmptyResponse {
		t.Errorf("empty response = %v, want %q", empty, i18n.M.AgentEmptyResponse)
	}

	bal := ExplainError(&provider.APIError{Provider: "deepseek", Status: 402, Body: "Insufficient Balance"})
	if bal.Error() != i18n.M.ProviderErrInsufficientBalance {
		t.Errorf("402 = %q, want the insufficient-balance message", bal.Error())
	}

	auth := ExplainError(&provider.AuthError{Provider: "deepseek", KeyEnv: "DEEPSEEK_API_KEY", Status: 401})
	if !strings.Contains(auth.Error(), "DEEPSEEK_API_KEY") {
		t.Errorf("401 should name the key env: %q", auth.Error())
	}

	for _, status := range []int{400, 422, 429, 500, 503} {
		got := ExplainError(&provider.APIError{Provider: "p", Status: status})
		if got.Error() == "" || got.Error() == (&provider.APIError{Provider: "p", Status: status}).Error() {
			t.Errorf("status %d should map to a localized message, got %q", status, got.Error())
		}
	}

	jsonBody := ExplainError(&provider.APIError{Provider: "deepseek", Status: 400, Body: `{"error":{"message":"This model's maximum context length is 65536 tokens.","type":"invalid_request_error"}}`})
	if !strings.Contains(jsonBody.Error(), i18n.M.ProviderErrBadRequest) || !strings.Contains(jsonBody.Error(), "maximum context length") {
		t.Errorf("400 should append the provider reason from a JSON body, got %q", jsonBody.Error())
	}

	rawBody := ExplainError(&provider.APIError{Provider: "deepseek", Status: 422, Body: "some unparseable detail"})
	if !strings.Contains(rawBody.Error(), "some unparseable detail") {
		t.Errorf("422 should fall back to the raw body, got %q", rawBody.Error())
	}

	noLeak := ExplainError(&provider.APIError{Provider: "deepseek", Status: 429, Body: `{"error":{"message":"slow down"}}`})
	if noLeak.Error() != i18n.M.ProviderErrRateLimited {
		t.Errorf("429 body must not leak into the message, got %q", noLeak.Error())
	}

	plain := errors.New("some other failure")
	if ExplainError(plain) != plain {
		t.Error("unknown errors should pass through unchanged")
	}

	canceled := ExplainError(errors.New(`deepseek-flash: request failed: Post "https://zenmux.ai/api/v1/chat/completions": context canceled`))
	if canceled == nil || canceled.Error() != i18n.M.AgentRequestCanceled {
		t.Errorf("context canceled = %q, want %q", canceled, i18n.M.AgentRequestCanceled)
	}

	streamEOF := ExplainError(errors.New("deepseek-flash: read stream: unexpected EOF"))
	if streamEOF == nil || streamEOF.Error() != i18n.M.AgentStreamInterrupted {
		t.Errorf("stream EOF = %q, want %q", streamEOF, i18n.M.AgentStreamInterrupted)
	}

	network := ExplainError(errors.New("mimo-pro: request failed: Get \"https://api.example.com/v1/models\": EOF"))
	if network == nil || network.Error() != i18n.M.AgentNetworkError {
		t.Errorf("network EOF = %q, want %q", network, i18n.M.AgentNetworkError)
	}

	validateWrapped := ExplainError(errors.New(`validate: deepseek-flash: request failed: Post "https://zenmux.ai/api/v1/chat/completions": context canceled`))
	if validateWrapped == nil || validateWrapped.Error() != i18n.M.AgentRequestCanceled {
		t.Errorf("validate wrapped cancel = %q, want %q", validateWrapped, i18n.M.AgentRequestCanceled)
	}

	statusString := ExplainError(errors.New("deepseek-flash: status 402: Insufficient Balance"))
	if statusString == nil || statusString.Error() != i18n.M.ProviderErrInsufficientBalance {
		t.Errorf("status string = %q, want %q", statusString, i18n.M.ProviderErrInsufficientBalance)
	}

	streamMsg := ExplainError(errors.New("deepseek-flash: model is overloaded, please retry later"))
	if streamMsg == nil || streamMsg.Error() != "model is overloaded, please retry later" {
		t.Errorf("stream message = %q", streamMsg)
	}
}
