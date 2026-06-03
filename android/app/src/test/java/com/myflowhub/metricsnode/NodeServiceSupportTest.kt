package com.myflowhub.metricsnode

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class NodeServiceSupportTest {
    @Test
    fun shouldRestoreRequiresDesiredSnapshot() {
        assertFalse(NodeServiceSupport.shouldRestore(null))
        assertFalse(NodeServiceSupport.shouldRestore(NodeRunSnapshot(addr = "127.0.0.1:9000")))

        val snapshot = NodeRunSnapshot(
            addr = "127.0.0.1:9000",
            deviceId = "android-demo",
            nodeId = 7,
            desiredConnected = true,
        )

        assertTrue(NodeServiceSupport.shouldRestore(snapshot))
        assertNull(NodeServiceSupport.restoreError(snapshot))
    }

    @Test
    fun restoreErrorRejectsMissingAddr() {
        val snapshot = NodeRunSnapshot(desiredReporting = true)

        assertTrue(NodeServiceSupport.shouldRestore(snapshot))
        assertEquals("Restore failed: missing addr", NodeServiceSupport.restoreError(snapshot))
    }

    @Test
    fun foregroundTextSummarizesState() {
        assertEquals("Running | Notify", NodeServiceSupport.foregroundText(NodeState(connected = true, reporting = true, notify = true)))
        assertEquals("Connected | Notify", NodeServiceSupport.foregroundText(NodeState(connected = true, notify = true)))
        assertEquals("Disconnected", NodeServiceSupport.foregroundText(NodeState()))
    }
}
