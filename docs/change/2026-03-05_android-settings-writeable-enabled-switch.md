# 2026-03-05 Android Settings 顺序与开关控件调整

## 变更背景 / 目标

用户要求调整 Android Settings 页面中的两个布尔配置项展示顺序，并将 checkbox 风格控件改为开关：

- 列顺序从 `Enabled + Writeable` 改为 `Writeable + Enabled`
- 控件从 checkbox 视觉/语义改为 Switch

本次仅做 UI 层改动，不改变 `MetricSetting` 字段、保存链路和服务侧逻辑。

## 具体变更内容

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- `SettingsHeaderRow`：
  - 表头文案顺序调整为 `Writeable` 在前，`Enabled` 在后。
  - 对应列宽从 `58.dp` 调整为 `72.dp`，避免 Switch 显示拥挤。
- `SettingsRow`（宽屏表格行）：
  - 顺序改为先渲染 `Writeable`（受 `controllable` 控制），再渲染 `Enabled`。
  - 两列交互控件改为 `CompactSwitch`（内部为 Material3 `Switch`）。
- `SettingsCompactRow`（窄屏卡片行）：
  - 顺序改为 `Writeable` 在前、`Enabled` 在后。
  - `Writeable` 在不可控 metric 时显示 `Writeable: -` 占位文本。
- `SettingToggle`：
  - 使用 `CompactSwitch` 代替原 `CompactCheck`。
- `CompactCheck` -> `CompactSwitch`：
  - 删除自绘 checkbox（`toggleable + 勾号`）实现。
  - 改为直接封装 Material3 `Switch`。
- imports：
  - 新增 `androidx.compose.material3.Switch`
  - 删除 checkbox 自绘相关无用 import（`toggleable`、`Role`、`background`）。

2. `todo.md`
- 新建并维护本次 workflow 的可交接任务拆分与执行状态。

### 新增 / 删除

- 新增：`docs/change/2026-03-05_android-settings-writeable-enabled-switch.md`（本文件）
- 删除：无

## 对应 plan 任务映射

- T1 需求与架构确认 -> 已完成
- T2 UI 顺序与控件改造 -> 已完成
- T3 构建验证 -> 已完成
- T4 Code Review（3.3） -> 已完成
- T5 归档变更（4） -> 已完成

## 关键设计决策与权衡

1. 使用 Material3 `Switch` 代替自绘 checkbox
- 原因：平台一致性更好，组件可维护性更高，减少自定义状态绘制代码。
- 权衡：视觉会更接近系统默认样式，失去原自定义 checkbox 的紧凑外观。

2. 保留内部字段 `writable`，仅调整展示文案为 `Writeable`
- 原因：避免协议/存储字段变更带来的兼容风险。
- 权衡：UI 文案与内部字段拼写不同，但业务语义保持稳定。

3. 宽屏列宽增至 `72.dp`
- 原因：Switch 组件较 checkbox 更宽，防止布局拥挤。
- 性能影响：仅布局参数变化，不引入额外 I/O、循环或计算复杂度。

## 测试与验证方式 / 结果

### 执行命令

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
$env:ANDROID_SDK_ROOT='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug
```

### 结果

- `BUILD SUCCESSFUL`（`android/app:assembleDebug`）
- 说明：首次构建因本地未配置 SDK 路径失败（环境问题），注入 SDK 环境变量后构建通过。

## 潜在影响与回滚方案

### 潜在影响

- UI 文案由 `Writable` 变为 `Writeable`，如果有截图比对或 UI 自动化文本断言，需同步更新。
- 开关交互视觉变化可能影响既有设计预期，但不影响 settings 保存语义。

### 回滚方案

- 仅回滚 `MainActivity.kt` 本次提交即可恢复原顺序与 checkbox 控件。
- 若仅需恢复文案，可单独回滚 `Writeable` 相关文本改动。
