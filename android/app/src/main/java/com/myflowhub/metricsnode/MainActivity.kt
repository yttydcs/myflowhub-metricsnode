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
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.delay
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import java.io.File

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
    var nodeIdText by remember { mutableStateOf(prefs.getString("node_id", "") ?: "") }

    val bridge: NodeBridge = remember {
        try {
            GoNodeBridge()
        } catch (t: Throwable) {
            StubNodeBridge(t.message ?: t.toString())
        }
    }

    val workDir = remember { File(context.filesDir, "metricsnode").absolutePath }

    var state by remember { mutableStateOf(bridge.status()) }

    var page by remember { mutableStateOf(0) } // 0=Connect 1=Settings

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
        runCatching { bridge.init(workDir) }
        while (true) {
            state = bridge.status()
            delay(1000)
        }
    }

    LaunchedEffect(state.auth.nodeId) {
        if (nodeIdText.isBlank() && state.auth.nodeId > 0) {
            nodeIdText = state.auth.nodeId.toString()
            prefs.edit().putString("node_id", nodeIdText).apply()
        }
    }

    MaterialTheme {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
                val bridgeKind = remember(bridge) { if (bridge is StubNodeBridge) "Stub" else "Go" }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text("MyFlowHub MetricsNode", style = MaterialTheme.typography.headlineSmall)
                        Text(
                            "Android client. Bridge: $bridgeKind. WorkDir: $workDir",
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        StatusPill(text = if (state.connected) "Connected" else "Disconnected", ok = state.connected)
                        StatusPill(text = if (state.reporting) "Reporting" else "Stopped", ok = state.reporting)
                    }
                }

                if (Build.VERSION.SDK_INT >= 33 && !hasNotifPermission) {
                    Button(
                        modifier = Modifier.padding(top = 12.dp),
                        onClick = { notificationLauncher.launch(Manifest.permission.POST_NOTIFICATIONS) },
                    ) {
                        Text("Grant Notification Permission")
                    }
                }

                TabRow(selectedTabIndex = page, modifier = Modifier.padding(top = 12.dp)) {
                    Tab(selected = page == 0, onClick = { page = 0 }, text = { Text("Connect") })
                    Tab(selected = page == 1, onClick = { page = 1 }, text = { Text("Settings") })
                }

                Box(modifier = Modifier.fillMaxSize().padding(top = 12.dp)) {
                    if (page == 0) {
                        ConnectPage(
                            context = context,
                            prefs = prefs,
                            addr = addr,
                            onAddrChange = {
                                addr = it
                                prefs.edit().putString("hub_addr", it).apply()
                            },
                            deviceId = deviceId,
                            onDeviceIdChange = {
                                deviceId = it
                                prefs.edit().putString(DeviceId.PrefKey, it).apply()
                            },
                            nodeIdText = nodeIdText,
                            onNodeIdChange = {
                                nodeIdText = it
                                prefs.edit().putString("node_id", it).apply()
                            },
                            state = state,
                        )
                    } else {
                        SettingsPage(
                            bridge = bridge,
                            state = state,
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ConnectPage(
    context: Context,
    prefs: android.content.SharedPreferences,
    addr: String,
    onAddrChange: (String) -> Unit,
    deviceId: String,
    onDeviceIdChange: (String) -> Unit,
    nodeIdText: String,
    onNodeIdChange: (String) -> Unit,
    state: NodeState,
) {
    val canReport = state.connected && state.auth.loggedIn
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Bootstrap", style = MaterialTheme.typography.titleSmall)

                BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                    val wide = maxWidth >= 720.dp
                    if (wide) {
                        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            OutlinedTextField(
                                modifier = Modifier.weight(1f),
                                value = addr,
                                onValueChange = onAddrChange,
                                label = { Text("Hub Addr") },
                                singleLine = true,
                            )
                            OutlinedTextField(
                                modifier = Modifier.weight(1f),
                                value = deviceId,
                                onValueChange = onDeviceIdChange,
                                label = { Text("Device ID") },
                                singleLine = true,
                            )
                        }
                    } else {
                        OutlinedTextField(
                            modifier = Modifier.fillMaxWidth(),
                            value = addr,
                            onValueChange = onAddrChange,
                            label = { Text("Hub Addr") },
                            singleLine = true,
                        )
                        OutlinedTextField(
                            modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                            value = deviceId,
                            onValueChange = onDeviceIdChange,
                            label = { Text("Device ID") },
                            singleLine = true,
                        )
                    }
                }

                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(
                        modifier = Modifier.weight(1f),
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_CONNECT
                                putExtra(NodeService.EXTRA_ADDR, addr.trim())
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Connect") }

                    OutlinedButton(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_DISCONNECT
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Disconnect") }
                }
            }
        }

        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Auth", style = MaterialTheme.typography.titleSmall)

                OutlinedTextField(
                    modifier = Modifier.fillMaxWidth(),
                    value = nodeIdText,
                    onValueChange = onNodeIdChange,
                    label = { Text("Node ID (for login)") },
                    singleLine = true,
                )

                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
                        onClick = {
                            val prefs2 = prefs
                            val id = deviceId.trim().ifBlank { DeviceId.ensure(prefs2, "android") }
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_REGISTER
                                putExtra(NodeService.EXTRA_DEVICE_ID, id)
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Register") }

                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = state.connected,
                        onClick = {
                            val prefs2 = prefs
                            val id = deviceId.trim().ifBlank { DeviceId.ensure(prefs2, "android") }
                            val nodeId = nodeIdText.trim().toLongOrNull() ?: 0L
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_LOGIN
                                putExtra(NodeService.EXTRA_DEVICE_ID, id)
                                putExtra(NodeService.EXTRA_NODE_ID, nodeId)
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Login") }
                }

                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    KeyValueRow("Logged In", if (state.auth.loggedIn) "Yes" else "No")
                    KeyValueRow("Node ID", state.auth.nodeId.toString())
                    KeyValueRow("Hub ID", state.auth.hubId.toString())
                    KeyValueRow("Role", state.auth.role.ifBlank { "-" })
                    KeyValueRow("Last", state.auth.lastAction.ifBlank { "-" })
                    if (state.auth.lastMessage.isNotBlank()) {
                        KeyValueRow("Msg", state.auth.lastMessage)
                    }
                }
            }
        }

        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text("Reporting", style = MaterialTheme.typography.titleSmall)

                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(
                        modifier = Modifier.weight(1f),
                        enabled = canReport && !state.reporting,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_START_REPORTING
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Start Reporting") }

                    OutlinedButton(
                        modifier = Modifier.weight(1f),
                        enabled = state.reporting,
                        onClick = {
                            val intent = Intent(context, NodeService::class.java).apply {
                                action = NodeService.ACTION_STOP_REPORTING
                            }
                            ContextCompat.startForegroundService(context, intent)
                        },
                    ) { Text("Stop") }
                }

                OutlinedButton(
                    modifier = Modifier.fillMaxWidth(),
                    onClick = {
                        val intent = Intent(context, NodeService::class.java).apply {
                            action = NodeService.ACTION_STOP_ALL
                        }
                        context.startService(intent)
                    },
                ) { Text("Stop All") }
            }
        }

        if (state.lastError.isNotBlank()) {
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)) {
                Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("LastError", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onErrorContainer)
                    Text(state.lastError, color = MaterialTheme.colorScheme.onErrorContainer)
                }
            }
        }

        Spacer(modifier = Modifier.size(12.dp))
    }
}

