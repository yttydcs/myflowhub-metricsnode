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
