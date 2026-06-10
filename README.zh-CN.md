<p align="center">
  <img src="docs/logo.svg" alt="ArcDesk" width="440"/>
</p>

<p align="center">
  <a href="https://www.npmjs.com/package/ARCDESK"><img src="https://img.shields.io/npm/v/ARCDESK.svg?style=flat-square&color=cb3837&labelColor=161b22&logo=npm&logoColor=white" alt="npm"/></a>
  <a href="./LICENSE"><img src="https://img.shields.io/npm/l/ARCDESK.svg?style=flat-square&color=8b949e&labelColor=161b22" alt="MIT"/></a>
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/stargazers"><img src="https://img.shields.io/github/stars/esengine/DeepSeek-ARCDESK?style=flat-square&color=dbab09&labelColor=161b22&logo=github&logoColor=white" alt="stars"/></a>
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/esengine/DeepSeek-ARCDESK/ci.yml?branch=main-v2&style=flat-square&label=ci&labelColor=161b22&logo=githubactions&logoColor=white" alt="CI"/></a>
  <a href="https://discord.gg/XF78rEME2D"><img src="https://img.shields.io/badge/discord-join-5865F2.svg?style=flat-square&labelColor=161b22&logo=discord&logoColor=white" alt="Discord"/></a>
</p>

<p align="center">
  <a href="./README.md">English</a>
  &nbsp;·&nbsp;
  <strong>简体中文</strong>
  &nbsp;·&nbsp;
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/releases">发布页</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">规格</a>
  &nbsp;·&nbsp;
  <a href="./SECURITY.md">安全</a>
  &nbsp;·&nbsp;
  <a href="./CONTRIBUTING.md">贡献</a>
  &nbsp;·&nbsp;
  <a href="https://discord.gg/XF78rEME2D">Discord</a>
</p>

<br/>

# ArcDesk

**MIT 开源的 DeepSeek 原生 coding agent —— 桌面应用 + CLI，共用同一 Go 内核。**

长会话不必每轮为完整上下文付全价。**ArcDesk** 是桌面产品名；**`ARCDESK`** 是 CLI 命令与配置前缀（`ARCDESK.toml`）。

| | |
|---|---|
| **桌面优先** | Windows · macOS · Linux — 对话、工具、内联 diff、项目工作区 |
| **DeepSeek 成本** | 面向前缀缓存的 append-only 会话；可选执行器 + 规划器分离 |
| **开放可控** | MCP（stdio + HTTP）、`.mcp.json`、TOML 权限规则、MIT 源码 |

<p align="center">
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/releases">
    <img src="docs/screenshots/desktop-hero.svg" alt="ArcDesk 桌面工作台（截图占位）" width="900"/>
  </a>
</p>

<p align="center">
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/releases"><strong>下载桌面版</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#cli-安装"><strong>安装 CLI</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="./docs/SPEC.md"><strong>阅读规格</strong></a>
  &nbsp;&nbsp;·&nbsp;&nbsp;
  <a href="#常见问题"><strong>FAQ</strong></a>
</p>

<br/>

## 快速安装

### 桌面版

