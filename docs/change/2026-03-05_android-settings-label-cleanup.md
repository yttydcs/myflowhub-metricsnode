# 2026-03-05 Android Settings 文案清理

## 变更背景 / 目标

根据反馈继续优化 Android `Settings` 页面：

- 移除 `Metric` 列中无价值的 `Report only` 文案
- 移除每行重复的 `Var Name/var_name` 文案

目标是在不改变配置语义与保存链路的前提下，降低视觉噪声，提升信息扫描效率。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`

- `validate()` 错误文案：
  - `invalid var_name for ...` -> `invalid mapping for ...`
  - `duplicate enabled var_name: ...` -> `duplicate enabled mapping: ...`

- `SettingsHeaderRow`：
  - 列名 `Var Name` -> `Mapping`

- `SettingsRow`（宽屏）：
  - Metric 区域标签由 `Control + Report / Report only` 改为仅在可控项显示 `Control`
  - 移除每行 `Var Name` 标签，仅保留输入框

- `SettingsCompactRow`（窄屏）：
  - Metric 区域标签同样仅在可控项显示 `Control`
  - 移除每行 `Var Name` 标签，仅保留输入框

- `CompactVarNameField`：
  - 移除占位文案 `var_name`

### 新增

- `docs/change/2026-03-05_android-settings-label-cleanup.md`（本文件）

### 删除

- 无文件删除

## 对应 plan/todo 任务映射

- C1 需求与方案确认 -> 已完成
- C2 文案清理实现 -> 已完成
- C3 构建验证 -> 已完成
- C4 Code Review -> 已完成
- C5 归档变更 -> 已完成

## 关键设计决策与权衡（性能 / 扩展性）

1. 决策：仅做文案与显示层清理，不改状态逻辑
- 原因：避免行为回归，保持可回滚性。

2. 决策：保留 `Control` 标签，移除 `Report only`
- 原因：`Control` 仍有信息价值；`Report only` 噪声更高。

3. 性能说明
- 仅文案/UI 显示改动，无新增 I/O、循环复杂度和线程开销。

## 测试与验证方式 / 结果

### 构建验证

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
$env:ANDROID_SDK_ROOT='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug
```

结果：`BUILD SUCCESSFUL`

### 关键路径检查

- `load / validate / scheduleSave / saveNow` 链路保持不变
- 宽屏与窄屏均不再出现 `Report only`
- 每行不再出现 `Var Name/var_name`

## 潜在影响与回滚方案

### 潜在影响

- 错误文案由 `var_name` 改为 `mapping`，若有文本断言需同步更新。

### 回滚方案

- 回滚 `MainActivity.kt` 本次提交即可恢复原文案。
