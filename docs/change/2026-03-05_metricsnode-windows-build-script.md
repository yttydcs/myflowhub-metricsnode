# 2026-03-05 MetricsNode Windows 构建脚本固化

## 变更背景 / 目标

- 背景：Windows 端 Wails 构建存在“手工执行顺序不一致导致绑定失配”的风险，曾出现 `wailsjs` 导出与 `App.vue` 导入不一致的编译错误。
- 目标：新增可复用脚本，固化 `GOWORK`、绑定清理、构建执行与产物校验流程，减少人为操作差异。

## 具体变更内容

### 新增

- `scripts/build-windows.ps1`
  - 参数：
    - `-WindowsDir`（默认 `windows`）
    - `-SkipCleanBindings`
    - `-SkipGenerateBindings`
    - `-KeepGoWork`
  - 默认流程：
    1) 设置 `GOWORK=off`（可选保留）
    2) 清理 `windows/frontend/wailsjs/go`
    3) 若 `frontend/dist` 存在则执行 `wails generate module`
    4) 执行 `wails build`
    5) 校验 `windows/build/bin/windows.exe` 产物
  - 错误处理：路径不存在、命令缺失、外部命令失败均立即终止并输出原因。

### 修改

- `todo.md`
  - 追加本次 workflow 计划与执行记录（W1-W5）。

### 删除

- 无。

## 对应 plan/todo 任务映射

- W1 需求基线与接口定义 -> 脚本参数与默认行为确定。
- W2 实现构建脚本 -> 完成 `scripts/build-windows.ps1`。
- W3 构建验证 -> 两轮实跑验证通过。
- W4 Code Review -> 七项门禁均通过。
- W5 归档变更 -> 本文档。

## 关键设计决策与权衡

- 性能：默认仅清理 `wailsjs/go`，避免全量清理带来的额外 I/O。
- 稳定性：对 `frontend/dist` 缺失场景做分支处理，首次构建不会因 `go:embed all:frontend/dist` 失败。
- 可扩展性：参数化设计支持后续扩展其他构建模式，减少脚本重写。
- 权衡：默认保留 `wails build` 的完整流程（含前端构建），优先保证一致性和成功率。

## 测试与验证方式 / 结果

- 验证命令：
  - `powershell -ExecutionPolicy Bypass -File scripts/build-windows.ps1`
- 验证结果：
  1) 无 `frontend/dist` 场景：脚本提示跳过 `generate module`，`wails build` 成功。
  2) 有 `frontend/dist` 场景：`generate module` + `wails build` 连续成功。
  3) 产物检查：`windows/build/bin/windows.exe` 存在。

## 潜在影响与回滚方案

- 潜在影响：
  - 构建时会清理 `windows/frontend/wailsjs/go` 目录（仅生成物目录）。
- 回滚方案：
  1) 删除 `scripts/build-windows.ps1`。
  2) 恢复手工构建流程（`wails build` 或按需 `wails generate module` + `wails build`）。
