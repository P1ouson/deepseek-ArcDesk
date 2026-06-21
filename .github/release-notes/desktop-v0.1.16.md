## ArcDesk Desktop desktop-v0.1.16

底部模型切换与 DeepSeek effort 修复（2026-06-21）。

> 本版本 **仅发布 Windows (amd64) 安装包**。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

---

### 修复

- **模型切换** — 修复切换后闪回旧模型、点击报 `no active workspace tab`、以及 `[unhandledrejection]` 的问题
- **模型列表** — 后端按 SKU 合并重复项并正确标记 `current`，切换后 UI 与 tab 状态一致
- **Effort 按钮** — DeepSeek 中转/自定义 URL 模型也显示 auto/high/max 档位
- **构建脚本** — `build-dev.ps1` 在前端 tsc 失败时立即中止，避免嵌入旧 frontend/dist

---

包含 v0.1.15 的全部改进（设置页保存含 `/` 的模型 ID、切回官方 API 回退等）。
