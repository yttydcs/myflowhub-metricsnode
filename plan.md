# Plan - MyFlowHub-MetricsNode（UI：Windows 连接页/设置页小幅打磨）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-ui-polish`  
> 分支：`fix/metricsnode-ui-polish`  
> 日期：2026-03-03  
>
> 本 workflow 目标：按你的反馈对 MetricsNode 的 UI 做一次“可读性/一致性”打磨，主要针对 Windows（Wails），Android 如存在同类问题则同步修正。

---

## 0. 当前状态

- Windows（Wails）：已有 Connect/Settings 两页；Settings 页用表格展示 enabled/writable/var_name/value；enabled/writable 目前是原生 checkbox；Connect 页布局较密集。
- Android（Compose）：已有 Connect/Settings 两页；Settings 已使用 Switch。

---

## 1. 需求分析（待确认）

### 1.1 目标

#### Windows（必须）
1) Settings 页：`Reload` / `Ready` 与 `Metrics Settings` 标题在同一行视觉对齐（baseline/垂直居中）。
2) Settings 页：表格列顺序调整为 `Metric | Var Name | Value | Enabled | Writable`（最后两列为开关）。
3) Settings 页：`Enabled` / `Writable` 使用“开关样式”组件（不是裸 checkbox）；saving 时禁用交互。
4) Connect 页：布局回到“分块卡片”风格（类似旧版 `Bootstrap/Auth/Metrics` 三块），但去掉底部 metrics dump/调试信息（因为 Settings 已提供实时值）。

#### Android（可选：仅当存在同类问题）
- 若发现 Settings 仍有 checkbox 或列布局与 Windows 一致性明显较差，则做同等层级的 UI 打磨；否则不改。

### 1.2 不做
- 不改协议/后端逻辑。
- 不引入新的前端依赖（用现有 CSS 实现开关样式）。

### 1.3 验收标准
- Windows Settings 页：标题与右侧操作区对齐；enabled/writable 为开关样式；列顺序符合预期；功能不回归（保存/禁用/校验仍正常）。
- Windows Connect 页：信息分组清晰；仍可完成 Connect/Register/Login/StartReporting；不再显示 metrics dump。

---

## 2. 架构设计（分析）

- 仅改 UI：集中修改 `windows/frontend/src/App.vue` 的 template + scoped CSS。
- 开关组件实现：使用 `label + hidden checkbox + span(track)` 的纯 CSS 方案，保证可访问性与可点击区域。
- 表格列重排：调整 `.thead/.tr` 的 DOM 顺序与 `grid-template-columns`。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认
- 目标：分支/工作区正确且干净。
- 验收：`git status --porcelain` 为空。
- 回滚点：无（仅检查）。

### T1 - Windows：Settings 页标题行对齐 + 列重排
- 目标：标题行对齐；列顺序改为 `Metric | Var Name | Value | Enabled | Writable`。
- 涉及文件：
  - `windows/frontend/src/App.vue`
- 验收：手工打开页面观察对齐与列顺序正确。
- 回滚点：revert 本次提交。

### T2 - Windows：Enabled/Writable 改为开关样式
- 目标：用开关组件替代 checkbox（保留原逻辑：saving 时禁用）。
- 涉及文件：
  - `windows/frontend/src/App.vue`
- 验收：手工点击可切换；禁用态不可切换；保存逻辑不变。
- 回滚点：revert 本次提交。

### T3 - Windows：Connect 页恢复分块卡片布局 + 移除 metrics dump
- 目标：Connect 页按 Bootstrap/Auth/Reporting 分块；不展示 metrics dump/调试信息。
- 涉及文件：
  - `windows/frontend/src/App.vue`
- 验收：手工确认 UI 分块清晰；四步操作可用。
- 回滚点：revert 本次提交。

### T4 - 验证
- Windows 前端：`cd windows/frontend; npm run build`
- （可选）Go：`GOWORK=off go test ./... -count=1 -p 1`

### T5 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 归档输出：`docs/change/2026-03-03_metricsnode-ui-polish.md`
