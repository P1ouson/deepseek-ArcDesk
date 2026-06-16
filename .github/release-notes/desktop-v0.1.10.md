## ArcDesk Desktop desktop-v0.1.10

桌面版 **Agent 内核 P0–P8、推理过程 UI 与稳定性修复**（2026-06-16）。

> 本版本 **仅发布 Windows (amd64) 安装包**；macOS / Linux 构建暂缓，后续版本恢复。

### 下载

| 平台 | 文件 |
|------|------|
| **Windows (amd64)** | `arcdesk-desktop-windows-amd64-installer.exe` |

> Windows 仅分发 **NSIS 安装包**（内含应用 exe 与 WebView2 引导）；无需单独下载裸 exe。

---

### 内核（P0–P8）

- **P0–P2 Session Tool Cache** — 重复读/搜场景显著减少工具执行；fixture 验收 +50pp
- **P3 Knowledge / P5 Plan Cache / P6 Failure Memory** — 知识注入与 plan 复用；语义检索 fallback
- **P4 Workspace Refresh** — docs-only 变更不全量重扫工作区
- **P7 Prefix Runtime** — KV prefix 稳定性、cache health、verify 裁剪
- **P8 Hot Path** — agent 热路径 IO 与刷新优化

### 新增

- **推理过程 UI** — 动态阶段标题（探索代码库 / 规划下一步等）、工具动词、完成统计与 Auto 徽章
- **写作区** — 发送时注入代码区项目上下文；`writeSkill` 辅助与单测

### 优化

- **技能设置** — 未信任项目仍显示 repo 技能（标记为禁用 + quarantine 警告）
- **代码清理** — 移除未引用 dock / preview / coach 组件；`.gitignore` 忽略 benchmark 产物

### 修复

- **删除工作区** — 同步关闭 tab、会话移入回收站、清空话题元数据
- **主题 / 强调色** — 前后端预设对齐（studio / parchment / nightfall）；legacy 暖色迁移至 indigo，保存不再报错
- **Agent compaction** — 模型直接给出最终答案时也会触发压缩，避免大工具输出撑爆窗口
- **E2E 测试** — callgraph / dependency verify 重试与 compaction 循环测试恢复稳定

---

安装后填入 [DeepSeek API Key](https://platform.deepseek.com/)，导入工作区即可使用。

### 首次启动（未签名构建）

- **Windows** — SmartScreen → 更多信息 → 仍要运行；必要时安装 [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/)
