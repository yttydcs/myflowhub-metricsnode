# 2026-03-03 MetricsNode：CI 修复 Android 构建失败（并保证 Win+Android 可编译）

## 变更背景 / 目标

在 GitHub Actions 中，`build-android-debug` job 出现构建失败，导致仓库无法稳定产出 Android Debug APK。

本次变更目标：

1. CI 必须稳定产出 Windows EXE 与 Android Debug APK。
2. `myflowhub.aar` 尽量生成；即使 gomobile 失败，也不能阻断 APK/EXE 的构建与发布，并且需要保留可定位的日志。
3. 保持 `push(main)` 时 `debug-latest` release 直链发布能力可用。

## 具体变更内容

### 1) Android SDK/NDK 安装与发现更稳健

文件：`.github/workflows/ci.yml`

- 对 `ANDROID_SDK_ROOT` / `ANDROID_HOME` 做兜底读取（`sdk_root`），并在缺失时直接报错退出（避免后续步骤报“找不到 SDK location”）。
- 当 `sdkmanager` 不在 `PATH` 时，尝试从 `${sdk_root}/cmdline-tools/latest/bin` 补齐到 `PATH`。
- 通过 `sdkmanager --sdk_root="${sdk_root}" --install ...` 显式指定 SDK 根目录。
- `yes | sdkmanager --licenses` 在 `pipefail` 下可能因 `yes` 的 SIGPIPE 返回非 0；这里改为 `|| true`，避免误判失败。
- 同时设置 `ANDROID_NDK_HOME` 与 `ANDROID_NDK_ROOT`，兼容不同工具链读取方式。

### 2) Gradle 显式读取 local.properties（sdk.dir / ndk.dir）

文件：`.github/workflows/ci.yml`

- CI 中写入 `android/local.properties`，包含：
  - `sdk.dir=<sdk_root>`
  - `ndk.dir=<sdk_root>/ndk/26.1.10909125`

### 3) 固定 gomobile/gobind 版本 + AAR 失败不阻断 APK

文件：`.github/workflows/ci.yml`

- 安装 `gomobile` / `gobind` 时固定版本（与 `nodemobile/go.mod` 对齐），避免 `@latest` 漂移。
- `Build AAR (gomobile)` 设置为 `continue-on-error: true`：
  - 失败时仍继续执行 `assembleDebug`，确保 APK 产出。
  - 同时将 gomobile 输出写入 `gomobile-build.log` 并始终上传为 artifact，便于定位失败原因。
- `Verify outputs` / `Prepare artifacts` 将 AAR 视为可选：存在则上传；缺失则输出 WARN（但不失败）。
- `debug-latest` release 上传时 AAR 改为可选：找不到 `myflowhub.aar` 时跳过，不阻断发布 EXE/APK。

### 4) 触发方式（用于验证）

文件：`.github/workflows/ci.yml`

- 为了便于在修复分支上快速验证，将 `push` 触发分支暂时包含 `fix/metricsnode-ci-android`。
- `publish-debug-latest` job 仍仅在 `push(main)` 时运行，因此不会影响非 main 分支发布行为。

## 对应 plan.md 任务映射

- T1 - 修复 Android CI 环境与 gomobile
  - Android SDK/NDK 安装与发现（`sdk_root`、`sdkmanager`、NDK env）
  - `local.properties` 写入
  - gomobile 固定版本、日志产物、AAR 失败不阻断
- T2 - debug-latest 发布对齐（EXE/APK 必须，AAR 可选）
  - 发布时 AAR 可选化处理

## 关键设计决策与权衡

- **显式化环境**：优先通过 `local.properties` + `--sdk_root` 使 Gradle/Android tooling 的 SDK/NDK 发现路径确定化，减少 runner 差异导致的 flaky。
- **降低版本漂移**：固定 `gomobile/gobind` 版本，避免 `@latest` 在 CI 中不确定升级导致失败。
- **不阻断核心产物**：APK/EXE 作为主交付物必须成功；AAR 在失败时保留日志并继续，以保证开发者可用性与定位能力。

## 测试与验证方式 / 结果

- GitHub Actions：`ci`（fix 分支）验证通过：
  - Windows job：成功产出 `windows.exe`
  - Android job：成功产出 `app-debug.apk`（AAR 亦成功生成时一并上传）

## 潜在影响与回滚方案

### 潜在影响

- 若 gomobile 生成 AAR 失败，Android APK 仍能构建，但会运行在 stub bridge（功能受限，取决于 App 的具体使用方式）。
- `sdkmanager --licenses` 不再作为硬失败点；若 license 接受失败但后续安装成功，流程仍继续（以产物为准）。

### 回滚方案

- 直接回滚 `.github/workflows/ci.yml` 中与 Android job / publish job 相关的改动提交即可（逐步回退以定位问题引入点）。

