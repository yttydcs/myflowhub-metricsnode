# 2026-03-07 MetricsNode：修复 CI 构建失败（Windows embed dist + gomobile tidy）

## 变更背景 / 目标

GitHub Actions `ci` 在 `MyFlowHub-MetricsNode` 上出现构建失败，导致：

- Windows job（`build-windows-amd64`）无法产出 `windows.exe`
- Android job（`build-android-debug`）在 gomobile 生成 AAR 阶段失败，进而导致 APK 也无法继续构建
- `publish-debug-latest` 由于 `needs` 依赖上述两个 job，整体发布链路被阻塞

本次变更目标：

1. 修复 Windows job 的 `go:embed` 目录缺失问题，使 `wails build` 能在 clean checkout 场景下稳定执行。
2. 修复 Android job 的 gomobile 构建失败根因（`go mod tidy` 漂移导致 `-mod=readonly` 报错），确保 AAR 可生成。
3. 保持改动最小化：不提交 `dist/` 产物，不改业务逻辑，仅修复构建链路。

## 具体变更内容（新增 / 修改 / 删除）

### 1) Windows CI：构建前创建 `frontend/dist` 占位文件

文件：`.github/workflows/ci.yml`

- 在 Windows job 中新增 step：`Prepare frontend dist (go:embed)`
- 作用：
  - 创建 `windows/frontend/dist` 目录
  - 写入 `windows/frontend/dist/.keep`（空文件）
- 原因：
  - `windows/main.go` 存在 `//go:embed all:frontend/dist`
  - Wails 在生成 bindings 时会触发 Go 编译检查；若 `frontend/dist` 无任何文件，Go 会报 `pattern ...: no matching files found`，导致 CI 直接失败
- 说明：
  - Vite 默认会在 build 时清空 `dist` 目录后输出真实产物；即使 `.keep` 未被清空，最多是 embed 多了一个空文件，风险可接受

### 2) Android gomobile：提交 `nodemobile` 子模块的 tidy 结果

文件：

- `nodemobile/go.mod`
- `nodemobile/go.sum`

变更：

- 执行 `GOWORK=off go mod tidy`
- 使 `nodemobile` 子模块的间接依赖与当前仓库依赖一致（关键点：`github.com/yttydcs/myflowhub-sdk` 间接依赖从 `v0.1.0` 对齐到 `v0.1.2`）

原因：

- CI 的 `scripts/build_aar.sh` 会在 `nodemobile/` 目录执行 `gomobile bind`
- gomobile 在调用 Go 工具链时会触发 `-mod=readonly` 场景；当模块图需要改写（提示 `go mod tidy`）时会直接失败

### 3) 本地脚本：对齐 Windows 构建链路（避免本地也踩 `go:embed`）

文件：`scripts/build-windows.ps1`

- 在执行任何 Wails 命令前确保：
  - `windows/frontend/dist` 目录存在
  - `.keep` 文件存在
- 目的：本地 clean checkout 场景下执行脚本也不会因 `go:embed` 失败。

## 对应计划文档映射

计划文档：`todo.md`（章节：`MetricsNode CI 构建失败修复（2026-03-07）`）

- CI2：Windows `frontend/dist` 占位（`.github/workflows/ci.yml` + `scripts/build-windows.ps1`）
- CI3：`nodemobile` 模块 tidy（`nodemobile/go.mod` + `nodemobile/go.sum`）

## 关键设计决策与权衡

- **选择“占位文件”而非提交 dist 产物**：
  - dist 属于构建产物，不应进入版本库；占位文件只为满足 Go 的 embed pattern“至少命中 1 个文件”的要求。
- **提交 tidy 结果而非在 CI 动态 tidy**：
  - 保持依赖变化可审计、可回滚；避免 CI 运行时改写模块文件造成不可控漂移。

## 测试与验证方式 / 结果

本地验证（worktree：`d:\project\MyFlowHub3\worktrees\fix-metricsnode-ci`）：

- Windows：
  - 创建 `windows/frontend/dist/.keep` 后运行 `wails build -platform windows/amd64 -nopackage` 成功产出 `windows/build/bin/windows.exe`
- Android：
  - `bash scripts/build_aar.sh` 成功生成 `android/app/libs/myflowhub.aar`
  - `android` 工程 `:app:assembleDebug` 本地构建通过（验证 APK 能产出）

CI 验证（建议）：

- 合并到 `main` 后观察 GitHub Actions：
  - `build-windows-amd64` 通过
  - `build-android-debug` 通过
  - `publish-debug-latest` 自动更新 release assets

## 潜在影响与回滚方案

### 潜在影响

- Windows CI checkout 中会出现 `windows/frontend/dist/.keep`（未纳入版本库），属于构建过程临时文件，不影响最终 artifact。
- `nodemobile` 子模块间接依赖对齐到 `myflowhub-sdk v0.1.2`，与仓库主模块一致，属于预期修复。

### 回滚方案

- 回滚本次修复提交即可：
  - 删除 `.github/workflows/ci.yml` 中的占位 step
  - 回退 `nodemobile/go.mod` 与 `nodemobile/go.sum`
  - 回退 `scripts/build-windows.ps1` 的占位逻辑

