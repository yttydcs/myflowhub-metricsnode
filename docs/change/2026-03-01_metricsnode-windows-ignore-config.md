# 2026-03-01 MetricsNode：忽略 Windows 本地 config 目录（防止误提交密钥/快照）

## 变更背景 / 目标

在 `MyFlowHub-MetricsNode/windows` 本地运行（`wails dev` / `wails build`）时，会在工作目录生成 `windows/config/`，用于保存：

- `bootstrap.json`（Hub addr、device_id 等）
- `auth_snapshot.json`（node_id/hub_id 等登录快照）
- `node_keys.json`（节点密钥）
- `runtime_config.json`（运行时配置）

这些文件属于本机状态与敏感信息，不应被 Git 追踪；同时也会导致仓库长期 `dirty`，影响开发与合并。

目标：将 `windows/config/` 加入忽略列表，避免误提交并保持工作区干净。

## 具体变更

- 更新 `windows/.gitignore`：新增忽略规则 `config/`。

## plan.md 任务映射

- T0：基线确认（可编译/可测试）
- T1：忽略 `windows/config/`
- T2：Code Review + 归档

## Code Review（阶段 3.3）

- 需求覆盖：通过（`windows/config/` 被忽略，不再出现在 `git status` 未跟踪列表）
- 架构合理性：通过（局部 `.gitignore`，影响范围最小）
- 性能风险：无
- 可读性与一致性：通过（规则简洁，带注释）
- 可扩展性与配置化：通过（如未来迁移到用户目录，可移除此规则）
- 稳定性与安全：通过（降低密钥/快照被误提交风险）
- 测试覆盖：通过（`GOWORK=off go test ./...` 无回归；该变更不影响运行逻辑）

## 测试与验证

- Go 编译/单测（非必须但用于回归）：`GOWORK=off go test ./... -count=1 -p 1`
- 手工建议：
  1) `cd windows; wails dev` 运行一次（会生成 `config/*`）
  2) `cd ..; git status --porcelain` 不应出现 `windows/config/`

## 潜在影响与回滚方案

- 影响：仅 Git 忽略规则变化，不影响运行时逻辑。
- 回滚：回滚该提交即可恢复原行为（但不建议）。

