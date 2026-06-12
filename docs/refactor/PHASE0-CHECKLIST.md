# Deep Maintainability Refactor — Phase 0

## Dependency map (frontend critical path)

```
main.tsx → App.tsx
  ├─ useController (state/events)
  ├─ useWorkbenchDock / useTerminalPanel / useBrowserPanel
  ├─ MessageTimeline → buildTimelineRows → ActionStream | ShellCommandCard
  ├─ FloatingComposer → Composer
  ├─ GitPanel | ChangesPanel | FilesPanel → useWorkspaceChanges
  └─ SettingsPanel → settings/* sections

bridge.ts → Go App (Wails)
motion: design-system.css → motion.css → styles.css → studio-layout.css
```

## Risk points

| Module | Risk | Mitigation |
|--------|------|------------|
| useController.ts | High — all agent state | Run useController.test.ts after changes |
| bridge.ts | High — IPC contract | check:bridge script |
| MessageTimeline routing | Medium — shell/tool path | actionStream.test.ts |
| CSS motion | Medium — visual regressions | Token single source in lib/motion |
| Git/Changes panels | Medium — git ops | Manual smoke after Phase 2 |

## Phase checklist

- [x] Branch `refactor/deep-maintainability`
- [x] Dependency map
- [ ] Phase 1 commit
- [ ] Phase 2 commit
- [ ] Phase 3 commit

## Residual search commands (each phase)

```powershell
rg "deprecated|FIXME|TODO" desktop/frontend/src
rg "String\(\(e as Error\)" desktop/frontend/src
rg "ComposerModeToggle|HljsDiff|permissionSuggest|canOpenFileFromTool" desktop/frontend/src
rg "@keyframes (spin|pulse|fade-in)" desktop/frontend/src --glob "*.css"
```
