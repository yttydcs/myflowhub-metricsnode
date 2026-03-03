# Plan - MyFlowHub-MetricsNode（Windows + Android：两页 UI + 指标设置页）

> Worktree：`d:\project\MyFlowHub3\worktrees\feat-metricsnode-settings-ui`  
> 分支：`feat/metricsnode-settings-ui`  
> 日期：2026-03-03  
>
> 本 workflow 目标：为 MetricsNode（Windows/Wails + Android/Compose+Service）实现“连接页 + 设置页”两页 UI，并在设置页提供 **声明式** 的指标配置（enabled / writable / var_name）。配置通过现有 Management `config_list/get/set`（devices config）持久化与远程编辑。

---

## 0. 当前状态

- Windows：Wails 单页 UI（Bootstrap/Auth/StartReporting + metrics dump）。
- Android：Compose 单页（Start/Stop Service）+ `NodeService` 后台采集与执行。
- Go runtime：
  - runtime config 已支持 `metrics.bindings_json` / `metrics.visibility_default` / `metrics.battery.no_battery_value`
  - Management 子协议已支持 `config_list/get/set` 写入 runtime_config.json
  - VarStore inbound 已支持 volume/brightness/flashlight 控制与只读纠偏（当前“可写限制”尚未按配置化）
- 约束：
  - `enabled=false` 必须停止采集/轮询（Windows collectors 与 Android Service 都要做到“真正不采集”）。
  - 允许全关（bindings 允许为空）。

---

## 1. 需求分析（已确认）

### 1.1 目标

- Windows 与 Android 都改成 2 个界面：
  - **连接页**：`Connect / Register / Login / StartReporting` 四步按钮。
  - **设置页**：以列表展示所有指标，并配置：
    - `enabled`：是否启用（控制上报 + 下行映射）；`enabled=false` 停采集/轮询；UI 显示 `-`。
    - `writable`：仅对可控指标提供；`false` 时收到写入不执行控制，并纠偏回真实值。
    - `var_name`：绑定到 VarStore 的变量名；输入 debounce 保存；自动立即生效。
- Android 连接页增加 **Stop（Stop All）**：停止上报、断开、停止前台服务并退出采集。
- 配置存储：使用 runtime config（devices config）保存；选择 **A：单 Key + JSON**；key 名为 `metrics.settings_json`。

### 1.2 不做

- 不做“市场/扩展包”。
- 不新增协议字段（仍使用既有 VarStore/Management 子协议）。

### 1.3 验收标准

- Windows/Android：连接页能完成四步操作并启动上报；Android Stop All 停止前台服务并断开。
- 设置页：变更 enabled/writable/var_name 会自动保存并立即生效。
- enabled=false：对应指标不再采集/轮询；设置页显示 `-`。
- 全关：允许保存成功（bindings 为空不报错）。

---

## 2. 架构设计（分析）（已确认）

### 2.1 总体方案

- 新增声明式配置 key：`metrics.settings_json`，作为 MetricsNode 的“能力配置快照”。
- runtime 负责：校验、落盘、派生 `Bindings(仅 enabled)` 与 `WritableByMetric`，并触发配置变更通知。
- Windows collectors：接收 `enabled(metric)` + `configChanged` 信号，disabled 时阻塞等待，不轮询、不做系统调用。
- Android：
  - `NodeService` 是唯一运行载体（连接/鉴权/上报 + 采集线程 + 下行执行）。
  - UI 通过 Intent 触发四步操作；Stop 为 Stop All（StopReporting+Disconnect+停止服务）。

### 2.2 配置存储形态（A）

- key：`metrics.settings_json`
- value 示例：
  ```json
  [
    {"metric":"battery_percent","enabled":true,"var_name":"sys_battery_percent"},
    {"metric":"volume_percent","enabled":true,"writable":true,"var_name":"sys_volume_percent"},
    {"metric":"brightness_percent","enabled":false,"writable":true,"var_name":"sys_brightness_percent"}
  ]
  ```

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。
- 回滚点：无（仅检查）。

### T1 - Core：新增 settings_json + 允许空 bindings

- 目标：
  - 引入 key：`metrics.settings_json`（单 JSON）；作为 canonical 配置。
  - 允许全关（bindings 为空）。
  - 兼容：保留 `metrics.bindings_json`（读/写都支持），与 settings_json 自动互转，避免配置分裂。
- 涉及文件：
  - `core/runtime/config.go`
  - `core/runtime/config_test.go`
- 验收：
  - `config_set(metrics.settings_json)` 校验正确；enabled 全 false 可保存。
  - `metrics.bindings_json="[]"` 生效为全关。
  - 两 key 互转一致（settings → derived bindings；bindings → normalized settings）。
