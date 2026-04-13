// Context: This file belongs to the MetricsNode application layer around vite-env.d.

/// <reference types="vite/client" />

declare module '*.vue' {
    import type {DefineComponent} from 'vue'
    const component: DefineComponent<{}, {}, any>
    export default component
}