| 平台 | 下载 |
|------|------|
| **Windows** | [`.exe` 安装包](https://github.com/esengine/DeepSeek-ARCDESK/releases/latest/download/arcdesk-desktop-amd64-installer.exe)（小安装器，可选路径） |
| **macOS** | [通用 `.dmg`](https://github.com/esengine/DeepSeek-ARCDESK/releases/latest/download/ARCDESK-darwin-universal.dmg) |
| **Linux** | [`.tar.gz` (amd64)](https://github.com/esengine/DeepSeek-ARCDESK/releases/latest/download/ARCDESK-linux-amd64.tar.gz) |

1. 从表格或 **[GitHub Releases](https://github.com/esengine/DeepSeek-ARCDESK/releases)** 下载。
2. 打开 **ArcDesk**，在引导页粘贴 [DeepSeek API Key](https://platform.deepseek.com/)（本地保存）。
3. **打开项目文件夹**，输入任务即可。

校验和与 **minisign** 签名见发布页 · [`SECURITY.md`](./SECURITY.md) · [`desktop/README.md`](./desktop/README.md)。

> 首次启动：macOS Gatekeeper / Windows SmartScreen 可能拦截未签名构建 — 见 [故障排查](#故障排查)。

### CLI 安装 {#cli-安装}

```sh
npm i -g ARCDESK
```

安装后为原生 **Go 二进制**（Node 仅作安装器）。macOS 可选：`brew install esengine/ARCDESK/ARCDESK`。

<br/>

## 60 秒上手

**桌面**：安装 → API Key → 打开项目 → 输入任务。

**CLI**

```sh
export DEEPSEEK_API_KEY=sk-...     # 或：ARCDESK setup
ARCDESK chat
ARCDESK run "解释这个仓库"
```

<br/>

## 为什么选 ArcDesk？

| | **ArcDesk** | **Cursor** | **Cline / Roo** | **Claude Code** | **OpenCode** |
|---|:---:|:---:|:---:|:---:|:---:|
| **桌面应用** | 原生 (Wails) | VS Code 分支 | 编辑器插件 | CLI / 插件 | 终端优先 |
| **DeepSeek / 成本** | 前缀缓存会话设计 | 多模型 IDE | 模型无关 | Claude 生态 | 模型无关 CLI |
| **MCP** | stdio + HTTP；`.mcp.json` | 生态 | 支持 | 支持 MCP | 各异 |
| **本地控制** | TOML、权限、沙箱 | 账号策略 | 插件设置 | Anthropic 账号 | 配置 / 环境 |

<br/>

## 安全与信任

默认 **先询问再执行**。详见 [`SECURITY.md`](./SECURITY.md) · [`desktop/README.md`](./desktop/README.md) · [`docs/SPEC.md`](./docs/SPEC.md) §9。

<br/>

## 常见问题 {#常见问题}

**ArcDesk 和 ARCDESK 有什么区别？** — ArcDesk 是产品与桌面应用；ARCDESK 是 CLI 命令与配置命名空间，底层同一内核。

**是否免费？** — 软件 MIT 免费；模型 API（如 DeepSeek）按用量计费。

**必须用桌面吗？** — 否；`ARCDESK chat` / `run` 与桌面共用引擎。

**支持非 DeepSeek 模型吗？** — 可接入任意 OpenAI 兼容端点（`ARCDESK.toml` 的 `[[providers]]`），但**内核会话设计与成本优化主要针对 DeepSeek**（前缀缓存、flash/pro 预设、长会话控费等）；其他模型可用，体验与经济性未必相同。

**0.x 如何迁移？** — 见 [`docs/MIGRATING.md`](./docs/MIGRATING.md)；legacy 在 [`v1`](https://github.com/esengine/DeepSeek-ARCDESK/tree/v1) 分支。

<br/>

## 故障排查 {#故障排查}

| 现象 | 处理 |
|------|------|
| macOS「应用已损坏」 | `xattr -dr com.apple.quarantine /Applications/ARCDESK.app` |
| Windows SmartScreen | *更多信息 → 仍要运行* |
| Windows 空白窗口 | 安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) |
| Linux 空白/闪烁 | 安装 WebKitGTK 4.1；可试 `WEBKIT_DISABLE_COMPOSITING_MODE=1` |
| MCP 未加载 | 在桌面 UI 信任项目/服务器；检查 `.mcp.json` |

更多：[Discussions](https://github.com/esengine/DeepSeek-ARCDESK/discussions) · [Discord](https://discord.gg/XF78rEME2D)

<br/>

---

> **命名**：**ArcDesk** = 产品 · **ARCDESK** = CLI / 配置 · 仓库 [`esengine/DeepSeek-ARCDESK`](https://github.com/esengine/DeepSeek-ARCDESK)

<br/>

## 特性

- **配置驱动**：provider、agent、启用的工具、插件全部在 `ARCDESK.toml` 中声明，
  内核无硬编码模型。
- **多模型 · 可组合**：DeepSeek（flash/pro）与 MiMo 作为预设内置；也可接入 OpenAI 兼容
  端点，但**长会话成本与缓存策略主要围绕 DeepSeek 优化**。可选让两个模型协同（执行器 + 规划器），各自独立、缓存稳定的 session。
- **插件驱动**：外部工具以子进程形式运行，通过 stdio JSON-RPC 通信（MCP 兼容）；
  内置工具在编译期自注册。
- **零摩擦分发**：`CGO_ENABLED=0` 单二进制；一条命令交叉编译到六个目标平台。
  桌面版为原生 Wails 构建。

### 从源码构建

```sh
make build      # -> bin/ARCDESK(.exe)
make cross      # -> dist/（darwin|linux|windows × amd64|arm64）
cd desktop && wails build   # 桌面应用（见 desktop/README.md）
```

**Windows 安装向导**（别人那种小安装包，可选安装目录）：

```powershell
# 需先安装 NSIS：winget install NSIS.NSIS
cd desktop
./scripts/build-windows-installer.ps1
# 产物：build/bin/arcdesk-desktop-amd64-installer.exe（约 10MB）
# 用户双击 → 欢迎页 → 选文件夹 → 安装 → 开始菜单/桌面快捷方式
```

## 配置

优先级：**flag > `./ARCDESK.toml` > `~/.config/ARCDESK/config.toml` > 内置默认值**。
密钥经环境变量通过 `api_key_env` 注入，绝不写入配置文件。

```toml
default_model = "deepseek-flash"   # 执行器；设 [agent].planner_model 可加规划器
# language    = "zh"               # 界面语言；为空则按 $LANG / $ARCDESK_LANG 自动检测

[agent]
# planner_model = "mimo-pro"          # 可选的低频规划器
# subagent_model = "deepseek-pro"     # runAs=subagent skill 的默认模型
# subagent_models = { review = "deepseek-pro", security_review = "deepseek-pro" }
auto_plan = "off"                  # off|on；off 表示计划模式仅手动开启
# auto_plan_classifier = "deepseek-flash"   # 可选；只在边界任务上调用

[[providers]]
name        = "deepseek-flash"
kind        = "openai"
base_url    = "https://api.deepseek.com"
model       = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
# 还有预设：deepseek-pro、mimo-pro（mimo-v2.5-pro）、mimo-flash（mimo-v2-flash） @ api.xiaomimimo.com/v1

[tools]
enabled = []   # 省略/为空 = 全部内置工具

[skills]
# paths = ["~/my-skills", "../shared/skills"]   # 额外的自定义技能目录
# disabled_skills = ["review"]                  # 隐藏技能，直到 /skill enable <name>

[permissions]
mode  = "ask"                                # 无规则命中时 writer 的兜底：ask|allow|deny
deny  = ["bash(rm -rf*)", "bash(git push*)"] # 任何模式下都硬阻断
allow = ["bash(go test*)"]                   # 从不询问

[sandbox]
# workspace_root = ""          # 文件写工具被限制在此目录；留空 = 当前目录
# allow_write    = ["/tmp"]    # write_file/edit_file/multi_edit 额外可写的目录

[[plugins]]
name    = "example"
command = "ARCDESK-plugin-example"
```

权限逐次调用把关：`deny` > `ask` > `allow` > 兜底（只读工具永远 allow，writer 落到
`mode`）。`ARCDESK chat` 会在 writer 调用前征求同意（`1` 本次 · `2` 本会话 · `3` 拒绝，兼容 `y/a/n`）；
`ARCDESK run` 保持自主运行但仍然遵守 `deny`。完整 schema 与契约见
[`docs/SPEC.md`](docs/SPEC.md)。

权限是**策略**（哪些调用放行/询问），**沙盒**是**强制**：文件写工具
（`write_file` / `edit_file` / `multi_edit`）拒绝 `[sandbox] workspace_root`
之外的任何路径（默认当前目录，编辑不出项目），并解析符号链接与 `..`，使链接无法
打洞越界。读不受限。`bash` 本身在 macOS 默认进沙盒（`[sandbox] bash`，Seatbelt）：
命令只能写这些 root（外加临时目录与工具链缓存），`[sandbox] network` 为真时才能联网；
其它平台暂回退为不沙盒运行（越界问一次与 Linux 支持见 `docs/SPEC.md` §9）。

### 插件（MCP）

ARCDESK 是一个 MCP 客户端。`[[plugins]]` 的 `type` 选择传输：`stdio`（默认）启动本地子进
程（`command`/`args`/`env`）；`http`（Streamable HTTP）连接远程 `url`，可带静态
`headers`（`${VAR}` / `${VAR:-default}` 从环境展开，密钥不入文件）。工具以
`mcp__<server>__<tool>` 暴露给模型，与 Claude Code 一致；声明 MCP `readOnlyHint: true`
的工具会参与并行调度并命中权限层的只读默认放行。

服务器的 **prompts** 会暴露成 `/mcp__<server>__<prompt>` 斜杠命令（命令后空格分隔参
数）；**resources** 通过在消息里写 `@<server>:<uri>` 拉入；`/mcp` 列出已连接服务器及
各自暴露的内容。`make build` 还会产出 `bin/ARCDESK-plugin-example`——一个可直接运行的
stdio 参考实现（`echo`、`wordcount`、一个 `review` prompt、一个 style-guide 资源），
可照抄。

```toml
[[plugins]]                       # 本地 stdio 服务器
name    = "example"
command = "ARCDESK-plugin-example"

[[plugins]]                       # 远程 Streamable HTTP 服务器
name    = "stripe"
type    = "http"
url     = "https://mcp.stripe.com"
headers = { Authorization = "Bearer ${STRIPE_KEY}" }
```

**已有 Claude Code 的 `.mcp.json`？** 直接放到项目根目录，ARCDESK 会原样读取——其
`mcpServers` 规范（`command`/`args`/`env`、`type`/`url`/`headers`、`${VAR}` 展开）
与 `[[plugins]]` 字段一一对应。两处来源会合并加载；同名时以 `ARCDESK.toml` 为准。

```json
{
  "mcpServers": {
    "filesystem": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"] },
    "stripe": { "type": "http", "url": "https://mcp.stripe.com", "headers": { "Authorization": "Bearer ${STRIPE_KEY}" } }
  }
}
```

**从 `0.x` 升级？** 旧的 `~/.ARCDESK/config.json` 仍会被读取(读其 `mcpServers`、并遵从
`mcpDisabled`),作为最低优先级来源——所以 MCP 服务器照常可用;方便时再把它们挪进
`ARCDESK.toml` 的 `[[plugins]]` 或 `.mcp.json`。

### 斜杠命令

`ARCDESK chat` 里,内置命令(`/compact`、`/new`、`/rewind`、`/tree`、`/branch`、`/switch`、`/todo`、`/model`、`/effort`、`/mcp`、`/help`)在本地执行。
`/tree` 查看已保存的对话分支,`/branch [name]` 从当前对话末端分支,`/branch <turn> [name]`
从较早的 checkpoint 轮次分支,`/switch <id|name>` 切换到另一个分支。**自定义命令**
是放在 `.ARCDESK/commands/`(项目)或 `~/.config/ARCDESK/commands/`(用户)下的 Markdown 文件——
`review.md` 即 `/review`,子目录构成命名空间(`git/commit.md` → `/git:commit`)。文件正文
是 prompt 模板,调用即作为一轮对话发出。

```markdown
---
description: Review the staged diff
argument-hint: [focus-area]
---
Review the staged diff. Focus on $ARGUMENTS, list bugs with file:line.
```

`$ARGUMENTS` 展开为全部空格分隔参数,`$1`…`$N` 为位置参数。MCP prompts 也以
`/mcp__<server>__<prompt>` 形式出现在这里。

### @ 引用

在消息里写 `@` 引用,ARCDESK 会在发送前解析成带标签的上下文块:`@path/to/file`(或
`@dir`)注入本地文件内容(或目录清单),`@<server>:<uri>` 注入 MCP 资源。本地路径**只有
真实存在**时才当作引用,普通 `@mention` 保持原文。敲 `/` 或 `@` 会弹出补全菜单——斜杠
命令,或**逐层**的文件导航(一次只列当前一层目录、可下钻进子目录)外加 MCP 资源。

### 双模型协同（可选）

`ARCDESK setup` 刻意保持首次体验极简：选 provider → 输入 key（所选 provider 的所有
SKU 都会启用）。若要让两个模型协同（执行器 + 规划器，各自独立、缓存稳定的
session），向导后手动在 `ARCDESK.toml` 加一行即可：

```toml
[agent]
planner_model = "deepseek-pro"   # 作为低频规划器
```

Subagent skills 默认继承执行器模型。设置 `subagent_model` 可让它们统一走另一个已配置
模型；设置 `subagent_models` 则只覆盖 `review`、`security_review` 等指定 skill。

交互式前端中，计划模式默认手动开启。设置 `agent.auto_plan = "on"` 后，看起来复杂
的任务会自动进入 plan mode：ARCDESK 先只读生成计划，待用户批准后才
编辑文件或执行有副作用的命令。`auto_plan_classifier` 可以指定便宜的 provider，例如
`deepseek-flash`；它只在边界输入上调用，分类失败会回退到启发式规则。也可以用
`ARCDESK chat` 里的 `/auto-plan off|on` 修改用户级设置，或在 shell/脚本里用
`ARCDESK config auto-plan off|on`。只有明确想写项目级覆盖时，才给 shell 命令加
`--local`。

## 架构

三层可扩展性，全部藏在内核按名解析的 registry 之后：

1. **Registry**：`Provider` 与 `Tool` 是接口；内核没有 `switch model`。
2. **编译期内置**：provider（`provider/openai`）和 tool（`tool/builtin`）通过
   `init()` 自注册，`main` 用 blank import 拉入。新增内置 = 一个文件 + 一行 import。
3. **运行时插件**：配置里声明的可执行文件，通过 stdin/stdout 上的
   newline-delimited JSON-RPC 2.0（MCP stdio 约定）通信，每个远程 tool 适配成
   `Tool` 接口。

## 状态

已完成：基于 registry 的 provider/tool、OpenAI 兼容流式 + 工具调用（429/5xx 有界重
试）、九个内置工具（read_file、write_file、edit_file、multi_edit、bash、ls、glob、
grep、web_fetch）、TOML 配置、交互式 `ARCDESK setup` 向导、双模型协同（执行器 + 规划器，
各自独立、缓存稳定的 session）、低频上下文压缩、子 agent（`task`）、bubbletea 聊天
TUI（markdown、plan mode、上下文仪表盘、`/compact` `/new` `/tree` `/branch` `/switch`）、会话持久化 + 恢复、
逐次调用**权限**（allow/ask/deny 规则；chat 在 writer 前询问，deny 在各模式硬阻断）、
**工作区沙盒**（把文件写工具限制在项目内，符号链接/`..` 安全）、
MCP 客户端——**stdio + Streamable HTTP** 传输、工具（`mcp__server__tool`,支持
`readOnlyHint`）、prompts（斜杠命令）、resources（`@` 引用）、`/mcp`，可经
`[[plugins]]` 或 Claude 风格的项目 `.mcp.json` 配置——自定义斜杠命令
（`.ARCDESK/commands/*.md`）、`@file` / `@resource` 引用、外加可运行的参考插件
（`cmd/ARCDESK-plugin-example`）、harness 主循环、CLI。chat 在终端普通缓冲区运行(原生
scrollback)并带 `/` 与 `@` 输入补全。后续:给 `bash` 套 OS 级沙盒（macOS Seatbelt /
Linux bubblewrap，"盒子里放行、边界上询问"）、Anthropic 原生 provider、MCP OAuth +
legacy SSE。见 `docs/SPEC.md` §9。

<br/>

## 渊源 — 与 Reasonix 的关系

ArcDesk 的 **Go agent 内核**参考并延续自
[**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix) ——
面向 DeepSeek 前缀缓存设计的 coding agent 循环（工具、子 agent、skills、hooks、
MCP 客户端、计划模式、CodeGraph 等）。感谢 Reasonix 项目及其贡献者打下的基础。

ArcDesk 是**独立产品方向**（桌面优先、Wails 壳层与下文优化）。
Reasonix 仍以终端为主；我们保留从旧版 `~/.reasonix/` 配置与 skills 的**一次性、
非破坏性**导入路径。

### 在 Reasonix 内核上，ArcDesk 做了哪些优化

| 方向 | ArcDesk 的改动 |
|------|----------------|
| **桌面壳层** | 原生 **Wails** 应用 — studio 工作台（图标栏、项目抽屉、内联 diff、右侧 dock），而非仅 CLI |
| **分发方式** | Windows **NSIS 安装向导**（可选目录、开始菜单/桌面快捷方式、按用户安装、无需管理员）；带签名校验的自动更新 |
| **安全加固** | 桌面 **Phase 9** — 凭证 / 局域网 / 隧道 / 高风险 shell 前 **系统原生确认**；MCP **按项目信任前隔离**；手机配对 **限流**；本地密钥文件权限收紧 |
| **稳定性** | OpenAI 兼容 **SSE 截断检测**（缺少 `[DONE]` 时重连）；agent **步数上限**避免仅工具回复时死循环；桌面 **单实例**锁 |
| **迁移与配置** | `arcdesk.toml` / `.arcdesk/` 品牌命名；从 Reasonix `~/.reasonix/config.json` 及 v1 TOML **无损导入** |
| **体验默认值** | 更合理的**默认窗口尺寸**，保证左侧项目栏完整展开；中英文界面 |
| **工程化** | CI + Go 工具链检查；含桌面安全回归在内的广泛测试 |

详见 [`SECURITY.md`](./SECURITY.md) · [`desktop/README.md`](./desktop/README.md) · [`docs/MIGRATING.md`](./docs/MIGRATING.md)。

<br/>

## 致谢

下面这些朋友的工作塑造了 ARCDESK 今天的样子 —— 综合 commit 数和代码量两个维度。
**按字母顺序排列，排名不分先后。** 完整贡献者列表在
[GitHub](https://github.com/esengine/DeepSeek-ARCDESK/graphs/contributors)。

- [**ctharvey**](https://github.com/ctharvey)
- [**dimasd-angga**](https://github.com/dimasd-angga)（Dimas D. Angga）
- [**Evan-Pycraft**](https://github.com/Evan-Pycraft)
- [**ForeverYoungPp**](https://github.com/ForeverYoungPp)
- [**GTC2080**](https://github.com/GTC2080)（TaoMu）
- [**kabaka9527**](https://github.com/kabaka9527)
- [**lisniuse**](https://github.com/lisniuse)（Richie）
- [**wade19990814-hue**](https://github.com/wade19990814-hue)
- [**wviana**](https://github.com/wviana)（Wesley Viana）

另外特别感谢 [**Bernardxu123**](https://github.com/Bernardxu123) 设计的项目 logo，
以及 [AIGC Link](https://xhslink.com/m/80ngts127cA) 在小红书上的推广。

**上游项目：** [**Reasonix**](https://github.com/esengine/DeepSeek-Reasonix) ——
ArcDesk Go 内核所参考的基础（见 [渊源 — 与 Reasonix 的关系](#渊源--与-reasonix-的关系)）。

<p align="center">
  <a href="https://github.com/esengine/DeepSeek-ARCDESK/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=esengine/DeepSeek-ARCDESK&max=100&columns=12" alt="esengine/DeepSeek-ARCDESK 贡献者" width="860"/>
  </a>
</p>

<br/>

---

<p align="center">
  <sub>MIT —— 见 <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub>由 <a href="https://github.com/esengine/DeepSeek-ARCDESK/graphs/contributors">esengine/DeepSeek-ARCDESK</a> 社区共建</sub>
</p>
