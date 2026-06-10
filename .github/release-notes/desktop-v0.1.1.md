## ArcDesk Desktop v0.1.1

三平台**安装包**发布（Windows / macOS / Linux）。

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

### 首次启动

- **Windows** SmartScreen →「更多信息」→「仍要运行」（未签名构建）；需要 WebView2
- **macOS** 若提示已损坏：`xattr -dr com.apple.quarantine /Applications/ArcDesk.app`
- **Linux** 确保 `~/.local/bin` 在 PATH 中