- 测试点：settings 解析/校验、空 bindings、互转逻辑、迁移策略（旧 config 无 settings_json 时从 bindings_json 补齐）。
- 回滚点：恢复旧 `metrics.bindings_json` 路径（不写 settings_json）。

### T2 - Core：writable=false 下行拦截与纠偏

- 目标：对可控指标实现 `writable=false`：收到 `notify_set` 不执行控制，纠偏写回真实值。
- 涉及文件：
  - `core/runtime/varstore_inbound.go`
  - `core/runtime/varstore_inbound_test.go`
- 验收：
  - `writable=false` 时不会 enqueue 控制动作；可纠偏时会写回正确值。
- 回滚点：移除 writable 校验（恢复默认可写）。

### T3 - Core：Windows collectors 按 enabled 停采集/轮询

- 目标：Windows 上 disabled 指标不轮询、不做系统调用（阻塞等待 config change）。
- 涉及文件：
  - `core/metrics/collectors_windows.go`
  - `core/metrics/collectors_other.go`（签名对齐）
  - `core/runtime/runtime.go`（传入 enabled + configChanged）
- 验收：enabled=false 时对应 loop 阻塞等待配置变化；enabled=true 时恢复采集。
- 性能点：disabled 时不 wake-up；仅 config change 或 ctx.Done 唤醒。
- 回滚点：恢复“始终轮询、仅不发布”的老模式。

### T4 - Windows/Wails：后端暴露 supported metrics + settings_json get/set

- 目标：Wails 前端可读取支持指标清单与 settings_json，并写回立即生效。
- 涉及文件：
  - `windows/app.go`
  - `core/metrics/metrics.go`（新增 capability 列表/DTO，如需）
  - `windows/frontend/wailsjs/*`（bindings 生成）
- 验收：
  - Windows 不返回 Android-only 能力（如 flashlight）。
  - settings_json 的 get/set 可用。
- 回滚点：仅保留旧连接页功能（不提供设置页）。

### T5 - Windows/Wails：前端两页 UI（连接页 + 设置页）

- 目标：
  - 两页切换（无需路由也可用 tab）。
  - 设置页：列表配置 enabled/writable/var_name；数值实时显示；disabled 显示 `-`。
  - `var_name` 保存策略：debounce 400ms + blur/回车立即提交；无效不提交并提示。
- 涉及文件：
  - `windows/frontend/src/App.vue`（拆分组件/视图）
- 验收：
  - 连接页：四步按钮与 Start/StopReporting 正常。
  - 设置页：修改后自动保存并立即生效；全关可保存。
- 回滚点：恢复单页 UI。

### T6 - Android：nodemobile 导出分步接口 + config get/set

- 目标：支持四步按钮调用；并支持读写 `metrics.settings_json`。
- 涉及文件：
  - `nodemobile/nodemobile.go`
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`
- 验收：
  - `Connect/Register/Login/StartReporting/StopAll`（或等价）可用；Status 正确更新。
  - `RuntimeConfigGet/Set(metrics.settings_json)` 可用。
- 回滚点：保留旧 `Start()` 一键启动（兼容旧行为）。

### T7 - Android：NodeService 按 settings 启停采集/轮询 + Stop All

- 目标：
  - enabled=false 必须停止对应采集/轮询（停止线程/receiver）。
  - Stop All：停止上报、断开、停止前台服务并退出采集。
- 涉及文件：
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- 验收：
  - UI 改配置后立即生效；远端 `config_set` 改配置后 1s 内生效。
  - Stop All 后通知栏消失，Status 变为未连接/未上报。
- 回滚点：恢复“启动后全部采集一直跑”的模式。

### T8 - Android：Compose UI 两页（连接页 + 设置页）

- 目标：实现连接页四步按钮 + Stop All；设置页指标列表配置并实时显示。
- 涉及文件：
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`
- 验收：两页切换；配置 debounce 保存并生效；disabled 显示 `-`。
- 回滚点：恢复原 Start/Stop Service 单页。

### T9 - 回归验证

- Go：`GOWORK=off go test ./... -count=1 -p 1`
- Windows module：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`
- nodemobile：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
- Android：`cd android; ./gradlew :app:assembleDebug`
- 备注：worktree 下运行 Wails 如遇 go.work 冲突，使用 `GOWORK=off` 环境变量（例如 PowerShell：`$env:GOWORK='off'; wails dev`）。

### T10 - Code Review（阶段 3.3）+ 归档（阶段 4）

- 归档输出：`docs/change/2026-03-03_metricsnode-settings-ui.md`
