package com.myflowhub.metricsnode
// Context: This file belongs to the MetricsNode application layer around GomobileLoader.

internal object GomobileLoader {
    fun loadNodeClass(): Class<*> {
        val candidates = listOf(
            // Default scripts/build_aar.*: -javapkg com.myflowhub.gomobile + Go pkg "nodemobile"
            "com.myflowhub.gomobile.nodemobile.Nodemobile",
            "com.myflowhub.gomobile.Nodemobile",
            // Backward-compatible fallbacks.
            "com.myflowhub.native.nodemobile.Nodemobile",
            "com.myflowhub.native.Nodemobile",
        )

        var lastError: Throwable? = null
        for (fqcn in candidates) {
            try {
                return Class.forName(fqcn)
            } catch (t: Throwable) {
                lastError = t
            }
        }

        throw IllegalStateException(
            "未找到 gomobile 生成类；请确认 android/app/libs/myflowhub.aar 已打包进 APK。已尝试：${candidates.joinToString()}",
            lastError,
        )
    }
}

