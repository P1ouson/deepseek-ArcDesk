package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
)

func writeScopedSession(t *testing.T, dir, name, scope, workspaceRoot, prompt string, updatedAt time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":`+strconv.Quote(prompt)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write scoped session: %v", err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		CreatedAt:     updatedAt.Add(-time.Minute),
		UpdatedAt:     updatedAt,
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	return path
}

func writeEmptyScopedSession(t *testing.T, dir, name, scope, workspaceRoot string, updatedAt time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	// No user turns — only a system line, matching a fresh NewSession before first send.
	if err := os.WriteFile(path, []byte(`{"role":"system","content":"system prompt"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write empty scoped session: %v", err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		CreatedAt:     updatedAt,
		UpdatedAt:     updatedAt,
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	return path
}

func TestFindScopedTabSessionResumesLatestEmptyTopicSession(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	older := writeScopedSession(t, dir, "older-global.jsonl", "global", globalRoot, "older prompt", time.Now().Add(-2*time.Hour))
	newer := writeScopedSession(t, dir, "newer-global.jsonl", "global", globalRoot, "newer prompt", time.Now().Add(-time.Hour))
	projectRoot := t.TempDir()
	writeScopedSession(t, dir, "project.jsonl", "project", projectRoot, "project prompt", time.Now())

	if got := findScopedTabSession(dir, "global", globalRoot); got != newer {
		t.Fatalf("findScopedTabSession = %q, want %q (not %q)", got, newer, older)
	}
	if got := findScopedTabSession(dir, "project", projectRoot); got == "" {
		t.Fatalf("findScopedTabSession for project scope should match")
	}
}

func TestDefaultGlobalTabResumesScopedSessionWithEmptyTopicID(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	sessionPath := writeScopedSession(t, dir, "autosaved-global.jsonl", "global", globalRoot, "resume after restart", time.Now().Add(-time.Minute))

	tab := &WorkspaceTab{
		ID:            "tab_global",
		Scope:         "global",
		WorkspaceRoot: globalRoot,
		Ready:         false,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabs:        map[string]*WorkspaceTab{"tab_global": tab},
		tabOrder:    []string{"tab_global"},
		activeTabID: "tab_global",
	}
	app.buildTabController(tab)
	if tab.Ctrl != nil {
		defer tab.Ctrl.Close()
	}
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built")
	}
	if tab.Ctrl.SessionPath() != sessionPath {
		t.Fatalf("tab session path = %q, want %q", tab.Ctrl.SessionPath(), sessionPath)
	}
	history := tab.Ctrl.History()
	if len(history) == 0 || !strings.Contains(history[0].Content, "resume after restart") {
		t.Fatalf("tab history = %#v, want resumed conversation", history)
	}
}

func TestFindScopedTabSessionResumesLastActiveInABCSequence(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	base := time.Now().Add(-3 * time.Hour)
	sessionA := writeScopedSession(t, dir, "a-global.jsonl", "global", globalRoot, "session A", base)
	sessionB := writeScopedSession(t, dir, "b-global.jsonl", "global", globalRoot, "session B", base.Add(time.Hour))
	sessionC := writeScopedSession(t, dir, "c-global.jsonl", "global", globalRoot, "session C", base.Add(2*time.Hour))

	if got := findScopedTabSession(dir, "global", globalRoot); got != sessionC {
		t.Fatalf("A→B→C restore = %q, want last active %q (not %q or %q)", got, sessionC, sessionA, sessionB)
	}
}

func TestFindScopedTabSessionPrefersMetaUpdatedAtOverFileModTime(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	now := time.Now().UTC()
	// File mtime is newer, but meta UpdatedAt is older.
	staleMeta := writeScopedSession(t, dir, "stale-meta.jsonl", "global", globalRoot, "stale meta", now.Add(-2*time.Hour))
	if err := os.Chtimes(staleMeta, now, now); err != nil {
		t.Fatalf("chtimes stale meta session: %v", err)
	}
	freshMeta := writeScopedSession(t, dir, "fresh-meta.jsonl", "global", globalRoot, "fresh meta", now.Add(-time.Hour))
	if err := os.Chtimes(freshMeta, now.Add(-3*time.Hour), now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("chtimes fresh meta session: %v", err)
	}

	if got := findScopedTabSession(dir, "global", globalRoot); got != freshMeta {
		t.Fatalf("restore should follow meta UpdatedAt = %q, want %q (not file-mtime winner %q)", got, freshMeta, staleMeta)
	}
}

func TestFindScopedTabSessionTieBreakIsStableByPath(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	when := time.Now().UTC().Truncate(time.Second)
	first := filepath.Join(dir, "aaa-tie.jsonl")
	second := filepath.Join(dir, "zzz-tie.jsonl")
	for _, item := range []struct {
		path, prompt string
	}{
		{first, "tie first"},
		{second, "tie second"},
	} {
		if err := os.WriteFile(item.path, []byte(`{"role":"user","content":`+strconv.Quote(item.prompt)+`}`+"\n"), 0o644); err != nil {
			t.Fatalf("write tie session: %v", err)
		}
		if err := agent.SaveBranchMetaPreserveUpdated(item.path, agent.BranchMeta{
			CreatedAt:     when,
			UpdatedAt:     when,
			Scope:         "global",
			WorkspaceRoot: globalRoot,
		}); err != nil {
			t.Fatalf("save tie session meta: %v", err)
		}
	}

	got := findScopedTabSession(dir, "global", globalRoot)
	if got != first {
		t.Fatalf("equal UpdatedAt tie-break = %q, want lexicographically first path %q (not %q)", got, first, second)
	}
}

func TestFindScopedTabSessionSkipsEmptySessions(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	now := time.Now().UTC()
	writeEmptyScopedSession(t, dir, "empty-newest.jsonl", "global", globalRoot, now)
	real := writeScopedSession(t, dir, "real-older.jsonl", "global", globalRoot, "has turns", now.Add(-time.Hour))

	if got := findScopedTabSession(dir, "global", globalRoot); got != real {
		t.Fatalf("empty session must be skipped: got %q, want %q", got, real)
	}
}

func TestFindScopedTabSessionFallbackAfterDelete(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	base := time.Now().Add(-3 * time.Hour)
	writeScopedSession(t, dir, "a-global.jsonl", "global", globalRoot, "session A", base)
	sessionB := writeScopedSession(t, dir, "b-global.jsonl", "global", globalRoot, "session B", base.Add(time.Hour))
	sessionC := writeScopedSession(t, dir, "c-global.jsonl", "global", globalRoot, "session C", base.Add(2*time.Hour))

	if err := deleteSessionFile(dir, sessionC); err != nil {
		t.Fatalf("delete latest session: %v", err)
	}
	if got := findScopedTabSession(dir, "global", globalRoot); got != sessionB {
		t.Fatalf("after deleting C, restore = %q, want %q", got, sessionB)
	}
}

func TestFindScopedTabSessionIgnoresTopicBoundSessions(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	now := time.Now().UTC()
	topicPath := writeScopedSession(t, dir, "topic-newest.jsonl", "global", globalRoot, "topic bound", now)
	meta, ok, err := agent.LoadBranchMeta(topicPath)
	if err != nil || !ok {
		t.Fatalf("load topic session meta: ok=%v err=%v", ok, err)
	}
	meta.TopicID = "topic_bound"
	meta.TopicTitle = "Topic bound"
	if err := agent.SaveBranchMeta(topicPath, meta); err != nil {
		t.Fatalf("save topic-bound meta: %v", err)
	}
	unscoped := writeScopedSession(t, dir, "unscoped-older.jsonl", "global", globalRoot, "unscoped", now.Add(-time.Hour))

	if got := findScopedTabSession(dir, "global", globalRoot); got != unscoped {
		t.Fatalf("topic-bound session must not win for empty topic tab: got %q, want %q", got, unscoped)
	}
}

func TestMultiTabScopedSessionRestoreUsesWorkspaceScope(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	projectRoot := t.TempDir()
	now := time.Now().UTC()
	globalSession := writeScopedSession(t, dir, "global-tab.jsonl", "global", globalRoot, "global chat", now)
	projectSession := writeScopedSession(t, dir, "project-tab.jsonl", "project", projectRoot, "project chat", now)

	globalTab := &WorkspaceTab{
		ID:            "tab_global",
		Scope:         "global",
		WorkspaceRoot: globalRoot,
		disabledMCP:   map[string]ServerView{},
	}
	projectTab := &WorkspaceTab{
		ID:            "tab_project",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabs: map[string]*WorkspaceTab{
			"tab_global":  globalTab,
			"tab_project": projectTab,
		},
		tabOrder:    []string{"tab_global", "tab_project"},
		activeTabID: "tab_global",
	}

	app.buildTabController(globalTab)
	if globalTab.Ctrl != nil {
		defer globalTab.Ctrl.Close()
	}
	app.buildTabController(projectTab)
	if projectTab.Ctrl != nil {
		defer projectTab.Ctrl.Close()
	}

	if globalTab.Ctrl == nil || projectTab.Ctrl == nil {
		t.Fatalf("controllers were not built")
	}
	if got := globalTab.Ctrl.SessionPath(); got != globalSession {
		t.Fatalf("global tab session = %q, want %q", got, globalSession)
	}
	if got := projectTab.Ctrl.SessionPath(); got != projectSession {
		t.Fatalf("project tab session = %q, want %q", got, projectSession)
	}
}

func TestColdRestartResumesLastActiveGlobalSessionABC(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	globalRoot := globalTabWorkspaceRoot()
	base := time.Now().Add(-3 * time.Hour)
	writeScopedSession(t, dir, "a-global.jsonl", "global", globalRoot, "session A", base)
	writeScopedSession(t, dir, "b-global.jsonl", "global", globalRoot, "session B", base.Add(time.Hour))
	sessionC := writeScopedSession(t, dir, "c-global.jsonl", "global", globalRoot, "session C", base.Add(2*time.Hour))

	buildGlobalTab := func() *WorkspaceTab {
		tab := &WorkspaceTab{
			ID:            "tab_global",
			Scope:         "global",
			WorkspaceRoot: globalRoot,
			disabledMCP:   map[string]ServerView{},
		}
		app := &App{
			tabs:        map[string]*WorkspaceTab{"tab_global": tab},
			tabOrder:    []string{"tab_global"},
			activeTabID: "tab_global",
		}
		app.buildTabController(tab)
		return tab
	}

	first := buildGlobalTab()
	if first.Ctrl == nil {
		t.Fatalf("first boot controller missing")
	}
	if got := first.Ctrl.SessionPath(); got != sessionC {
		t.Fatalf("first boot session = %q, want %q", got, sessionC)
	}
	first.Ctrl.Close()

	second := buildGlobalTab()
	defer func() {
		if second.Ctrl != nil {
			second.Ctrl.Close()
		}
	}()
	if second.Ctrl == nil {
		t.Fatalf("second boot controller missing")
	}
	if got := second.Ctrl.SessionPath(); got != sessionC {
		t.Fatalf("cold restart session = %q, want last active %q", got, sessionC)
	}
	if history := second.Ctrl.History(); len(history) == 0 || !strings.Contains(history[0].Content, "session C") {
		t.Fatalf("cold restart history = %#v, want session C content", history)
	}
}

func writeTopicSession(t *testing.T, dir, name, topicID, topicTitle, workspaceRoot string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		TopicID:       topicID,
		TopicTitle:    topicTitle,
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	return path
}

func writeLegacySession(t *testing.T, dir, name, prompt string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":`+strconv.Quote(prompt)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy session: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes legacy session: %v", err)
	}
	return path
}

func writeLegacyEventSession(t *testing.T, dir, name, prompt, reply string, modTime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir legacy sessions: %v", err)
	}
	path := filepath.Join(dir, name)
	body := `{"type":"user.message","id":1,"ts":"t","turn":0,"text":` + strconv.Quote(prompt) + `}` + "\n" +
		`{"type":"model.final","id":2,"ts":"t","turn":0,"content":` + strconv.Quote(reply) + `,"toolCalls":[],"usage":{},"costUsd":0}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write legacy event session: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes legacy event session: %v", err)
	}
	return path
}

func TestDeleteTopicKeepsSessionHistory(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_keep_history"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Keep history"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "keep.jsonl", topicID, "Keep history", projectRoot)

	if err := NewApp().DeleteTopic(topicID); err != nil {
		t.Fatalf("delete topic: %v", err)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("delete topic should keep session history: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestRenameProjectUpdatesSidebarTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := NewApp().RenameProject(projectRoot, "Client API"); err != nil {
		t.Fatalf("rename project: %v", err)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1", len(nodes))
	}
	if got := nodes[0].Label; got != "Client API" {
		t.Fatalf("project label = %q, want Client API", got)
	}

	if err := NewApp().RenameProject(projectRoot, ""); err != nil {
		t.Fatalf("clear project title: %v", err)
	}
	nodes = NewApp().ListProjectTree()
	if got, want := nodes[0].Label, filepath.Base(projectRoot); got != want {
		t.Fatalf("cleared project label = %q, want %q", got, want)
	}
}

func TestListWorkspacesUsesProjectRegistryTitles(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, "Client API"); err != nil {
		t.Fatalf("add project: %v", err)
	}

	workspaces := NewApp().ListWorkspaces()
	if len(workspaces) != 1 {
		t.Fatalf("workspaces len = %d, want 1: %+v", len(workspaces), workspaces)
	}
	if got := workspaces[0].Path; got != projectRoot {
		t.Fatalf("workspace path = %q, want %q", got, projectRoot)
	}
	if got := workspaces[0].Name; got != "Client API" {
		t.Fatalf("workspace name = %q, want Client API", got)
	}
}

func TestListWorkspacesMigratesLegacyWorkspaceList(t *testing.T) {
	isolateDesktopUserDirs(t)

	legacyRoot := t.TempDir()
	rememberWorkspace(legacyRoot)

	workspaces := NewApp().ListWorkspaces()
	if len(workspaces) != 1 {
		t.Fatalf("workspaces len = %d, want 1: %+v", len(workspaces), workspaces)
	}
	if got := workspaces[0].Path; got != legacyRoot {
		t.Fatalf("workspace path = %q, want %q", got, legacyRoot)
	}
	projects := loadProjectsFile().Projects
	if len(projects) != 1 || projects[0].Root != legacyRoot {
		t.Fatalf("legacy workspace was not migrated into projects: %+v", projects)
	}
}

func TestLegacySessionsMigrateIntoGlobalTopics(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	older := writeLegacySession(t, dir, "older.jsonl", "older imported prompt", time.Now().Add(-2*time.Hour))
	newer := writeLegacySession(t, dir, "newer.jsonl", "newer imported prompt", time.Now().Add(-time.Hour))

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("global migrated topics = %d, want 2: %#v", got, nodes[0].Children)
	}
	if got, want := nodes[0].Children[0].TopicID, legacySessionTopicID(newer); got != want {
		t.Fatalf("newest topic first = %q, want %q", got, want)
	}
	if got, want := nodes[0].Children[1].TopicID, legacySessionTopicID(older); got != want {
		t.Fatalf("older topic second = %q, want %q", got, want)
	}

	meta, ok, err := agent.LoadBranchMeta(newer)
	if err != nil || !ok {
		t.Fatalf("load migrated meta: ok=%v err=%v", ok, err)
	}
	if meta.Scope != "global" || meta.WorkspaceRoot != "" || meta.TopicID != legacySessionTopicID(newer) {
		t.Fatalf("migrated meta = %+v", meta)
	}

	nodes = NewApp().ListProjectTree()
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("migration should be idempotent, global topics = %d", got)
	}
}

func TestV05LegacyEventSessionsImportIntoGlobalTopic(t *testing.T) {
	home := isolateDesktopUserDirs(t)

	legacyDir := filepath.Join(home, ".arcdesk", "sessions")
	destDir := config.SessionDir()
	writeLegacyEventSession(t, legacyDir, "v053-chat.events.jsonl", "hello from v0.53", "hi from v0.53", time.Now().Add(-time.Hour))

	imported, err := agent.MigrateLegacySessions(legacyDir, destDir)
	if err != nil {
		t.Fatalf("migrate legacy sessions: %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported legacy sessions = %d, want 1", imported)
	}
	migratedSession := filepath.Join(destDir, "v053-chat.jsonl")
	if _, err := os.Stat(migratedSession); err != nil {
		t.Fatalf("legacy v0.5 session was not imported to %s: %v", migratedSession, err)
	}

	wantTopicID := legacySessionTopicID(migratedSession)
	migratedTopics := migrateLegacySessionsIntoGlobalTopics(destDir)
	if len(migratedTopics) != 1 || migratedTopics[0] != wantTopicID {
		t.Fatalf("migrated topics = %#v, want imported v0.5 topic %q", migratedTopics, wantTopicID)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != wantTopicID {
		t.Fatalf("global topics = %#v, want imported v0.5 topic %q", nodes[0].Children, wantTopicID)
	}
	meta, ok, err := agent.LoadBranchMeta(migratedSession)
	if err != nil || !ok {
		t.Fatalf("load imported v0.5 meta: ok=%v err=%v", ok, err)
	}
	if meta.Scope != "global" || meta.TopicID != wantTopicID {
		t.Fatalf("imported v0.5 meta = %+v", meta)
	}
}

func TestLegacySessionTopicIDsKeepNormalizedNameCollisionsDistinct(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	dotted := writeLegacySession(t, dir, "chat.1.jsonl", "dotted prompt", time.Now().Add(-2*time.Hour))
	underscored := writeLegacySession(t, dir, "chat_1.jsonl", "underscored prompt", time.Now().Add(-time.Hour))

	dottedTopic := legacySessionTopicID(dotted)
	underscoredTopic := legacySessionTopicID(underscored)
	if dottedTopic == underscoredTopic {
		t.Fatalf("normalized legacy topic IDs collided: %q", dottedTopic)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("global migrated topics = %d, want 2: %#v", got, nodes[0].Children)
	}
	seen := map[string]bool{}
	for _, child := range nodes[0].Children {
		seen[child.TopicID] = true
	}
	if !seen[dottedTopic] || !seen[underscoredTopic] {
		t.Fatalf("global topics = %#v, want %q and %q", nodes[0].Children, dottedTopic, underscoredTopic)
	}
}

func TestDefaultGlobalTabGetsMigratedTopicID(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "legacy-tab.jsonl", "resume this legacy tab", time.Now().Add(-time.Hour))

	tab := &WorkspaceTab{
		ID:            "tab_legacy",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		Ready:         false,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabs:        map[string]*WorkspaceTab{"tab_legacy": tab},
		tabOrder:    []string{"tab_legacy"},
		activeTabID: "tab_legacy",
	}
	app.buildTabController(tab)
	if tab.Ctrl != nil {
		defer tab.Ctrl.Close()
	}

	wantTopicID := legacySessionTopicID(sessionPath)
	if tab.TopicID != wantTopicID {
		t.Fatalf("tab topicID = %q, want %q", tab.TopicID, wantTopicID)
	}
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built")
	}
	if tab.Ctrl.SessionPath() != sessionPath {
		t.Fatalf("tab session path = %q, want %q", tab.Ctrl.SessionPath(), sessionPath)
	}
	f := loadTabsFile()
	if len(f.Tabs) != 1 || f.Tabs[0].ID != "tab_legacy" || f.Tabs[0].TopicID != wantTopicID {
		t.Fatalf("desktop tabs file = %+v, want tab id and migrated topic", f)
	}
}

func TestReorderProjectsPersistsSidebarAndWorkspaceOrder(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	third := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}
	if err := addProject(third, "Third"); err != nil {
		t.Fatalf("add third project: %v", err)
	}

	app := NewApp()
	if err := app.ReorderProjects([]string{third, first, second}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 3 {
		t.Fatalf("project tree len = %d, want 3: %+v", len(nodes), nodes)
	}
	if got := []string{nodes[0].Root, nodes[1].Root, nodes[2].Root}; got[0] != third || got[1] != first || got[2] != second {
		t.Fatalf("project tree order = %v, want %v", got, []string{third, first, second})
	}
	workspaces := app.ListWorkspaces()
	if len(workspaces) != 3 {
		t.Fatalf("workspaces len = %d, want 3: %+v", len(workspaces), workspaces)
	}
	if got := []string{workspaces[0].Path, workspaces[1].Path, workspaces[2].Path}; got[0] != third || got[1] != first || got[2] != second {
		t.Fatalf("workspace order = %v, want %v", got, []string{third, first, second})
	}
}

func TestReorderProjectsRejectsInvalidOrder(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}
	app := NewApp()
	for name, order := range map[string][]string{
		"missing":   {first},
		"unknown":   {first, filepath.Join(t.TempDir(), "missing")},
		"duplicate": {first, first},
	} {
		t.Run(name, func(t *testing.T) {
			if err := app.ReorderProjects(order); err == nil {
				t.Fatalf("ReorderProjects(%v) succeeded, want error", order)
			}
		})
	}

	nodes := app.ListProjectTree()
	if got := []string{nodes[0].Root, nodes[1].Root}; got[0] != first || got[1] != second {
		t.Fatalf("project tree order changed after invalid reorder: %v", got)
	}
}

func TestRemoveWorkspaceUsesSharedProjectRegistryForCurrentProject(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, "Current Project"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, "topic_current", "tab_current")
	app.tabs[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.RemoveWorkspace(projectRoot); err != nil {
		t.Fatalf("remove current project: %v", err)
	}
	if got := app.ListWorkspaces(); len(got) != 0 {
		t.Fatalf("workspaces after remove = %+v, want empty", got)
	}
	if got := app.ListProjectTree(); len(got) != 0 {
		t.Fatalf("project tree after remove = %+v, want empty", got)
	}
	if len(app.tabs) != 0 {
		t.Fatalf("tabs after remove = %d, want 0", len(app.tabs))
	}
}

func TestRemoveWorkspaceClearsSessionsAndTopicMetadata(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_remove_workspace"
	if err := addProject(projectRoot, "Removable"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Old chat"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "remove-me.jsonl", topicID, "Old chat", projectRoot)

	app := NewApp()
	if err := app.RemoveWorkspace(projectRoot); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	if _, err := os.Stat(sessionPath); err == nil {
		t.Fatalf("session file should be moved to trash, still at %s", sessionPath)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be cleared, got %q", got)
	}

	if err := addProject(projectRoot, "Removable"); err != nil {
		t.Fatalf("re-add project: %v", err)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1", len(nodes))
	}
	if len(nodes[0].Children) != 0 {
		t.Fatalf("re-added project should have no topics, got %#v", nodes[0].Children)
	}
}

func TestRestoredProjectTabUsesStoredTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_stored_title"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "你是谁"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, topicID, "tab1")
	app.tabs[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1", len(tabs))
	}
	if got := tabs[0].TopicTitle; got != "你是谁" {
		t.Fatalf("tab title = %q, want 你是谁", got)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project with one topic", nodes)
	}
	if got := nodes[0].Children[0].Label; got != tabs[0].TopicTitle {
		t.Fatalf("tree title = %q, want same as tab title %q", got, tabs[0].TopicTitle)
	}
}

func TestUntitledProjectTopicUsesSameFallbackEverywhere(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_without_title"
	if err := saveProjectsFile(desktopProjectFile{Projects: []desktopProject{{
		Root:   projectRoot,
		Topics: []string{topicID},
	}}}); err != nil {
		t.Fatalf("save projects: %v", err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, topicID, "tab1")
	app.tabs[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1", len(tabs))
	}
	if got := tabs[0].TopicTitle; got != defaultTopicTitle {
		t.Fatalf("tab title = %q, want %q", got, defaultTopicTitle)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project with one topic", nodes)
	}
	if got := nodes[0].Children[0].Label; got != defaultTopicTitle {
		t.Fatalf("tree title = %q, want %q", got, defaultTopicTitle)
	}
}

func TestCreateTopicDefaultsToAutoNewSessionTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if got := topic.Title; got != defaultTopicTitle {
		t.Fatalf("topic title = %q, want %q", got, defaultTopicTitle)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != defaultTopicTitle {
		t.Fatalf("stored title = %q, want %q", got, defaultTopicTitle)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceAuto {
		t.Fatalf("title source = %q, want auto", got)
	}
}

func TestCreateTopicAppearsFirstInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	first, err := app.CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create first topic: %v", err)
	}
	second, err := app.CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create second topic: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 2 {
		t.Fatalf("project tree = %#v, want one project with two topics", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != second.ID {
		t.Fatalf("first visible topic = %q, want newest %q", got, second.ID)
	}
	if got := nodes[0].Children[1].TopicID; got != first.ID {
		t.Fatalf("second visible topic = %q, want older %q", got, first.ID)
	}
}

func TestCreateGlobalTopicAppearsFirstInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	first, err := app.CreateTopic("global", "", "", "continue")
	if err != nil {
		t.Fatalf("create first global topic: %v", err)
	}
	second, err := app.CreateTopic("global", "", "", "continue")
	if err != nil {
		t.Fatalf("create second global topic: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 2 {
		t.Fatalf("project tree = %#v, want Global with two topics", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != second.ID {
		t.Fatalf("first visible global topic = %q, want newest %q", got, second.ID)
	}
	if got := nodes[0].Children[1].TopicID; got != first.ID {
		t.Fatalf("second visible global topic = %q, want older %q", got, first.ID)
	}
}

func TestSwitchWorkspaceRegistersDefaultTopicInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	if got, err := app.SwitchWorkspace(projectRoot); err != nil {
		t.Fatalf("SwitchWorkspace: %v", err)
	} else if got != projectRoot {
		t.Fatalf("SwitchWorkspace root = %q, want %q", got, projectRoot)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1: %+v", len(nodes), nodes)
	}
	if got := nodes[0].Root; got != projectRoot {
		t.Fatalf("project root = %q, want %q", got, projectRoot)
	}
	if len(nodes[0].Children) != 1 {
		t.Fatalf("project children len = %d, want 1: %+v", len(nodes[0].Children), nodes[0].Children)
	}
	child := nodes[0].Children[0]
	if got := child.Label; got != defaultTopicTitle {
		t.Fatalf("default topic label = %q, want %q", got, defaultTopicTitle)
	}
	if strings.TrimSpace(child.TopicID) == "" {
		t.Fatalf("default topic ID should be persisted in the project tree: %+v", child)
	}
	tabs := app.ListTabs()
	if len(tabs) != 1 || tabs[0].TopicID != child.TopicID {
		t.Fatalf("opened tab should use the persisted topic, tabs=%+v child=%+v", tabs, child)
	}
}

func TestRenameTopicLocksTitleManual(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if err := app.RenameTopic(topic.ID, "手动标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "手动标题" {
		t.Fatalf("stored title = %q, want 手动标题", got)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceManual {
		t.Fatalf("title source = %q, want manual", got)
	}
}

func TestRenameTopicUpdatesOpenTabMeta(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "旧标题", "continue")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	tab, err := app.OpenProjectTab(projectRoot, topic.ID)
	if err != nil {
		t.Fatalf("open project tab: %v", err)
	}
	if tab.TopicTitle != "旧标题" {
		t.Fatalf("opened tab title = %q, want 旧标题", tab.TopicTitle)
	}

	if err := app.RenameTopic(topic.ID, "新标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1: %+v", len(tabs), tabs)
	}
	if got := tabs[0].TopicTitle; got != "新标题" {
		t.Fatalf("open tab title = %q, want 新标题", got)
	}
}

func TestAutoTitleTopicFromFirstUserMessage(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"讲讲这个代码库的架构"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated {
		t.Fatal("auto title should update")
	}
	if title != "讲讲这个代码库的架构" {
		t.Fatalf("generated title = %q", title)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != title {
		t.Fatalf("stored title = %q, want %q", got, title)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceAuto {
		t.Fatalf("title source = %q, want auto", got)
	}
}

func TestAutoTitleDoesNotOverrideManualTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "", "continue")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if err := app.RenameTopic(topic.ID, "手动标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"讲讲这个代码库的架构"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	if title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath); updated || title != "" {
		t.Fatalf("manual title should not auto-update, title=%q updated=%v", title, updated)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "手动标题" {
		t.Fatalf("stored title = %q, want 手动标题", got)
	}
}

func TestTrashTopicMovesRelatedSessionsToTrash(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_trash_history"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Trash history"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "trash-me.jsonl", topicID, "Trash history", projectRoot)

	if err := NewApp().TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("topic session should be removed from active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "trash-me.jsonl", "trash-me.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("topic session should be moved to trash: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestRestoreGlobalTopicSessionReindexesProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "restore-global.jsonl", "restore global history", time.Now().Add(-time.Hour))
	topicID := legacySessionTopicID(sessionPath)
	app := NewApp()

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("legacy session should start in Global, got %#v", nodes)
	}
	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash global topic: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-global.jsonl", "restore-global.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("global session should be in trash: %v", err)
	}
	if got := app.ListProjectTree(); len(got) != 0 {
		t.Fatalf("trashed global topic should leave project tree, got %#v", got)
	}

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore global session: %v", err)
	}
	if got := app.ListTrashedSessions(); len(got) != 0 {
		t.Fatalf("trash should be empty after restore, got %#v", got)
	}
	nodes = app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored global session should reappear in Global, got %#v", nodes)
	}
}

func TestRestoreProjectTopicSessionReindexesProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_restore_project"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Project restore"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	writeTopicSession(t, dir, "restore-project.jsonl", topicID, "Project restore", projectRoot)
	app := NewApp()

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash project topic: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-project.jsonl", "restore-project.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("project session should be in trash: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed while trashed, got %q", got)
	}

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore project session: %v", err)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored project session should reappear in project tree, got %#v", nodes)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "Project restore" {
		t.Fatalf("restored topic title = %q, want Project restore", got)
	}
}

func TestRestoreSessionWithoutTopicMetadataFallsBackToGlobal(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "restore-orphan.jsonl", "restore orphan history", time.Now().Add(-time.Hour))
	topicID := legacySessionTopicID(sessionPath)
	app := NewApp()
	if err := app.DeleteSession(sessionPath); err != nil {
		t.Fatalf("delete orphan session: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-orphan.jsonl", "restore-orphan.jsonl")

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore orphan session: %v", err)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored orphan session should fall back to Global, got %#v", nodes)
	}
}

func TestTrashTopicMovesOpenSessionToTrash(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_open_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Open trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := filepath.Join(dir, "open-trash.jsonl")
	if err := agent.SaveBranchMeta(sessionPath, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topicID,
		TopicTitle:    "Open trash",
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	openTab := &WorkspaceTab{
		ID:            "tab_open",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topicID,
		TopicTitle:    "Open trash",
		Ctrl:          controllerWithContent(t, sessionPath),
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	defer openTab.Ctrl.Close()
	otherTab := &WorkspaceTab{
		ID:            "tab_other",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       "topic_keep",
		TopicTitle:    "Keep",
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabs:        map[string]*WorkspaceTab{"tab_open": openTab, "tab_other": otherTab},
		tabOrder:    []string{"tab_open", "tab_other"},
		activeTabID: "tab_open",
	}

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	if _, ok := app.tabs["tab_open"]; ok {
		t.Fatalf("open tab for trashed topic should be removed")
	}
	if got := app.activeTabID; got != "tab_other" {
		t.Fatalf("active tab = %q, want tab_other", got)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("open topic session should be removed from active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "open-trash.jsonl", "open-trash.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("open topic session should be moved to trash: %v", err)
	}
	trashed := app.ListTrashedSessions()
	if len(trashed) != 1 || trashed[0].Path != trashPath {
		t.Fatalf("trashed sessions = %#v, want %q", trashed, trashPath)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestLegacyMigrationSkipsProjectScopedSessions(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeLegacySession(t, dir, "scoped.jsonl", "hello", time.Now())
	meta, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	meta.Scope = "project"
	meta.WorkspaceRoot = filepath.Join(t.TempDir(), "proj")
	meta.TopicID = ""
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}

	migrateLegacySessionsIntoGlobalTopics(dir)

	got, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "project" || got.WorkspaceRoot != meta.WorkspaceRoot {
		t.Fatalf("project-scoped legacy session must not be forced into Global: %+v", got)
	}
}

func TestLegacyMigrationConcurrentRunsHaveNoLostUpdates(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const n = 8
	want := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		p := writeLegacySession(t, dir, fmt.Sprintf("legacy-%d.jsonl", i), "hi", time.Now())
		want[legacySessionTopicID(p)] = true
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			migrateLegacySessionsIntoGlobalTopics(dir)
		}()
	}
	wg.Wait()

	gotSet := map[string]bool{}
	for _, id := range loadProjectsFile().GlobalTopics {
		gotSet[id] = true
	}
	for id := range want {
		if !gotSet[id] {
			t.Fatalf("concurrent migration lost topic %q; GlobalTopics=%v", id, loadProjectsFile().GlobalTopics)
		}
	}
}
