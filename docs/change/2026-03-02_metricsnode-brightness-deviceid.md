# 2026-03-02 MetricsNode：新增屏幕亮度 + Device ID 默认生成

## 背景 / 目标

1) **亮度指标**：在 Windows + Android 的 MetricsNode 中新增“屏幕亮度”采集，上报到 VarStore，便于其它节点订阅与展示。  
2) **默认 Device ID**：避免用户忘记填写 `Device ID` 导致 Register/Login 失败；当本地未持久化/为空时自动生成并落盘，后续保持稳定。

## 具体变更

### 1) 新增亮度 metric（`brightness_percent`）

- 新增 metric 常量：`brightness_percent`（值为 `0~100` 字符串；不可读上报 `-1`）。
- 默认 bindings 增加：`brightness_percent` → `sys_brightness_percent`（用户仍可通过 `metrics.bindings_json` 覆盖）。
- 兼容迁移：当检测到 `metrics.bindings_json` 仍为“旧默认 3 项”（电量 + 音量/静音）时，自动升级为“新默认 4 项”（增加亮度），不覆盖用户自定义配置。

### 2) Windows：主显示器亮度采集（轮询 2s）

- `core/metrics` 增加 brightness collector：
  - 读取主显示器亮度百分比；
  - 读取失败则上报 `-1`；
  - 错误日志做最小节流（同类错误 60s 至多记录一次），避免刷屏。

### 3) Android：亮度采集（轮询 1s）+ 变化去抖

- `NodeService` 新增 brightness poller：
  - 读取 `Settings.System.SCREEN_BRIGHTNESS` 并换算为 `0~100`；
  - 值不变不重复上报；
  - 读取失败上报 `-1`。
- `NodeBridge` 增加 `updateBrightnessPercent`，并在 GoBridge 中对 `UpdateBrightnessPercent` 采用**可选反射**：旧 AAR 缺少该方法时不会导致 GoBridge 初始化失败（仅亮度更新 no-op）。

### 4) Device ID 默认生成（Windows + Android）

- Windows（Wails）：bootstrap 初始化时生成默认 `Device ID`（前缀 `win-` + 随机 hex），并写入 `windows/config/bootstrap.json`；如发现持久化为空也会兜底生成。
- Android：SharedPreferences `device_id` 为空时生成默认值（前缀 `android-` + UUID）并写回；Service 启动时同样兜底，避免无 UI 启动失败。

### 5) Android 构建可用性修复（Gradle Wrapper jars）

由于仓库顶层 `.gitignore` 全局忽略 `*.jar`，导致 `android/gradle/wrapper/*.jar` 不能被纳入版本控制，从而 `./gradlew` 无法运行。本次变更：

- 在 `.gitignore` 中对 Gradle wrapper jars 做定向反忽略；
- 补齐并纳入版本控制：
  - `android/gradle/wrapper/gradle-wrapper.jar`
  - `android/gradle/wrapper/gradle-wrapper-shared.jar`

## 任务映射（plan.md）

- T1：core 新增 brightness metric + 默认 bindings
- T2：旧默认 bindings 安全迁移
- T3/T4：Windows 亮度采集 + 默认 Device ID
- T5/T6：Android 亮度采集 + 默认 Device ID + GoBridge 可选反射
- T7：回归验证

## 关键设计决策与权衡

- **扩展点复用**：亮度作为新 metric 走既有 `metric -> bindings -> varstore` 链路，不新增 schema，最大化可扩展性与一致性。
- **迁移策略保守**：仅当 bindings 确认为“旧默认”才自动升级；避免覆盖用户自定义 bindings。
- **Android 兼容性**：GoBridge 对新导出方法采用可选反射，避免旧 AAR 导致运行期崩溃/降级为 Stub。
- **安全默认**：Device ID 采用随机生成，不包含主机名/用户信息等可能敏感数据。

## 测试与验证

- Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1`
- Go（Windows 模块）：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd ..; GOWORK=off go test ./... -count=1 -p 1`
- Go（nodemobile）：`cd nodemobile; GOWORK=off go test ./... -count=1 -p 1`
- Android：在已配置 Android SDK 的环境下：
  - 设置 `ANDROID_HOME`/`ANDROID_SDK_ROOT` 后运行 `cd android; ./gradlew :app:assembleDebug`

## 潜在影响与回滚

- 影响：
  - Windows 亮度读取依赖设备/驱动支持；不支持时持续上报 `-1`（且不会刷屏日志）。
  - Android 若未重新生成 gomobile AAR，亮度更新将 no-op（不影响已有电量/音量链路）。
- 回滚：
  - `git revert` 本次提交即可回退新增 metric、采集逻辑与默认 Device ID；Gradle wrapper jars 与 `.gitignore` 也将随回滚恢复。

