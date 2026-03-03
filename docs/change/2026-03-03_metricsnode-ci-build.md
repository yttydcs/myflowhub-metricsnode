# 2026-03-03 MetricsNode：GitHub Actions 自动构建（Windows EXE + Android APK）+ debug-latest 直链

## 变更背景 / 目标

MetricsNode 已推送到 GitHub（`yttydcs/myflowhub-metricsnode`），需要补齐 CI：

- 自动构建 Windows（Wails）可执行文件（`.exe`）
- 自动构建 Android Debug APK（`.apk`），并在构建前生成 gomobile AAR（`myflowhub.aar`）
- 在 `main` 分支 push 后自动发布/更新 `debug-latest` 预发布版本，提供 EXE/APK/AAR 的直链下载

## 具体变更内容

### 新增

- GitHub Actions workflow：
  - `.github/workflows/ci.yml`

包含 job：

1) `build-windows-amd64`（`windows-latest`）
   - 安装 Go / Node / Wails CLI
   - `cd windows; wails build -platform windows/amd64 -nopackage`
   - 校验输出 `windows/build/bin/windows.exe`
   - 上传 artifact：`windows.exe`

2) `build-android-debug`（`ubuntu-latest`）
   - 安装 JDK17 / Go / Android SDK + NDK（固定版本）
   - `bash scripts/build_aar.sh` 生成 `android/app/libs/myflowhub.aar`
   - `cd android; ./gradlew :app:assembleDebug`
   - 校验输出：
     - `android/app/build/outputs/apk/debug/app-debug.apk`
     - `android/app/libs/myflowhub.aar`
   - 上传 artifact：`app-debug.apk` + `myflowhub.aar`

3) `publish-debug-latest`（仅 `push(main)`）
   - 下载上述 artifacts
   - 强制更新 tag `debug-latest` 指向本次 commit
   - 创建/更新 `debug-latest` 预发布 release，并上传：
     - `windows.exe`
     - `app-debug.apk`
     - `myflowhub.aar`
   - 在 Actions Summary 输出直链

## plan.md 任务映射

- T1：新增 CI workflow（build jobs）
- T2：发布 debug-latest Release（直链）

## 关键设计决策与权衡

- **只做 Debug APK**：避免引入 keystore/签名与 secrets 管理复杂度；后续如需 release 再扩展。
- **固定 NDK 版本**：降低 gomobile/Gradle 在 CI 上的偶发不兼容风险。
- **tag 强制指向最新 main commit**：确保 `debug-latest` 始终代表最新构建，并且下载直链稳定。

## 测试与验证

- 本地不强制跑完整 CI（依赖 GitHub runner 环境）；验证方式：
  1) push 到 `main` 后观察 Actions：
     - `build-windows-amd64` 成功，artifact 中有 `windows.exe`
     - `build-android-debug` 成功，artifact 中有 `app-debug.apk` 与 `myflowhub.aar`
  2) 打开 `debug-latest` release，确认 assets 与直链可下载

## 潜在影响与回滚方案

- 影响：仓库新增 CI，会在 PR/main push 时消耗 Actions minutes。
- 回滚：
  - 删除 `.github/workflows/ci.yml` 即可完全关闭该自动化流程。

