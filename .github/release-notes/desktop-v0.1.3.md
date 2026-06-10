## ArcDesk Desktop desktop-v0.1.3

桌面版 UI/交互、远程连接、预览与工作区会话体验的集中更新。README 已重写（含截图与运行实录）。Windows 安装包见下方下载；macOS / Linux 由 GitHub Actions 构建（推送后可在 Actions 中重跑 `release-desktop` 或等待同步）。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |
| **macOS (Universal)** | `arcdesk-desktop-darwin-universal.dmg` |
| **Linux (amd64)** | `arcdesk-desktop-linux-amd64-installer.tar.gz` |

---

### 预览 Hub（Preview / Browser / Terminal）

- 侧栏第三个 Hub **预览** 现含三项：**Preview**（静态 HTML/SVG 页面）、**Browser**（dev server，原「Web 预览」改名）、**Terminal**
- **Preview**：工作区 `html/htm/svg` 经本地 loopback 服务渲染，支持相对路径资源；文件树双击或右键「打开预览」
- **Browser**：iframe 直连 dev server；Go 端端口探测与可达性检查；离线每 5 秒重试；工作区文件变更后软刷新（HMR 兜底）
- Agent 新增 **`open_web_preview`** 工具，可主动打开 Browser 侧栏
- 输入框上方标签：Browser 显示 URL，Preview 显示当前文件路径

### 对话与时间线

- Cursor 风格时间线：用户文字 → 工具 → 再文字，交错展示，工具段可折叠
- 思考过程：思考中自动展开，结束后自动折叠
- **复制按钮** 每轮回答仅在**最底部一条**显示，复制整轮正文（不含多次 Thinking 分段）
- dev server 常驻 bash 时发送按钮不再卡在「停止」，输入框可继续发消息

### 会话恢复

- 关闭应用再打开时**恢复上次会话**（`desktop-tabs.json` 持久化 `sessionPath`），不再每次冷启动新建空会话
- 快照与关闭时同步保存会话路径

### 对话与审批 UI（v0.1.3 延续）

- 修复 AI 询问/审批框遮挡输入框；审批/询问按页面隔离
- 顶栏会话绿色呼吸灯；消息操作移至 AI 回复右下角
- YOLO / Plan / 普通模式随时可切换；侧栏新建会话、项目搜索弹窗、回收站二次确认等

### 远程连接与手机配对

- 局域网默认关闭；Cloudflare 穿透进度与 10 分钟空闲停穿透；URL 脱敏
- 已配对设备列表与**当前在线连接数**修正

### 设置、扩展与其它

- 设置页工作区数据（历史/记忆/回收站）独立居中弹窗
- 扩展页技能卡片与 `MotionUnfold` 展开动画
- 原生标题栏、默认窗口尺寸、关闭即退出
- 更新源指向 `P1ouson/deepseek-ArcDesk` releases

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
- **macOS** — 右键 → 打开，或 `xattr -dr com.apple.quarantine /Applications/ArcDesk.app`
- **Linux** — 解压后运行 `./install.sh`
