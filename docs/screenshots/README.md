# Screenshots

README 截图资源。路径相对于仓库根目录 `docs/screenshots/`。

## 已提交 / 待录制

| 文件 | 用途 |
|------|------|
| `demo-agent-loop.gif` | README 主 Demo（一条 Prompt 工具链） |
| `desktop-hero.svg` | 无 GIF 时的占位矢量图 |

录制剧本与 Prompt → [`demo/README.md`](../../demo/README.md)（演示项目 `demo/todo-api`）。

### 重新生成（旧脚本 · 欢迎页）

需先构建桌面版：`cd desktop && wails build`

```powershell
python desktop/scripts/capture-hero-gif.py   # → desktop-hero.gif（旧）
python desktop/scripts/capture-screenshot.py # → desktop-workbench.png
```

```powershell
# 备选 · PowerShell PNG（需 .NET System.Drawing）
desktop/scripts/capture-screenshot.ps1
```

**Capture tips:** 使用真实项目、打码 API Key 与敏感路径；等待 ≥12s 让欢迎页加载完成；Light/Dark 各一套可选。

## 待补（可选）

| 文件 | 对应侧栏 |
|------|----------|
| `sidebar-write.png` | 写作 |
| `sidebar-extensions.png` | 扩展 |
| `sidebar-schedule.png` | 定时 |
| `sidebar-connect.png` | 连接 |
| `sidebar-settings.png` | 设置 |
| `runtime-agent-approval.png` | Agent 审批实机 |
| `runtime-web-preview.png` | 沙盒 Browser（多标签 + 适应面板） |
| `runtime-writing.png` | 写作助手 |
