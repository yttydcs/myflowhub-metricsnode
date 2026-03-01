# Plan - MyFlowHub-MetricsNode（升级间接依赖 + 同步 package.json.md5）

> Worktree：`d:\project\MyFlowHub3\worktrees\chore-metricsnode-deps-upgrade`
> 分支：`chore/metricsnode-deps-upgrade`
>
> 本 workflow 用于把之前本地 `wails dev / go mod tidy` 产生的“合理依赖升级”与 `package.json.md5` 同步提交，减少安全/兼容风险并避免反复 dirty。

---

## 1. 需求分析

### 1.1 目标

- 将 `MyFlowHub-MetricsNode`（根模块 + Windows 子模块）的 `golang.org/x/*` 间接依赖升级到较新的版本。
- 将 `windows/frontend/package.json.md5` 修正为与实际 `package.json` 一致的 MD5（避免 Wails 认为前端依赖状态异常）。

### 1.2 范围

- 必须：
  - `go.mod/go.sum`：`golang.org/x/sys` 版本升级。
  - `windows/go.mod/windows/go.sum`：`golang.org/x/crypto` / `golang.org/x/sys` / `golang.org/x/text` 版本升级。
  - `windows/frontend/package.json.md5`：更新为当前 `package.json` 的真实 MD5。
- 不做：
  - 不升级或重写其它依赖（避免计划外抖动）。
  - 不引入新的业务逻辑变更。

### 1.3 验收标准

- Go（根模块）：`GOWORK=off go test ./... -count=1 -p 1` 通过。
- Go（Windows 模块）：`cd windows; GOWORK=off go test ./... -count=1 -p 1` 通过。
- `windows/frontend/package.json.md5` 与 `windows/frontend/package.json` 的 MD5 一致（大小写忽略）。

### 1.4 风险

- 依赖升级可能导致极少数间接 API/行为变化，但本次升级范围小且主要为 `x/*` 包，风险较低；需用编译/测试回归兜底。

---

## 2. 架构设计（分析）

### 2.1 总体方案

- 只做最小版本号调整（对齐 stash 里的 `go mod tidy` 结果），并通过 `go test` 回归验证。
- 不额外运行会引入大范围变更的 `go get -u ./...`。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 验收：当前 worktree clean（除待修改文件）。

### T1 - 依赖版本升级与 MD5 同步

- 涉及文件：
  - `go.mod`
  - `go.sum`
  - `windows/go.mod`
  - `windows/go.sum`
  - `windows/frontend/package.json.md5`
- 验收：变更仅限上述文件，且符合目标版本。

### T2 - 回归验证

- 根模块：
  - 若遇到 `missing go.sum entry for go.mod file`，先执行：`GOWORK=off go mod download golang.org/x/sys@v0.32.0`
  - 回归：`GOWORK=off go test ./... -count=1 -p 1`
- Windows 模块：
  - 由于 `//go:embed all:frontend/dist`，需先生成 `windows/frontend/dist`：
    - `cd windows/frontend; npm ci; npm run build`
  - 回归：`cd windows; GOWORK=off go test ./... -count=1 -p 1`

### T3 - Code Review（阶段 3.3）

- 检查：变更范围、潜在兼容性风险、是否引入计划外依赖漂移、测试结果。

### T4 - 归档（阶段 4）

- 输出：`docs/change/2026-03-01_metricsnode-deps-upgrade.md`
