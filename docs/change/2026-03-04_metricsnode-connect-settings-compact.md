# 2026-03-04 MetricsNode Connect 权限修复 + Settings 紧凑化（Android）

## 变更背景 / 目标

用户反馈两个问题：

1. Connect 页面点击连接后报错：`dial tcp 127.0.0.1:9000: socket: operation not permitted`。
2. Settings 页面组件视觉尺寸过大，信息密度偏低。

本次目标：

- 修复 Connect 的权限型阻塞错误（`operation not permitted`）。
- 在不改变 Settings 业务逻辑的前提下，实现紧凑化 UI。

## 具体变更内容（新增 / 修改 / 删除）

### 修改

1. `android/app/src/main/AndroidManifest.xml`
- 新增权限：`android.permission.INTERNET`。
- 目的：允许 App 发起 TCP 网络连接，避免 socket 在权限层被系统拒绝。

2. `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- Settings 页面紧凑化：
  - 页面/卡片内部间距由 `12dp` 收敛到 `8dp`（局部）。
  - 表格最小宽度由 `920dp` 收敛到 `860dp`。
  - 行分隔间距由 `8dp` 收敛到 `6dp`。
  - `Var Name` 输入框设置 `heightIn(min = 40.dp)` 并使用 `bodySmall` 文本样式。
  - `Enabled/Writable` 控件从 `Switch` 改为 `Checkbox`，列宽从 `90dp` 收敛到 `74dp`。
- 保持不变：
  - Settings 数据结构（`metric/var_name/enabled/writable`）
  - 本地校验规则
  - debounce 自动保存逻辑

### 新增

1. `todo.md`
- 本 workflow 可交接任务拆分文档（阶段 3.1）。

### 删除

- 无。

## 对应 todo.md 任务映射

- T1 修复 Connect 网络权限 -> `AndroidManifest.xml` 增加 `INTERNET`。
- T2 Settings 页面紧凑化 -> `MainActivity.kt` Settings 区域样式与控件调整。
- T3 本地构建验证 -> `ANDROID_HOME=d:\project\MyFlowHub3\_android-sdk` 下执行 `:app:assembleDebug`。
- T4 Code Review（3.3） -> 本文档下方“Code Review 结论”。
- T5 归档 docs/change（4） -> 本文档。

## 关键设计决策与权衡（性能 / 扩展性）

1. 权限修复采用最小变更：
- 仅新增 `INTERNET`，不改连接链路和错误传递逻辑，降低回归风险。

2. Settings 紧凑化采用“样式收敛而非结构重写”：
- 不引入新状态、不改变数据流，避免额外同步/状态错误。
- 通过更轻量控件（`Checkbox`）和更小间距提升可视密度，减少滚动与重排负担。

3. 扩展性：
- 仍保留现有列结构和通用列表渲染方式，后续增加指标字段时无需重做页面架构。

## Code Review 结论（阶段 3.3）

1. 需求覆盖：通过
- 连接权限报错根因（缺少 `INTERNET`）已修复。
- Settings 组件尺寸已整体收敛。

2. 架构合理性：通过
- 未改变服务调用链与桥接接口，属于低风险增量修复。

3. 性能风险：通过
- 无新增轮询、无新增 I/O、无额外复杂计算。

4. 可读性与一致性：通过
- 改动集中在单页 UI 样式和单条权限声明，结构清晰。

5. 可扩展性与配置化：通过
- 未引入硬编码分叉逻辑；保持原配置驱动。

6. 稳定性与安全：通过
- `INTERNET` 为联网必需权限；其余安全行为不变。

7. 测试覆盖情况：通过（构建验证）
- 完成 Android debug 包构建验证，见下一节。

## 测试与验证方式 / 结果

执行命令：

```powershell
$env:ANDROID_HOME='d:\project\MyFlowHub3\_android-sdk'
.\gradlew.bat :app:assembleDebug
```

结果：`BUILD SUCCESSFUL`。

说明：构建日志提示 `myflowhub.aar` 未找到时会使用 stub bridge，这是现有工程策略，不影响本次权限与 UI 改动的编译验证。

## 潜在影响与回滚方案

潜在影响：

1. 增加 `INTERNET` 后，应用具备联网能力；需继续依赖现有认证与服务端策略控制访问。
2. `Checkbox` 命中区域较 `Switch` 小，若个别设备误触率上升可再微调列宽与 padding。

回滚方案：

1. 权限回滚：移除 `AndroidManifest.xml` 中 `INTERNET` 声明。
2. UI 回滚：回退 `MainActivity.kt` Settings 页改动（恢复 `Switch`、旧间距和旧列宽）。
3. 全量回滚：按本次变更文件粒度回退到修改前版本。
