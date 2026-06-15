## ArcDesk Desktop desktop-v0.1.8

桌面版 **统一预览栏、右侧 40% 布局与文件树预览体验**（2026-06-15）。

> 本版本 **仅发布 Windows (amd64) 安装包**；macOS / Linux 构建暂缓，后续版本恢复。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

> Windows 仅分发 **NSIS 安装包**（内含应用 exe 与 WebView2 引导）；无需单独下载裸 exe。

---

### 新增

- **统一预览栏** — 文件 / 网页 / 终端 / 浏览器合并为单一预览列；顶部仅保留「+」与「×」，可追加终端或浏览器
- **文件树预览分栏** — 从文件树打开文件时，右侧 40% 预算内按 **1.5 : 2.5** 分配工作 Dock 与预览（预览更宽）
- **预览展开 / 收起 / 返回** — 展开时预览占满整个右侧栏；返回或收起恢复文件树 + 预览分栏

### 优化

- **右侧栏宽度** — 修复从文件树打开预览后右侧区域缩窄的问题；打开文件时立即按预算重算宽度，跳过 Dock 开启动画闪动
- **Studio 布局** — 右侧预览与工作 Dock 宽度计算统一至 `workbenchPanelLayout`；单栏默认占满 40% 预算
- **Harness 分层与待办进度** — Agent 控制层 harness 分层、回合管理与待办进度同步（后端）

### 修复

- **嵌入预览裁切** — 修复 Markdown 预览横向被裁切、仅显示一半的问题
- **预览列 DOM 顺序** — 文件树模式下 Dock 在左、预览在右，符合操作习惯

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