@Composable
private fun SettingsPage(
    bridge: NodeBridge,
    state: NodeState,
) {
    val scope = rememberCoroutineScope()

    var settings by remember { mutableStateOf<List<MetricSetting>>(emptyList()) }
    var settingsError by remember { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }

    val varNameErrors = remember { mutableStateMapOf<String, String>() }
    var saveJob by remember { mutableStateOf<Job?>(null) }

    val controllable = remember { setOf("volume_percent", "volume_muted", "brightness_percent", "flashlight_enabled") }

    fun isVarNameValid(name: String): Boolean {
        val trimmed = name.trim()
        if (trimmed.isEmpty()) return false
        return trimmed.all { it.isLetterOrDigit() || it == '_' }
    }

    fun load() {
        val raw = bridge.metricsSettingsGet()
        settings = MetricSettingJson.parseList(raw)
        settingsError = ""
        varNameErrors.clear()
    }

    fun validate(): String? {
        val seen = HashSet<String>()
        for (s in settings) {
            val varName = s.varName.trim()
            if (!isVarNameValid(varName)) return "invalid var_name for ${s.metric}"
            if (s.enabled) {
                if (!seen.add(varName)) return "duplicate enabled var_name: $varName"
            }
        }
        return null
    }

    fun saveNow() {
        val err = validate()
        if (err != null) {
            settingsError = err
            return
        }
        saving = true
        settingsError = ""
        val raw = MetricSettingJson.toJson(settings)
        scope.launch {
            runCatching { bridge.metricsSettingsSet(raw) }
                .onFailure { settingsError = it.message ?: it.toString() }
            saving = false
        }
    }

    fun scheduleSave() {
        saveJob?.cancel()
        saveJob = scope.launch {
            delay(400)
            saveNow()
        }
    }

    LaunchedEffect(Unit) {
        load()
    }

    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Card {
            Column(modifier = Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("Metrics Settings", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                    OutlinedButton(enabled = !saving, onClick = { load() }) { Text("Reload") }
                    StatusPill(text = if (saving) "Saving…" else "Ready", ok = !saving, warn = saving)
                }

                if (settingsError.isNotBlank()) {
                    Text("Error: $settingsError", color = MaterialTheme.colorScheme.error)
                }

                if (settings.isEmpty()) {
                    Text("No settings loaded.", style = MaterialTheme.typography.bodySmall)
                } else {
                    BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
                        val wide = maxWidth >= 880.dp
                        val columnArrangement = if (wide) Arrangement.spacedBy(0.dp) else Arrangement.spacedBy(8.dp)
                        Column(
                            modifier = Modifier.fillMaxWidth(),
                            verticalArrangement = columnArrangement,
                        ) {
                            if (wide) {
                                SettingsHeaderRow()
                                HorizontalDivider(modifier = Modifier.padding(vertical = 4.dp))
                            }

                            for (s in settings) {
                                val valueText = if (!s.enabled) "-" else (state.metrics[s.metric]?.ifBlank { "-" } ?: "-")
                                val onEnabledChange: (Boolean) -> Unit = { checked ->
                                    settings = settings.map { if (it.metric == s.metric) it.copy(enabled = checked) else it }
                                    scheduleSave()
                                }
                                val onWritableChange: (Boolean) -> Unit = { checked ->
                                    settings = settings.map { if (it.metric == s.metric) it.copy(writable = checked) else it }
                                    scheduleSave()
                                }
                                val onVarNameChange: (String) -> Unit = fun(v: String) {
                                    val trimmed = v.trim()
                                    settings = settings.map { if (it.metric == s.metric) it.copy(varName = v) else it }
                                    if (!isVarNameValid(trimmed)) {
                                        varNameErrors[s.metric] = "only A-Z a-z 0-9 _"
                                        return
                                    }
                                    varNameErrors.remove(s.metric)
                                    scheduleSave()
                                }

                                if (wide) {
                                    SettingsRow(
                                        setting = s,
                                        valueText = valueText,
                                        saving = saving,
                                        controllable = controllable,
                                        varNameError = varNameErrors[s.metric],
                                        onEnabledChange = onEnabledChange,
                                        onWritableChange = onWritableChange,
                                        onVarNameChange = onVarNameChange,
                                    )
                                    HorizontalDivider(modifier = Modifier.padding(vertical = 4.dp))
                                } else {
                                    SettingsCompactRow(
                                        setting = s,
                                        valueText = valueText,
                                        saving = saving,
                                        controllable = controllable,
                                        varNameError = varNameErrors[s.metric],
                                        onEnabledChange = onEnabledChange,
                                        onWritableChange = onWritableChange,
                                        onVarNameChange = onVarNameChange,
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.size(8.dp))
    }
}

@Composable
private fun StatusPill(
    text: String,
    ok: Boolean,
    warn: Boolean = false,
) {
    val colors = MaterialTheme.colorScheme
    val container = when {
        warn -> colors.secondaryContainer
        ok -> colors.tertiaryContainer
        else -> colors.errorContainer
    }
    val content = when {
        warn -> colors.onSecondaryContainer
        ok -> colors.onTertiaryContainer
        else -> colors.onErrorContainer
    }
    Surface(color = container, shape = RoundedCornerShape(999.dp)) {
        Text(
            text = text,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 6.dp),
            color = content,
            style = MaterialTheme.typography.labelMedium,
        )
    }
}

@Composable
private fun KeyValueRow(key: String, value: String) {
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(key, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
        Text(value, style = MaterialTheme.typography.bodySmall, textAlign = TextAlign.End)
    }
}

@Composable
private fun SettingsHeaderRow() {
    val style = MaterialTheme.typography.labelSmall
    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text("Metric", style = style, modifier = Modifier.weight(0.32f))
        Text("Var Name", style = style, modifier = Modifier.weight(0.44f))
        Text("Value", style = style, modifier = Modifier.weight(0.24f), fontFamily = FontFamily.Monospace)
        Text("Enabled", style = style, modifier = Modifier.width(58.dp), textAlign = TextAlign.End)
        Text("Writable", style = style, modifier = Modifier.width(58.dp), textAlign = TextAlign.End)
    }
}

@Composable
private fun SettingsRow(
    setting: MetricSetting,
    valueText: String,
    saving: Boolean,
    controllable: Set<String>,
    varNameError: String?,
    onEnabledChange: (Boolean) -> Unit,
    onWritableChange: (Boolean) -> Unit,
    onVarNameChange: (String) -> Unit,
) {
    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(setting.metric, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(0.32f))

        Column(modifier = Modifier.weight(0.44f)) {
            CompactVarNameField(
                modifier = Modifier.fillMaxWidth(),
                value = setting.varName,
                enabled = !saving,
                isError = !varNameError.isNullOrBlank(),
                onValueChange = onVarNameChange,
            )
            if (!varNameError.isNullOrBlank()) {
                Text(varNameError, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
            }
        }

        Text(
            valueText,
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.weight(0.24f),
            fontFamily = FontFamily.Monospace,
        )

        Box(modifier = Modifier.width(58.dp), contentAlignment = Alignment.CenterEnd) {
            CompactCheck(
                checked = setting.enabled,
                enabled = !saving,
                onCheckedChange = onEnabledChange,
            )
        }

        Box(modifier = Modifier.width(58.dp), contentAlignment = Alignment.CenterEnd) {
            if (controllable.contains(setting.metric)) {
                CompactCheck(
                    checked = setting.writable,
                    enabled = !saving,
                    onCheckedChange = onWritableChange,
                )
            } else {
                Text("-", style = MaterialTheme.typography.bodySmall, textAlign = TextAlign.End)
            }
        }
    }
}

@Composable
private fun SettingsCompactRow(
    setting: MetricSetting,
    valueText: String,
    saving: Boolean,
    controllable: Set<String>,
    varNameError: String?,
    onEnabledChange: (Boolean) -> Unit,
    onWritableChange: (Boolean) -> Unit,
    onVarNameChange: (String) -> Unit,
) {
    val colors = MaterialTheme.colorScheme
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .border(1.dp, colors.outline.copy(alpha = 0.55f), RoundedCornerShape(10.dp))
            .padding(10.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                setting.metric,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.weight(1f),
            )
            Text(
                valueText,
                style = MaterialTheme.typography.bodySmall,
                fontFamily = FontFamily.Monospace,
            )
        }

        CompactVarNameField(
            modifier = Modifier.fillMaxWidth(),
            value = setting.varName,
            enabled = !saving,
            isError = !varNameError.isNullOrBlank(),
            onValueChange = onVarNameChange,
        )

        if (!varNameError.isNullOrBlank()) {
            Text(varNameError, style = MaterialTheme.typography.labelSmall, color = colors.error)
        }

        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            SettingToggle(
                modifier = Modifier.weight(1f),
                label = "Enabled",
                checked = setting.enabled,
                enabled = !saving,
                onCheckedChange = onEnabledChange,
            )
            if (controllable.contains(setting.metric)) {
                SettingToggle(
                    modifier = Modifier.weight(1f),
                    label = "Writable",
                    checked = setting.writable,
                    enabled = !saving,
                    onCheckedChange = onWritableChange,
                )
            } else {
                Text(
                    text = "Writable: -",
                    modifier = Modifier.weight(1f),
                    style = MaterialTheme.typography.labelSmall,
                    textAlign = TextAlign.End,
                )
            }
        }
    }
}

