# 2026-05-27_windows-notify-presenter-switch

## 变更背景 / 目标

Windows NotifyNode 已能通过 PowerShell `NotifyIcon` 弹出通知，但该模式使用系统默认信息图标，无法展示应用自己的图标。本次目标是在保留旧脚本方案的前提下，新增可切换的 Windows Toast 通知方式，让用户可以在本机设置里选择通知呈现模式。

## 具体变更内容

- 新增 Windows 本地配置 `notify.presenter`，默认值为 `script`。
- 保留旧 `script` 模式，并将 PowerShell 调用改为 `-EncodedCommand` 与 base64 文本注入，避免通知标题/正文被命令行参数解析破坏。
- 新增 `toast` 模式，使用 `git.sr.ht/~jackmordaunt/go-toast/v2` 的 WinRT Toast 能力，并在本地配置目录写入嵌入的 `logo-universal.png` 作为通知图标。
- Windows Wails API 增加 `NotifyPresenterGet` / `NotifyPresenterSet`。
- Windows Vue Notify 页增加 `Notification Mode` 下拉设置，可在 `Script Balloon` 和 `Windows Toast` 之间切换。
- 将原来集中的 `windows/app.go` 拆分出 `app_api.go` 与 `notify_presenter.go`，降低 App 生命周期、Wails API、通知呈现逻辑之间的耦合。

## Requirements impact

none

现有需求只要求 Windows 能显示系统通知，并要求通知展示失败不影响 runtime；本次只是增加 Windows 本地呈现方式选择，不改变 TopicBus、订阅、队列或 Android 行为。

## Specs impact

updated

`docs/specs/notify-node.md` 已补充 `notify.presenter`、`script` / `toast` 两种 Windows presenter 语义。

## Lessons impact

none

本次是已知呈现方式扩展，未形成新的跨项目故障模式。

## Related requirements

- `docs/requirements/notify-node.md`

## Related specs

- `docs/specs/notify-node.md`

## Related lessons

- none

## 对应 plan.md 任务映射

- `WIN-PRESENTER-1`：新增 Windows 通知 presenter 配置、脚本模式修正与 Toast 模式。
- `WIN-UI-1`：增加 Windows Notify 页呈现模式下拉设置并同步 Wails bindings。
- `VERIFY-1`：补充 Go 测试、前端构建和差异检查。
- `ARCHIVE-1`：记录本次 follow-up 归档。

## 经验 / 教训摘要

- Windows `NotifyIcon` 气泡可作为低依赖 fallback，但图标受系统托盘气泡机制限制。
- Toast presenter 更接近现代 Windows 通知形态，但需要本地 app metadata 与图标资源准备。
- Presenter 选择应作为 Windows 本地 bootstrap 配置，而不是跨平台 runtime 配置，避免影响 Android 或 TopicBus 核心行为。

## 可复用排查线索

- 症状：NotifyNode 收到消息但 Windows 没有按预期图标展示。
- 触发条件：通知模式仍为 `script`，或 Toast 模式的本地 app metadata / 图标写入失败。
- 关键词：`notify.presenter`, `NotifyPresenterGet`, `NotifyPresenterSet`, `ShowNotification`, `go-toast`, `ToastGeneric`, `appLogoOverride`.
- 快速检查：
  - 在 Windows Notify 页确认 `Notification Mode` 选择的是 `Windows Toast`；
  - 检查 bootstrap 配置中的 `notify.presenter`；
  - 检查本地工作目录 `assets/notify-logo.png` 是否存在；
  - 若 Toast 不可用，切回 `Script Balloon` 验证 TopicBus 接收链路是否仍正常。

## 关键设计决策与权衡

- 默认保持 `script`，降低现有用户升级后的行为变化。
- Toast 模式只影响 Windows presenter，不改变 `core/notify` 队列、TopicBus 订阅、Android 通知 channel。
- 使用嵌入 PNG 写入本地配置目录，而不是依赖安装路径中的静态资源，避免打包后资源路径不稳定。
- `toast` 模式依赖 Windows Toast/WinRT 环境；旧脚本模式继续作为兼容 fallback。

## 测试与验证方式 / 结果

- `GOWORK=off go test ./... -count=1` in `windows`：通过。
- `npm run build` in `windows/frontend`：通过。
- `git diff --check`：通过。
- 真实 TopicBus 收包后的 Toast 弹出需要在用户机器上选择 `Windows Toast` 后做人工确认；旧 `Script Balloon` 链路已保留为默认 fallback。

## 潜在影响

- 新增 Windows module 依赖 `go-toast/v2`。
- Toast 模式会写入当前用户 registry app metadata，并在本地 config assets 目录写入 `notify-logo.png`。
- 如果目标 Windows 环境不支持 Toast/WinRT，用户可以切回默认 `Script Balloon`。

## 回滚方案

- 移除 `windows/notify_presenter.go` 中 Toast presenter、`notify.presenter` 设置和 `go-toast/v2` 依赖。
- 移除 Windows Vue 的 `Notification Mode` 下拉和对应 Wails binding。
- 恢复 `docs/specs/notify-node.md` 的单一 Windows NotifyIcon presenter 说明。

## 子Agent执行轨迹

- 未派发子Agent。原因：改动集中在 Windows Wails host、生成 bindings、单一 Vue 页面和文档归档，拆分收益低。
