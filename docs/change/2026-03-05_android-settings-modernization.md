# 2026-03-05 Android Settings 现代化与风格统一

## 变更背景 / 目标

在保持 Android `Connect` 页面整体风格一致的前提下，重构 Android `Settings` 页视觉层次与交互布局，使其更商务简洁，并同时提升：

- 视觉层次
- 可读性
- 操作效率

本次仅改 Android `Settings` UI 组件层，不改后端协议和设置保存语义。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`

- `SettingsPage`
  - 新增顶部说明文案与摘要区（Total / Enabled / Writeable）
  - 新增 `saving` 线性进度条，保存状态反馈更明显
  - 错误提示改为容器化错误条（`errorContainer`）
  - 设置列表外层新增 `Surface + BorderStroke`，形成清晰信息区块
  - 宽屏阈值调整为 `>= 920.dp`，并优化宽/窄屏的间距策略

- `SettingsHeaderRow`
  - 标题栏改为更弱化的 `onSurfaceVariant` 风格
  - 列宽与权重重新平衡，保留 `Writeable -> Enabled` 顺序

- `SettingsRow`（宽屏）
  - 行容器改为卡片式 `Surface`，提升分组感
  - 增加能力标签（`Control + Report` / `Report only`）
  - 值展示改为 `MetricValuePill`
  - 不可写项改为 `Read-only` 标签，而非单个 `-`

- `SettingsCompactRow`（窄屏）
  - 改为更清晰的卡片结构：标题区、变量名区、开关区
  - 对不可写项使用 `Writeable: Read-only` 容器化占位
  - 保持 `Writeable -> Enabled` 顺序

- 新增辅助组件
  - `MetricSummaryChip`
  - `CapabilityTag`
  - `MetricValuePill`
  - `SettingToggle`（样式升级）

- `CompactVarNameField`
  - 新增占位文本 `var_name`，空值状态更直观

- 业务链路保持不变
  - `load / validate / saveNow / scheduleSave` 逻辑未改
  - `MetricSetting` 字段语义未改

### 新增

- `docs/change/2026-03-05_android-settings-modernization.md`（本文件）

### 删除

- 无

## 对应 plan/todo 任务映射

- M1 需求与方案确认 -> 已完成
- M2 Settings 页面视觉重构 -> 已完成
- M3 构建验证 -> 已完成
- M4 Code Review -> 已完成
- M5 归档变更 -> 已完成

## 关键设计决策与权衡（性能 / 扩展性）

1. 决策：采用 Material3 组件轻重构，而非完全自定义视觉系统
- 原因：与 Connect 页视觉一致性更强，维护成本低。
- 权衡：品牌化表达空间有限，但整体稳定性更高。

2. 决策：引入摘要区与行级标签
- 原因：提升信息扫描效率与可读性。
- 权衡：UI 元素数量增加，需控制密度。

3. 决策：保留保存链路与字段语义不变
- 原因：降低行为回归风险，可回滚成本低。
- 权衡：不借此次改动引入更激进交互能力（如批量编辑、搜索过滤）。

4. 性能说明
- 新增计算仅为前端 `count` 聚合（O(n)），在现有 settings 规模下开销可忽略。
- 无新增 I/O、无新增网络调用、无新增线程竞争。

## 测试与验证方式 / 结果

### 构建验证

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
$env:ANDROID_SDK_ROOT='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug
```

结果：`BUILD SUCCESSFUL`

### 关键路径检查

- 空配置显示：`No settings loaded.` 路径保留
- 错误提示显示：`settingsError` 路径保留并增强
- 保存状态反馈：`StatusPill + LinearProgressIndicator`
- 编辑与保存：`onVarNameChange -> scheduleSave -> saveNow` 链路保持
- Writeable 可控判断：仍由 `controllable` 集合约束

## 潜在影响与回滚方案

### 潜在影响

- 布局层级更丰富，极小屏设备下摘要区会占用更多首屏高度。
- UI 视觉变化较大，若已有截图对比测试需更新基线。

### 回滚方案

- 直接回滚 `MainActivity.kt` 本次提交可恢复改造前样式。
- 若仅需局部回退，可单独回退新增辅助组件和 `SettingsPage` 摘要区变更。
