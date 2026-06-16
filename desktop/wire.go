package main

import "arcdesk/internal/event"

// wireEvent is the JSON shape an event.Event takes when emitted to the webview.
// It mirrors the serve transport's SSE wire form field-for-field on purpose: both
// frontends consume the identical typed stream, so the React client and a browser
// SSE client can share contract types. The Kind enum becomes a stable string and
// the TurnDone error becomes a message, since neither serializes cleanly.
//
// (Kept in step with internal/serve/wire.go by hand for now — the two transports
// may diverge later; if they don't, this is the obvious thing to lift into a
// shared event.ToWire.)
type wireEvent struct {
	Kind         string          `json:"kind"`
	Text         string          `json:"text,omitempty"`
	Reasoning    string          `json:"reasoning,omitempty"`
	Level        string          `json:"level,omitempty"`
	Code         string          `json:"code,omitempty"`
	Tool         *wireTool       `json:"tool,omitempty"`
	Usage        *wireUsage      `json:"usage,omitempty"`
	Approval     *wireApproval   `json:"approval,omitempty"`
	Ask          *wireAsk        `json:"ask,omitempty"`
	KnowledgeCapture *wireKnowledgeCapture `json:"knowledgeCapture,omitempty"`
	Compaction   *wireCompaction `json:"compaction,omitempty"`
	ToolReuse    *wireToolReuseStats `json:"toolReuse,omitempty"`
	Err          string          `json:"err,omitempty"`
	RetryAttempt int             `json:"retryAttempt,omitempty"`
	RetryMax     int             `json:"retryMax,omitempty"`
}

// wireCompaction is the JSON form of an event.Compaction. On a compaction_started
// event only Trigger is set; compaction_done carries the rest (an aborted pass
// leaves Summary empty so the frontend drops its placeholder).
type wireCompaction struct {
	Trigger  string `json:"trigger,omitempty"`
	Messages int    `json:"messages,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Archive  string `json:"archive,omitempty"`
}

type wireAskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type wireAskQuestion struct {
	ID      string          `json:"id"`
	Header  string          `json:"header,omitempty"`
	Prompt  string          `json:"prompt"`
	Options []wireAskOption `json:"options"`
	Multi   bool            `json:"multi,omitempty"`
}

type wireAsk struct {
	ID        string            `json:"id"`
	Questions []wireAskQuestion `json:"questions"`
}

type wireTool struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Args      string `json:"args,omitempty"`
	Output    string `json:"output,omitempty"`
	Err       string `json:"err,omitempty"`
	ReadOnly  bool   `json:"readOnly"`
	Truncated bool   `json:"truncated,omitempty"`
	Partial   bool   `json:"partial,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
	FileDiff  *wireFileDiff `json:"fileDiff,omitempty"`
}

