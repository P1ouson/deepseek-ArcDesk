## ArcDesk Desktop v0.1.2

三平台**安装包**发布（Windows / macOS / Linux），由 GitHub Actions 自动构建。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` — NSIS 安装向导，可选安装目录，无需管理员 |
| **macOS (Universal)** | `arcdesk-desktop-darwin-universal.dmg` — 拖入「应用程序」 |
| **Linux (amd64)** | `arcdesk-desktop-linux-amd64-installer.tar.gz` — 解压后运行 `./install.sh` |

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，打开项目文件夹即可。

### 亮点

- Wails 原生桌面 — studio 工作台（侧栏、项目抽屉、内联 diff）
- **主要针对 DeepSeek 优化** — 前缀缓存友好的长会话设计
- 安全加固 — MCP 按项目信任、敏感操作系统确认
- 独立 ArcDesk 品牌与图标；可从 Reasonix 配置无损导入

### 内核渊源

Go agent 内核参考 [Reasonix](https://github.com/esengine/DeepSeek-Reasonix)。

### 首次启动（未签名构建，属正常现象）

当前安装包**没有** Apple / Microsoft 代码签名证书，从 GitHub 下载后会触发系统安全提示——这不是应用损坏，开源项目在未购买签名证书前通常如此（Reasonix 桌面版同理）。

- **Windows** — 运行安装包时 SmartScreen 可能提示「未知发布者」→ **更多信息 → 仍要运行**。安装向导会**自动尝试安装 WebView2**；仅当安装后窗口空白时，再手动安装 [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)。
- **macOS** — 从浏览器下载的 `.dmg` 会带隔离属性，Gatekeeper 可能报「已损坏」或「无法验证开发者」。一次性执行：`xattr -dr com.apple.quarantine /Applications/ArcDesk.app`，或在 Finder 中**右键 → 打开**确认。
- **Linux** — 从应用菜单启动**不需要**改 PATH。只有要在终端里直接敲 `arcdesk-desktop` 时，才需把 `~/.local/bin` 加入 PATH（Ubuntu 等桌面版通常已默认包含）。
