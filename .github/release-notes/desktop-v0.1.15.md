## ArcDesk Desktop desktop-v0.1.15

设置页保存 API/URL 热修复（2026-06-18）。

> 本版本 **仅发布 Windows (amd64) 安装包**。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

---

### 修复

- **设置页保存** — 修复中转站模型 ID 含 `/`（如 `z-ai/glm-5.2-free`）时保存 URL/API 触发 `unknown model` 并 rebuild 失败的问题
- **设置页报错** — 保存失败时在设置页显示错误横幅，不再弹出 `[unhandledrejection]`

---

包含 v0.1.14 的全部改进（流式渲染、时间线顺序、友好错误提示等）。
