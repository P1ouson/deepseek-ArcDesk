## ArcDesk Desktop desktop-v0.1.12

桌面版 **代码块 UI、继续生成、Git 初始化** 等体验更新（2026-06-16）。

> 本版本 **仅发布 Windows (amd64) 安装包**。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

---

### 新功能

- **DeepSeek 风格代码块** — 语言标签 + 复制 / 下载 / 运行；顶栏固定，内容区独立滚动
- **继续生成** — 回复因 token 上限截断时，消息右下角可一键续写；输入框保持发送状态，可随时发新消息
- **创建 Git 仓库** — 非 Git 项目可一键 `git init`，自动忽略 `.arcdesk/` 运行时目录

### 修复与改进

-  detached shell 下 Git 初始化与 `gh` 探测更稳定
- 删除工作区后 Git / 变更面板状态同步
- 流式输出时 Markdown 代码块实时渲染

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。