type wireFileDiff struct {
	Diff    string `json:"diff"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

type wireUsage struct {
	PromptTokens     int                   `json:"promptTokens"`
	CompletionTokens int                   `json:"completionTokens"`
	TotalTokens      int                   `json:"totalTokens"`
	CacheHitTokens   int                   `json:"cacheHitTokens"`
	CacheMissTokens  int                   `json:"cacheMissTokens"`
	ReasoningTokens  int                   `json:"reasoningTokens,omitempty"`
	CacheDiagnostics *wireCacheDiagnostics `json:"cacheDiagnostics,omitempty"`
	ToolReuse        *wireToolReuseStats   `json:"toolReuse,omitempty"`
	// Session-cumulative cache tokens — the status line shows the aggregate
	// hit-rate Σhit/Σ(hit+miss), steadier than the single-turn CacheHitTokens.
	SessionCacheHitTokens  int     `json:"sessionCacheHitTokens"`
	SessionCacheMissTokens int     `json:"sessionCacheMissTokens"`
	Cost                   float64 `json:"cost,omitempty"`
	Currency               string  `json:"currency,omitempty"`
	// CostUSD is kept for older frontend/status consumers. It mirrors Cost and
	// does not imply USD.
	CostUSD float64 `json:"costUsd,omitempty"`
}

type wireCacheDiagnostics struct {
	PrefixHash          string   `json:"prefixHash"`
	PrefixChanged       bool     `json:"prefixChanged"`
	PrefixChangeReasons []string `json:"prefixChangeReasons,omitempty"`
	SystemHash          string   `json:"systemHash"`
	ToolsHash           string   `json:"toolsHash"`
	LogRewriteVersion   int      `json:"logRewriteVersion"`
	ToolSchemaTokens    int      `json:"toolSchemaTokens"`
	CacheMissTokens     int      `json:"cacheMissTokens"`
	CacheHitTokens      int      `json:"cacheHitTokens"`
}

type wireToolReuseStats struct {
	TurnCalls             int `json:"turnCalls"`
	TurnDuplicates        int `json:"turnDuplicates"`
	TurnCacheableCalls    int `json:"turnCacheableCalls"`
	TurnCacheableDupes    int `json:"turnCacheableDupes"`
	SessionCalls          int `json:"sessionCalls"`
	SessionDuplicates     int `json:"sessionDuplicates"`
	SessionCacheableCalls int `json:"sessionCacheableCalls"`
	SessionCacheableDupes    int `json:"sessionCacheableDupes"`
	SessionCacheHits         int `json:"sessionCacheHits"`
	SessionNormalizedDupes   int `json:"sessionNormalizedDupes"`
}

type wireApproval struct {
	ID      string `json:"id"`
	Tool    string `json:"tool"`
	Subject string `json:"subject"`
}

type wireKnowledgeCapture struct {
	ID          string   `json:"id"`
	Fingerprint string   `json:"fingerprint"`
	Signature   string   `json:"signature"`
	Summary     string   `json:"summary"`
	Error       string   `json:"error,omitempty"`
	Fix         string   `json:"fix"`
	Paths       []string `json:"paths,omitempty"`
}

// kindNames maps the event.Kind enum to stable wire strings.
var kindNames = map[event.Kind]string{
	event.TurnStarted:       "turn_started",
	event.Reasoning:         "reasoning",
	event.Text:              "text",
	event.Message:           "message",
	event.ToolDispatch:      "tool_dispatch",
	event.ToolResult:        "tool_result",
	event.Usage:             "usage",
	event.Notice:            "notice",
	event.Phase:             "phase",
	event.ApprovalRequest:   "approval_request",
	event.AskRequest:        "ask_request",
	event.TurnDone:          "turn_done",
	event.CompactionStarted: "compaction_started",
	event.CompactionDone:    "compaction_done",
	event.ToolProgress:      "tool_progress",
	event.Retrying:          "retrying",
	event.KnowledgeCaptureSuggest:  "knowledge_capture_suggest",
	event.KnowledgeCaptureRecorded: "knowledge_capture_recorded",
}

// toWireAsk converts an event.Ask into its JSON wire form.
func toWireAsk(a event.Ask) *wireAsk {
	qs := make([]wireAskQuestion, len(a.Questions))
	for i, q := range a.Questions {
		opts := make([]wireAskOption, len(q.Options))
		for j, o := range q.Options {
			opts[j] = wireAskOption{Label: o.Label, Description: o.Description}
		}
		qs[i] = wireAskQuestion{ID: q.ID, Header: q.Header, Prompt: q.Prompt, Options: opts, Multi: q.Multi}
	}
	return &wireAsk{ID: a.ID, Questions: qs}
}

// toWire converts an event.Event into its JSON wire form.
func toWire(e event.Event) wireEvent {
	w := wireEvent{Kind: kindNames[e.Kind], Text: e.Text, Reasoning: e.Reasoning}
	switch e.Kind {
	case event.Notice:
		if e.Level == event.LevelWarn {
			w.Level = "warn"
		} else {
			w.Level = "info"
		}
		w.Code = e.Code
	case event.ToolDispatch, event.ToolResult, event.ToolProgress:
		wt := &wireTool{
			ID: e.Tool.ID, Name: e.Tool.Name, Args: e.Tool.Args,
			Output: e.Tool.Output, Err: e.Tool.Err,
			ReadOnly: e.Tool.ReadOnly, Truncated: e.Tool.Truncated,
			Partial: e.Tool.Partial, ParentID: e.Tool.ParentID, Cached: e.Tool.Cached,
		}
		if e.Tool.Diff != "" {
			wt.FileDiff = &wireFileDiff{
				Diff: e.Tool.Diff, Added: e.Tool.Added, Removed: e.Tool.Removed,
			}
		}
		w.Tool = wt
	case event.Usage:
		if u := e.Usage; u != nil {
			w.Usage = &wireUsage{
				PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens,
				TotalTokens: u.TotalTokens, CacheHitTokens: u.CacheHitTokens,
				CacheMissTokens: u.CacheMissTokens, ReasoningTokens: u.ReasoningTokens,
				SessionCacheHitTokens: e.SessionHit, SessionCacheMissTokens: e.SessionMiss,
			}
			if e.CacheDiagnostics != nil {
				w.Usage.CacheDiagnostics = toWireCacheDiagnostics(e.CacheDiagnostics)
			}
			if e.ToolReuse != nil {
				w.Usage.ToolReuse = toWireToolReuse(e.ToolReuse)
			}
			if e.Pricing != nil {
				cost := e.Pricing.Cost(u)
				w.Usage.Cost = cost
				w.Usage.Currency = e.Pricing.Symbol()
				w.Usage.CostUSD = cost
			}
		}
	case event.ApprovalRequest:
		w.Approval = &wireApproval{ID: e.Approval.ID, Tool: e.Approval.Tool, Subject: e.Approval.Subject}
	case event.AskRequest:
		w.Ask = toWireAsk(e.Ask)
	case event.KnowledgeCaptureSuggest, event.KnowledgeCaptureRecorded:
		kc := e.KnowledgeCapture
		w.KnowledgeCapture = &wireKnowledgeCapture{
			ID:          kc.ID,
			Fingerprint: kc.Fingerprint,
			Signature:   kc.Signature,
			Summary:     kc.Summary,
			Error:       kc.Error,
			Fix:         kc.Fix,
			Paths:       append([]string(nil), kc.Paths...),
		}
	case event.CompactionStarted, event.CompactionDone:
		w.Compaction = &wireCompaction{
			Trigger: e.Compaction.Trigger, Messages: e.Compaction.Messages,
			Summary: e.Compaction.Summary, Archive: e.Compaction.Archive,
		}
	case event.TurnDone:
		if e.Err != nil {
			w.Err = e.Err.Error()
		}
		if e.ToolReuse != nil {
			w.ToolReuse = toWireToolReuse(e.ToolReuse)
		}
	case event.Retrying:
		w.RetryAttempt = e.RetryAttempt
		w.RetryMax = e.RetryMax
	}
	return w
}

func toWireCacheDiagnostics(d *event.CacheDiagnostics) *wireCacheDiagnostics {
	return &wireCacheDiagnostics{
		PrefixHash:          d.PrefixHash,
		PrefixChanged:       d.PrefixChanged,
		PrefixChangeReasons: append([]string(nil), d.PrefixChangeReasons...),
		SystemHash:          d.SystemHash,
		ToolsHash:           d.ToolsHash,
		LogRewriteVersion:   d.LogRewriteVersion,
		ToolSchemaTokens:    d.ToolSchemaTokens,
		CacheMissTokens:     d.CacheMissTokens,
		CacheHitTokens:      d.CacheHitTokens,
	}
}

func toWireToolReuse(s *event.ToolReuseStats) *wireToolReuseStats {
	if s == nil {
		return nil
	}
	return &wireToolReuseStats{
		TurnCalls:             s.TurnCalls,
		TurnDuplicates:        s.TurnDuplicates,
		TurnCacheableCalls:    s.TurnCacheableCalls,
		TurnCacheableDupes:    s.TurnCacheableDupes,
		SessionCalls:          s.SessionCalls,
		SessionDuplicates:     s.SessionDuplicates,
		SessionCacheableCalls: s.SessionCacheableCalls,
		SessionCacheableDupes:  s.SessionCacheableDupes,
		SessionCacheHits:       s.SessionCacheHits,
		SessionNormalizedDupes: s.SessionNormalizedDupes,
	}
}
