# 2026-02-28 - MetricsNode MVP（Windows + Android：电量/音量 → VarStore + Devices Config）

## 变更背景 / 目标

当前 MyFlowHub 仅实现了 Auth / VarStore / Management 等子协议，但缺少一个“上报者节点”应用来把本机指标暴露到变量池（VarStore）。

本次 MVP 目标：

- 提供一个普通客户端节点（node）应用：
  - Windows：独立 Wails App（与 `MyFlowHub-Win` 技术栈一致）
  - Android：独立 App + 前台服务（Foreground Service）常驻上报
- 采集并上报指标：
  - 电量：`battery_percent` → 默认变量 `sys_battery_percent`（无电池写 `-1`）
  - 音量：`volume_percent` → 默认变量 `sys_volume_percent`
  - 静音：`volume_muted` → 默认变量 `sys_volume_muted`（`1/0`）
- 通过 Devices Config（Management `config_list/get/set`）远程配置绑定与默认可见性。

## 具体变更内容

### 新增

- Go Core（`core/`）
  - `core/runtime`：连接、Auth（register/login）、指标上报开关、Management config_* 请求处理、配置落盘与热更新
  - `core/metrics`：Windows 采集（电量/音量）与差量触发
  - `core/varstore`：VarStore `set`（本地校验变量名、默认 public、仅变化时发送）
  - `core/configstore`：MapConfig 风格的 string KV 落盘（JSON）存储
- Windows Wails UI（`windows/`）
  - 最小 UI：Hub addr / DeviceID / NodeID、Connect/Disconnect、Register/Login、Start/Stop Reporting、状态与指标展示
- Android App（`android/`）
  - Compose UI：配置 addr/device_id、启动/停止前台服务、状态展示
  - `NodeService`：前台服务中启动 Go Runtime，并采集电量/音量（STREAM_MUSIC）推送到 Go（差量上报）
- Gomobile 绑定模块（`nodemobile/`）
  - 作为 AAR 产物的 Go 导出 API：`Start/Stop/Status/Update*`
  - `gomobile_deps.go`：保证 `gomobile bind` 的 module graph 稳定
- AAR 构建脚本（`scripts/build_aar.ps1`、`scripts/build_aar.sh`）

### 修改

- `android/app/build.gradle.kts`：可选依赖 `android/app/libs/myflowhub.aar`（无 AAR 自动回退 StubBridge）
- `windows/frontend/src/App.vue`：替换为 MetricsNode UI

### 删除

- `windows/frontend/src/components/HelloWorld.vue`：移除模板示例（避免 TS 编译错误）

## plan.md 任务映射

- T0：仓库脚手架与最小闭环（Go module + Wails + Android scaffold）
- T1：Go Core 连接 + Auth 状态机（register/login，密钥落盘）
- T2：指标采集与 VarStore 上报（Windows 轮询；Android 外部注入；差量 set）
- T3：Devices Config 远程配置（Management config_list/get/set；bindings_json 校验；配置落盘与热更新）
- T4：Windows UI（连接/登录/状态/开关上报）
- T5：Android App + 前台服务 + gomobile（桥接 + 指标采集）
- T6：关键逻辑单测（bindings_json 校验、变量名校验）

## 关键设计决策与权衡

- **Android 采用 gomobile AAR**：避免 Kotlin 侧重写协议栈；同时保留 StubBridge 以便无 AAR 也可编译/启动 UI。
- **差量上报**：按变量名维度做去重（Value + Visibility），只在变化时发送 `varstore:set`，避免无意义 I/O。
- **配置化绑定**：用 `metrics.bindings_json`（JSON 数组字符串）承载可扩展绑定；`config_set` 时严格校验（JSON/metric/var_name），防止坏配置落盘。
- **Windows 指标采集策略**：电量 30s 轮询、音量 1s 轮询；优先工程可落地与稳定（后续可升级为事件回调/可配置频率）。
- **Management 请求处理位置**：在 SDK await 的 `onUnmatchedFrame` 中直接处理并回包；config_set 落盘会占用读循环线程（低频场景可接受，后续可异步化）。

## 测试与验证

### Go 单测

```powershell
Set-Location d:\project\MyFlowHub3\worktrees\metricsnode-mvp\MyFlowHub-MetricsNode
$env:GOWORK='off'
go test ./... -count=1 -p 1
```

### Windows 冒烟（端到端）

1) 构建：

```powershell
Set-Location d:\project\MyFlowHub3\worktrees\metricsnode-mvp\MyFlowHub-MetricsNode\windows
$env:GOWORK='off'
wails build -nopackage
```

2) 运行 `windows/build/bin/windows.exe`
3) UI 中：
   - `Connect` 到 Hub（如 `127.0.0.1:9000`）
   - `Register`（首次）或 `Login`（已有 NodeID）
   - `Start Reporting`
4) 在另一台/另一个客户端（如 `MyFlowHub-Win` → VarPool）读取：
   - owner = 本节点 node_id
   - `sys_battery_percent`
   - `sys_volume_percent`
   - `sys_volume_muted`

### Devices Config 验证（远程改绑定）

在 `MyFlowHub-Win` 的 Devices 页面编辑该节点 Config：

- `metrics.bindings_json` 示例：

```json
[{"metric":"battery_percent","var_name":"sys_battery_percent"}]
```

- `metrics.visibility_default`：`public` 或 `private`

保存后应立即生效（新绑定会用当前指标值立刻补发一次）。

### Android 冒烟（端到端）

1) 构建 AAR（需本机具备 Android SDK/NDK/JDK，且 gomobile 可发现）：

```powershell
Set-Location d:\project\MyFlowHub3\worktrees\metricsnode-mvp\MyFlowHub-MetricsNode
.\scripts\build_aar.ps1 -Target android/arm64 -JavaPkg com.myflowhub.gomobile -OutFile android/app/libs/myflowhub.aar
```

2) 打开 `android/` 工程，安装运行 App
3) 配置 addr/device_id → `Start Service`
4) 观察前台通知与 App 内状态；并在其它客户端读取上述 VarStore 变量。

## 潜在影响与回滚方案

- 影响：
  - Windows：音量采集引入 CoreAudio COM 依赖（go-ole/go-wca）
  - Android：需要 gomobile AAR 才能启用真实上报（无 AAR 自动回退 stub，不影响编译）
- 回滚：
  - 回滚本分支对应提交即可；核心功能集中在 `core/`、`windows/`、`android/`、`nodemobile/`。

