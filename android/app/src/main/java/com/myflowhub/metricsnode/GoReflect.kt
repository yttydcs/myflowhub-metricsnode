package com.myflowhub.metricsnode

import java.lang.reflect.Method

internal object GoReflect {
    fun method(cls: Class<*>, name: String, vararg params: Class<*>): Method {
        val candidates = nameCandidates(name)
        var last: NoSuchMethodException? = null
        for (candidate in candidates) {
            try {
                return cls.getMethod(candidate, *params)
            } catch (t: NoSuchMethodException) {
                last = t
            }
        }

        val sig = params.joinToString(prefix = "(", postfix = ")") { it.simpleName }
        val msg = "未找到方法：${cls.name}.${name}${sig}；已尝试：${candidates.joinToString()}"
        throw NoSuchMethodException(msg).apply { initCause(last) }
    }

    private fun nameCandidates(name: String): List<String> {
        if (name.isBlank()) {
            return listOf(name)
        }
        val lowerFirst = name.replaceFirstChar { it.lowercaseChar() }
        val upperFirst = name.replaceFirstChar { it.uppercaseChar() }
        return listOf(name, lowerFirst, upperFirst).distinct()
    }
}

