# 2026-03-03 MetricsNode：P0 指标扩展 + Android 手电（VarStore）

## 背景 / 目标

在不修改 VarStore 子协议与 payload schema 的前提下，扩展 MetricsNode 的可上报指标（Windows + Android），并为 Android 增加一个可读可写的手电能力（通过 VarStore 变量控制）。

本次新增指标（VarStore `value` 均为 `string`）：

- Windows + Android：
  - `battery_charging`（`0/1/-1`；语义：**插电即 1（即便满电）**）
  - `battery_on_ac`（`0/1/-1`）
  - `net_online`（`0/1/-1`）
  - `net_type`（`none/wifi/ethernet/cellular/unknown/-1`）
  - `cpu_percent`（`0~100/-1`）
  - `mem_percent`（`0~100/-1`）
- Android 额外：
  - `flashlight_enabled`（`0/1/-1`，可读可写）

通用约定：
- `-1`：读不到 / 不支持 / 未知。
- “枚举”不做协议强类型，使用约定字符串集合表达（例如 `net_type=wifi`）。

## 具体变更

### 1) Go Core（metrics/runtime）

- `core/metrics/metrics.go`
  - 新增 metric 常量：`battery_charging/battery_on_ac/net_online/net_type/cpu_percent/mem_percent/flashlight_enabled`
  - 新增分类函数：`IsReadOnly()` / `IsControllable()`（用于下行与纠偏逻辑）

- `core/runtime/config.go`
  - 默认 bindings 扩展为 P0 指标（默认 var_name：`sys_<metric>`）
  - 平台默认差异：
    - Windows：不包含 `flashlight_enabled`
    - Android：默认包含 `flashlight_enabled` → `sys_flashlight_enabled`
  - 兼容迁移升级：仅当检测到用户仍使用旧默认 bindings（v0=3项、v1=4项）时自动升级为新默认；不覆盖用户自定义
  - `supportedMetric()` 扩展支持新增指标

- `core/runtime/varstore_inbound.go`
  - 新增 `flashlight_enabled` 下行解析（boolish → 入队控制动作）
  - 将“只读纠偏”从原先仅 `battery_percent` 扩展为 **所有只读指标**：
    - 收到对只读变量的 `notify_set` 时不执行控制；用当前已上报的正确值覆盖回 VarStore（未知则 warn 并忽略）

### 2) Windows 采集

- `core/metrics/collectors_windows.go`
  - battery：基于 `GetSystemPowerStatus` 上报：
    - `battery_percent`
    - `battery_on_ac`
    - `battery_charging`（本轮按“插电即 1”语义，与 `battery_on_ac` 同步）
  - cpu：基于 `GetSystemTimes` delta 上报 `cpu_percent`
  - mem：基于 `GlobalMemoryStatusEx` 上报 `mem_percent`
  - net：基于 `GetAdaptersAddresses` 近似判断：
    - `net_online`：存在 `OperStatusUp` 且有 Unicast 地址的非 loopback/tunnel 适配器即视为在线
    - `net_type`：按 IfType 映射到 `wifi/ethernet/cellular/unknown`（无候选则 `none`）

### 3) Android 采集与控制（含 gomobile）

- `nodemobile/nodemobile.go`
  - 新增 gomobile 导出更新接口（均为 `string` 入参）：
    - `UpdateBatteryCharging/UpdateBatteryOnAC`
    - `UpdateNetOnline/UpdateNetType`
    - `UpdateCPUPercent/UpdateMemPercent`
    - `UpdateFlashlightEnabled`

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
  - 扩展 `NodeBridge` 接口与 `GoNodeBridge` 反射绑定（新增 UpdateXxx 均采用 **可选反射**，避免旧 AAR 运行期崩溃）

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - battery：`ACTION_BATTERY_CHANGED` 事件驱动上报：
    - `battery_percent`、`battery_on_ac`、`battery_charging`
  - cpu：读取 `/proc/stat` delta 上报 `cpu_percent`（2s）
  - mem：`ActivityManager.MemoryInfo` 上报 `mem_percent`（2s）
  - net：`ConnectivityManager.activeNetwork` + `NetworkCapabilities` 上报：
    - `net_online`：存在 activeNetwork 即视为在线
    - `net_type`：按 transport 映射到 `wifi/ethernet/cellular/unknown`（无网络则 `none`）
  - 手电（`flashlight_enabled`）：
    - 读取：`CameraManager.registerTorchCallback` 事件驱动更新（无权限/不支持 → `-1`）
    - 写入：控制队列收到 `flashlight_enabled` 后调用 `CameraManager.setTorchMode`（失败不崩溃，并按已知状态或 `-1` 纠偏）

- `android/app/src/main/AndroidManifest.xml`
  - 新增权限：`CAMERA`、`ACCESS_NETWORK_STATE`、`FOREGROUND_SERVICE_CAMERA`
  - 新增 `uses-feature android.hardware.camera.flash`（`required=false`）
  - Service `foregroundServiceType` 扩展为 `dataSync|camera`

## 计划任务映射（plan.md）

- T1：新增 metrics 常量与分类（已完成）
- T2：默认 bindings 与迁移升级（已完成）
- T3：Windows 新增采集（已完成）
- T4：只读纠偏 + flashlight 下行路由（已完成）
- T5：Android 采集 P0 指标 + 手电读写（已完成）
- T6：回归验证（已完成，见下）

## 关键决策与权衡

- 不引入协议层“枚举/强类型”，保持 VarStore `value:string`，枚举用约定字符串表达（扩展性优先）。
- `battery_charging` 语义按确认口径实现：插电即 1（Windows/Android 均与 on_ac 同步）。
- `net_online/net_type` 口径存在平台差异，本轮以“合理近似 + 可观测”为主；后续可演进为更严格的 default route / validated 策略。
- Android 手电：
  - 读取依赖 torch callback，无法读取/无权限时统一上报 `-1`；
  - 写入失败不崩溃，依赖回调/后续上报完成纠偏。

## 测试与验证

已在本 worktree 执行：

- Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
- Windows frontend：`cd windows/frontend; npm ci; npm run build`
- Windows module：`cd windows; GOWORK=off go test ./... -count=1 -p 1`
- nodemobile：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
- Android：`cd android; ./gradlew.bat :app:assembleDebug`
- AAR（本地）：`$env:ANDROID_HOME/_SDK_ROOT/_NDK_HOME` 指向 `_android-sdk` 后执行 `scripts/build_aar.ps1`

## 潜在影响与回滚方案

- 影响：
  - 默认 bindings 增加新变量，会在首次启动/迁移后开始上报更多 VarStore 变量；
  - Android 手电需要 `CAMERA` 权限与硬件支持，否则 `flashlight_enabled` 将为 `-1`。
- 回滚：
  - 回滚本次提交即可恢复到旧指标集合与旧默认 bindings；
  - 如需快速停止上报某指标，可在 `metrics.bindings_json` 移除对应 binding（不影响其它功能）。

