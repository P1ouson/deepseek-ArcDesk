## ArcDesk Desktop desktop-v0.1.9

桌面版 **写作区、统一右侧边栏与 docx 预览**（2026-06-15）。

> 本版本 **仅发布 Windows (amd64) 安装包**；macOS / Linux 构建暂缓，后续版本恢复。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

> Windows 仅分发 **NSIS 安装包**（内含应用 exe 与 WebView2 引导）；无需单独下载裸 exe。

---

### 新增

- **写作区** — 文稿列表、编辑/预览切换、写作助手快捷操作；打开 docx 默认预览模式
- **统一右侧边栏** — 代码区外露变更 / 文件 / Git / 概览 / 待办；「+」仅新建终端与浏览器
- **写作区右侧边栏** — 精简为概览、待办与浏览器查资料；不再照搬代码区全套面板
- **docx 预览** — 表格、图片、中英混排（英文 Times New Roman）；跳过 Word 域代码（目录 / 超链接等）
- **写作 Agent 上下文** — 发送时注入当前文稿与代码区项目路径；写作模式不自动进入 Plan

### 优化

- **写作区布局** — 右侧边栏占宽 30%；工具栏收起按钮控制工作台边栏（非助手面板）
- **文稿侧栏** — 按扩展名显示图标；隐藏 Word 临时锁文件（`~$` 开头）
- **Ask 询问框** — 问题正文与选项纵向排版，修复单行截断
- **Agent 工作区** — 写作模式仅绑定代码区项目，避免在文稿保存目录生成 `.arcdesk` / `.codegraph`

### 修复

- **docx 读取** — `read_file` 可读 `.docx`；无效 zip 时降级纯文本，预览失败不阻断文稿加载
- **写作 tab** — 不再将桌面/文稿目录当作 project workspace 开 tab

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
