## ArcDesk Desktop desktop-v0.1.6

桌面版 **可维护性重构、Context 缓存费用估算、仓库清理与 Windows 构建修复**（2026-06-13）。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |
| **macOS (Universal)** | `arcdesk-desktop-darwin-universal.dmg` |
| **Linux (amd64)** | `arcdesk-desktop-linux-amd64-installer.tar.gz` |

> Windows 仅分发 **NSIS 安装包**（内含应用 exe 与 WebView2 引导）；无需单独下载裸 exe。

---

### 可维护性重构（Phase 0–3）

- **Phase 0** — 新增 `docs/refactor/PHASE0-CHECKLIST.md`：前端依赖关系图、风险点与分阶段验收命令
- **Phase 1** — 统一动效 token、错误展示、shell 路径与 Markdown 渲染；移除死代码
- **Phase 2** — Dock 面板逻辑去重；抽取共享 hooks 与 localStorage store
- **Phase 3** — 统一主题同步、浮层开闭生命周期、设置页 busy 流程；Composer 接入 `TurnProgressLine` 轮次进度条

### Context 面板

- 补齐 `cacheEconomy` 模块：存在 prompt cache 数据时，右侧概览显示 **DeepSeek 缓存费用节省估算**（相对无缓存 prompt 价）

### 仓库与构建

- 从版本库移除本地 benchmark 跑分产物（json / log / report），保留 `benchmarks/e2e/tasks/` 正式用例；`desktop/benchmarks/` 仅保留 `.gitkeep`
- 修复 Windows 完整构建链：补全 `cacheEconomy.ts`、`generate-appicon.py` 生成 `icon.ico`、`wails generate module` 恢复 `wailsjs` 绑定；验证 exe 可编译并正常启动
- 本地开发仍可用 `desktop/build-dev.ps1` 快速产出 exe；**Release 只上传 installer**

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
- **macOS** — 右键 → 打开，或 `xattr -dr com.apple.quarantine /Applications/ArcDesk.app`
- **Linux** — 解压后运行 `./install.sh`
