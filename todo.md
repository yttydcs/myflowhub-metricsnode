# MetricsNode Android Settings 再紧凑化（2026-03-05）

## 项目目标与当前状态

- 目标：解决 Android `Settings` 页面“控件仍显著偏大”的问题，在不改变业务逻辑（读取/保存 settings、校验、debounce、状态展示）的前提下，降低行高与视觉占用，提高信息密度。
- 当前状态：
  - `Enabled/Writable` 已从 `Switch` 替换为 `Checkbox`。
  - 但 `SettingsRow` 仍使用 `OutlinedTextField`，其默认最小高度与可点击控件最小交互尺寸导致行高偏大。

## 可执行任务清单（Checklist）

- [x] T1 需求与成因确认
  - 目标：明确“仍然偏大”的直接技术成因，避免误改数据流。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 形成可追溯结论：行高主要由输入组件和勾选组件默认最小尺寸共同撑大。
  - 测试点：
    - 静态代码审阅（关键尺寸 Modifier 与组件默认行为）。
  - 回滚点：
    - 无代码改动，无需回滚。

- [x] T2 Settings UI 紧凑化实现
  - 目标：在不改业务逻辑的前提下压缩行高、列宽、间距；确保可读性和稳定性。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 输入框高度、勾选控件视觉尺寸与行间距均明显收敛。
    - 页面仍可编辑 `var_name`，并保持 `enabled/writable` 可切换。
    - 保存逻辑（400ms debounce + 即时保存路径）不变。
  - 测试点：
    - 编译通过。
    - 手工验证：切换 `enabled/writable`、编辑 `var_name`、观察 `Saving/Ready` 状态。
  - 回滚点：
    - 仅回退 `MainActivity.kt` 本次改动。

- [x] T3 构建验证
  - 目标：确认改动可编译并产出可安装 APK。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
  - 测试点：
    - 本地 Gradle 构建日志无错误。
  - 回滚点：
    - 若构建失败，回退 T2 后重建。

- [x] T4 Code Review（3.3）
  - 目标：按质量门禁逐项审查。
  - 涉及模块/文件：
    - `MainActivity.kt`
  - 验收条件：
    - 需求覆盖、架构、性能、可读性、可扩展性、稳定性/安全、测试七项均给出“通过/不通过”结论。
  - 测试点：
    - 结合代码与构建结果审查。
  - 回滚点：
    - 不通过则回到 T2 修正。

- [x] T5 归档变更（4）
  - 目标：生成可审计变更文档。
  - 涉及模块/文件：
    - `docs/change/YYYY-MM-DD_*.md`
  - 验收条件：
    - 文档包含背景目标、改动明细、任务映射、设计权衡、测试结果、风险与回滚。
  - 测试点：
    - 文档字段完整、可交接。
  - 回滚点：
    - 文档修订直至满足要求。

## 依赖关系

- T2 依赖 T1。
- T3 依赖 T2。
- T4 依赖 T2/T3。
- T5 依赖 T4 通过。

## 风险与注意事项

- 压缩交互尺寸会影响可点击热区；需在“紧凑”与“可操作性”之间平衡。
- 仅做 UI 尺寸层改动，禁止引入计划外业务逻辑变更。
- 若发现需新增需求（例如完全改为卡片折叠布局），必须先更新本 `todo.md` 再实施。

---

# Windows 构建脚本固化（2026-03-05）

## 项目目标与当前状态

- 目标：新增 `scripts/build-windows.ps1`，把 `GOWORK=off`、`wailsjs` 绑定清理、`wails generate module`、`wails build` 固化为一条可复用命令，降低 Windows 启动与前端构建中的绑定失配问题。
- 当前状态：
  - Windows 构建命令可手动执行，但依赖人工顺序，容易遗留旧 `wailsjs` 产物。
  - 现有仓库仅有 `scripts/build_aar.ps1` / `scripts/build_aar.sh`，未提供 Windows 一键构建脚本。

## 可执行任务清单（Checklist）

