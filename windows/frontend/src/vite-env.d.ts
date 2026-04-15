// 本文件承载 MetricsNode Windows 前端中与 `vite-env` 相关的逻辑。

/// <reference types="vite/client" />

declare module '*.vue' {
    import type {DefineComponent} from 'vue'
    const component: DefineComponent<{}, {}, any>
    export default component
}
