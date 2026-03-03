# Plan - MyFlowHub-MetricsNode（CI：自动构建 Windows EXE + Android APK）

> Worktree：`d:\project\MyFlowHub3\worktrees\chore-metricsnode-ci-build`  
> 分支：`chore/metricsnode-ci-build`  
> 日期：2026-03-03  
>
> 本 workflow 目标：为 `yttydcs/myflowhub-metricsnode` 添加 GitHub Actions，自动构建 Windows（Wails）可执行文件与 Android Debug APK，并在 main 分支 push 后发布 `debug-latest` 预发布版本提供直链下载。

---

## 0. 当前状态

- 仓库已推送到 GitHub：`yttydcs/myflowhub-metricsnode`。
- Windows：Wails App 位于 `windows/`（输出 `windows/build/bin/windows.exe`）。
- Android：工程位于 `android/`；APK 构建依赖可选 gomobile AAR：`android/app/libs/myflowhub.aar`（脚本：`scripts/build_aar.sh`/`scripts/build_aar.ps1`）。
- 当前仓库尚无 `.github/workflows/`。

---

## 1. 需求分析（已确认）

### 1.1 目标

1) Workflow 触发：`push(main)` + `pull_request` + `workflow_dispatch`。
2) Android：构建 Debug APK（`assembleDebug`）。
3) 发布：在 `main` push 后，自动更新/发布 `debug-latest` release，提供直链下载。

### 1.2 不做

- 不做 Android Release 签名（不引入 keystore/secrets）。
- 不做 Windows 代码签名。

### 1.3 验收标准

- CI 能成功生成并上传：
  - Windows：`windows.exe`
  - Android：`app-debug.apk`
  -（附带）`myflowhub.aar`（用于启用真实 bridge，且便于复用/调试）
- `debug-latest` release 中可直接下载上述产物（直链稳定）。

---

## 2. 架构设计（分析）

### 2.1 总体方案

- 新增单一 workflow：`.github/workflows/ci.yml`
  - `build-windows-amd64`（windows-latest）：安装 Go/Node/Wails → `cd windows; wails build -platform windows/amd64 -nopackage` → 上传 exe artifact
  - `build-android-debug`（ubuntu-latest）：安装 JDK/Go/Android SDK+NDK → `bash scripts/build_aar.sh` → `cd android; ./gradlew :app:assembleDebug` → 上传 apk/aar artifact
  - `publish-debug-latest`（ubuntu-latest）：仅 `push(main)` 运行；下载两份 artifact → 强制更新 tag `debug-latest` 指向本次 commit → 创建/更新预发布 release → 上传 exe/apk/aar（`--clobber`）并在 Summary 输出直链。

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

### T1 - 新增 CI workflow（build jobs）
- 目标：新增 `.github/workflows/ci.yml`，包含 Windows/Android 两个 build job，能成功产出 artifact。
- 涉及文件：
  - `.github/workflows/ci.yml`
- 验收：GitHub Actions 运行成功；Artifacts 中存在 exe/apk/aar。
- 回滚点：删除 workflow 文件。

### T2 - 发布 debug-latest Release（直链）
- 目标：main push 后自动发布/更新 `debug-latest` 预发布版本，并上传 exe/apk/aar 作为 release assets。
- 涉及文件：
  - `.github/workflows/ci.yml`
- 验收：Release 页面存在 `debug-latest`，且 assets 可下载。
- 回滚点：移除 publish job（保留 build jobs）。

### T3 - 本地静态校验
- 目标：确保 workflow 引用路径/输出路径正确（不必本机完整跑 CI）。
- 验收：`yamllint` 不强制；最少保证文件存在且路径与仓库一致。
- 回滚点：无。

### T4 - Code Review（阶段 3.3）+ 归档（阶段 4）
- 归档输出：`docs/change/2026-03-03_metricsnode-ci-build.md`
