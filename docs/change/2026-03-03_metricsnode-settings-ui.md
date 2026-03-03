# 2026-03-03 MetricsNode 两页 UI + 声明式指标设置（Windows + Android）

## 变更背景 / 目标

MetricsNode 现状已实现 VarStore/Management 子协议与若干指标采集/控制，但缺少可供用户配置的应用层界面与“声明式配置”能力。

本次变更目标：

- Windows（Wails）与 Android（Compose + Service）均实现 **两页 UI**：
  - 连接页：`Connect / Register / Login / StartReporting` 四步按钮（Android 额外提供 Stop All）。
  - 设置页：以列表配置每个指标的 `enabled / writable / var_name`，并实时显示数值；`enabled=false` 显示 `-`。
- 配置通过 Management `config_list/get/set`（devices config，落盘到本地 `runtime_config.json`）持久化。
- `enabled=false` 必须 **停止采集/轮询**（Windows collectors 与 Android Service 都要真正停掉系统调用/线程）。
- `writable=false`：收到 `notify_set` 不执行控制动作，并将变量纠偏回当前真实值。

## 具体变更内容

### Core（Go runtime）

- 新增 runtime config key：`metrics.settings_json`（canonical）。
  - 支持 `enabled / writable / var_name`。
  - 允许“全关”（所有指标 enabled=false），并保持设置列表“全量输出”（UI 永远能拿到完整条目）。
  - `metrics.bindings_json` 保留兼容：`settings_json` 派生 `bindings_json`；外部改 `bindings_json` 会自动转换为 `settings_json`，避免配置分裂。
- 变更通知：
  - runtime 每次 `reloadRuntimeConfig` 后广播 configChanged（close+recreate channel），供 Windows collectors 阻塞等待。
- Windows collectors：
  - `enabled=false` 时停止 ticker 并阻塞等待 configChanged；不轮询、不做系统调用。
- VarStore inbound：
  - 使用 `{var_name -> (metric,writable)}` 映射处理下行。
  - 对可控指标实现 `writable=false`：不 enqueue 控制动作，改为纠偏写回真实值（与只读指标同类逻辑）。
- Windows 亮度写入（WMI）健壮性：
  - `WmiMonitorBrightnessMethods` 写入改为“逐实例尝试直到成功”，并扩大 VARIANT 参数组合覆盖，提升兼容性。

### Windows（Wails）

- 后端新增：
  - `MetricsSettingsGet/Set`：读取/写入 `metrics.settings_json`（通过 runtime config）。
- 前端（Vue）改为两页：
  - Connect 页：四步按钮 + 状态展示。
  - Settings 页：指标列表（enabled/writable/var_name + 当前值），`var_name` 400ms debounce + blur/enter 立即保存；本地校验 var_name 与 enabled 重名冲突。
- 更新 `wailsjs` 类型声明，保证 `npm run build` 可通过。

### Android（Compose + Service）

- nodemobile（Go mobile bindings）新增分步 API：
  - `Init/Connect/Disconnect/Register/Login/StartReporting/StopReporting/StopAll`
  - `MetricsSettingsGet/Set`（读写 `metrics.settings_json`）
  - 保留 `Start/Stop` 作为兼容路径。
- NodeService：
  - 新增 Intent actions：`CONNECT/REGISTER/LOGIN/START_REPORTING/STOP_ALL`（保留 START/STOP 兼容）。
  - Stop All：停止 observers、停止上报、断开并停止前台服务、`stopSelf()`。
  - settings watcher：轮询 `metrics.settings_json`，变更后立即应用。
  - enabled gating：按 settings 真正启动/停止对应线程/receiver；system poller 内部按子指标启用状态跳过 CPU/Mem/Net 的系统调用。
- MainActivity：
  - 两页 UI（Connect/Settings），Connect 页按钮触发 Service actions；Settings 页读取并写入 `metrics.settings_json`，并实时显示指标值（disabled 显示 `-`）。

## plan.md 任务映射

- T1：Core settings_json + 空 bindings（全关）支持
- T2：Core writable=false 下行拦截与纠偏
- T3：Windows collectors 按 enabled 停采集/轮询
- T4–T5：Windows 后端接口 + 两页 UI（Connect/Settings）
- T6：nodemobile 分步接口 + settings get/set
- T7：Android NodeService 按 settings 启停采集 + Stop All
- T8：Android 两页 UI（Connect/Settings）

## 关键设计决策与权衡

- `metrics.settings_json` 作为 canonical：避免分散配置与“配置漂移”，并能承载未来更多能力字段。
- 兼容 `metrics.bindings_json`：不破坏旧工具链/脚本，且通过互转保持一致性。
- Windows collectors 使用“阻塞等待 configChanged”：disabled 时 0 wake-up（除 config change / ctx.Done），满足性能与“真正停止采集”要求。
- Android 使用 settings watcher 轮询：避免跨语言回调复杂度；代价是 1s 级别的配置生效延迟（满足需求）。

## 测试与验证方式 / 结果

已在本 worktree 执行：

- Core：`GOWORK=off go test ./... -count=1 -p 1` ✅
- nodemobile：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1` ✅
- Windows 前端：`cd windows/frontend; npm ci; npm run build` ✅
- Windows 模块：`cd windows; GOWORK=off go test ./... -count=1 -p 1` ✅（依赖已生成 `frontend/dist`）

Android：

- `android/app/libs/myflowhub.aar` 需要在本地具备 Android SDK/NDK 后通过 `scripts/build_aar.ps1` 生成（本仓库 `.gitignore` 会忽略该产物）。
- Gradle 构建需要配置 `ANDROID_HOME` 或 `android/local.properties (sdk.dir=...)`。

## 潜在影响

- Windows/Android UI 依赖 `metrics.settings_json`；若 runtime_config.json 被手动破坏，UI 保存会提示错误并回退到默认配置（runtime 侧容错）。
- Android 运行必须重新生成 AAR（包含新增 Go 导出方法），否则会退回 StubBridge（功能不可用）。

## 回滚方案

- 回滚 Core：恢复仅使用 `metrics.bindings_json` 的逻辑，移除 settings_json 与 enabled/writable gating。
- 回滚 UI：恢复 Windows/Android 单页 UI 与旧的 Start/Stop Service 交互。
- Android：保留 `Start/Stop` 旧入口可作为临时回退路径。

