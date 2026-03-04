# Plan - MyFlowHub-MetricsNode（Android UI 对齐 Windows + 修复 debug-latest 点击无响应）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-android-ui`  
> 分支：`fix/metricsnode-android-ui`  
> 日期：2026-03-04  
>
> 本 workflow 目标：让 Android 版 UI 与 Windows 版（Wails）在信息结构与交互上对齐，并修复 `debug-latest` APK 上「Connect 点击无反应 / Settings 仅有 Reload」的问题；同时 Windows 连接页按钮去掉 `1/2/3` 编号。

---

## 0. 当前状态

- `debug-latest` Android APK 现象：
  - Connect 页面按钮点击后无明显状态变化
  - Settings 页面仅显示 `Reload`
- Windows：连接页按钮仍包含 `1. / 2. / 3.` 编号
- 根因定位（已复现）：
  1) **gomobile 绑定缺失 `login(...)`**：当前 `nodemobile.Login(deviceID string, nodeID uint32)` 的 `uint32` 参数无法被 gomobile 导出，导致生成的 `com.myflowhub.gomobile.nodemobile.Nodemobile` **没有 `login` 方法**；而 Android 端 `GoNodeBridge` 构造时强制反射 `Login`，因此初始化必然失败 → 自动降级为 `StubNodeBridge` → Connect/Settings 全部“无反应”。
  2) **AAR 单 ABI（arm64-v8a）**：即便修复 login 绑定，如果 AAR 仍只包含 `arm64-v8a` 的 `libgojni.so`，在 **非 arm64 设备**（`armeabi-v7a`）或 **x86_64/x86 环境** 上仍会触发 `UnsatisfiedLinkError` → 降级 stub。
  3) **CI 允许 AAR 构建失败**：当前 workflow 的 AAR 步骤是 `continue-on-error: true`，会导致发布一个“可安装但只能 stub”的 `debug-latest` APK。

---

## 1. 需求分析（已确认）

### 1.1 目标

1) Android：Connect / Settings 两页信息结构对齐 Windows 版：
   - Connect：Bootstrap / Auth / Reporting 三块 + 状态展示（Connected/Reporting 等）
   - Settings：表格化展示（Metric / Var Name / Value / Enabled / Writable）+ 顶部 `Reload + Ready`
2) Android：`debug-latest` APK 上按钮点击必须有可见反馈（状态变化/错误提示），Settings 必须显示完整指标列表。
3) Windows：连接页按钮去除 `1/2/3/4` 前缀。
4) Win/Android：本地与 CI 均可成功编译。

### 1.2 不做

- 不做市场/插件系统等扩展议题
- 不引入签名、上架配置等发布流程改动

### 1.3 验收标准

- Android（安装 `debug-latest`）：
  - Connect 点击后，`Connected/Reporting/LastError` 至少一项可更新（有错误则展示错误）
  - Settings 默认展示全量指标行，且可修改 Enabled/Writable/VarName 并自动保存生效
  - 若运行在不支持的 ABI/缺少 AAR 的情况下，必须在 UI 明确提示（而非静默无反应）
- Windows：Connect 页按钮文案无编号（`Connect/Register/Login/Start Reporting`）。
- CI：Win + Android job 成功；`debug-latest` release 可下载 EXE/APK（AAR 仍尽量生成）。

---

## 2. 架构设计（分析）

### 2.1 总体方案

- **根因修复（login 绑定缺失）**：将 `nodemobile.Login` 的签名改为 gomobile 可导出的类型（建议 `int64` → Java `long`），确保 AAR 生成 `login(...)`。
- **兼容更多设备（多 ABI AAR）**：将 gomobile AAR 默认构建目标扩展为多 ABI（`android/arm64,android/arm,android/amd64,android/386`），覆盖 arm64/armv7/x86_64/x86。
- **CI 保真（禁止 stub 发布）**：CI 中 AAR 构建失败直接失败；并校验 AAR 内必须包含四种 ABI 的 `libgojni.so`。
- **可诊断性增强**：Android UI 显示当前 bridge 类型（Go/Stub）与加载失败原因（例如缺少方法/ABI 不匹配），避免“按钮没反应”。
- **UI 对齐**：Android 使用 Material3 + Compose，以 Windows 版的页面结构为蓝本实现两页布局与 Settings 表格布局。

### 2.2 模块职责

- Android UI（Compose）：表单输入、触发 NodeService action、展示 NodeState/设置列表、即时保存配置。
- NodeService：前台服务承载采集/轮询与执行（控制类指标），并通过 Go runtime 更新状态。
- Go runtime（nodemobile/core）：真实连接/注册/登陆/上报、settings_json 持久化与校验。
- 构建脚本（scripts/build_aar.*）：生成可被 Android app 引用的 AAR（含多 ABI 的 native libs）。

