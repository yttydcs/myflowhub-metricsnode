# Plan - MyFlowHub-MetricsNode（VarStore 下行控制：音量；只读纠偏：电量）

> Worktree：`d:\project\MyFlowHub3\worktrees\feat-metricsnode-var-control`
> 分支：`feat/metricsnode-var-control`
>
> 本文档是本 workflow 的唯一执行清单；任何实现性改动必须严格按本计划逐项完成、验证、Review、归档。

---

## 0. 项目目标与当前状态

### 0.1 当前状态

- MetricsNode MVP（采集电量/音量 → VarStore + Devices Config 配置）已在主线完成并归档：`docs/change/2026-02-28_metricsnode-mvp.md`。
- 当前缺口：VarStore 的 **下行通知（notify_set）** 到达 MetricsNode 时未被处理，导致“把变量当命令”的反向控制无法生效。

### 0.2 目标（本 workflow）

在不改变现有 bindings schema 的前提下，让 MetricsNode 支持“同一变量既是状态也是命令”：

1) **可写指标（Writable）**
   - 变量 `sys_volume_percent` / `sys_volume_muted` 被其它节点修改（owner=本节点）后：
     - MetricsNode 能接收到 VarStore 下行通知；
     - 执行本机音量/静音调整；
     - 并确保后续采集上报不被差量去重逻辑“卡住”。

2) **只读指标（Read-only）**
   - 变量 `sys_battery_percent` 被其它节点修改后：
     - MetricsNode **不执行任何本机行为**；
     - 并将变量 **纠偏回写** 为当前真实电量值（自愈）。

### 0.3 非目标（本 workflow 不做）

- 市场/插件系统/第三方扩展包。
- 通用的“任意变量 → 任意执行器”的声明式执行框架（本次只做音量/电量的最小闭环，但需为后续扩展留边界）。
- 变更 `metrics.bindings_json` 的结构（不新增可选字段）。

### 0.4 已确认决策（来自需求/架构阶段）

- 平台：Windows + Android 同时支持。
- 音量 clamp：`0..100`，clamp 后执行。
- 音量修改策略：仅修改音量，不自动切换静音（策略 A）。
- “可改”默认开启；但对电量这类无意义修改，收到修改通知后不执行，只纠偏回写。
- Android：允许增加权限 `android.permission.MODIFY_AUDIO_SETTINGS`。

---

## 1. 总体方案（落地到代码的约定）

> 目标：遵守 VarStore 子协议语义，不阻塞主收包循环，跨平台差异收敛在“执行层”。

### 1.1 关键设计点

- **协议入口**：在 `core/runtime` 增加 VarStore 下行帧处理（识别 `SubProtoVarStore` 的 `MajorCmd` 帧，并处理 `ActionNotifySet`）。
- **绑定反查**：通过现有 bindings（metric ↔ var_name）将 `var_name` 反查到 metric 类型，再决定“执行 / 纠偏回写”。
- **去重兼容**：收到下行后同步更新本地 shadow（或提供“强制回写”路径），避免采集上报被差量判断误过滤。
- **执行解耦**：
  - Windows：Go 侧直接执行（COM endpoint volume），但必须放到独立 worker，避免阻塞 runtime 的网络循环；并避免每次 set 都重复 COM 初始化。
  - Android：Go 侧仅入队“控制动作（actions）”，Kotlin `NodeService` 轮询拉取并执行（gomobile 导出函数），避免 Kotlin 重复实现协议栈。
- **队列策略**：同一控制项只保留“最新动作”，避免下行频繁时堆积与抖动。

### 1.2 安全与输入校验（最低要求）

- 仅处理来自 Hub 的 VarStore notify_set（即 runtime 收到的 VarStore frame）；不接受本地外部进程注入。
- 对 value 做类型/范围校验：无法解析的输入不执行，并记录告警日志。
- 只读指标收到 set：不执行、纠偏回写（避免被恶意/误操作污染公共变量池）。

---

## 2. 可执行任务清单（Checklist）

> 约定：每次编码必须标注对应 Task ID；不允许计划外改动。

### T0 - Worktree 基线与回归保障

- 目标：确保在本 worktree 上可以稳定编译/测试，作为后续改动的回归基线。
- 涉及模块/文件：
  - `plan.md`
- 验收条件：
  - `GOWORK=off go test ./... -count=1 -p 1` 通过（至少可编译）。
- 回滚点：
  - 回滚本分支提交即可恢复。

### T1 - Go Core：接收 VarStore notify_set 并路由为“执行 / 纠偏”

- 目标：在 MetricsNode runtime 中处理 VarStore 下行，完成反向控制与只读纠偏的核心逻辑。
- 涉及模块/文件（预期）：
  - `core/runtime/runtime.go`（接入 VarStore frame 处理）
  - `core/runtime/*`（新增：notify_set 解码、绑定反查、路由逻辑）
  - `core/metrics/*`（必要时暴露“当前真实值”读取接口/快照）
