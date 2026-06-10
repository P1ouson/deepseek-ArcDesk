# Phase 9A Freeze Baseline

Recorded before PR-1 (Read Path Confinement).

## Git state

- **Branch:** `ui-redesign`
- **HEAD:** `8f93589f28b3866302bdb13731c49facf27c6c15`
- **Note:** Working tree has unstaged Phase 8 / UI changes (not included in this freeze commit).

## Toolchain

- **Go:** `go.mod` requires `>= 1.25.0`; tests run with `GOTOOLCHAIN=auto`.

## Baseline test status (2026-06-10)

| Suite | Command | Result |
|-------|---------|--------|
| Go unit (full) | `GOTOOLCHAIN=auto go test ./...` | **FAIL** (pre-existing) |
| Go unit (control) | `go test ./internal/control/...` | **PASS** |
| Go build | `go build ./cmd/ARCDESK` | Not run (toolchain pin) |

### Known pre-existing failures (unrelated to Phase 9A)

- `arcdesk/internal/tool/builtin` — `TestBashPowerShellRunsNativeCommand`, `TestEditFile`, `TestMultiEdit`, `TestMultiEditGB18030RoundTrip`
- `arcdesk/internal/provider/openai` — `TestStreamDoesNotReplayAfterOutput`

### E2E

- Frontend: `pnpm test:e2e` (Playwright) — not run at freeze (requires browser install + dev server).

## Rollback reference

```bash
git checkout -- internal/tool/builtin internal/control/refs.go internal/control/controller.go internal/boot/boot.go
```

---

## PR-2 freeze (before Mobile / LAN Auth Hardening)

Recorded immediately before PR-2 implementation.

- **Branch:** `ui-redesign`
- **Scope:** `desktop/mobile_connect.go`, `desktop/mobile_decision.go`, `desktop/mobile_page.html`, new auth tests only

---

## PR-3 freeze (before LAN Bind Hardening)

Recorded immediately before PR-3 implementation.

- **PR-1 / PR-2 behavior preserved**
- **Scope:** `desktop/channel_bridge.go`, `desktop/mobile_connect.go`, bind tests, minimal `allowLAN` config + UI toggle

---

## PR-4 freeze (before Headless Approval Hardening)

Recorded immediately before PR-4 implementation (read-only audit; no code changes at freeze).

- **Branch:** `ui-redesign`
- **HEAD:** `8f93589f28b3866302bdb13731c49facf27c6c15`
- **PR-1 / PR-2 / PR-3 behavior preserved**
- **Scope (planned):** subagent gate inheritance, mobile/claw interactive approval routing — no reducer, no permission-system rewrite, no mobile auth / LAN bind changes
- **Working tree:** unstaged Phase 8 / UI / PR-1–PR-3 changes present (same as PR-1 freeze note)

### Rollback reference (cumulative through PR-3)

```bash
git checkout -- \
  internal/tool/builtin internal/control/refs.go internal/control/controller.go internal/boot/boot.go \
  desktop/mobile_connect.go desktop/mobile_decision.go desktop/mobile_page.html \
  desktop/channel_bridge.go desktop/cloudflared_tunnel.go \
  desktop/frontend/src/components/ConnectPhoneView.tsx
```

---

## PR-4 implementation (Headless Approval Hardening)

Recorded at PR-4 implementation start (both tracks approved).

- **Branch:** `ui-redesign`
- **HEAD:** `8f93589f28b3866302bdb13731c49facf27c6c15`
- **PR-1 / PR-2 / PR-3 behavior preserved**
- **Track A:** desktop-only subagent gate inheritance (`EnableDesktopSubagentGate`; CLI/TUI/headless unchanged)
- **Track B:** claw/mobile/WeCom interactive approval via `__claw__` sentinel + `clawRunCtrl` routing
- **Explicitly untouched:** reducer, controller turn lifecycle, `permission.Gate.Check` nil-approver semantics, PR-2 auth, PR-3 bind

### PR-4 rollback

```bash
git checkout -- \
  internal/agent/agent.go internal/agent/task.go internal/boot/boot.go \
  internal/permission/permission.go internal/control/controller.go \
  desktop/claw_agent.go desktop/app.go desktop/mobile_decision.go \
  desktop/tabs.go desktop/app.go desktop/settings_app.go

rm -f internal/agent/task_gate_test.go desktop/headless_approval_test.go desktop/desktop_interactive.go
```

### PR-4 test status (2026-06-10)

