# Plan - MyFlowHub-MetricsNode（新增屏幕亮度 + Device ID 默认生成）

> Worktree：`d:\project\MyFlowHub3\worktrees\feat-metricsnode-brightness-deviceid`
> 分支：`feat/metricsnode-brightness-deviceid`
> 日期：2026-03-02
>
> 本 workflow 目标：
> 1) Windows + Android：新增屏幕亮度采集并上报到 VarStore（走既有 `metrics.bindings_json` 绑定体系）。
> 2) Windows + Android：当本地未持久化/为空时，自动生成稳定的默认 Device ID 并落盘（避免忘填导致 Auth 不可用）。

---

## 1. 需求分析

### 1.1 目标

- 新增 metric：`brightness_percent`（字符串，取值 `0~100`；不可读时上报 `-1`）。
- 默认 bindings 增加：`brightness_percent` → `sys_brightness_percent`。
- Windows：采集“主显示器”亮度（轮询 `2s`）。
- Android：采集系统亮度（轮询 `1s`）。
- Device ID 默认生成：
  - Windows：若 `windows/config/bootstrap.json` 中 `auth.device_id` 为空/不存在，自动生成并写回。
  - Android：若 SharedPreferences `device_id` 为空/不存在，自动生成并写回；Service 也需兜底生成（避免无 UI 启动失败）。

### 1.2 范围

- 必须：
  - Go（core）：新增亮度 metric 常量；运行时配置支持该 metric；默认 bindings 含亮度；对“旧默认 bindings”做安全迁移（仅当 bindings **完全等于旧默认** 时才自动升级）。
  - Windows：亮度采集 + Device ID 默认生成。
  - Android：亮度采集（Kotlin 侧）+ Device ID 默认生成（Activity + Service 兜底）+ GoMobile API 扩展（新增 `UpdateBrightnessPercent`，并以“可选反射”方式兼容旧 AAR）。
- 可选：
  - UI：当 Device ID 为空时禁用 Register/Login（理论上默认生成后不会触发，但可作为防御）。
- 不做：
  - 不实现“修改亮度变量 → 调整系统亮度”的反向控制（本轮仅采集上报）。
  - 不变更 `metrics.bindings_json` schema（不新增字段），继续用现有 Binding 机制扩展。

### 1.3 使用场景

- 用户启动 MetricsNode（Windows/Android）后无需手填 Device ID 即可完成 Register/Login 并开始上报 metrics。
- 用户在系统里调节亮度后，MetricsNode 能把最新亮度同步到 VarStore 变量（默认 `sys_brightness_percent`）。

### 1.4 功能需求

- 亮度采集：
  - Windows：读取主显示器当前亮度百分比；失败则上报 `-1`。
  - Android：读取系统亮度（0-255）并换算为百分比；失败则上报 `-1`。
  - 去抖：亮度值不变时不重复上报（Android 侧）；Runtime 侧继续使用既有差量发布去重。
- Device ID 默认生成：
  - 仅在本地未持久化/为空时生成一次；生成后持久化，后续保持稳定。
  - 生成值应为随机串（不包含主机名等可能敏感信息）。

### 1.5 非功能需求

- 性能：
  - Windows 亮度轮询 `2s`（避免高频 WinAPI 调用）。
  - Android 亮度轮询 `1s`，并做“变化才上报”。
- 稳定性：
  - 亮度读取失败不应影响其它指标采集/上报。
  - Android 若仍使用旧 gomobile AAR，不应因缺少亮度导出方法导致 GoBridge 初始化失败。

### 1.6 输入输出

- 输入：系统亮度（Windows API / Android Settings）。
- 输出：`brightness_percent` metric → bindings → VarStore `set`（默认 var：`sys_brightness_percent`）。

### 1.7 边界异常

- Windows 外接显示器/驱动不支持亮度读取：持续上报 `-1`。
- Android ROM/权限限制导致读取失败：上报 `-1`。
- 读到的值超范围：clamp 到 `0~100`；仅 `-1` 作为不可读哨兵值保留。

### 1.8 验收标准

