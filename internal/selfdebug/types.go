package selfdebug

import (
	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/instruction"
	"arcdesk/internal/runtime"
	"arcdesk/internal/verification"
)

// Phase is the current stage of the self-debug cycle.
type Phase string

const (
	PhaseIdle   Phase = "idle"
	PhaseVerify Phase = "verify"
	PhaseFailed Phase = "failed"
	PhaseFix    Phase = "fix"
)

// Snapshot is the loop state exposed to tools and retry prompts.
type Snapshot struct {
	Phase         Phase    `json:"phase"`
	Attempt       int      `json:"attempt"`
	MaxRetries    int      `json:"maxRetries"`
	FailedCmd     string   `json:"failedCmd,omitempty"`
	PendingChecks []string `json:"pendingChecks,omitempty"`
	PassedChecks  []string `json:"passedChecks,omitempty"`
	WrittenPaths  []string `json:"writtenPaths,omitempty"`
}

// Input carries everything needed to build retry or immediate hints.
type Input struct {
	FailedCmd      string
	Stderr         string
	WrittenPaths   []string
	CallgraphPaths []string
	Checks         []instruction.VerifyCheck
	WriterIndex    int
	HasWriter      bool
	Attempt        int
	MaxRetries     int

	DepIndex   *dependency.Index
	CGIndex    *callgraph.Index
	RuntimeHub *runtime.Hub

	HasSuccessfulCommandAfter func(cmd string, after int) bool
}

// Plan is the configured verification set wired into the loop.
type Plan struct {
	Checks []instruction.VerifyCheck
	Policy verification.Policy
}
