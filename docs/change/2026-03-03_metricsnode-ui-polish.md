# 2026-03-03 MetricsNode：Windows UI 打磨（Connect/Settings）

## 背景 / 目标

在 MetricsNode（Windows/Wails）已完成“Connect/Settings 两页”之后，根据使用反馈对 UI 做一轮小幅打磨，提升可读性与一致性：

- Settings 页：标题区对齐、表格列顺序更合理、Enabled/Writable 用开关样式替代裸 checkbox。
- Connect 页：恢复更清晰的分块卡片布局，但去掉旧版底部 metrics dump（调试信息已在 Settings 页实时显示）。

> 备注：本轮主要针对 Windows；Android 端已使用 Switch，未发现同类问题，因此未改动。

## 具体变更内容

### Windows（Wails 前端）

- Settings 页：
  - 标题行改为 `card-header`（`Metrics Settings` 与 `Reload/Ready` 垂直居中对齐）。
  - 表格列顺序调整为：`Metric | Var Name | Value | Enabled | Writable`（Enabled/Writable 置于最右侧）。
  - Enabled/Writable 改为纯 CSS 开关组件（label + hidden checkbox + track），并沿用 saving 时禁用交互逻辑。
- Connect 页：
  - 从“单卡片堆叠”调整为 `Bootstrap / Auth / Reporting` 三段卡片分组，减少信息密度与按钮混杂。
  - 不再显示 metrics dump/调试信息（仅保留连接/鉴权状态与操作按钮）。

涉及文件：
- `windows/frontend/src/App.vue`

## Plan 任务映射

对应 `plan.md`：
- T1：Windows Settings 标题行对齐 + 列重排
- T2：Windows Enabled/Writable 开关样式
- T3：Windows Connect 分块卡片 + 移除 metrics dump
- T4：验证（前端 build）

## 关键设计决策与权衡

- 未引入任何 UI 依赖：开关组件使用 scoped CSS 实现，避免为小改动引入额外依赖与打包/升级负担。
- 保持行为不变：仅替换展示与布局，不修改保存/校验/禁用等业务逻辑，降低回归风险。

## 测试与验证

- Windows 前端构建：`cd windows/frontend; npm ci; npm run build`（通过）

## 潜在影响与回滚方案

- 影响：
  - Connect 页的控件分布位置变化（但字段/按钮功能不变）。
  - Settings 页列顺序变化，Enabled/Writable 的交互样式变化（但事件与保存逻辑不变）。
- 回滚：
  - 直接回滚本分支对应提交即可恢复到改动前 UI。