- [x] W1 需求基线与接口定义
  - 目标：明确脚本输入参数、默认行为和失败边界，避免引入不必要逻辑。
  - 涉及模块/文件：
    - `scripts/build-windows.ps1`（新建）
    - `windows/`（构建目录，仅调用不改业务）
  - 验收条件：
    - 脚本参数与默认行为文档化（`WindowsDir`、是否跳过清理/生成、是否保留 `GOWORK`）。
  - 测试点：
    - 静态审阅脚本参数和路径拼接逻辑。
  - 回滚点：
    - 删除新增脚本即可完全回滚。

- [x] W2 实现构建脚本
  - 目标：在 PowerShell 中实现可重复构建流程，提供清晰日志与错误处理。
  - 涉及模块/文件：
    - `scripts/build-windows.ps1`
  - 验收条件：
    - 默认执行流程：清理 `windows/frontend/wailsjs/go` -> `wails generate module` -> `wails build`。
    - 错误即停止并输出可定位信息。
  - 测试点：
    - 路径不存在、`wails` 未安装、外部命令失败时均有明确报错。
  - 回滚点：
    - 回退该脚本文件。

- [x] W3 构建验证
  - 目标：确认脚本在当前仓库可直接执行并产出 `windows.exe`。
  - 涉及模块/文件：
    - `scripts/build-windows.ps1`
    - `windows/build/bin/windows.exe`（产物验证）
  - 验收条件：
    - 执行脚本成功且目标产物存在。
  - 测试点：
    - 本地运行 `powershell -File scripts/build-windows.ps1`。
  - 回滚点：
    - 若脚本不稳定，先回退 W2，再恢复手工构建流程。

- [x] W4 Code Review（阶段 3.3）
  - 目标：按门禁逐项审查需求覆盖、性能、可读性、扩展性、稳定性与测试。
  - 涉及模块/文件：
    - `scripts/build-windows.ps1`
  - 验收条件：
    - 七项审查均给出通过/不通过结论。
  - 测试点：
    - 结合代码与执行日志审阅。
  - 回滚点：
    - 不通过则返回 W2 修正。

- [x] W5 归档变更（阶段 4）
  - 目标：在 `docs/change/` 产出可审计交接文档。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_metricsnode-windows-build-script.md`
  - 验收条件：
    - 文档包含背景、改动、任务映射、设计权衡、验证结果、风险与回滚。
  - 测试点：
    - 字段完整性检查。
  - 回滚点：
    - 修订文档直至完整。

## 依赖关系

- W2 依赖 W1。
- W3 依赖 W2。
- W4 依赖 W2/W3。
- W5 依赖 W4 通过。

## 风险与注意事项

- 清理目录必须限定在 `windows/frontend/wailsjs/go`，避免误删非生成文件。
- `GOWORK` 只在脚本进程内覆盖并恢复，防止影响调用者环境。
- 禁止引入计划外业务代码改动；若需要新增功能（如 dev 模式启动），必须先更新本 `todo.md`。

---

# Android Settings 响应式与可见性修复（2026-03-05）

## 项目目标与当前状态

- 目标：彻底修复 Android `Settings` 页面“看起来等比例放大、只能看到复选框”的问题，确保窄屏设备默认可见关键信息且无需横向拖动才能编辑核心字段。
- 当前状态：
  - 页面使用 `horizontalScroll + widthIn(min=740dp)` 的桌面表格布局。
  - 列宽依赖 `Row.weight`，且处于横向可滚动容器中，存在列宽分配不可预期风险。
  - 近期改动集中在控件尺寸收敛，未触达布局根因。

## 可执行任务清单（Checklist）

- [x] R1 根因复盘与证据收集
  - 目标：确认“只有复选框”现象由布局策略导致，而非数据层问题。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 形成可追溯结论：横向滚动容器 + 固定最小宽度 + 权重列导致窄屏可见区域异常。
  - 测试点：
    - 静态代码审阅（滚动容器、列宽策略、控件可见路径）。
  - 回滚点：
    - 无代码改动，无需回滚。

- [x] R2 Settings 布局重构为响应式
  - 目标：移除导致异常的横向滚动表格方案，改为窄屏友好的单列卡片/双行布局，同时保留桌面端信息完整性。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 默认进入 `Settings` 时可直接看到 `metric/var_name/value` 与 `enabled/writable` 交互控件。
    - 不依赖横向滚动即可完成常见编辑操作。
    - 保持既有保存链路（本地校验 + debounce + bridge set）不变。
  - 测试点：
    - 手工代码路径检查：`load -> render -> onChange -> scheduleSave -> metricsSettingsSet`。
    - UI 结构检查：无 `horizontalScroll` 依赖。
  - 回滚点：
    - 回退 `MainActivity.kt` 本次响应式改动。

- [x] R3 构建与关键路径验证
  - 目标：确认改动可编译，且不引入行为回归。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
    - Settings 编辑与勾选逻辑代码路径保持可达。
  - 测试点：
    - 本地 Gradle 构建日志。
  - 回滚点：
    - 若失败，回退 R2 后重建定位。

- [x] R4 Code Review（阶段 3.3）
  - 目标：按门禁逐项审查需求覆盖、架构、性能、可读性、扩展性、稳定性与测试。
  - 涉及模块/文件：
    - `MainActivity.kt`
  - 验收条件：
    - 七项门禁均输出通过/不通过及理由。
  - 测试点：
    - 结合改动 diff 与构建结果审阅。
  - 回滚点：
    - 不通过则返回 R2 修正。

- [x] R5 归档变更（阶段 4）
  - 目标：形成可审计、可交接的变更文档。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_metricsnode-android-settings-responsive.md`（预期）
  - 验收条件：
    - 文档包含背景、改动、任务映射、设计权衡、验证结果、影响与回滚。
  - 测试点：
    - 文档字段完整性检查。
  - 回滚点：
    - 修订文档直至满足归档要求。

