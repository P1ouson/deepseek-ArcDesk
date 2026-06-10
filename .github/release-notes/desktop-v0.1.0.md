## ArcDesk Desktop v0.1.0

首个面向 **P1ouson/ArcDesk** 仓库的 Windows 桌面发布。

### 下载

| 平台 | 文件 | 说明 |
|------|------|------|
| **Windows (amd64)** | `arcdesk-desktop-amd64-installer.exe` | 约 10 MB 安装向导，可选安装目录，无需管理员 |

安装后从开始菜单或桌面快捷方式启动 **ArcDesk**，粘贴 [DeepSeek API Key](https://platform.deepseek.com/)，打开项目文件夹即可使用。

> macOS / Linux 桌面包将在后续 release 提供。CLI 可从源码 `make build` 构建。

### 本版本亮点

- **原生桌面应用** — Wails + Go 内核，studio 工作台（侧边项目栏、内联 diff、右侧 dock）
- **DeepSeek 优化** — 前缀缓存友好的 append-only 会话，长任务更省 token
- **Windows 安装器** — NSIS 向导（简体中文），开始菜单 + 桌面快捷方式
- **安全加固** — MCP 按项目信任、敏感操作系统确认、本地密钥文件权限收紧
- **独立品牌** — ArcDesk 图标与 `arcdesk.toml` 配置；可从 Reasonix 配置无损导入

### 内核渊源

Go agent 内核参考 [Reasonix](https://github.com/esengine/DeepSeek-Reasonix)。详见 README「渊源 — 与 Reasonix 的关系」。

### 首次启动提示

- **Windows SmartScreen** 可能拦截未签名构建 → 点击「更多信息」→「仍要运行」
- 需要 [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（Win10/11 通常已自带）

### 从源码构建

```powershell
cd desktop
.\scripts\build-windows-installer.ps1
```

---

**Full changelog:** `ui-redesign` branch commits through desktop branding, installer scripts, window defaults, and README polish.
