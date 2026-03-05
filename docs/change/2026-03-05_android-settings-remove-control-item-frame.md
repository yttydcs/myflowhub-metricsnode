# 2026-03-05 Android Settings 移除 Control 标签与项目外框

## 变更背景 / 目标

继续精简 Android `Settings` 页面视觉噪声：

- 移除 `Metric` 列中的 `Control` 标签
- 移除每个设置项最外层圆角矩形边框

目标是在不改变设置语义和保存链路的前提下，让列表更干净、信息更聚焦。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`

- `SettingsRow`（宽屏）
  - 删除 metric 列中的 `Control` 标签渲染分支
  - 删除每项外层 `Surface + BorderStroke + RoundedCornerShape(10.dp)` 包裹，改为直接 `Row`

- `SettingsCompactRow`（窄屏）
  - 删除 metric 区中的 `Control` 标签渲染分支
  - 删除每项外层 `Surface + BorderStroke + RoundedCornerShape(12.dp)` 包裹，改为直接 `Column`

- 其余行为保持不变
  - `Writeable/Enabled` 开关逻辑不变
  - `Read-only` 展示逻辑保留
  - `varName` 校验、保存链路不变

### 新增

- `docs/change/2026-03-05_android-settings-remove-control-item-frame.md`（本文件）

### 删除

- 无文件删除

## 对应 plan/todo 任务映射

- D1 需求与方案确认 -> 已完成
- D2 UI 调整实现 -> 已完成
- D3 构建验证 -> 已完成
- D4 Code Review -> 已完成
- D5 归档变更 -> 已完成

## 关键设计决策与权衡（性能 / 扩展性）

1. 决策：仅移除标签与项目外框，不改内层功能控件
- 原因：满足需求同时把行为回归风险降到最低。

2. 决策：保留 `Read-only` 视觉提示
- 原因：虽然去掉 `Control`，但不可写状态仍需明确提示。

3. 性能说明
- 本次仅 UI 结构精简，无新增 I/O 或额外计算。

## 测试与验证方式 / 结果

### 构建验证

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
$env:ANDROID_SDK_ROOT='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug
```

结果：`BUILD SUCCESSFUL`

### 关键路径检查

- 宽屏/窄屏均不再出现 `Control` 标签
- 宽屏/窄屏每项外层圆角外框已移除
- 设置编辑与保存链路保持正常

## 潜在影响与回滚方案

### 潜在影响

- 视觉层级会比之前更扁平，依赖分隔线与留白维持可读性。

### 回滚方案

- 回滚 `MainActivity.kt` 本次提交即可恢复 `Control` 标签与外框。