## 依赖关系

- R2 依赖 R1。
- R3 依赖 R2。
- R4 依赖 R2/R3。
- R5 依赖 R4 通过。

## 风险与注意事项

- 响应式重构需保持原数据流与校验策略，避免引入计划外业务变化。
- 若改动触及组件抽象边界过大，应优先采用单文件内最小可回滚方案。
- 若需新增模式（例如平板专用表格分支），必须同步更新本清单。

---

# Android Settings 开关顺序与控件类型调整（2026-03-05）

> Worktree: `d:\project\MyFlowHub3\repo\MyFlowHub-MetricsNode\worktrees\fix-android-settings-switch-order`
> Branch: `fix/android-settings-switch-order`

## 项目目标与当前状态

- 目标：将 Android Settings 页面中的两个开关列顺序调整为 `Writeable` 在前、`Enabled` 在后，并将 checkbox 风格控件替换为开关（Switch）。
- 当前状态：
  - 宽屏表格头顺序为 `Enabled` 后 `Writable`。
  - 宽屏行与窄屏行都使用 `CompactCheck`（checkbox 视觉与语义）。

## 可执行任务清单（Checklist）

- [x] T1 需求与架构确认
  - 目标：确认变更仅限 Android Settings UI，不影响 settings 保存语义。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 明确改动点：Header、宽屏 Row、窄屏 Row、Toggle 组件。
  - 测试点：
    - 代码静态审阅。
  - 回滚点：
    - 无代码变更，不需要回滚。

- [x] T2 UI 顺序与控件改造
  - 目标：
    - 顺序改为 `Writeable` -> `Enabled`。
    - checkbox 替换为 Material3 `Switch`。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 宽屏表头顺序正确。
    - 宽屏行第一列是 Writeable 开关，第二列是 Enabled 开关。
    - 窄屏行 `SettingToggle` 顺序一致。
    - `controllable` 不包含该 metric 时仍显示占位文本。
  - 测试点：
    - 编译检查（`android` 模块）。
    - 手工检查 UI 顺序和开关行为。
  - 回滚点：
    - 回滚 `MainActivity.kt` 本次改动。

- [x] T3 构建验证
  - 目标：确保改动后 Android 可编译。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
  - 测试点：
    - Gradle 构建日志无错误。
  - 回滚点：
    - 失败时回退 T2 后重试。