| Suite | Command | Result |
|-------|---------|--------|
| Agent (PR-4 targeted) | `go test ./internal/agent -run 'TestEffectiveSubagentGate\|TestExecuteOneStamps\|TestExecuteOneOmits\|TestTaskSubagent'` | **PASS** |
| Control | `go test ./internal/control/...` | **PASS** |
| Permission | `go test ./internal/permission/...` | **PASS** |
| Desktop (full) | `go test ./desktop/...` | **PASS** (includes PR-2 auth + PR-3 bind + PR-4 headless) |
| Agent (full) | `go test ./internal/agent/...` | **TIMEOUT** (pre-existing e2e; unrelated to PR-4) |

---

## PR-5 freeze (before Security Batch: IPC / MCP / Skill Trust)

Recorded immediately before PR-5 implementation.

- **Branch:** `ui-redesign`
- **PR-1 through PR-4 behavior preserved**
- **Track A:** IPC privilege split — native confirm on high-risk Wails bindings
- **Track B:** `.mcp.json` repo MCP quarantine + per-server project-scoped trust
- **Track C:** Project skill quarantine + `install_skill` write guard on desktop
- **Explicitly untouched:** reducer, controller turn lifecycle, PR-1–PR-4 paths

### PR-5 rollback (planned)

```bash
git checkout -- \
  desktop/privilege.go desktop/app.go desktop/settings_app.go desktop/tabs.go \
  desktop/mcp_trust.go internal/hook/trust.go internal/config/config.go \
  internal/config/mcpjson.go internal/boot/boot.go internal/skill/skill.go \
  internal/skill/tools.go

rm -f desktop/privilege_test.go desktop/mcp_trust_test.go \
  internal/boot/mcp_trust_test.go internal/skill/skill_trust_test.go \
  internal/hook/trust_mcp_test.go
```

### PR-5 test status (2026-06-10)

| Suite | Command | Result |
|-------|---------|--------|
| Desktop (full) | `go test ./desktop/...` | **PASS** |
| Desktop privilege | `go test ./desktop -run TestConfirm` | **PASS** |
| Boot MCP trust | `go test ./internal/boot -run TestFilterTrusted` | **PASS** |
| Hook MCP trust | `go test ./internal/hook -run TestMCPServer` | **PASS** |
| Skill trust | `go test ./internal/skill -run TestProjectSkills` | **PASS** |

---

## PR-9B freeze (before Final Security Closure Batch)

Recorded immediately before PR-9B implementation (read-only audit complete; no code changes at freeze).

- **Branch:** `ui-redesign`
- **HEAD:** `8f93589f28b3866302bdb13731c49facf27c6c15`
- **PR-1 through PR-5 behavior preserved**
- **Track A:** tunnel confirm + auth-state display + idle auto-shutdown (default on)
- **Track B:** safe-tier IPC — decision route validation, AddMCPServer confirm
- **Track C:** credential promotion confirm, sensitive file permissions, log redaction, mobile session expiry
- **Explicitly untouched:** reducer, controller turn lifecycle, auth redesign, relay rewrite, CSP/Wails rewrite

### PR-9B rollback (planned)

```bash
git checkout -- \
  desktop/cloudflared_tunnel.go desktop/channel_bridge.go desktop/mobile_connect.go \
  desktop/app.go desktop/privilege.go desktop/dotenv.go desktop/tabs.go \
  desktop/mobile_decision_note.go desktop/mobile_decision.go \
  desktop/frontend/src/components/ConnectPhoneView.tsx \
  desktop/frontend/src/lib/types.ts desktop/frontend/src/locales/en.ts \
  desktop/frontend/src/locales/zh.ts

rm -f desktop/decision_route.go desktop/redact.go desktop/security_closure_test.go
```

### PR-9B implementation (Final Security Closure Batch)

Recorded at PR-9B completion.

- **Track A:** native confirm before tunnel start; auth-state fields on `MobileTunnelStatus`; idle auto-shutdown (default 30 min); UI warning + idle toggle
- **Track B:** `decisionRouteStore` binds approval/ask IDs to issuing tab; cross-tab `ApproveTab`/`AnswerQuestionForTab` rejected; `AddMCPServer` confirm
- **Track C:** promotable-key detection + confirm before `.env` → global credentials; `saveSensitiveJSON` / credentials `0600`; log URL redaction; mobile session idle (72h) + max-age (7d) expiry

### PR-9B test status (2026-06-10)

| Suite | Command | Result |
|-------|---------|--------|
| Desktop (full) | `go test ./desktop/...` | **PASS** |
| Desktop (9B targeted) | `go test ./desktop -run 'TestDecisionRoute\|TestStartMobileTunnel\|TestTunnelStatus\|TestMobileSession\|TestRedact\|TestResolveDecision'` | **PASS** |
