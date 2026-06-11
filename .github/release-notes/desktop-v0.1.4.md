## ArcDesk Desktop desktop-v0.1.4

桌面版 **沙盒 Browser / 预览侧栏**、远程连接与设置体验的集中更新（2026-06-10）。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |
| **macOS (Universal)** | `arcdesk-desktop-darwin-universal.dmg` |
| **Linux (amd64)** | `arcdesk-desktop-linux-amd64-installer.tar.gz` |

---

### 沙盒 Browser 与预览侧栏

- **多标签 Browser**：每个 URL 独立标签；关闭标签即结束会话；收起侧栏时后台保持运行
- **地址栏可编辑**：修复输入时被旧 URL 覆盖的问题；Enter / → 导航
- **适应面板缩放**（类似 Cursor）：默认整页缩放进侧栏，无需来回拖动；工具栏提供适应 / 放大 / 缩小与比例显示
- **侧栏尺寸**：Browser / 页面预览默认更宽（约 36% 主区域）；**展开预览**占 **整个 workbench 宽度 50%**（全屏下约为半窗）
- **布局修复**：`dock-slot` 高度链与 iframe 缩放层，预览区铺满侧栏
- **输入框会话标签**：每个终端 / Browser 标签独立 chip，自动换行；悬停显示 ×；点击恢复对应面板
- **预览 Hub**：菜单项不再显示 ✓、选中后按钮不变蓝

### 终端

- 最小化（−）与关闭（×）分离；最小化后会话继续，输入框上方保留标签
- 终端标签栏支持换行；关闭按钮悬停显示

### 远程连接与安全

- **Windows 局域网配对**：修复原生确认框返回值（`Yes`/`No`）导致「开启局域网配对失败」
- 监听策略改为始终绑定 `0.0.0.0` + 中间件控制，切换局域网无需重启服务
- 手机端**在线设备数**实时刷新；断开隧道后清零
- 配对令牌一次性、会话撤销、relay 日志脱敏与相关测试补强

### 设置与模型

- OpenRouter / 自定义 Provider 连接与模型列表 UX 修复
- 模型下拉滚动、代码/写作 Tab 分离、消息顺序等交互修正

### 对话与时间线（延续）

- Cursor 风格交错时间线、思考块自动折叠、复制整轮回答等（v0.1.3 基础上继续打磨）
- 关闭应用后恢复上次会话路径

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
- **macOS** — 右键 → 打开，或 `xattr -dr com.apple.quarantine /Applications/ArcDesk.app`
- **Linux** — 解压后运行 `./install.sh`
