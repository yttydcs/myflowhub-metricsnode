# Plan - MyFlowHub-MetricsNode（忽略 Windows 本地 config 目录）

> Worktree：`d:\project\MyFlowHub3\worktrees\chore-metricsnode-ignore-windows-config`
> 分支：`chore/metricsnode-ignore-windows-config`
>
> 说明：本 workflow 很小，但仍按 MyFlowHub3 工程纪律（阶段 1→4）执行并归档，便于审计与交接。

---

## 1. 需求分析

### 1.1 目标

- 避免 `MyFlowHub-MetricsNode/windows` 在本地运行（`wails dev/build`）时生成的 `windows/config/` 被 Git 识别为未跟踪文件，造成工作区长期 dirty。

### 1.2 范围

- 必须：
  - 将 `windows/config/` 加入 `windows/.gitignore`。
- 可选：
  - 无。
- 不做：
  - 不修改运行时配置读取/写入逻辑（仍写入 `windows/config/`）。
  - 不调整 Go module 依赖版本（不应用之前的 `go mod tidy` 产生的依赖升级 stash）。

### 1.3 使用场景

- 开发者在 `repo/MyFlowHub-MetricsNode/windows` 运行 `wails dev` 进行联调；程序会写入本地 bootstrap/auth/keys/runtime_config 等文件到 `windows/config/`。

### 1.4 功能需求

- `windows/config/` 不再出现在 `git status` 的未跟踪列表中。

### 1.5 非功能需求

- 安全：避免误提交包含 `node_keys.json` 等敏感信息的本地文件。
- 可维护性：改动应最小化，仅影响 `.gitignore`。

### 1.6 输入输出

- 输入：无
- 输出：更新后的 `windows/.gitignore`。

### 1.7 边界异常

- 若开发者希望追踪某些 config 文件（极少数情况），可在本地使用 `git add -f` 强制添加（不建议）。

### 1.8 验收标准

- 在 `repo/MyFlowHub-MetricsNode`：
  - 运行一次 `wails dev` 后（生成 `windows/config/*`），`git status --porcelain` 不应出现 `windows/config/`。

### 1.9 风险

- 低风险：仅影响 Git 忽略规则，不影响运行时行为。

---

## 2. 架构设计（分析）

### 2.1 总体方案

- 使用 `windows/.gitignore` 局部忽略（而不是全局 `.gitignore`）：
  - 影响范围更小；
  - 只针对 Windows Wails 工程产生的运行时文件。

### 2.2 备选对比

- 备选 A：修改 Go runtime 的 `workDir` 指向系统用户目录（如 `%APPDATA%`）。
  - 优点：完全避免在 repo 内生成文件。
  - 缺点：属于运行时行为变更，范围更大；不符合本次“只做忽略规则”的目标。
  - 结论：不选。

---

## 3.1 计划拆分（Checklist）

### T0 - 基线确认

- 目标：确认 worktree 可正常提交与合并。
- 验收：`git status` 干净（除待改文件）。

### T1 - 忽略 windows/config

- 目标：将 `config/` 加入 `windows/.gitignore`。
- 涉及文件：`windows/.gitignore`
- 验收：
  - 本地生成 `windows/config/*` 后，`git status` 不再出现该目录。
- 回滚点：回滚该提交即可恢复。

### T2 - Code Review + 归档（阶段 3.3/4）

- 目标：完成 review 清单并归档变更文档。
- 输出：
  - `docs/change/2026-03-01_metricsnode-windows-ignore-config.md`

