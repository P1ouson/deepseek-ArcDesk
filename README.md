<p align="center">
  <img src="desktop/build/appicon.png" alt="ArcDesk 图标" width="96" height="96"/>
</p>

<p align="center">
  <strong>ArcDesk</strong> — DeepSeek 原生 AI 编程助手（Windows 桌面版）
</p>

<p align="center">
  <a href="./README.en.md">English</a>
  &nbsp;·&nbsp;
  <a href="https://github.com/P1ouson/ArcDesk/releases">下载安装包</a>
  &nbsp;·&nbsp;
  <a href="./SECURITY.md">安全说明</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">技术规格</a>
</p>

<br/>

**ArcDesk** 是基于 Go 内核的桌面 coding agent：对话、读写代码、内联 diff、MCP 工具、项目工作区。**会话与成本设计主要面向 [DeepSeek](https://platform.deepseek.com/) 优化**（前缀缓存友好的长会话）；也可配置其他 OpenAI 兼容模型，但体验与经济性未必相同。

| | |
|---|---|
| **当前发布** | **Windows** 安装包（约 10 MB 向导，可选安装目录） |
| **内核参考** | [Reasonix](https://github.com/esengine/DeepSeek-Reasonix) Go agent 循环 |
| **许可证** | MIT（见 [`LICENSE`](./LICENSE)） |

<p align="center">
  <img src="docs/screenshots/desktop-hero.svg" alt="ArcDesk 桌面工作台界面示意" width="900"/>
</p>

<p align="center">
  <a href="https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-amd64-installer.exe"><strong>下载 Windows 安装包</strong></a>
</p>

<br/>

## 安装（Windows）

1. 下载 **[arcdesk-desktop-amd64-installer.exe](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-amd64-installer.exe)**（[Release 页](https://github.com/P1ouson/ArcDesk/releases)）。
2. 双击安装向导，选择安装目录（默认用户目录，**无需管理员**）。
3. 从开始菜单或桌面快捷方式打开 **ArcDesk**，粘贴 DeepSeek API Key，**打开项目文件夹**即可使用。

> **macOS / Linux 桌面版** 尚未在本仓库发布；需要可自行从源码构建（见 [`desktop/README.md`](./desktop/README.md)）。

> **SmartScreen**：未签名构建可能被拦截 →「更多信息」→「仍要运行」。需 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)（Win10/11 通常已自带）。

<br/>

## 60 秒上手

1. 安装并启动 ArcDesk  
2. 填入 [DeepSeek API Key](https://platform.deepseek.com/)（仅保存在本机）  
3. 打开本地项目目录，在对话框描述任务  

<br/>

## 常见问题

**和 ARCDESK / Reasonix 是什么关系？**  
**ArcDesk** 是本仓库的桌面产品名。CLI 侧仍使用 `ARCDESK` / `arcdesk` 命令与 `arcdesk.toml` 配置。Go 内核参考 [Reasonix](https://github.com/esengine/DeepSeek-Reasonix)，见下文「渊源」。

**是否免费？**  
软件 MIT 开源免费；模型 API 按 DeepSeek（或其他 provider）用量计费。

**必须用 DeepSeek 吗？**  
**推荐 DeepSeek**——内核的长会话、前缀缓存与预设主要为此优化。其他 OpenAI 兼容端点可在 `arcdesk.toml` 的 `[[providers]]` 中配置。

**支持 MCP 吗？**  
支持。项目根目录 `.mcp.json` 或 `arcdesk.toml` 的 `[[plugins]]`；桌面端对新 MCP 源需**按项目信任**后才启用。

<br/>

## 从源码构建

```powershell
# Windows 桌面安装包（需 NSIS：winget install NSIS.NSIS）
cd desktop
.\scripts\build-windows-installer.ps1
```

CLI 与内核：

```sh
make build    # -> bin/ARCDESK(.exe)
cd desktop && wails build
```

详见 [`desktop/README.md`](./desktop/README.md)、[`CONTRIBUTING.md`](./CONTRIBUTING.md)。

<br/>

## 渊源 — 与 Reasonix

ArcDesk 的 **Go agent 内核**参考并延续自 [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix)（工具、子 agent、skills、MCP、计划模式、CodeGraph 等）。感谢 Reasonix 项目及其贡献者。

本仓库在 Reasonix 内核之上侧重：

- **Wails 原生桌面** — studio 工作台（侧栏、项目抽屉、内联 diff）
- **Windows NSIS 安装器** — 可选路径、开始菜单 / 桌面快捷方式
- **安全加固** — 敏感操作系统确认、MCP 隔离、配对限流等
- **独立品牌与配置** — `arcdesk.toml` / `.arcdesk/`；可从 `~/.reasonix/` **无损导入**旧配置

更多细节见 [`SECURITY.md`](./SECURITY.md)。

<br/>

---

<p align="center">
  <sub>MIT · <a href="https://github.com/P1ouson/ArcDesk">P1ouson/ArcDesk</a></sub>
</p>
