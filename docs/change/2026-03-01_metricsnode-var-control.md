# 2026-03-01 MetricsNode：VarStore 下行控制（音量）与只读纠偏（电量）

## 背景 / 目标

此前 `MyFlowHub-MetricsNode` 已能将本机指标（电量/音量）上报到 VarStore，但当其它节点修改（`set(owner=<MetricsNodeNodeID>, name=...)`）这些变量时：

- MetricsNode 未处理 VarStore 的下行通知（`notify_set`），无法实现“变量既是状态也是命令”的反向控制；
- 对“无意义可写”的指标（如电量）缺少纠偏机制，容易被误操作污染变量池。

本次目标：

1) 支持 `sys_volume_percent` / `sys_volume_muted` 被修改后，MetricsNode 执行本机音量/静音调整（Windows + Android）。
2) 支持 `sys_battery_percent` 被修改后，MetricsNode 不执行本机行为，并将变量回写为当前真实值（只读纠偏）。
3) 不修改 `metrics.bindings_json` schema（不新增可选字段），仍通过 bindings 完成 metric ↔ var_name 映射。

## 具体变更内容

### Go Core（跨平台）

- 新增 VarStore 下行处理：识别 `SubProtoVarStore` 的 `notify_set`，反查 bindings 并路由为：
  - 音量/静音：入队控制动作；
  - 电量：读取当前真实值并回写纠偏。
- 增加“最新动作队列”：
  - 同一 metric 只保留最新 value；
  - 提供 `DequeueActions()` 供 Android 侧轮询消费。
- 修复去重兼容：收到下行时同步更新本地 `lastPublished` shadow，避免纠偏回写被差量判断误过滤。

涉及文件：

- `core/runtime/runtime.go`
- `core/runtime/varstore_inbound.go`
- `core/runtime/control_queue.go`
- `core/runtime/control_actions.go`

### Windows（执行）

- 新增 Windows 侧音量执行器（默认输出设备 master volume）：
  - COM 初始化 + `LockOSThread`；
  - endpoint 复用（避免每次 set 重新枚举/初始化 COM）。
- 新增控制 worker：异步消费动作队列，避免阻塞网络收包循环。

涉及文件：

- `core/actuator/volume_windows.go`
- `core/actuator/volume_other.go`
- `core/runtime/control_worker_windows.go`
- `core/runtime/control_worker_other.go`
- `core/metrics/collectors_windows.go`（增加 `LockOSThread`，提升 COM 稳定性）

### Android（执行）

- 通过 gomobile 导出新增：`nodemobile.DequeueActions()`（JSON 数组字符串）。
- `NodeService` 增加控制线程：轮询 actions 并用 `AudioManager` 执行：
  - `volume_percent`：`setStreamVolume(STREAM_MUSIC, ...)`，并尽量保持 mute 状态不被“音量调整”隐式改变；
  - `volume_muted`：`adjustStreamVolume(ADJUST_MUTE/ADJUST_UNMUTE, ...)`。
- 增加权限：`android.permission.MODIFY_AUDIO_SETTINGS`。

涉及文件：

- `nodemobile/nodemobile.go`
- `android/app/src/main/AndroidManifest.xml`
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`

## plan.md 任务映射

- T0：基线 `go test` 通过
- T1：VarStore `notify_set` 解码与路由（执行/纠偏）
- T2：Windows 音量执行器 + 异步 worker
- T3：Android actions 轮询执行 + 权限
- T4：补齐 Go 单测 + 验收步骤
- T5：Code Review + 归档

## 关键设计决策与权衡

1) **遵守子协议语义**
   - 使用 VarStore 已有 `notify_set` 作为 owner 下行通知入口，不引入新协议动作。

2) **去重与纠偏的冲突处理**
   - `publishVar` 有差量去重（value+visibility），如果外部把只读变量改错，而本地真实值恰好等于 `lastPublished`，纠偏会被误过滤。
   - 方案：收到下行后先更新 `lastPublished` shadow 为“外部写入值”，从而纠偏回写一定会被视为变化并发送。

3) **跨平台差异收敛**
   - Go Core 只做“解析/校验/入队”，执行在平台侧完成：
     - Windows：Go 内部 worker 直接执行；
     - Android：Kotlin `NodeService` 轮询执行（避免 Kotlin 重复实现协议栈）。

4) **性能与稳定性**
   - Windows：COM 初始化与 endpoint 枚举只在 worker 生命周期做一次；并使用 `LockOSThread` 避免 COM 线程模型问题。
   - Android：控制轮询间隔 250ms（响应性优先，后续如有电量压力可再调优/加事件唤醒机制）。

## 测试与验证

### Go 单测（已执行）

在 `worktrees/feat-metricsnode-var-control`：

- `GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### 手工端到端（建议步骤）

前置：

- 确保 MetricsNode 已连接 Hub 并完成登录（有 `node_id`）。
- 触发下行必须对 owner 写入：`set(owner=<MetricsNodeNodeID>, name=...)`。

用例 1：音量控制（Windows / Android）

1) 在任意可写 VarStore 的客户端（如 `MyFlowHub-Win` VarPool）执行：
   - `sys_volume_percent` 写入 `30`（或 `200`，验证 clamp→100）
   - `sys_volume_muted` 写入 `1`/`0`
2) 观察：
   - 本机系统音量/静音状态改变；
   - 随后 MetricsNode 的采集上报会把 VarStore 值同步为实际值（例如 clamp 后的 100）。

用例 2：电量只读纠偏（Windows / Android）

1) 写入 `sys_battery_percent=123`
2) 观察：
   - MetricsNode 不执行本机行为；
   - 很快将变量纠偏回当前真实电量（无电池设备则为配置后的 `no_battery_value`）。

说明：

- Android Gradle wrapper 的 `gradle-wrapper.jar` 当前未纳入仓库，无法在本环境直接执行 `gradlew` 编译验证；需要在具备 Gradle wrapper/本机 Gradle 的环境编译 APK 进行端测。

## 潜在影响与回滚方案

潜在影响：

- Windows：新增控制 worker goroutine + 独占 OS thread（COM）；长期运行会占用 1 个线程（与现有音量采集 goroutine 类似）。
- Android：新增控制轮询线程 + 新权限 `MODIFY_AUDIO_SETTINGS`（需用户同意安装/授予）。

回滚方案：

- 回滚本分支对应提交即可恢复到“仅上报、无下行控制/纠偏”的行为。

