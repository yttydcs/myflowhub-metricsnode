# Plan - MyFlowHub-MetricsNode（P0 指标扩展 + Android 手电）

> Worktree：`d:\project\MyFlowHub3\worktrees\feat-metricsnode-p0-metrics-flashlight`  
> 分支：`feat/metricsnode-p0-metrics-flashlight`  
> 日期：2026-03-02  
>
> 本 workflow 目标：在既有 VarStore 子协议与 bindings 机制上扩展 P0 指标（Windows + Android），并为 Android 增加 `flashlight_enabled`（可读可写）。

---

## 1. 需求分析

### 1.1 目标

- 扩展 P0 指标（Windows + Android）：
  - `battery_charging`（0/1/-1）
  - `battery_on_ac`（0/1/-1）
  - `net_online`（0/1/-1）
  - `net_type`（none/wifi/ethernet/cellular/unknown/-1）
  - `cpu_percent`（0~100/-1）
  - `mem_percent`（0~100/-1）
- Android 额外能力：
  - `flashlight_enabled`（0/1/-1），可读可写（写入后触发系统手电开关）。

### 1.2 背景/现状

- VarStore `value` 当前为 `string`，因此“枚举”使用约定字符串集合表达，不在协议层做强类型校验。
- MetricsNode 当前已支持：电量/音量/静音/亮度（含 Windows 亮度 WMI fallback）。

### 1.3 范围

- 必须：
  - 新增上述 metric 常量，并纳入 `metrics.bindings_json` 的默认 bindings：
    - 跨平台默认：P0 指标 + 既有电量/音量/静音/亮度；
    - Android 默认额外包含 `flashlight_enabled`；
    - 保持“仅迁移旧默认，不覆盖用户自定义”。
  - Windows：新增采集（battery 状态/网络/CPU/内存）。
  - Android：新增采集（battery 状态/网络/CPU/内存/手电状态）。
  - 下行控制：
    - `flashlight_enabled`：Android 执行写入；不支持/失败时不崩溃并纠偏为真实值或 `-1`。
  - 只读变量被外部写入时：
    - 不执行；并尽量将 VarStore 纠偏回当前正确值（若当前值未知则 warn 并忽略）。
- 不做：
  - 不改变 VarStore 子协议与 payload schema。
  - 不引入 WiFi/蓝牙/锁屏等其它可控项（后续单开）。

### 1.4 使用场景

- 监控：UI 订阅 `sys_cpu_percent/sys_mem_percent/sys_net_type` 等变量显示设备状态。
- Android 手电：
  - 其它节点对 owner=<AndroidMetricsNodeNodeID> 写 `sys_flashlight_enabled=1` → 打开手电；
  - 写 `0` → 关闭手电；
  - 不支持/无权限 → 上报 `-1` 并纠偏。

### 1.5 功能需求（值域/语义）

- 通用约定：
  - `-1`：读不到/不支持/未知；
  - 数值均为十进制字符串；
  - boolish：`0/1`。
- `battery_charging`：
  - 语义按用户确认：**插电（AC/USB/无线）即视为 1**（即便电量已满）。
- `net_type`：
  - 固定枚举字符串：`none/wifi/ethernet/cellular/unknown`，读不到/无权限时 `-1`。
- `flashlight_enabled`：
  - `1`：手电开；`0`：关；读不到/不支持 `-1`。
  - 写入失败：不崩溃，后续由“读取/回调上报”纠偏为真实值或 `-1`。

### 1.6 非功能需求

- 稳定性：长时间运行不泄漏，不崩溃；权限不足/硬件不支持时 graceful degrade。
- 性能：能事件驱动则事件驱动；轮询保持秒级且轻量（CPU/内存）。
- 安全：下行控制仍只接受 owner 匹配（沿用既有校验逻辑）。

### 1.7 输入输出

