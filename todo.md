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




---

# Android Settings 文案清理（2026-03-05）

> Worktree: `d:\project\MyFlowHub3\repo\MyFlowHub-MetricsNode\worktrees\refactor-android-settings-label-cleanup`
> Branch: `refactor/android-settings-label-cleanup`

## 项目目标与当前状态

- 目标：
  - 移除 `Metric` 列中的 `Report only` 文案。
  - 移除每行 `Var Name/var_name` 文案，避免重复标签噪声。
- 当前状态：
  - `Metric` 行目前通过 `CapabilityTag` 展示 `Control + Report` / `Report only`。
  - 宽屏和窄屏每行均显示 `Var Name` 标签，输入框占位为 `var_name`。

## 可执行任务清单（Checklist）

- [x] C1 需求与方案确认（阶段 1/2）
  - 目标：确认仅做 Android Settings UI 文案清理，不改数据语义。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 明确改动点与不改动边界。
  - 测试点：
    - 静态代码审阅。
  - 回滚点：
    - 无代码改动。

- [x] C2 文案清理实现（阶段 3.2）
  - 目标：
    - 移除 `Report only` 显示。
    - 移除每行 `Var Name` 标签和 `var_name` 占位文案。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 界面不再出现 `Report only`。
    - 每行不再出现 `Var Name` 标签。
    - 输入框占位不含 `var_name`。
  - 测试点：
    - 宽屏/窄屏两套布局静态检查。
  - 回滚点：
    - 回滚 `MainActivity.kt` 本次改动。

- [x] C3 构建验证
  - 目标：确保改动可编译。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
  - 测试点：
    - 构建日志无错误。
  - 回滚点：
    - 构建异常时回退 C2。

- [x] C4 Code Review（阶段 3.3）
  - 目标：按门禁输出通过/不通过。
  - 涉及模块/文件：
    - `MainActivity.kt`
    - `todo.md`
  - 验收条件：
    - 七项审查结论完整。
  - 测试点：
    - diff + 构建结果审查。
  - 回滚点：
    - 不通过返回 C2 修正。

- [x] C5 归档变更（阶段 4）
  - 目标：形成可审计文档。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_android-settings-label-cleanup.md`
  - 验收条件：
    - 文档字段完整可交接。
  - 测试点：
    - 字段完整性检查。
  - 回滚点：
    - 文档修订直至完整。

## 依赖关系

- C2 依赖 C1。
- C3 依赖 C2。
- C4 依赖 C2、C3。
- C5 依赖 C4 通过。

## 风险与注意事项

- 仅做文案和展示层清理，禁止改动 settings 存储字段与校验逻辑。
- 避免影响宽/窄屏可读性，保持当前布局结构和交互行为。



---

# Android Settings 去除 Control 标签与项目外框（2026-03-05）

> Worktree: `d:\project\MyFlowHub3\repo\MyFlowHub-MetricsNode\worktrees\refactor-android-settings-remove-control-and-item-frame`
> Branch: `refactor/android-settings-remove-control-and-item-frame`

## 项目目标与当前状态

- 目标：
  - 移除 `Metric` 列的 `Control` 标签。
  - 移除每个设置项目外层圆角矩形边框。
- 当前状态：
  - `SettingsRow` 与 `SettingsCompactRow` 在 metric 区域显示 `Control`。
  - `SettingsRow` 与 `SettingsCompactRow` 顶层均使用圆角 `Surface + BorderStroke` 外框。

## 可执行任务清单（Checklist）

- [x] D1 需求与方案确认（阶段 1/2）
  - 目标：确认仅改 Android Settings 展示层，不改保存/校验语义。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 改动边界明确：仅 Control 标签与项目外框。
  - 测试点：
    - 静态代码审阅。
  - 回滚点：
    - 无代码改动。

- [x] D2 UI 调整实现（阶段 3.2）
  - 目标：
    - 去掉 metric 区 `Control` 标签。
    - 去掉每个项目最外层圆角矩形框。
  - 涉及模块/文件：
    - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - 验收条件：
    - 宽屏/窄屏均不再出现 `Control`。
    - 宽屏/窄屏每项不再有最外层圆角边框。
  - 测试点：
    - 代码路径检查与 UI 结构检查。
  - 回滚点：
    - 回滚 `MainActivity.kt`。

- [x] D3 构建验证
  - 目标：确保改动可编译。
  - 涉及模块/文件：
    - `android/` Gradle 工程
  - 验收条件：
    - `:app:assembleDebug` 成功。
  - 测试点：
    - 构建日志无错误。
  - 回滚点：
    - 构建失败回退 D2。

- [x] D4 Code Review（阶段 3.3）
  - 目标：输出七项审查结论。
  - 涉及模块/文件：
    - `MainActivity.kt`
    - `todo.md`
  - 验收条件：
    - 七项审查通过/不通过明确。
  - 测试点：
    - diff + 构建结果。
  - 回滚点：
    - 不通过返回 D2 修正。

- [x] D5 归档变更（阶段 4）
  - 目标：形成可审计归档。
  - 涉及模块/文件：
    - `docs/change/2026-03-05_android-settings-remove-control-item-frame.md`
  - 验收条件：
    - 文档字段完整可交接。
  - 测试点：
    - 字段完整性检查。
  - 回滚点：
    - 文档修订直至完整。

## 依赖关系

- D2 依赖 D1。
- D3 依赖 D2。
- D4 依赖 D2、D3。
- D5 依赖 D4 通过。

## 风险与注意事项

- 仅移除目标标签和项目外框，不改其他交互部件（开关/输入/保存状态）。
- 去掉外框后需保持分隔可读性（依赖列表容器和分隔线）。


---

# MetricsNode CI 构建失败修复（2026-03-07）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-ci`  
> Branch：`fix/metricsnode-ci`  
> 基线：`origin/main`（commit `450558b`）  
>
> 目标：修复 GitHub Actions `ci` 在 `MyFlowHub-MetricsNode` 上的构建失败，使其能稳定产出 Windows EXE 与 Android Debug APK（含 gomobile AAR），并保障 `push(main)` 的 `debug-latest` 发布链路恢复可用。

