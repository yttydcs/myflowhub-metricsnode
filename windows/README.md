# README

## About

This is the official Wails Vue-TS template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## Recommended Build Script

To avoid stale `wailsjs` bindings and keep build steps consistent, prefer running the script from repo root:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

If you are currently in the `windows` directory:

```powershell
powershell -ExecutionPolicy Bypass -File ..\scripts\build-windows.ps1
```

The script will:

1. Set `GOWORK=off` (only inside script process)
2. Clean `windows/frontend/wailsjs/go`
3. Run `wails generate module` when `windows/frontend/dist` exists
4. Run `wails build`
5. Check build output `windows/build/bin/windows.exe`

Common options:

```powershell
# Skip bindings cleanup
powershell -ExecutionPolicy Bypass -File ..\scripts\build-windows.ps1 -SkipCleanBindings

# Skip bindings generation
powershell -ExecutionPolicy Bypass -File ..\scripts\build-windows.ps1 -SkipGenerateBindings

# Keep current GOWORK env
powershell -ExecutionPolicy Bypass -File ..\scripts\build-windows.ps1 -KeepGoWork
```