@Composable
private fun SettingToggle(
    label: String,
    checked: Boolean,
    enabled: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.heightIn(min = 28.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(label, style = MaterialTheme.typography.labelSmall, modifier = Modifier.weight(1f))
        CompactCheck(
            checked = checked,
            enabled = enabled,
            onCheckedChange = onCheckedChange,
        )
    }
}

@Composable
private fun CompactVarNameField(
    value: String,
    enabled: Boolean,
    isError: Boolean,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = MaterialTheme.colorScheme
    val borderColor = when {
        isError -> colors.error
        enabled -> colors.outline
        else -> colors.outline.copy(alpha = 0.6f)
    }
    val textColor = if (enabled) colors.onSurface else colors.onSurfaceVariant
    Box(
        modifier = modifier
            .heightIn(min = 34.dp)
            .clip(RoundedCornerShape(8.dp))
            .border(1.dp, borderColor, RoundedCornerShape(8.dp))
            .padding(horizontal = 8.dp, vertical = 6.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        BasicTextField(
            value = value,
            enabled = enabled,
            singleLine = true,
            onValueChange = onValueChange,
            textStyle = MaterialTheme.typography.bodySmall.copy(color = textColor),
            cursorBrush = SolidColor(colors.primary),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun CompactCheck(
    checked: Boolean,
    enabled: Boolean,
    onCheckedChange: (Boolean) -> Unit,
) {
    val colors = MaterialTheme.colorScheme
    val border = when {
        !enabled -> colors.outline.copy(alpha = 0.5f)
        checked -> colors.primary.copy(alpha = 0.9f)
        else -> colors.outline
    }
    val container = when {
        !enabled -> colors.surfaceVariant.copy(alpha = 0.35f)
        checked -> colors.primaryContainer
        else -> colors.surfaceVariant.copy(alpha = 0.65f)
    }
    val markColor = if (enabled) colors.onPrimaryContainer else colors.onSurfaceVariant.copy(alpha = 0.5f)

    Box(
        modifier = Modifier
            .size(22.dp)
            .clip(RoundedCornerShape(4.dp))
            .background(container)
            .border(1.dp, border, RoundedCornerShape(4.dp))
            .toggleable(
                value = checked,
                enabled = enabled,
                role = Role.Checkbox,
                onValueChange = onCheckedChange,
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (checked) {
            Text("✓", style = MaterialTheme.typography.labelSmall, color = markColor, textAlign = TextAlign.Center)
        }
    }
}