- Windows：
  - 亮度变化后，MetricsNode UI 的 `Metrics` 中 `brightness_percent` 会变化；
  - VarStore 中默认变量 `sys_brightness_percent` 同步变化。
- Android：
  - 亮度变化后，上报 `brightness_percent` 并同步到 VarStore（默认 `sys_brightness_percent`）。
- Device ID：
  - Windows：首次启动未配置时自动生成并在 UI 中可见；不手填也可完成 Register/Login。
  - Android：首次启动未配置时自动生成并在 UI 中可见；Service 启动不会因空 device_id 失败。
- 回归命令通过：
  - Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
  - Go（Windows 模块）：`cd windows/frontend; npm ci; npm run build` 后 `cd ..; GOWORK=off go test ./... -count=1 -p 1`
  - Go（nodemobile）：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
  - Android：`cd android; ./gradlew :app:assembleDebug`

### 1.9 风险

- Windows 亮度读取存在设备差异；按约定 `-1` 降级，并尽量避免高频刷屏日志。
- Android 反射调用兼容性：必须确保亮度方法为“可选”，旧 AAR 仍可正常使用既有功能。

---

## 2. 架构设计（分析）

### 2.1 总体方案（含选型理由）

- 把“亮度”作为一个新的 metric 加入现有链路（最小侵入，扩展性最好）：
  - 平台采集（Windows：Go collector；Android：Kotlin poller）
  - 写入 Runtime：`handleMetricUpdate(metric,value)`
  - 通过 bindings 映射到 VarStore：`publishVar(owner=node_id, target=hub_id, name=var_name, value, visibility)`
- Device ID 默认生成放在“应用层”完成（Windows bootstrap / Android prefs），不改变 auth 子协议与 server 行为。

### 2.2 模块职责

- `core/metrics`：定义 metric 常量；Windows 平台采集（亮度/电量/音量）。
- `core/runtime`：运行时配置（bindings/visibility）；接收 metric 更新并差量发布到 VarStore。
- `windows`：Wails App：bootstrap（addr/device_id）持久化、Auth 操作、状态展示。
- `nodemobile`：gomobile 导出：Android 通过 JNI 将亮度/音量/电量推入 Go Runtime。
- `android/app`：后台 Service 采集与推送；前台 Activity 提供 addr/device_id 配置与启动/停止。

### 2.3 数据 / 调用流

**Windows**

1) `windows/app.go` 启动时初始化 `runtime.New("config")` 与 `bootstrap.json` store  
2) 若 `auth.device_id` 为空 → 生成默认 ID 并 `boot.Set(...)`  
3) `StartReporting()` → `metrics.StartPlatformCollectors()` → `brightnessLoop()`  
4) `emit("brightness_percent", "N")` → Runtime `handleMetricUpdate` → VarStore `set`

**Android**

1) Activity/Service 读取 prefs；若 `device_id` 为空 → 生成并写回  
2) Service 轮询 `Settings.System.SCREEN_BRIGHTNESS` → 计算百分比  
3) `bridge.updateBrightnessPercent(percent)` → GoMobile `UpdateBrightnessPercent` → Runtime `UpdateMetric` → VarStore `set`

### 2.4 接口草案

- metric：`core/metrics.MetricBrightnessPercent = "brightness_percent"`
- 默认 bindings（`metrics.bindings_json`）新增：
  - `{ "metric": "brightness_percent", "var_name": "sys_brightness_percent" }`
- Android GoMobile 导出（`nodemobile`）新增：
  - `UpdateBrightnessPercent(percent string)`

### 2.5 错误与安全

- 亮度采集失败：emit `-1`（符合需求）；日志应可观测但避免高频刷屏（可用简单节流）。
- Device ID：随机生成（不使用 hostname / 用户名 / ANDROID_ID），降低隐私暴露风险。
- Android：亮度导出方法使用“可选反射”；即便旧 AAR 不含该方法，也不影响 GoBridge 其它能力。

### 2.6 性能与测试策略

- 性能关键点：
  - Windows 亮度轮询 `2s`，避免高频 WinAPI + 物理监视器枚举。
  - Android 亮度轮询 `1s`，并在 Kotlin 侧“变化才上报”。
  - Runtime 已有 `lastPublished` 去重，避免重复 VarStore set。
