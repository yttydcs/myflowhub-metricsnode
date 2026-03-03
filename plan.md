# Plan - MyFlowHub-MetricsNode（CI：修复 Android 构建失败，确保 Win+Android 均可编译）

> Worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-ci-android`  
> 分支：`fix/metricsnode-ci-android`  
> 日期：2026-03-03  
>
> 本 workflow 目标：修复 GitHub Actions 中 Android 构建失败的问题，并确保同一套 CI 能稳定产出 Windows EXE 与 Android APK；同时保持 `debug-latest` 直链发布可用。

---

## 0. 当前状态

- 仓库已推送到 GitHub：`yttydcs/myflowhub-metricsnode`。
- CI 已存在：`.github/workflows/ci.yml`
  - Windows：`wails build` 生成 `windows/build/bin/windows.exe`
  - Android：`scripts/build_aar.sh` 生成 `android/app/libs/myflowhub.aar` + `./gradlew :app:assembleDebug` 生成 `app-debug.apk`
  - main push：发布/更新 `debug-latest` 预发布 release（直链）
- 现状问题：Android 构建在 GitHub Actions 上失败（需要修复并稳定化）。

---

## 1. 需求分析（已确认）

### 1.1 目标

1) 继续保持 `push(main)` / `pull_request(main)` / `workflow_dispatch` 触发。
2) CI 必须稳定产出：
   - Windows：`windows.exe`
   - Android：`app-debug.apk`
3) 发布：`push(main)` 后，`debug-latest` release 必须可用（至少包含 EXE+APK 的直链）。

### 1.2 不做

- 不做 Android Release 签名（不引入 keystore/secrets）。
- 不做 Windows 代码签名。

### 1.3 验收标准

- Actions 绿灯：Windows job 与 Android job 均成功。
- `debug-latest` release 资产可下载：
  - `windows.exe`
  - `app-debug.apk`
- `myflowhub.aar`：尽量生成并上传（若 gomobile 仍失败，不应阻断 APK/EXE 的发布；失败原因需要在日志中可定位）。

---

## 2. 架构设计（分析）

### 2.1 总体方案

- 在不改变现有功能目标的前提下，对 Android job 做“环境显式化 + 降低不确定性”：
  - 固定 gomobile 版本（与 `nodemobile/go.mod` 的 `golang.org/x/mobile` 对齐），避免 `@latest` 漂移。
  - 写入 `android/local.properties`（`sdk.dir` / `ndk.dir`），降低 Gradle/NDK 发现路径差异。
  - NDK 环境变量兼容：同时设置 `ANDROID_NDK_HOME` / `ANDROID_NDK_ROOT`。
  - 将 AAR 生成作为“强烈建议但不阻断 APK”的步骤：失败时保留日志，继续构建 APK；发布时 AAR 存在则上传，否则仅发布 EXE+APK。

### 2.2 依赖与缓存

- Go：`actions/setup-go` 启用缓存（依赖 `go.sum`、`windows/go.sum`、`nodemobile/go.sum`）。
- Node：`actions/setup-node` 启用 npm 缓存（依赖 `windows/frontend/package-lock.json`）。
- Android：使用 `android-actions/setup-android` + 安装固定 NDK 版本（降低 flaky）。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认
- 目标：worktree/分支正确，工作区干净。
- 验收：`git status --porcelain` 为空。
- 回滚点：无（仅检查）。

### T1 - 修复 Android CI 环境与 gomobile
- 目标：让 Android job 在 GitHub Actions 上稳定构建 APK；AAR 失败不再阻断主流程，并输出可定位日志。
- 涉及文件：
  - `.github/workflows/ci.yml`
- 验收：Android job 成功；artifact 至少包含 `app-debug.apk`；AAR 若失败有清晰日志。
- 回滚点：回滚 workflow 对 Android job 的改动。

### T2 - debug-latest 发布对齐（EXE/APK 必须，AAR 可选）
- 目标：发布 job 不因 AAR 缺失而失败；EXE/APK 始终可发布。
- 涉及文件：
  - `.github/workflows/ci.yml`
- 验收：`debug-latest` release 可下载 EXE+APK；AAR 存在则也可下载。
- 回滚点：回滚 publish job 对 AAR 的“可选化”处理。

### T3 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 归档输出：`docs/change/2026-03-03_metricsnode-ci-android-fix.md`

### T3 - 本地静态校验
- 目标：确保 workflow 引用路径/输出路径正确（不必本机完整跑 CI）。
- 验收：`yamllint` 不强制；最少保证文件存在且路径与仓库一致。
- 回滚点：无。

### T4 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 归档输出：`docs/change/2026-03-03_metricsnode-ci-build.md`
