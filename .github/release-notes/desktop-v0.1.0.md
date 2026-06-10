## ArcDesk Desktop v0.1.0

首个 **Windows 桌面**发布（本仓库当前仅提供 Windows 安装包）。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-amd64-installer.exe` — 约 10 MB，NSIS 安装向导，可选安装目录，无需管理员 |

安装后从开始菜单或桌面快捷方式启动，填入 [DeepSeek API Key](https://platform.deepseek.com/)，打开项目文件夹即可。

> macOS / Linux 桌面包尚未发布，可从源码构建（见 README）。

### 亮点

- Wails 原生桌面 — studio 工作台（侧栏、项目抽屉、内联 diff）
- **主要针对 DeepSeek 优化** — 前缀缓存友好的长会话设计
- 安全加固 — MCP 按项目信任、敏感操作系统确认
- 独立 ArcDesk 品牌与图标；可从 Reasonix 配置无损导入

### 内核渊源

Go agent 内核参考 [Reasonix](https://github.com/esengine/DeepSeek-Reasonix)。

### 首次启动

- Windows SmartScreen →「更多信息」→「仍要运行」（未签名构建）
- 需要 WebView2 运行时