- 测试策略：
  - Go：`go test` 编译 +（必要时）补充 bindings 迁移单测。
  - Windows：需要 `frontend/dist` 才能 `go test ./...`，按既有流程先 `npm run build`。
  - Android：`assembleDebug` 保证 Kotlin 编译与反射兼容。

### 2.7 可扩展性设计点

- 新增更多系统指标时：只需
  1) 在 `core/metrics` 增加 metric 常量；
  2) 在 `core/runtime/config.go` 放行该 metric（`supportedMetric` + 默认 bindings）；
  3) 在平台采集层采集并调用 `emit/UpdateMetric`。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：确保分支与 worktree 正确，且变更可控。
- 验收：
  - 在本 worktree：`git status --porcelain` 为空。

### T1 - core：新增 brightness metric + 默认 bindings

- 目标：亮度 metric 能被 bindings 识别并可默认发布到 `sys_brightness_percent`。
- 涉及文件：
  - `core/metrics/metrics.go`
  - `core/runtime/config.go`
- 验收：
  - `supportedMetric("brightness_percent")==true`
  - `defaultBindings()` 含亮度绑定。

### T2 - core：旧默认 bindings 安全迁移（可选加单测）

- 目标：升级后“未自定义 bindings 的老用户”自动获得亮度绑定；已自定义的用户不被覆盖。
- 涉及文件：
  - `core/runtime/config.go`
  - （可选）`core/runtime/config_test.go`
- 验收：
  - 当 `metrics.bindings_json` 完全等于“旧默认 3 项”时自动升级到“新默认 4 项”；
  - 其它情况不改动用户 bindings。

### T3 - Windows：亮度采集（主显示器）+ 失败 `-1`

- 目标：Windows 可读到亮度则上报 `0~100`；不可读上报 `-1`。
- 涉及文件：
  - `core/metrics/collectors_windows.go`
- 验收：
  - 亮度读取失败不会 panic，且 emit `-1`；
  - 轮询间隔 `2s`。

### T4 - Windows：Device ID 默认生成并持久化

- 目标：首次启动未配置 Device ID 时自动生成并写入 bootstrap。
- 涉及文件：
  - `windows/app.go`
- 验收：
  - `bootstrap.json` 中 `auth.device_id` 不为空；
  - UI `Device ID` 输入框默认显示该值。

### T5 - Android：亮度采集 + Device ID 默认生成（Activity + Service 兜底）

- 目标：Android 在无手工输入时也能稳定启动并上报亮度。
- 涉及文件：
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- 验收：
  - prefs `device_id` 为空时会生成并写回；
  - Service 从 intent 取不到 device_id 时会兜底读取/生成；
  - 亮度变化触发上报（无变化不重复上报）。

### T6 - Android：GoMobile API 扩展 + 可选反射兼容

- 目标：新增亮度导出方法，同时不破坏旧 AAR 的 GoBridge 初始化。
- 涉及文件：
  - `nodemobile/nodemobile.go`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
- 验收：
  - 亮度方法缺失时不会抛出导致 GoBridge 构造失败（仅亮度更新 no-op）。
- 备注（开发验证/交付）：
  - 若需要实际启用 GoBridge，请运行：`pwsh -File scripts/build_aar.ps1` 生成 `android/app/libs/myflowhub.aar`。

### T7 - 回归验证

- Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
- Go（Windows 模块）：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`
- Go（nodemobile）：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
- Android：`cd android; ./gradlew :app:assembleDebug`

### T8 - Code Review（阶段 3.3）

- 需求覆盖：亮度上报 + 默认 Device ID（Win/Android）。
- 架构合理性：沿用 metric→bindings→varstore 的扩展点，不引入新 schema。
- 性能风险：轮询频率、WinAPI 调用成本、Android 去抖。
- 稳定性与安全：失败降级 `-1`；Device ID 不含敏感信息；Android 可选反射。
- 测试覆盖：回归命令通过；（若加单测）迁移逻辑覆盖。

### T9 - 归档（阶段 4）

- 输出：`docs/change/2026-03-02_metricsnode-brightness-deviceid.md`
