# 2026-03-01 MetricsNode：升级间接依赖 + 同步 package.json.md5

## 背景 / 目标

本次变更用于把之前本地执行 `wails dev` / `go mod tidy` 产生的“合理依赖升级”与 `windows/frontend/package.json.md5` 同步提交，避免：

- 安全/兼容性风险长期滞留在本地未提交状态；
- 前端依赖状态（Wails 用于判定前端是否变更）不一致导致的反复 dirty 或异常提示。

## 具体变更

### 1) Go 间接依赖升级（仅 `golang.org/x/*`）

- 根模块：
  - `golang.org/x/sys`：`v0.1.0` → `v0.32.0`（indirect）
- `windows` 子模块：
  - `golang.org/x/crypto`：`v0.33.0` → `v0.37.0`（indirect）
  - `golang.org/x/sys`：`v0.30.0` → `v0.32.0`（indirect）
  - `golang.org/x/text`：`v0.22.0` → `v0.24.0`（indirect）

对应 `go.sum` / `windows/go.sum` 同步更新。

### 2) 修正 `package.json.md5`

- `windows/frontend/package.json.md5` 更新为当前 `windows/frontend/package.json` 的真实 MD5 值，确保一致。

## 任务映射（plan.md）

- T0：基线确认
- T1：依赖升级 + MD5 同步
- T2：回归验证

## 关键决策与权衡

- 变更范围严格控制在 `go.mod/go.sum`、`windows/go.mod/windows/go.sum` 与 `windows/frontend/package.json.md5`，避免计划外依赖漂移。
- 根模块在 `x/sys` 升级后，若遇到 `missing go.sum entry for go.mod file`，使用 `go mod download` 精准补齐缺失的 `go.mod` 校验和，而不做全量升级。
- Windows 模块由于 `//go:embed all:frontend/dist`，执行 `go test` 前需先生成 `windows/frontend/dist`（通过 `npm ci && npm run build`）；该目录在仓库中被忽略，不纳入提交。

## 测试与验证

- 根模块：
  - `GOWORK=off go test ./... -count=1 -p 1`
- Windows 模块：
  - `cd windows/frontend; npm ci; npm run build`
  - `cd windows; GOWORK=off go test ./... -count=1 -p 1`
- MD5 一致性：
  - `windows/frontend/package.json.md5` 与 `windows/frontend/package.json` 的 MD5 对比一致（忽略大小写与首尾空白）。

## 潜在影响与回滚

- 影响：间接依赖版本提升可能带来极少数平台/实现细节变化，但已通过编译与测试回归兜底。
- 回滚：`git revert` 本次提交即可恢复到升级前版本（或将相关 `go.mod/go.sum` 回退到上一提交状态）。