- 验收条件：
  - 远端 set（owner=本节点）`sys_volume_percent/sys_volume_muted` 后，本节点能产生对应控制动作（Windows 直接执行；Android 入队）。
  - 远端 set `sys_battery_percent` 后，本节点会很快将变量回写为真实电量值。
- 测试点：
  - `volume_percent`：`-10`、`200`、`"abc"`。
  - `volume_muted`：`"true"/"false"`、非法输入。
- 回滚点：
  - 回滚 runtime VarStore handler 提交即可回到仅上报。

### T2 - Windows：音量/静音执行器（Actuator）

- 目标：实现 Windows 侧的 `SetVolumePercent/SetMuted`，并确保线程模型与性能可接受。
- 涉及模块/文件（预期）：
  - `core/actuator/*_windows.go`（新增）
  - `core/runtime/*`（调用执行器、worker 管理）
- 验收条件：
  - 远端 set 后系统音量/静音会变化；重复快速 set 不导致卡顿/崩溃。
- 性能关键点：
  - COM 初始化与 endpoint 获取应在 worker 生命周期内复用，避免每次 set 都初始化 COM。
- 回滚点：
  - 回滚 actuator 提交即可禁用 Windows 反向控制。

### T3 - Android：控制动作队列 + NodeService 执行

- 目标：实现 Android 侧音量/静音的实际执行，并与 Go core 解耦。
- 涉及模块/文件（预期）：
  - `nodemobile/nodemobile.go`（新增导出：`DequeueActions()`，JSON 数组字符串）
  - `android/app/src/main/java/**/NodeService.kt`（轮询 actions 并调用 `AudioManager` 执行）
  - `android/app/src/main/AndroidManifest.xml`（增加 `MODIFY_AUDIO_SETTINGS`）
- 验收条件：
  - 远端 set 后，Android 媒体音量/静音生效。
- 队列策略验收：
  - 高频下行时仅执行最新动作（不积压）。
- 回滚点：
  - 回滚 Android 执行与 gomobile 导出提交即可回到仅采集上报。

### T4 - 测试与验收（端到端）

- 目标：提供可复现的验证步骤（Win + Android），并补齐必要的 Go 单测（纯逻辑部分）。
- 形式：
  - Go 单测：clamp/解析/只读纠偏策略/队列合并策略。
  - 手工冒烟（必须写清步骤）：
    - 使用 `MyFlowHub-Win`（或任意可 set VarStore 的客户端）对 owner=本节点的变量执行 set。
- 验收条件：
  - Go：`GOWORK=off go test ./... -count=1 -p 1` 通过。
  - 手工：两端到端用例都能稳定复现。
- 回滚点：
  - 测试仅辅助，可单独回滚。

### T5 - 阶段 3.3 Code Review 与阶段 4 归档

- 目标：完成强制 Review 清单，并归档变更。
- 输出：
  - `docs/change/2026-03-01_metricsnode-var-control.md`
- 验收条件：
  - Review 结论：通过；归档文档齐全。

---

## 3. 本地验证命令（建议）

为避免临时目录权限/并发导致的问题，可使用：

```powershell
$env:GOTMPDIR='d:\\project\\MyFlowHub3\\.tmp\\gotmp'
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
GOWORK=off go test ./... -count=1 -p 1
```

---

## 4. 依赖关系、风险与注意事项

### 4.1 依赖

- Hub 已启用 Auth/VarStore 子协议，且 Server 会将 `notify_set` 下发给变量 owner（本节点）。
- 用于触发下行的客户端能够执行 VarStore `set(owner=<MetricsNodeNodeID>, name=...)`（否则会写到自己的 owner 下，看起来“没同步”）。

### 4.2 风险

- **反馈环**：下行控制后，采集上报会再把最新状态写回 VarStore；需要确保差量去重/本地 shadow 更新正确，否则可能出现“下行生效但 UI 值不更新/不回写”的错觉。
- **只读纠偏刷屏**：若外部持续写入只读变量（电量），纠偏会频繁回写；必要时考虑加最小间隔（若出现实际问题，再新增任务）。
- **Windows COM 稳定性**：需要遵守 COM 线程模型，避免在高频 set 下频繁初始化导致异常或卡顿。
- **Android 版本差异**：不同版本对静音 API 的支持不同；需在实现阶段确认最低兼容策略（以当前工程既有实现为准）。

### 4.3 注意事项

- 本 workflow 不改变 `metrics.bindings_json` schema；“可写/只读”由代码内对已知 metric 的执行器能力决定。
- 所有控制执行必须异步化，不能阻塞 runtime 的网络收包循环。
