# 2026-03-02 MetricsNode：VarStore 下行控制（亮度）

## 背景 / 目标

`MyFlowHub-MetricsNode` 已支持将本机亮度上报到 VarStore（`brightness_percent` → 默认变量 `sys_brightness_percent`），但当其它节点修改该变量时，MetricsNode 未执行“反向控制”，导致亮度变量只能作为状态，无法作为命令使用。

本次目标：

1) 支持 `sys_brightness_percent`（或用户自定义绑定的亮度变量）被修改后，MetricsNode 执行本机亮度调整（Windows + Android）。
2) 不修改 `metrics.bindings_json` schema（不新增字段），仍通过 bindings 完成 metric ↔ var_name 映射。
3) 值解析遵循既有控制语义：字符串整数 → clamp 到 `0~100` → 执行。

## 具体变更内容

### Go Core（下行路由）

- 在 VarStore `notify_set` 下行处理中新增亮度路由：
  - 通过 bindings 反查 `var_name` 对应 metric；
  - 对 `brightness_percent` 解析整数并 clamp 到 `0~100`；
  - 入队控制动作，供平台侧执行。
- 新增单测覆盖：
  - clamp（`200` → `100`）；
  - 非法值忽略；
  - owner 不匹配忽略。

涉及文件：

- `core/runtime/varstore_inbound.go`
- `core/runtime/varstore_inbound_test.go`

### Windows（执行）

- 新增 Windows 亮度执行器：设置“主显示器”亮度百分比（best-effort）。
- 控制 worker 增加 `brightness_percent` 动作消费与执行。

涉及文件：

- `core/actuator/brightness_windows.go`
- `core/runtime/control_worker_windows.go`

### Android（执行）

- `NodeService` 的控制动作执行增加 `brightness_percent`：
  - `value` 解析为 `0~100`；
  - 仅在 `Settings.System.canWrite(...) == true` 时写入 `SCREEN_BRIGHTNESS`（best-effort，不崩溃）。
- AndroidManifest 增加权限声明：`android.permission.WRITE_SETTINGS`。

涉及文件：

- `android/app/src/main/java/com/myflowhub/metricsnode/NodeService.kt`
- `android/app/src/main/AndroidManifest.xml`

## plan.md 任务映射

- T1：Go core 亮度下行路由 + 单测
- T2：Windows 亮度执行 + worker 消费
- T3：Android 亮度执行 + 权限声明
- T4：端到端手工验证步骤（见下）
- T5：Code Review + 归档

## 关键设计决策与权衡

1) **不引入新协议/字段**
   - 继续使用 VarStore 子协议的 `notify_set` 作为 owner 下行入口，保持框架自洽。

2) **metric ↔ var 的映射仍由 bindings 决定**
   - 下行写入通过 bindings 反查 metric，保证用户自定义 `var_name` 时也能正常控制。

3) **best-effort + 采集纠偏**
   - 执行失败不影响运行；后续由采集上报将 VarStore 值纠偏为真实值（例如 clamp 后的 100）。

4) **Android WRITE_SETTINGS 的现实约束**
   - 仅声明权限不足以保证生效，仍需要用户在系统设置中授予“修改系统设置”授权；
   - 本次先保证链路完整与不崩溃，未额外加入 UI 引导（可后续迭代）。

## 测试与验证

### Go（已执行）

在 worktree 根目录：

- `GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### Windows module（已执行）

- `cd windows/frontend; npm ci; npm run build`
- `cd ..; GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### nodemobile（已执行）

- `cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`

结果：通过。

### Android（已执行）

在 `android/`：

- `ANDROID_SDK_ROOT=<path-to-sdk> ANDROID_HOME=<path-to-sdk> ./gradlew :app:assembleDebug`

结果：通过。

### 端到端手工验证（建议步骤）

前置：

- MetricsNode 已连接 Hub 且完成登录（有 `node_id`），并开启 Reporting。
- 触发下行必须对 owner 写入：`set(owner=<MetricsNodeNodeID>, name=<亮度绑定变量>)`。

步骤：

1) 在任意可写 VarStore 的客户端（如 `MyFlowHub-Win` VarPool）写入：
   - `sys_brightness_percent=30`
   - `sys_brightness_percent=200`（验证 clamp→100）
2) 观察：
   - Windows / Android 设备亮度变化；
   - 随后亮度采集上报会将 VarStore 值同步为实际值（例如 clamp 后的 `100`）。

## 潜在影响与回滚方案

潜在影响：

- Windows：部分外接显示器/驱动可能不支持亮度设置，执行会失败并记录告警。
- Android：新增 `WRITE_SETTINGS` 权限声明；未授予系统授权时亮度写入会被跳过（不崩溃）。

回滚方案：

- 回滚本分支对应提交即可恢复为“仅上报亮度、无下行控制”的行为。

