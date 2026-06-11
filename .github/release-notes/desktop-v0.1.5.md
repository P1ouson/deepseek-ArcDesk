## ArcDesk Desktop desktop-v0.1.5

桌面版 **多标签工作区、时间线与会话概览** 集中更新（2026-06-10）。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |
| **macOS (Universal)** | `arcdesk-desktop-darwin-universal.dmg` |
| **Linux (amd64)** | `arcdesk-desktop-linux-amd64-installer.tar.gz` |

---

### 多标签工作区（核心）

- **顶部标签栏**：从左侧项目树点击话题/工作区后打开标签；统一宽度；关闭后重启可恢复（自动过滤已删除话题的脏数据）
- **状态呼吸灯**：每个标签右侧显示绿色（空闲/就绪）或黄色（Agent 运行中）
- **不再自动开标签**：新建话题仅创建记录，需点击侧栏才打开；允许关闭最后一个标签，零标签时进入就绪空态
- **侧栏去掉重复圆点**：运行状态以标签呼吸灯为准

### 时间线与后台任务

- **打开/切换/恢复会话**时默认滚到底部，不再停在顶部
- **后台子任务完成**（`task-1` / `task-2` 等）在任务完成时立即显示，不再整轮结束后才批量弹出
- **工具输出截断**（`tool output truncated … paging with offset/limit`）为正常提示：单次工具结果超过约 32KB 时分段读取，不是报错

### 右侧概览 · Token 用量

- 新增 **Token 用量**卡片：本次请求的输入/输出/推理/合计
- **Prompt 缓存**：命中与未命中 token 数、本次命中率、会话平均命中率及进度条

### Windows 与构建

- 启动时隐藏 `git` / `netsh` 子进程窗口，减少黑框闪烁
- 新增 `desktop/build-dev.ps1`：本地快速构建 exe（跳过 NSIS）；安装包脚本修复 NSIS 中文编码问题

### 其他打磨

- 工作台侧栏/预览拖拽布局与最小宽度约束
- 底部状态栏缓存命中率（本次 / 平均）与 Composer 用量展示
- OpenRouter / 自定义 Provider 连接与模型列表 UX 继续修正
- `read_file` 自适应读取与 repomap 读取遥测（内核侧）

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
- **macOS** — 右键 → 打开，或 `xattr -dr com.apple.quarantine /Applications/ArcDesk.app`
- **Linux** — 解压后运行 `./install.sh`
