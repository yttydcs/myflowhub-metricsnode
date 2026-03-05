# 2026-03-05 MetricsNode Android Settings 再紧凑化

## 变更背景 / 目标

用户在 Android `Settings` 页面确认已是 `Checkbox` 后仍反馈“选项看起来偏大”。  
本次目标是在不改变业务逻辑的前提下继续收敛控件尺寸与行密度，提升同屏信息量。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- `Settings` 表格最小宽度由 `860dp` 收敛至 `740dp`，降低横向拥挤与大屏留白。
- 行分隔间距由 `6dp` 收敛至 `4dp`。
- `Enabled/Writable` 列宽由 `74dp` 收敛至 `58dp`。
- `SettingsRow` 中 `Var Name` 编辑控件从 `OutlinedTextField` 切换为 `CompactVarNameField`（`BasicTextField` + 紧凑边框容器），避免默认最小高度导致行高偏大。
- `Enabled/Writable` 勾选控件改为 `CompactCheck`（22dp 轻量勾选框），降低视觉占用。

### 新增

1. `todo.md`
- 本次 workflow 的可交接执行清单（阶段 3.1）。

### 删除

- 无。

## 对应 todo.md 任务映射

- T1 需求与成因确认 -> `MainActivity.kt` 中尺寸来源分析（默认最小输入/交互尺寸）。
- T2 Settings UI 紧凑化实现 -> `MainActivity.kt` 紧凑输入与勾选控件、列宽与间距收敛。
- T3 构建验证 -> `android` 执行 `:app:assembleDebug`。
- T4 Code Review（3.3） -> 本文档“Code Review 结论”章节。
- T5 归档变更（4） -> 本文档。

## 关键设计决策与权衡（性能 / 扩展性）

1. 保持业务逻辑零改动
- `load/validate/save/debounce` 与桥接调用均未调整，仅改 UI 尺寸层，降低行为回归风险。

2. 使用轻量输入组件替代默认高基线组件
- `OutlinedTextField` 默认最小高度较高，改为 `BasicTextField` 外包边框容器，显式控制高度与内边距。
- 权衡：可访问性触达面积下降，需在真机交互中确认误触率。

3. 勾选控件改为紧凑自绘
- 通过 `toggleable` + 小尺寸容器降低行高。
- 权衡：相比系统 `Checkbox` 视觉一致性略弱，但密度收益更高，且保留了语义角色（`Role.Checkbox`）。

4. 性能
- 无新增 I/O、无新增轮询、无新增复杂计算；仅 UI 结构轻量化，重组成本不升反降。

## Code Review 结论（阶段 3.3）

1. 需求覆盖：通过
- 直接针对“控件仍偏大”做尺寸收敛，且未影响 Settings 数据编辑与保存路径。

2. 架构合理性：通过
- 保持现有页面分层与数据流；新增组件为本地私有 `@Composable`，无跨模块耦合。

3. 性能风险：通过
- 无 N+1、无重复计算热点、无新增 I/O；仅 UI 组件替换与布局参数收敛。

4. 可读性与一致性：通过
- 紧凑组件抽为 `CompactVarNameField/CompactCheck`，职责清晰，可维护性较好。

5. 可扩展性与配置化：通过
- 保留表格列结构，后续新增列仍可复用当前布局权重与控件模式。

6. 稳定性与安全：通过
- 未涉及权限、鉴权、网络、存储写入策略变更；风险面限定在 UI 表现层。

7. 测试覆盖情况：通过（构建 + 关键路径静态验证）
- 本地 `assembleDebug` 成功，关键交互路径代码（编辑/切换/保存）仍在原调用链。

## 测试与验证方式 / 结果

执行命令：

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug --stacktrace --no-daemon --console=plain
```

结果：`BUILD SUCCESSFUL`。  
备注：日志提示缺少 `android/app/libs/myflowhub.aar` 时使用 stub bridge，属于既有工程策略，本次 UI 编译验证不受影响。

## 潜在影响与回滚方案

潜在影响：

1. 控件更紧凑后，触达面积下降，部分设备可能出现误触/漏触概率上升。
2. 自绘勾选框视觉与 Material 默认控件略有差异。

回滚方案：

1. 仅回滚 `MainActivity.kt` 本次变更（恢复 `OutlinedTextField + Checkbox + 旧列宽/间距`）。
2. 如需部分回退，可仅恢复 `CompactCheck` 为 `Checkbox`，保留文本输入紧凑化改动。
