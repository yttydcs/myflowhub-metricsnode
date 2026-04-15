// 本文件承载 MetricsNode Windows 宿主中与 `vite.config` 相关的逻辑。

import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()]
})