- 输入：
  - Windows：系统 API（电源/网络/CPU/内存）。
  - Android：系统服务（Battery/Connectivity/ActivityManager/CameraManager）。
  - VarStore `notify_set`（用于下行控制与只读纠偏）。
- 输出：
  - 上报：metrics → bindings → VarStore set（默认 `sys_<metric>`；可被用户覆盖）。
  - 控制：Android 手电开关；失败则纠偏为真实状态或 `-1`。

### 1.8 验收标准

- Android：
  - `flashlight_enabled` 可读可写：写 `sys_flashlight_enabled=1/0` 能切换手电；不支持/读不到则上报 `-1`。
  - P0 指标按值域持续上报。
- Windows：
  - P0 指标按值域持续上报。
- 回归命令通过：
  - Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
  - Go（Windows 模块）：`cd windows/frontend; npm ci; npm run build` 后 `cd ..; GOWORK=off go test ./... -count=1 -p 1`
  - Go（nodemobile）：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
  - Android：`cd android; ./gradlew :app:assembleDebug`

### 1.9 风险

- `net_online/net_type` 的平台口径差异：本轮以“合理近似 + 可观测”为主，后续可演进为更精确的“默认路由/validated”策略。
- Android 手电需要 `CAMERA` 权限与硬件支持：无权限/不支持时必须稳定输出 `-1` 且忽略写入并纠偏。

---

## 2. 架构设计（分析）

### 2.1 总体方案（不改协议）

- VarStore `value: string` 承载全部指标：
  - “枚举”以约定字符串表达（例如 `net_type=wifi`）。
- 上行：
  - Windows：Go collectors 采集 → `emit(metric,value)` → runtime 按 bindings 发布。
  - Android：Kotlin NodeService 采集 → `NodeBridge.updateXxx(value)` →（gomobile 导出）→ Go runtime 按 bindings 发布。
- 下行（Android 手电）：
  - VarStore `notify_set` → `handleVarStoreNotifySet` 入队 `flashlight_enabled` 控制动作 → Android NodeService 轮询 `DequeueActions` 并调用 CameraManager 执行 → torch callback / re-read 上报真实状态完成纠偏。
- 只读纠偏：
  - 收到对只读变量的 `notify_set`，以“最近一次已发布的正确值”覆盖回 VarStore（若未知则 warn 并忽略）。

### 2.2 模块职责

- `core/metrics/metrics.go`：
  - 新增 metric 常量；
  - 增加“是否可控/是否只读”的判定（供校验/纠偏使用）。
- `core/runtime/config.go`：
  - 默认 bindings 与迁移逻辑升级（仅迁移旧默认，不覆盖自定义）；
  - 维护平台默认（Windows 不包含手电；Android 包含手电）。
- `core/runtime/varstore_inbound.go`：
  - 新增 `flashlight_enabled` 下行路由（boolish 解析 → 控制动作）；
  - 将只读纠偏从“仅 battery_percent”扩展到本轮新增的只读 metrics。
- Windows：
  - `core/metrics/collectors_windows.go`：新增 battery 状态、网络、CPU、内存采集 loop。
- Android：
  - `android/.../NodeService.kt`：新增网络/CPU/内存采集；新增手电状态采集与执行写入。
  - `android/.../NodeBridge.kt`：新增 update 方法（仍采用可选反射策略，避免旧 AAR 运行期崩溃）。
- Go ↔ Android（gomobile）：
  - `nodemobile/nodemobile.go`：新增 `UpdateXxx(string)` 导出函数。

### 2.3 数据/调用流

- 上行（Android）：
  - `NodeService` → `bridge.updateFlashlightEnabled("1")` → `nodemobile.UpdateFlashlightEnabled("1")` → Go runtime → VarStore set
- 下行（Android 手电）：
  - `notify_set` → enqueue `ControlAction{metric:"flashlight_enabled",value:"1"}` → `NodeService` `DequeueActions()` → `CameraManager.setTorchMode(...)` → torch callback → 上报真实状态

