# Wails 真机冒烟清单

在 `pnpm release:check` 通过（含 `RELEASE_WAILS=1` 可选编译）后，请在 **Wails 二进制** 上手工确认：

## 启动与导航
- [ ] 冷启动无白屏 / 无 embed 404
- [ ] 侧边栏切换 code / write / schedule / plugins 模式
- [ ] 项目抽屉展开/收起状态重启后保持

## 核心会话
- [ ] 新建会话并发送消息，流式输出正常
- [ ] 切换 Tab 后回到原 Tab，历史消息保留
- [ ] Plan / YOLO 模式切换生效

## 设置同步
- [ ] 修改 Git 合并方式后，Git 面板无需重开即刷新（CustomEvent）
- [ ] 修改 Code Review 默认范围后，Changes 面板同步

## 集成
- [ ] 文件预览、终端、Right Dock 打开/关闭
- [ ] 托盘 locale 与 UI 语言一致

## 发布
- [ ] `RELEASE_WAILS=1 pnpm release:check` 编译通过
- [ ] 安装包在本机 OS 上可启动

自动化：`release-check.mjs` 在 `RELEASE_WAILS=1` 时执行 `wails build -s`；本清单覆盖 WebView 行为，mock 预览无法替代。
