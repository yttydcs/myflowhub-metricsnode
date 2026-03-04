# 2026-03-04 - MetricsNode Android UI 对齐 Windows + 修复 debug-latest 无响应（gomobile login 绑定 & 多 ABI）

## 变更背景 / 目标

用户在 Android 平板安装 `debug-latest` 时出现：

- Connect 页面按钮点击无反应（底部状态全部 `No`）
- Settings 页面仅显示 `Reload`，无法看到任何指标配置

同时希望：

- Android UI 信息结构与 Windows（Wails）对齐（Connect/Settings 两页、Settings 表格五列）
- Windows Connect 页按钮去掉 `1/2/3/4` 编号前缀
- `debug-latest` 覆盖更多设备（多 ABI）

## 具体变更内容

### 1) 修复 gomobile 绑定缺失 `login(...)`（根因）

- `nodemobile.Login` 的导出签名从 `uint32` 调整为 `int64`，确保 gomobile 能生成 Java binding 的 `login(String,long)`。
- Android 侧反射调用 `Login` 时使用 primitive `long` 作为参数类型（`java.lang.Long.TYPE`）。

影响：

- 解决 `GoNodeBridge()` 构造期反射失败导致静默降级 `StubNodeBridge` 的问题，从根上修复“按钮没反应 / Settings 空”。

### 2) 默认生成多 ABI 的 AAR（覆盖更多设备/模拟器）

- `scripts/build_aar.ps1`、`scripts/build_aar.sh` 默认 target 改为：
  - `android/arm64,android/arm,android/amd64,android/386`
- 生成的 AAR 将包含：
  - `arm64-v8a` / `armeabi-v7a` / `x86_64` / `x86` 的 `libgojni.so`

### 3) CI 保真：禁止发布 stub APK

- `.github/workflows/ci.yml`
  - 取消 AAR 构建的 `continue-on-error`
  - 强制校验 AAR 必须存在且包含 4 个 ABI slice（缺任意一个则 job 失败）
  - artifacts 始终上传 `myflowhub.aar`

### 4) Android UI 对齐 Windows + 错误可见化

- Connect 页按 Windows 的 Bootstrap/Auth/Reporting 三块卡片布局重排，并增加 `Disconnect`、`Stop Reporting` 操作（由 `NodeService` 新 action 承载）。
- Settings 页改为表格布局（`Metric / Var Name / Value / Enabled / Writable`），并将 `Reload` 与 `Ready/Saving` 状态与标题对齐展示。
- `StubNodeBridge` 现在携带初始化失败原因到 `NodeState.lastError`，避免出现“静默无反应”。

### 5) Windows Connect 页按钮去编号

- `Connect/Register/Login/Start Reporting` 文案去掉 `1/2/3/4` 前缀。

## 对应 plan.md 任务映射

- T1：修复 gomobile Login 绑定缺失（Go + Kotlin 反射签名）
- T2：多 ABI AAR + CI 强制成功
- T3：Android UI 对齐 Windows + 错误展示
- T4：Windows Connect 按钮去编号
- T5：本地编译验证（Android + Windows）

## 关键设计决策与权衡

- **为什么改 `Login(uint32)` → `Login(int64)`**：gomobile 对导出函数参数类型有约束，`uint32` 导致 Java binding 不生成 `login`，从而 Android 端必然反射失败。改为 `int64` 可稳定导出到 Java `long`，并在 Go 内部做范围校验后再转换成 `uint32`。
- **为什么默认多 ABI**：满足“覆盖更多设备/模拟器”的诉求，但代价是 AAR/APK 体积增大；通过 CI 强制校验，避免发布“只能 stub”的包。
- **为什么让 CI 直接失败**：`debug-latest` 的定位是“可用的最新 debug 包”，因此宁可失败也不要发布一个装得上但完全不可用的 stub APK。

## 测试与验证方式 / 结果

- 本地生成 AAR（多 ABI）：
  - `scripts/build_aar.ps1` 成功，AAR 含 `arm64-v8a/armeabi-v7a/x86_64/x86`。
  - `javap` 验证 `com.myflowhub.gomobile.nodemobile.Nodemobile` 含 `login(String,long)`。
- Android：
  - `android/gradlew :app:assembleDebug` 成功。
- Windows：
  - `wails build -platform windows/amd64 -nopackage`（`GOWORK=off`）成功。

## 潜在影响与回滚方案

- 影响：
  - 多 ABI AAR 会增大包体与 CI 构建时间。
- 回滚：
  - 若需要回到单 ABI：恢复 `build_aar.*` 默认 target 为 `android/arm64`，并移除 CI 的 ABI slice 强校验（不建议，会降低设备覆盖与可靠性）。
  - 若需回到旧 `Login(uint32)`：会回到 Android 端无法生成 `login` binding 的状态（不建议）。

