# Screenshots

README 截图资源。路径相对于仓库根目录 `docs/screenshots/`。

## 已提交（Phase A · 第一眼）

| 文件 | 用途 |
|------|------|
| `desktop-hero.gif` | README 顶部 30 秒预览（实机窗口录帧） |
| `desktop-workbench.png` | 静态主图：代码工作区欢迎页 |
| `desktop-hero.svg` | 无实机时的占位矢量图（备用） |

### 重新生成

需先构建桌面版：`cd desktop && wails build`

```powershell
# Windows · PNG
python desktop/scripts/capture-screenshot.py

# Windows · GIF（8 帧）
python desktop/scripts/capture-hero-gif.py
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