## 项目目标与当前状态

- 当前失败现象（已本地复现，与 CI 注释一致）：
  - Windows：`wails build` 在 “Generating bindings” 阶段失败：`pattern all:frontend/dist: no matching files found`  
    - 根因：Wails 在生成 bindings 时会触发 Go 编译检查 `//go:embed all:frontend/dist`；而 `windows/frontend/dist` 在 clean checkout 中不存在（被 `.gitignore` 忽略），导致直接报错。
  - Android：`scripts/build_aar.sh`（`gomobile bind`）失败：`go: updates to go.mod needed; to update it: go mod tidy`  
    - 根因：`nodemobile` 子模块依赖图未对齐（疑似与 SDK 升级到 `v0.1.2` 后间接依赖变化有关），导致 gomobile 在只读模式下拒绝自动改写 `go.mod`。

## 可执行任务清单（Checklist）

- [ ] CI1 需求与验收基线确认（阶段 1/2 复核）
  - 目标：把“必须修复”的 CI 产物与门禁明确化，避免只修一个 job 导致 `publish-debug-latest` 仍被 needs 阻塞。
  - 涉及模块/文件：
    - `.github/workflows/ci.yml`
  - 验收条件：
    - `build-windows-amd64` 成功产出 `windows/build/bin/windows.exe`
    - `build-android-debug` 成功产出：
      - `android/app/build/outputs/apk/debug/app-debug.apk`
      - `android/app/libs/myflowhub.aar`
    - `push(main)` 时 `publish-debug-latest` 可执行并上传 EXE/APK（AAR 也应存在）。
  - 测试点：
    - 本地复现命令记录（Windows `wails build` / `scripts/build_aar.sh`）。
  - 回滚点：
    - 无代码改动，本任务无回滚点。

