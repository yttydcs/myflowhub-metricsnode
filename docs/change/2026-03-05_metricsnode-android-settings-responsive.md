# 2026-03-05 MetricsNode Android Settings 响应式修复（根因修正）

## 变更背景 / 目标

用户持续反馈 Android `Settings` 页面“像被等比例放大，且只能看到复选框”，此前多轮改动只做了控件尺寸收敛（间距、列宽、checkbox 体积），体感无改善。

本次目标：

1. 修正布局根因，而非继续微调控件尺寸。
2. 窄屏设备默认可见 `metric / var_name / value / enabled / writable` 的可编辑信息。
3. 保持既有设置数据流（读取、校验、400ms debounce 保存、bridge 调用）不变。

## 根因结论（排查结果）

1. `Settings` 页面此前采用 `horizontalScroll + widthIn(min=740dp)` 桌面表格策略。
2. 列宽依赖 `Row.weight(...)`，但该行处于横向滚动语境中，列宽分配存在不稳定/不可预期行为。
3. 在窄屏下用户首屏易落在右侧操作列，出现“只看到复选框”的主观体验；继续缩小 checkbox 无法解决信息列不可见问题。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- 移除 `Settings` 的横向滚动表格依赖（删除 `horizontalScroll` 与 `widthIn(min=740dp)` 路径）。
- 在 `SettingsPage` 中引入响应式分支：
  - `maxWidth >= 880dp`：保留宽屏表格行（`SettingsHeaderRow + SettingsRow`）。
  - `maxWidth < 880dp`：使用窄屏卡片行（`SettingsCompactRow`），每项纵向展示 metric/value、var_name 输入、enabled/writable 交互。
- 新增 `SettingsCompactRow`、`SettingToggle` 两个私有组件，复用现有 `CompactVarNameField` 与 `CompactCheck`。
- 保持以下逻辑不变：
  - `metricsSettingsGet -> parse -> render`
  - `var_name` 本地校验（字符规则）
  - `scheduleSave` 400ms debounce + `metricsSettingsSet`
  - `enabled/writable` 开关保存链路

2. `todo.md`
- 新增 “Android Settings 响应式与可见性修复（2026-03-05）” 工作流章节并更新执行状态。

### 新增

- 本文档（归档文档）。

### 删除

- 无。

## 对应 todo.md 任务映射

- R1 根因复盘与证据收集 -> `MainActivity.kt` 旧布局路径审计（scroll + fixed width + weighted columns）。
- R2 Settings 布局重构为响应式 -> `MainActivity.kt` 中 `SettingsPage` 分支与新增 `SettingsCompactRow/SettingToggle`。
- R3 构建与关键路径验证 -> `android :app:assembleDebug`。
- R4 Code Review（3.3） -> 本文档下方“Code Review 结论”章节。
- R5 归档变更（4） -> 本文档。

## 关键设计决策与权衡（性能 / 扩展性）

1. 选择“响应式分支”而非“继续缩放控件”
- 原因：问题本质是布局策略，不是控件像素大小。
- 权衡：代码量增加（宽屏/窄屏两套呈现），但行为稳定性显著提升。

2. 宽屏保留表格，窄屏改卡片
- 原因：兼顾平板信息密度与手机可读性。
- 权衡：UI 形态在不同宽度下不完全一致，但用户可用性更高。

3. 保留原保存与校验逻辑
- 原因：避免引入业务回归，确保改动集中在 View 层。
- 性能：无新增网络 I/O、无新增轮询；仅 UI 结构调整。

## Code Review 结论（阶段 3.3）

1. 需求覆盖：通过
- 命中“首屏可见性与可操作性”根因，不再仅调控件尺寸。

2. 架构合理性：通过
- 仍在 `MainActivity.kt` 局部封装，数据流和 bridge 边界不变，无跨模块耦合扩散。

3. 性能风险：通过
- 无 N+1、无重复 I/O、无新增后台任务；Compose 仅增加少量布局节点。

4. 可读性与一致性：通过
- 新增组件职责清晰（`SettingsCompactRow`、`SettingToggle`），且复用现有输入/勾选组件。

5. 可扩展性与配置化：通过
- 后续新增字段可继续在 `SettingsCompactRow`/`SettingsRow` 双分支扩展；宽度阈值集中在单点。

6. 稳定性与安全：通过
- 未改权限、鉴权、存储协议与桥接接口；错误处理与校验逻辑保持原样。

7. 测试覆盖情况：通过（编译 + 关键路径静态验证）
- `assembleDebug` 通过。
- 关键编辑/保存路径代码保持可达。

## 测试与验证方式 / 结果

执行命令：

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug --stacktrace --no-daemon --console=plain
```

结果：

- `BUILD SUCCESSFUL`（本次改动后复测通过）。
- 日志中 `gomobile AAR not found` 为当前工程既有 stub 提示，不影响本次 UI 编译验证。

## 潜在影响与回滚方案

潜在影响：

1. UI 在宽屏与窄屏下形态不同（表格 vs 卡片），截图对比需按设备宽度判定。
2. 880dp 阈值是经验值，后续可根据真机反馈微调。

回滚方案：

1. 仅回滚 `MainActivity.kt` 本次响应式改动，恢复原单一表格实现。
2. 若需部分回滚，可保留宽屏分支，仅回退窄屏卡片分支。
