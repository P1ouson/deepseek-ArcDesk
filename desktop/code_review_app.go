package main

// CodeReviewResult is returned by RunCodeReview.
type CodeReviewResult struct {
	Text string `json:"text"`
	Err  string `json:"err,omitempty"`
}

// RunCodeReview runs the built-in review or security_review subagent directly
// for the active tab, without relying on the main model to invoke the tool.
func (a *App) RunCodeReview(mode, scope string, paths []string) CodeReviewResult {
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return CodeReviewResult{Err: "no active session"}
	}
	r := ctrl.RunCodeReview(mode, scope, paths)
	return CodeReviewResult{Text: r.Text, Err: r.Err}
}