- [ ] CI2 修复 Windows：确保 Wails 生成 bindings 前 `frontend/dist` 非空
  - 目标：让 clean checkout 场景下的 `wails build` 不再因 `go:embed all:frontend/dist` 为空而失败。
  - 方案（最小变更，推荐）：
    - 在 `.github/workflows/ci.yml` 的 Windows job 里，在安装 Wails 后、执行 `wails build` 前新增一步：
      - 创建目录 `windows/frontend/dist`
      - 写入临时占位文件（例如 `windows/frontend/dist/.keep`）
    - 说明：Vite 默认会清空 `dist` 目录再输出真实产物，因此占位文件不会污染最终 embed 内容。
  - 可选加固（若需要覆盖本地开发）：
    - 同步修正 `scripts/build-windows.ps1`：若 `frontend/dist` 不存在则创建并写占位文件，再执行 `wails build`。
  - 涉及模块/文件：
    - `.github/workflows/ci.yml`
    - （可选）`scripts/build-windows.ps1`
  - 验收条件：
    - 在本地（模拟 clean 状态：删除 `windows/frontend/dist`）执行：
      - `cd windows; $env:GOWORK='off'; wails build -platform windows/amd64 -nopackage`
      - 可成功生成 `windows/build/bin/windows.exe`
    - GitHub Actions `build-windows-amd64` 通过。
  - 测试点：
    - 本地 `wails build` 成功一次（输出 exe）
  - 回滚点：
    - 回退 `.github/workflows/ci.yml`（以及可选脚本）本次提交。

- [ ] CI3 修复 Android：提交 `nodemobile` 模块 tidy，消除 gomobile 的只读失败
  - 目标：让 `bash scripts/build_aar.sh` 在 CI 中可稳定生成 `android/app/libs/myflowhub.aar`。
  - 实施：
    - 在 `nodemobile/` 目录执行 `GOWORK=off go mod tidy`，提交 `nodemobile/go.mod` + `nodemobile/go.sum` 变更。
  - 涉及模块/文件：
    - `nodemobile/go.mod`
    - `nodemobile/go.sum`
  - 验收条件：
    - 本地执行 `bash scripts/build_aar.sh`（或 CI 中同等步骤）不再出现 `go mod tidy` 提示。
    - CI `Verify outputs` 阶段能找到 `android/app/libs/myflowhub.aar`。
  - 测试点：
    - 本地（Windows 可用 Git-Bash）：`bash scripts/build_aar.sh`
  - 回滚点：
    - 回退 `nodemobile/go.mod/go.sum` 本次提交。

- [ ] CI4 回归验证（本地 + CI）
  - 目标：在合并前尽可能本地验证关键链路，减少“上 CI 才发现问题”的成本。
  - 涉及模块/文件：
    - `windows/`、`android/`、`scripts/`、`.github/workflows/ci.yml`
  - 验收条件：
    - Windows：`wails build` 可产出 exe
    - Android：`scripts/build_aar.sh` 可产出 AAR；`android/gradlew :app:assembleDebug` 可产出 APK
  - 测试点：
    - Windows：`cd windows; $env:GOWORK='off'; wails build -platform windows/amd64 -nopackage`
    - Android：`bash scripts/build_aar.sh`；`cd android; ./gradlew :app:assembleDebug --stacktrace --no-daemon --console=plain`
  - 回滚点：
    - 若失败，按任务粒度逐个回退 CI2/CI3 的提交定位。

- [ ] CI5 Code Review（阶段 3.3）+ 归档（阶段 4）
  - 目标：按门禁输出审查结论，并新增可审计变更文档。
  - 涉及模块/文件：
    - `todo.md`
    - `docs/change/2026-03-07_metricsnode-ci-fix.md`（新增）
  - 验收条件：
    - Code Review 七项均有结论（通过/不通过）
    - 归档文档包含：背景目标、改动明细、任务映射、关键权衡、测试结果、风险与回滚
  - 测试点：
    - 文档自查完整性
  - 回滚点：
    - 文档可迭代修订直至完整。

## 依赖关系

- CI2/CI3 依赖 CI1 明确验收基线。
- CI4 依赖 CI2 + CI3。
- CI5 依赖 CI4 验证通过。

## 风险与注意事项

- `frontend/dist` 占位文件策略需要确保不会进入最终产物：依赖 Vite 默认清空 `dist`；若未来前端构建配置改变，需要重新确认。
- `go mod tidy` 可能带来间接依赖版本变化；需通过 CI4 的 AAR/APK/EXE 构建验证兜底。


