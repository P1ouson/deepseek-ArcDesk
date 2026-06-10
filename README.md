<p align="center">
  <img src="docs/logo.svg" alt="ArcDesk" width="440"/>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/P1ouson/ArcDesk?style=flat-square&color=8b949e&labelColor=161b22" alt="MIT"/></a>
  <a href="https://github.com/P1ouson/ArcDesk/releases"><img src="https://img.shields.io/github/v/release/P1ouson/ArcDesk?include_prereleases&style=flat-square&color=0153e5&labelColor=161b22" alt="release"/></a>
</p>

<p align="center">
  <a href="./README.en.md">English</a>
  &nbsp;·&nbsp;
  <strong>简体中文</strong>
  &nbsp;·&nbsp;
  <a href="https://github.com/P1ouson/ArcDesk/releases">发布页</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">规格</a>
  &nbsp;·&nbsp;
  <a href="./SECURITY.md">安全</a>
</p>

<br/>

**面向 DeepSeek 的原生 coding agent 桌面应用。** 同一 Go 内核也驱动 CLI（`ARCDESK`）。长会话围绕前缀缓存设计，成本可控；工具、MCP、内联 diff、项目工作区都在一个原生窗口里。

> **命名**：**ArcDesk** = 产品与桌面应用 · **ARCDESK** = CLI / 配置前缀（`ARCDESK.toml`）

<p align="center">
  <a href="https://github.com/P1ouson/ArcDesk/releases">
    <img src="docs/screenshots/desktop-workbench.png" alt="ArcDesk 桌面工作台" width="900"/>
  </a>
</p>

<p align="center">
  <a href="https://github.com/P1ouson/ArcDesk/releases"><strong>下载安装包</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#cli">CLI / 源码</a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#常见问题">FAQ</a>
</p>

<br/>

## 安装

支持 **Windows · macOS · Linux**。从 [Releases](https://github.com/P1ouson/ArcDesk/releases) 下载**安装包**（不是源码 zip）。

| 平台 | 安装包 | 说明 |
|------|------|------|
| **Windows** | [`.exe`](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-windows-amd64-installer.exe) | 安装向导，可选目录，无需管理员 |
| **macOS** | [`.dmg`](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-darwin-universal.dmg) | 拖入「应用程序」 |
| **Linux** | [`.tar.gz`](https://github.com/P1ouson/ArcDesk/releases/latest/download/arcdesk-desktop-linux-amd64-installer.tar.gz) | 解压后 `./install.sh` |

1. 安装并打开 **ArcDesk**
2. 粘贴 [DeepSeek API Key](https://platform.deepseek.com/)（本地保存）
3. 打开项目文件夹，描述任务

Linux 示例：`tar -xzf arcdesk-desktop-linux-amd64-installer.tar.gz && ./install.sh && arcdesk-desktop`

> 安装包尚未代码签名。macOS / Windows 首次启动可能被拦截 — 见 [故障排查](#故障排查)。

### CLI / 从源码 {#cli}

本仓库**未发布 npm 包**。本地构建 CLI：

```sh
make build
./bin/arcdesk chat
./bin/arcdesk run "解释这个仓库"
```

桌面从源码：`cd desktop && wails build` · Windows 安装包：`desktop/scripts/build-windows-installer.ps1`（需 NSIS）。详见 [`desktop/README.md`](./desktop/README.md)。

<br/>

## 快速上手

| 步骤 | 桌面 | CLI |
|------|------|-----|
| 1 | 安装对应平台安装包 | `make build` |
| 2 | 填入 API Key | `export DEEPSEEK_API_KEY=sk-...` 或 `ARCDESK setup` |
| 3 | 打开项目，输入任务 | `ARCDESK chat` 或 `ARCDESK run "..."` |

<br/>

## 和 Reasonix 有什么不同？

Go agent 内核参考 [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix)。ArcDesk 走**桌面优先**：

- **Wails 原生壳** — 侧栏、项目抽屉、内联 diff，不是终端 TUI
- **三平台安装包** — Windows NSIS / macOS dmg / Linux tar.gz，GitHub Actions 自动构建
- **安全加固** — MCP 按项目信任、敏感操作确认、工作区沙盒（见 [`SECURITY.md`](./SECURITY.md)）
- **配置迁移** — `arcdesk.toml`，可从 `~/.reasonix/` 非破坏性导入

<br/>

## 对比

| | **ArcDesk** | **Cursor** | **Claude Code** | **Reasonix** |
|---|:---:|:---:|:---:|:---:|
| **形态** | 原生桌面 + CLI | IDE 分支 | CLI / 插件 | 终端 / 桌面（上游） |
| **DeepSeek 成本** | 前缀缓存会话 | 多模型 | Claude 生态 | **DeepSeek 专精** |
| **MCP** | stdio + HTTP | 生态 | 支持 | 支持 |
| **开源** | MIT | 闭源 | 闭源 | MIT |

<br/>

## 配置

TOML 驱动：`./ARCDESK.toml`（项目）· `~/.config/arcdesk/config.toml`（用户）· 支持 `.mcp.json` 与 Claude 风格 MCP 配置。

完整 schema、权限规则、斜杠命令、插件契约 → [`docs/SPEC.md`](./docs/SPEC.md) · 示例 → [`ARCDESK.example.toml`](./ARCDESK.example.toml)

<br/>

## 常见问题 {#常见问题}

**ArcDesk 和 ARCDESK？** — 同一内核；ArcDesk 是桌面产品名，ARCDESK 是 CLI 命令。

**免费吗？** — 软件 MIT 免费；DeepSeek API 按用量计费。

**必须用 DeepSeek 吗？** — **推荐**。也支持 OpenAI 兼容 `[[providers]]`，但会话与成本优化主要针对 DeepSeek。

**必须用桌面吗？** — 否，`ARCDESK chat` / `run` 即可。

<br/>

## 故障排查 {#故障排查}

| 现象 | 处理 |
|------|------|
| macOS「应用已损坏」 | `xattr -dr com.apple.quarantine /Applications/ArcDesk.app` |
| Windows SmartScreen | *更多信息 → 仍要运行* |
| Windows 空白窗口 | 安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) |
| Linux 空白/闪烁 | WebKitGTK 4.1；可试 `WEBKIT_DISABLE_COMPOSITING_MODE=1` |
| MCP 未加载 | 桌面 UI 信任项目/服务器；检查 `.mcp.json` |

<br/>

## 文档

- [`docs/SPEC.md`](./docs/SPEC.md) — 配置、工具、MCP、权限
- [`desktop/README.md`](./desktop/README.md) — 桌面构建与开发
- [`SECURITY.md`](./SECURITY.md) — 安全模型
- [`docs/MIGRATING.md`](./docs/MIGRATING.md) — 从 Reasonix / 0.x 迁移

<br/>

## 致谢

Go agent 内核参考 [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix) 及其贡献者。

---

<p align="center">
  <sub>MIT — <a href="./LICENSE">LICENSE</a> · <a href="https://github.com/P1ouson/ArcDesk">P1ouson/ArcDesk</a></sub>
</p>
