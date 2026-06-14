package knowledge

import (
	"context"
	"fmt"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/event"
	"arcdesk/internal/failuremem"
)

// CaptureProposal is a pending lesson awaiting user confirmation.
type CaptureProposal struct {
	ID          string
	Fingerprint string
	Signature   string
	Summary     string
	Error       string
	Fix         string
	Paths       []string
}

func emitKnowledgeCapture(sink event.Sink, kind event.Kind, proposal CaptureProposal) {
	if sink == nil {
		return
	}
	sink.Emit(event.Event{
		Kind: kind,
		KnowledgeCapture: event.KnowledgeCapture{
			ID:          proposal.ID,
			Fingerprint: proposal.Fingerprint,
			Signature:   proposal.Signature,
			Summary:     proposal.Summary,
			Error:       proposal.Error,
			Fix:         proposal.Fix,
			Paths:       append([]string(nil), proposal.Paths...),
		},
	})
}

// CaptureOnVerifyPass asks the judger whether to record, then writes when approved.
func CaptureOnVerifyPass(ctx context.Context, judger CaptureJudger, store *failuremem.Store, cfg config.KnowledgeConfig, sink event.Sink, failedCmd, errOut string, paths []string) {
	if store == nil || judger == nil || !cfg.ShouldEnable() || !cfg.VerifyAutoCaptureEnabled() {
		return
	}
	failedCmd = strings.TrimSpace(failedCmd)
	errOut = strings.TrimSpace(errOut)
	if failedCmd == "" || errOut == "" {
		return
	}

	paths = append([]string(nil), paths...)
	judgment, err := judger.JudgeCapture(ctx, CaptureJudgeInput{
		FailedCmd: failedCmd,
		ErrOut:    errOut,
		Paths:     paths,
	})
	sourceFix := pathsIncludeSourceFix(paths)
	if err != nil {
		if sourceFix {
			if tryHeuristicSourceFixCapture(store, cfg, sink, failedCmd, errOut, paths) {
				return
			}
		}
		emitCaptureSkipNotice(sink, "Knowledge capture skipped: judge unavailable.")
		return
	}
	if !judgment.Record {
		if sourceFix {
			if tryHeuristicSourceFixCapture(store, cfg, sink, failedCmd, errOut, paths) {
				return
			}
		}
		if reason := strings.TrimSpace(judgment.Reason); reason != "" {
			emitCaptureSkipNotice(sink, "Knowledge capture skipped: "+reason)
		}
		return
	}

	proposal, ok := proposalFromJudgment(judgment, paths)
	if !ok {
		if sourceFix && tryHeuristicSourceFixCapture(store, cfg, sink, failedCmd, errOut, paths) {
			return
		}
		return
	}
	writeCaptureProposal(store, cfg, sink, proposal)
}

func tryHeuristicSourceFixCapture(store *failuremem.Store, cfg config.KnowledgeConfig, sink event.Sink, failedCmd, errOut string, paths []string) bool {
	proposal, ok := heuristicSourceFixProposal(failedCmd, errOut, paths)
	if !ok {
		return false
	}
	writeCaptureProposal(store, cfg, sink, proposal)
	return true
}

func writeCaptureProposal(store *failuremem.Store, cfg config.KnowledgeConfig, sink event.Sink, proposal CaptureProposal) {
	root := store.WorkspaceRoot()
	if IsCaptureDismissed(root, proposal.Fingerprint) {
		return
	}
	if cfg.CaptureRequiresConfirm() {
		emitKnowledgeCapture(sink, event.KnowledgeCaptureSuggest, proposal)
		return
	}
	if err := RecordCaptureProposal(store, proposal); err != nil {
		emitCaptureSkipNotice(sink, "Knowledge capture skipped: could not write entry.")
		return
	}
	emitKnowledgeCapture(sink, event.KnowledgeCaptureRecorded, proposal)
}

func emitCaptureSkipNotice(sink event.Sink, text string) {
	if sink == nil || strings.TrimSpace(text) == "" {
		return
	}
	sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: text})
}

// SuggestCaptureOnVerifyPass is deprecated; use CaptureOnVerifyPass with a judger.
func SuggestCaptureOnVerifyPass(store *failuremem.Store, cfg config.KnowledgeConfig, sink event.Sink, failedCmd, errOut string, paths []string) {
	_ = store
	_ = cfg
	_ = sink
	_ = failedCmd
	_ = errOut
	_ = paths
}

// RecordCaptureProposal writes a confirmed proposal as draft knowledge.
func RecordCaptureProposal(store *failuremem.Store, p CaptureProposal) error {
	if store == nil {
		return fmt.Errorf("knowledge store not configured")
	}
	_ = store.Record(failuremem.Entry{
		Signature:  p.Signature,
		Error:      p.Error,
		Fix:        p.Fix,
		Paths:      append([]string(nil), p.Paths...),
		Kind:       failuremem.KindFix,
		Confidence: failuremem.ConfidenceDraft,
	})
	return nil
}