### 2.3 数据/调用流（摘要）

1) UI 点击 Connect/Register/Login/StartReporting → `startForegroundService` 发送 action → NodeService 调用 Go bridge 静态方法
2) UI 每秒轮询 `bridge.status()` → 展示 Connected/Reporting/Auth/LastError/metrics snapshot
3) Settings 页面：`metricsSettingsGet()` 拉取 settings_json → 编辑后 `metricsSettingsSet()` → NodeService watcher/poller 按设置启停

### 2.4 错误与安全

- Go bridge 加载失败（class/so 缺失、ABI 不匹配）必须暴露到 UI；不再静默降级。
- Settings 保存仍依赖 Go runtime 的校验（重复 var_name/非法字符等）；UI 同时做轻量本地校验，减少无效请求。

### 2.5 性能与测试策略

- UI 轮询保持 1s（已有），避免更高频率消耗；Settings 保存做 400ms debounce。
- 本地验证：
  - Windows：`wails build`（`GOWORK=off`）
  - Android：生成 AAR + `./gradlew :app:assembleDebug`
- CI 验证：合并后观察 `ci` 与 `debug-latest` release 资产。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认
- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。
- 回滚点：无（仅检查）。

### T1 - 修复 gomobile Login 绑定缺失（阻断 stub 根因）
- 目标：让 AAR 生成 `login(...)`，Android 端 `GoNodeBridge` 不再因反射 `Login` 失败而降级 stub。
- 涉及文件：
  - `nodemobile/nodemobile.go`（`Login` 签名与参数校验）
  - `android/app/src/main/java/com/myflowhub/metricsnode/NodeBridge.kt`（反射签名：primitive `long`）
- 验收：
  - 本地生成 AAR 后，`com.myflowhub.gomobile.nodemobile.Nodemobile` 包含 `login(String,long)`（或等价方法）。
  - Android debug 运行时不再出现“底部全部 No 且无 LastError”的静默 stub。
- 测试点：
  - 本地执行 `scripts/build_aar.ps1` 生成 AAR，并检查生成类方法表（`javap` 或等效手段）。
- 回滚点：恢复 `Login` 原签名（不建议，会回到不可用状态）。

### T2 - gomobile AAR 多 ABI + CI 强制成功（覆盖更多设备）
- 目标：默认生成包含多 ABI 的 `myflowhub.aar`，覆盖真机/模拟器；并确保 CI 不会发布 stub APK。
- 涉及文件：
  - `scripts/build_aar.sh`
  - `scripts/build_aar.ps1`
  - `.github/workflows/ci.yml`
- 验收：AAR 解包后至少包含：
  - `jni/arm64-v8a/libgojni.so`
  - `jni/armeabi-v7a/libgojni.so`
  - `jni/x86_64/libgojni.so`
  - `jni/x86/libgojni.so`
- 测试点：
  - 本地运行脚本生成 AAR 并检查 ABI 目录齐全
  - CI：Android job 必须生成 AAR，否则直接失败（不再 `continue-on-error`）
- 回滚点：恢复默认 target 为 `android/arm64`。

### T3 - Android UI 对齐 Windows（Connect/Settings）+ 明确错误展示
- 目标：Android 两页布局与 Windows 结构一致；Settings 以表格形式展示全部指标并支持实时保存。
- 涉及文件（预期）：
  - `android/app/src/main/java/com/myflowhub/metricsnode/MainActivity.kt`（拆分或重构 Compose）
  - （如需要）新增 `android/app/src/main/java/com/myflowhub/metricsnode/ui/*.kt`
- 验收：
  - Connect：分块展示与按钮布局对齐，点击后状态/错误可见
  - Settings：`Metric/VarName/Value/Enabled/Writable` 五列 + `Reload/Ready` 顶部对齐
- 回滚点：回退 UI 改动文件。

### T4 - Windows Connect 按钮去编号
- 目标：移除 `1./2./3./4.` 前缀，保持逻辑不变。
- 涉及文件：
  - `windows/frontend/src/App.vue`
- 验收：按钮文字更新；功能不受影响。
- 回滚点：回退该文件改动。

### T5 - 编译验证（Win + Android）
- 目标：确保本地可编译，且不破坏 CI。
- 验收：
  - Android：`assembleDebug` 成功
  - Windows：`wails build` 成功
- 回滚点：逐步回滚至最近可编译提交。

### T6 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 归档输出：`docs/change/2026-03-04_metricsnode-android-ui-align.md`
