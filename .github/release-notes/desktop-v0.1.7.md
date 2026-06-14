## ArcDesk Desktop desktop-v0.1.7

桌面版 **P2 能力栈、Slash 命令修复、Knowledge Studio、MCP/Skills 市场与 Windows 专项发布**（2026-06-14）。

> 本版本 **仅发布 Windows (amd64) 安装包**；macOS / Linux 构建暂缓，后续版本恢复。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

> Windows 仅分发 **NSIS 安装包**（内含应用 exe 与 WebView2 引导）；无需单独下载裸 exe。

---

### 新增

- **Knowledge Studio** — 项目知识捕获、启发式判断、索引与桌面侧栏「知识」面板；会话结束后可 Y/N 记录踩坑经验
- **MCP 安装向导** — 检测 Node/npx 前置依赖，桌面内一键安装 MCP 服务器
- **Skills 市场弹窗** — 浏览与安装 bundled / 远程 Skills
- **P2 Agent 能力栈** — Runtime/UI RAG、Guardian、Task DAG、Cost Router、Context 压缩、ArchRAG、GitRAG、RepoRAG、Rollback、SelfDebug、Verification 等模块与 boot 接线
- **Slash 子命令完整实现** — `/mcp list|show|tools|add|connect|remove`、`/hooks trust`、`/auto-plan`、`/language`、`/skills show|paths|new`
- **桌面命令参数提示** — `/model`、`/effort`、`/theme` 等补全与动态 hint；无 controller 时也可完成 model/effort 补全

### 优化

- **跨 Tab 共享工作区运行时** — 配置缓存、MCP Host、索引复用；首条消息前懒启动 Agent，加快 Tab/会话切换
- **会话 UX** — 修复聊天时间线、工作区打开、新建会话 Tab 流程；减少首条命令后 CMD 闪烁
- **默认窗口** — 高度 850px；侧边栏 rail 紧凑化，导航项无需滚动即可见「设置」
- **验证策略** — 项目检查默认 advisory；`enforce_final_answer` 开启后才强制 write-backed 完成前跑校验
- **Failure memory** — 条目元数据与检索增强，供后续回合注入

### 修复

- **Slash 路由** — 修复 `/mcp` 子命令全部回落到 server list 的问题
- **桌面发送路由** — `/theme graphite` 等样式名正确路由到设置层
- **MCP 解析** — CLI 与桌面共用 `internal/mcpcmd` 参数解析

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
