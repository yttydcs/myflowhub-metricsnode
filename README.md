# MyFlowHub-MetricsNode

## Windows 构建（推荐脚本）

为避免 `wailsjs` 绑定缓存导致的构建不一致，建议从仓库根目录使用脚本构建：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

脚本默认会执行：

1. `GOWORK=off`（仅脚本进程内生效）
2. 清理 `windows/frontend/wailsjs/go`
3. 若存在 `windows/frontend/dist`，执行 `wails generate module`
4. 执行 `wails build`
5. 校验产物 `windows/build/bin/windows.exe`

可选参数示例：

```powershell
# 不清理绑定目录
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1 -SkipCleanBindings

# 跳过绑定生成（仅 build）
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1 -SkipGenerateBindings

# 保留当前 GOWORK 环境变量
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1 -KeepGoWork
```

## 其他说明

- Windows 子项目基础说明见：`windows/README.md`
- Android AAR 构建脚本见：`scripts/build_aar.ps1` / `scripts/build_aar.sh`