- [x] T4 Code Review（3.3）
  - 目标：按流程输出七项审查结论。
  - 涉及模块/文件：
    - `MainActivity.kt`
    - `todo.md`
  - 验收条件：
    - 需求覆盖、架构、性能、可读性、扩展性、稳定性/安全、测试覆盖给出通过/不通过。
  - 测试点：
    - 结合代码 diff + 构建结果。
  - 回滚点：
    - 不通过则回到 T2 修正。

- [x] T5 归档变更（4）
  - 目标：形成可审计变更记录。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_android-settings-writeable-enabled-switch.md`
  - 验收条件：
    - 文档包含背景目标、改动明细、任务映射、设计权衡、验证结果、回滚方案。
  - 测试点：
    - 文档字段完整且可交接。
  - 回滚点：
    - 文档修订直至满足要求。

## 依赖关系

- T2 依赖 T1。
- T3 依赖 T2。
- T4 依赖 T2、T3。
- T5 依赖 T4 通过。

## 风险与注意事项

- `Writeable/Writable` 存在拼写差异：本次按用户要求展示 `Writeable`，内部字段仍保持 `writable`，避免协议变化。
- 宽屏列宽若过窄可能导致开关拥挤，需同步调整列宽保持可用性。

---

# Android Settings 现代化与风格统一（2026-03-05）

> Worktree: `d:\project\MyFlowHub3\repo\MyFlowHub-MetricsNode\worktrees\refactor-android-settings-modern-ui`
> Branch: `refactor/android-settings-modern-ui`

## 项目目标与当前状态

- 目标：仅优化 Android `Settings` 页面，使其更现代、商务简洁，并与 `Connect` 页保持一致视觉语言。
- 当前状态：
  - 结构可用但层次较平，状态信息和编辑区域视觉区分不足。
  - 宽窄屏布局可用，但“可读性 / 操作效率”仍有提升空间。

## 可执行任务清单（Checklist）

- [x] M1 需求与方案确认（阶段 1/2）
  - 目标：确认本次只改 Android Settings UI，不变更数据语义和保存链路。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 需求、范围、验收标准、风险、方案与模块职责明确。
  - 测试点：
    - 代码路径静态审阅。
  - 回滚点：
    - 无代码改动，无需回滚。

- [x] M2 Settings 页面视觉重构（阶段 3.2）
  - 目标：提升视觉层次、可读性、操作效率，同时保持 Connect 风格一致。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 顶部状态与摘要信息更清晰（保存状态/统计信息）。
    - 列表容器和行样式更现代（层次、间距、对齐优化）。
    - 宽屏与窄屏均保持 `Writeable -> Enabled` 顺序。
    - 不改变 `load/validate/scheduleSave/saveNow` 语义。
  - 测试点：
    - 手工检查空数据、错误提示、saving 状态、可写/不可写行展示。
  - 回滚点：
    - 回滚 `MainActivity.kt` 本次改动。

- [x] M3 构建验证
  - 目标：确认改动可编译。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
  - 测试点：
    - Gradle 构建日志无错误。
  - 回滚点：
    - 构建异常时回退 M2 并重试。

- [x] M4 Code Review（阶段 3.3）
  - 目标：按门禁输出通过/不通过。
  - 涉及模块/文件：
    - `MainActivity.kt`
    - `todo.md`
  - 验收条件：
    - 需求覆盖、架构合理性、性能、可读性、扩展性、稳定性/安全、测试覆盖给出结论。
  - 测试点：
    - 结合 diff 与构建结果审查。
  - 回滚点：
    - 不通过返回 M2 修正。

- [x] M5 归档变更（阶段 4）
  - 目标：形成可审计交接文档。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_android-settings-modernization.md`
  - 验收条件：
    - 文档包含背景目标、变更明细、任务映射、关键权衡、验证结果、影响与回滚。
  - 测试点：
    - 字段完整性检查。
  - 回滚点：
    - 文档修订直至完整。

## 依赖关系

- M2 依赖 M1。
- M3 依赖 M2。
- M4 依赖 M2、M3。
- M5 依赖 M4 通过。

## 风险与注意事项

- 仅允许 UI 层改动，禁止改动 settings 协议字段和 service 行为。
- 视觉升级需克制，避免与 `Connect` 页面风格割裂。
- 若出现计划外需求，必须先更新 `todo.md` 再实施。



