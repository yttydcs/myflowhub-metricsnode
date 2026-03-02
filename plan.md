# Plan - MyFlowHub-MetricsNode（亮度下行控制）

> Worktree：`d:\project\MyFlowHub3\worktrees\feat-metricsnode-brightness-control`  
> 分支：`feat/metricsnode-brightness-control`  
> 日期：2026-03-02  
>
> 本 workflow 目标：在既有 VarStore 子协议与 bindings 机制上，补齐 **亮度变量的“下行写入→执行”**（Windows + Android）。

---

## 1. 需求分析

### 1.1 目标

- 当其它节点对 MetricsNode 的亮度变量执行 `set(owner=<MetricsNodeNodeID>, name=<绑定的var>)` 后：
  - **Windows**：同步调整“主显示器”亮度；
  - **Android**：同步调整系统亮度（`Settings.System.SCREEN_BRIGHTNESS`）。
- 仍遵循既有映射：`metric (brightness_percent)` ↔ `bindings_json.var_name`（默认 `sys_brightness_percent`，但允许用户自定义）。

### 1.2 范围

- 必须：
  - Go（core）：VarStore `notify_set` 下行路由新增 `brightness_percent` → 控制动作队列；
  - Windows：消费控制动作并执行亮度设置；
  - Android：消费控制动作并执行亮度设置；补齐所需权限声明；
  - 回归：Go 单测/编译通过；Android `assembleDebug` 通过。
- 可选：
  - Android：若缺少“修改系统设置”授权（`canWrite=false`）时的 UI 引导（本轮默认仅 best-effort 执行 + 不崩溃）。
- 不做：
  - 不新增/修改 `metrics.bindings_json` schema（不加字段）。
  - 不做市场/插件系统等更大范围扩展。

### 1.3 使用场景

- 用户在 `MyFlowHub-Win` 的 VarPool 中将 `sys_brightness_percent` 写入 `30`：
  - Win 机亮度变为约 30%；
  - Android 机亮度变为约 30%（已授权前提下）。

### 1.4 功能需求

- 下行入口：继续使用 VarStore 子协议 `notify_set`（owner 下行通知）。
- 值解析：
  - 支持整数文本（如 `0`、`50`、`100`、`200`）；非数字视为无效并忽略；
  - clamp 到 `0~100` 后执行（与音量控制行为一致）。
- 失败处理：
  - Windows/Android 执行失败不影响其它指标上报；
  - 失败仅记录日志/吞掉异常（Android 侧），后续由采集上报纠偏为“真实值”。

### 1.5 非功能需求

- 性能：动作队列保持“同一 metric 仅保留最新值”，避免频繁写入造成抖动。
- 稳定性：不可用平台能力（外接显示器不支持、Android 未授权等）应 graceful degrade。
- 安全：只接受与本节点 owner 匹配的下行写入（沿用现有校验逻辑）。

### 1.6 输入输出

- 输入：VarStore `notify_set`（`name` 为 bindings 的 `var_name`，`value` 为期望亮度百分比）。
- 输出：
  - Windows：调用 Monitor brightness API 设置；
  - Android：写入 `Settings.System.SCREEN_BRIGHTNESS`。

### 1.7 边界异常

- Windows：主显示器不支持亮度设置（API 调用失败）→ 记录告警，不崩溃。
- Android：未授予“修改系统设置”权限或 ROM 限制 → best-effort，失败不崩溃。

### 1.8 验收标准

- 在 MetricsNode 已登录且 Reporting 开启时：
  - 写入 `sys_brightness_percent=30` → 对应设备亮度发生变化；
  - 写入 `sys_brightness_percent=200` → 实际亮度按 100 执行（clamp），随后 VarStore 值也会被采集上报纠偏为 `100`；
  - 写入非法值（如 `abc`）→ 不执行，不崩溃。
- 回归命令通过：
  - Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
  - Go（Windows 模块）：`cd windows/frontend; npm ci; npm run build` 后 `cd ..; GOWORK=off go test ./... -count=1 -p 1`
  - Go（nodemobile）：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
  - Android：`cd android; ./gradlew :app:assembleDebug`

### 1.9 风险

- Android `WRITE_SETTINGS` 属于特殊授权：未引导情况下可能执行失败（本轮先保证链路完整与不崩溃）。
- Windows 亮度 API 设备差异较大：按 best-effort + 日志可观测处理。

---

## 2. 架构设计（分析）

### 2.1 总体方案

沿用已存在的“变量既是状态也是命令”机制：

1) 其它节点写入 VarStore（owner 指向 MetricsNode 的 NodeID）  
2) MetricsNode 在 `notify_set` 中识别亮度变量（通过 bindings 反查 metric）  
3) 入队控制动作（`brightness_percent`）  
4) 平台侧消费动作并执行：  
   - Windows：Go 控制 worker 直接调用 WinAPI 设置亮度  
   - Android：`NodeService` 轮询 actions 并写系统亮度设置