### 2.4 接口草案（值域）

- `battery_charging`：`-1/0/1`
- `battery_on_ac`：`-1/0/1`
- `net_online`：`-1/0/1`
- `net_type`：`-1/none/wifi/ethernet/cellular/unknown`
- `cpu_percent`：`-1/0..100`
- `mem_percent`：`-1/0..100`
- `flashlight_enabled`：`-1/0/1`（可写）

### 2.5 错误与安全

- 权限/硬件不支持：
  - 读取：上报 `-1`；
  - 写入：记录 warn，不崩溃；随后上报真实状态或 `-1` 完成纠偏。
- 只处理 owner 匹配的 notify_set（沿用既有逻辑）。

### 2.6 性能与测试策略

- Android：
  - 网络：`ConnectivityManager` callback 优先；fallback 轮询（必要时）。
  - 手电：`CameraManager.registerTorchCallback` 事件驱动。
  - CPU：读取 `/proc/stat` delta（2s）。
  - 内存：`ActivityManager.MemoryInfo`（2s）。
- Windows：
  - CPU：`GetSystemTimes` delta（2s）。
  - 内存：`GlobalMemoryStatusEx`（2s）。
  - 网络：`GetAdaptersAddresses`（5s；避免过密）。
- 测试：
  - Go：扩展 bindings 迁移单测 + varstore_inbound（只读纠偏 + 手电入队）单测。
  - Android：`assembleDebug`；手工验证 torch 切换。

### 2.7 可扩展性设计点

- 指标均通过 bindings 可配置 var_name；
- 只读/可控语义集中在 Go core（便于新增指标时统一处理纠偏与入队规则）。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。

### T1 - 新增 metrics 常量与只读/可控分类

- 涉及文件：
  - `core/metrics/metrics.go`
- 验收：
  - 新增常量：
    - `battery_charging` / `battery_on_ac`
    - `net_online` / `net_type`
    - `cpu_percent` / `mem_percent`
    - `flashlight_enabled`
  - 新增分类辅助：可控（volume/mute/brightness/flashlight）与只读（battery*、net*、cpu/mem）。

### T2 - 默认 bindings 与迁移升级

- 涉及文件：
  - `core/runtime/config.go`
  - `core/runtime/config_test.go`
- 验收：
  - 旧默认 bindings 安全迁移（不覆盖用户自定义）：
    - legacy → 加入新 P0 指标（以及 brightness/手电在对应平台默认内）。
  - Windows 默认不包含手电；Android 默认包含手电。

### T3 - Windows：新增采集（battery 状态/网络/CPU/内存）

- 涉及文件：
  - `core/metrics/collectors_windows.go`
- 验收：
  - 按值域输出；异常时 `-1`；日志节流合理。

### T4 - Go Core：只读纠偏 + flashlight 下行路由

- 涉及文件：
  - `core/runtime/varstore_inbound.go`
  - `core/runtime/varstore_inbound_test.go`
- 验收：
  - 只读变量被 set 时会纠偏为当前正确值（未知则 warn）。
  - `flashlight_enabled` 写入可入队控制动作。

### T5 - Android：采集 P0 指标 + 手电读写

- 涉及文件：
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
  - `android/app/src/main/AndroidManifest.xml`（新增 `CAMERA`、`ACCESS_NETWORK_STATE`）
  - `nodemobile/nodemobile.go`
- 验收：
  - 手电可读可写；不支持/读不到则 `-1`。
  - 网络/CPU/内存/电源状态按值域上报。

### T6 - 回归验证

- Go：`GOWORK=off go test ./... -count=1 -p 1`
- Windows module：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`
- nodemobile：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
- Android：`cd android; ./gradlew :app:assembleDebug`

### T7 - Code Review（阶段 3.3）+ 归档（阶段 4）

- 归档输出：
  - `docs/change/2026-03-02_metricsnode-p0-metrics-flashlight.md`
