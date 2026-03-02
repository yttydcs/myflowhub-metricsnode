package com.myflowhub.metricsnode

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.delay

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MetricsNodeApp()
        }
    }
}

@Composable
private fun MetricsNodeApp() {
    val context = androidx.compose.ui.platform.LocalContext.current
    val prefs = remember { context.getSharedPreferences("metricsnode", Context.MODE_PRIVATE) }

    var addr by remember { mutableStateOf(prefs.getString("hub_addr", "127.0.0.1:9000") ?: "127.0.0.1:9000") }
    val initialDeviceId = remember { DeviceId.ensure(prefs, "android") }
    var deviceId by remember { mutableStateOf(initialDeviceId) }

    val bridge: NodeBridge = remember {
        try {
            GoNodeBridge()
        } catch (_: Throwable) {
            StubNodeBridge()
        }
    }

    var state by remember { mutableStateOf(bridge.status()) }

    val notificationLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.RequestPermission(),
        onResult = { /* no-op */ },
    )

    val hasNotifPermission = remember {
        if (Build.VERSION.SDK_INT < 33) true else {
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        }
    }

    LaunchedEffect(Unit) {
        while (true) {
            state = bridge.status()
            delay(1000)
        }
    }

    MaterialTheme {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(
                modifier = Modifier.fillMaxSize().padding(24.dp),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("MyFlowHub MetricsNode", style = MaterialTheme.typography.headlineSmall)

                if (Build.VERSION.SDK_INT >= 33 && !hasNotifPermission) {
                    Button(
                        modifier = Modifier.padding(top = 12.dp),
                        onClick = { notificationLauncher.launch(Manifest.permission.POST_NOTIFICATIONS) },
                    ) {
                        Text("Grant Notification Permission")
                    }
                }

                OutlinedTextField(
                    modifier = Modifier.padding(top = 16.dp),
                    value = addr,
                    onValueChange = {
                        addr = it
                        prefs.edit().putString("hub_addr", it).apply()
                    },
                    label = { Text("Hub Addr") },
                )

                OutlinedTextField(
                    modifier = Modifier.padding(top = 12.dp),
                    value = deviceId,
                    onValueChange = {
                        deviceId = it
                        prefs.edit().putString(DeviceId.PrefKey, it).apply()
                    },
                    label = { Text("Device ID") },
                )

                Row(modifier = Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(onClick = {
                        val intent = Intent(context, NodeService::class.java).apply {
                            action = NodeService.ACTION_START
                            putExtra(NodeService.EXTRA_ADDR, addr.trim())
                            putExtra(NodeService.EXTRA_DEVICE_ID, deviceId.trim())
                        }
                        ContextCompat.startForegroundService(context, intent)
                    }) {
                        Text("Start Service")
                    }
                    Button(onClick = {
                        val intent = Intent(context, NodeService::class.java).apply {
                            action = NodeService.ACTION_STOP
                        }
                        context.startService(intent)
                    }) {
                        Text("Stop Service")
                    }
                }

                Column(modifier = Modifier.padding(top = 20.dp), horizontalAlignment = Alignment.Start) {
                    Text("Running: ${state.running}")
                    Text("Connected: ${state.connected}")
                    Text("Reporting: ${state.reporting}")
                    Text("Addr: ${state.addr}")
                    Text("WorkDir: ${state.workDir}")
                    if (state.lastError.isNotBlank()) {
                        Text("LastError: ${state.lastError}")
                    }
                }
            }
        }
    }
}