### 2.2 模块职责

- `core/runtime/varstore_inbound.go`：下行通知解析 + 值校验/clamp + 入队控制动作
- `core/runtime/control_worker_windows.go`：Windows 侧消费动作并执行
- `core/actuator/*`：封装 Windows 亮度执行细节（避免 runtime 直接堆 WinAPI 调用）
- `android/.../NodeService.kt`：Android 侧消费动作并执行（Settings 写入）
- `android/.../AndroidManifest.xml`：声明所需权限

### 2.3 数据/调用流（简图）

`VarStore notify_set` → `Runtime.handleVarStoreNotifySet` → `controlQ.Enqueue("brightness_percent","N")` →  
Windows：`controlWorker` → `actuator.SetPrimaryMonitorBrightnessPercent(N)`  
Android：`NodeService.dequeueActions()` → `applyControlActions()` → `Settings.System.putInt(..., raw)`

### 2.4 接口草案（不改协议）

- 继续使用：
  - VarStore：`notify_set`
  - 控制动作：`ControlAction{metric,value}`（JSON）
  - Android：`nodemobile.DequeueActions()`（已存在）

### 2.5 错误与安全

- 只处理 owner 匹配的下行写入（沿用现有逻辑）。
- 执行失败：Windows 记录 warn；Android runCatching 吞掉异常；由采集上报纠偏 VarStore 值。

### 2.6 性能与测试策略

- 性能关键点：
  - actionQueue 按 metric 覆盖，避免重复写入；
  - Android 控制轮询仅在 actions 非空时才执行写入。
- 测试策略：
  - Go：补充 `varstore_inbound` 对 `brightness_percent` 的路由单测（验证 clamp + 入队）。
  - Android：`assembleDebug` 确保编译与权限声明正确。

### 2.7 可扩展性设计点

- 后续新增更多“可控”指标（如亮度/网络开关等）时：
  - 只需在 `handleVarStoreNotifySet` 增加 metric 分支，并在平台侧实现执行即可。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：worktree/分支正确，且工作区干净。
- 验收：
  - `git status --porcelain` 为空。
- 回滚点：
  - 删除 worktree + 删除分支即可回滚（未改 main）。

### T1 - Go core：新增 brightness 下行路由

- 目标：`sys_brightness_percent`（或用户自定义绑定）被写入时，入队 `brightness_percent` 控制动作。
- 涉及文件：
  - `core/runtime/varstore_inbound.go`
  - `core/runtime/varstore_inbound_test.go`（新增用例）
- 验收：
  - `value=200` 入队为 `100`；
  - `value=abc` 不入队；
  - owner 不匹配不处理。
- 测试点：
  - `GOWORK=off go test ./... -count=1 -p 1`
- 回滚点：
  - 回滚上述文件修改即可。

### T2 - Windows：执行亮度控制

- 目标：Windows 控制 worker 能执行 `brightness_percent` 动作并设置主显示器亮度。
- 涉及文件：
  - `core/runtime/control_worker_windows.go`
  - `core/actuator/brightness_windows.go`（新增）
- 验收：
  - 动作入队后，系统亮度发生变化；
  - 执行失败仅 warn，不影响其它动作与上报。
- 测试点：
  - `GOWORK=off go test ./... -count=1 -p 1`（包含 windows build）
- 回滚点：
  - 回滚控制分支与 actuator 文件。

### T3 - Android：执行亮度控制

- 目标：Android `NodeService` 能消费 `brightness_percent` 动作并写入系统亮度。
- 涉及文件：
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
  - `android/app/src/main/AndroidManifest.xml`
- 验收：
  - 写入后系统亮度变化（已授权前提）；未授权时不崩溃。
- 测试点：
  - `cd android; ./gradlew :app:assembleDebug`
- 回滚点：
  - 回滚 Kotlin 与 manifest 修改。

### T4 - 回归验证（端到端）

- 目标：Win 端通过 VarPool 写入亮度变量，Windows/Android 执行生效。
- 步骤：
  - 启动：`pwsh -File d:\\project\\MyFlowHub3\\scripts\\run-dev.ps1 -WaitServer`
  - 在 `MyFlowHub-Win` 写入 `sys_brightness_percent=30/200`
  - 观察本机亮度变化 + VarStore 值纠偏（clamp）。
- 回滚点：
  - 回滚分支提交即可。

### T5 - Code Review（阶段 3.3）+ 归档（阶段 4）

- Code Review 清单：
  - 需求覆盖、架构合理性、性能风险、稳定性与安全、测试覆盖
- 归档输出：
  - `docs/change/YYYY-MM-DD_metricsnode-brightness-control.md`

