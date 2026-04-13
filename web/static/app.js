// Sofar HYD Diagnostic Tool - Application Logic

'use strict';

const STORAGE_KEY = 'sofar_connection';
var PV_STORAGE_KEY = 'sofar_pv_channels';
var PV_DEFAULT_CHANNELS = 2;

// Hardcoded topology constants (Phase 06, D-01, D-02)
var TOPO_TOWERS = 2;
var TOPO_PACKS = 10;

// Timing configuration (Phase 07)
var TIMING_STORAGE_KEY = 'sofar_timing';
var TIMING_DEFAULTS = { readDelay: 500, packSettle: 1000 };

// Cycle delay configuration (Phase 8)
var CYCLE_DELAY_KEY = 'sofar_cycle_delay';

// Browser-driven refresh state (Phase 8)
var refreshState = {
    active: false,      // whether auto-refresh is enabled
    cycleCount: 0,      // number of completed cycles (shown in button label)
    delayTimer: null,    // setTimeout handle for cycle delay
    cycleDelay: 0,       // delay in ms between cycles (0 = Continuous)
    readingInProgress: false  // true while waiting for section_complete
};

// === Phase 10: Section Value Cache (DISP-02, D-08 to D-14) ===
// In-memory cache of last-read values per section. Cleared on disconnect (D-12).
// Key: section name (e.g., "system") or pack key (e.g., "bms:pack:1:1:3")
// Value: Map of data-register key to { value, registerAddr, rawValue, timestamp, error }
var sectionCache = new Map();

function getCacheKey() {
    if (App.activeSection === 'bms' && packViewState.mode === 'pack_detail') {
        return 'bms:pack:' + packViewState.selectedInput + ':' +
               packViewState.selectedTower + ':' + packViewState.selectedPack;
    }
    return App.activeSection;
}

function updateCache(registerKey, entry) {
    var cacheKey = getCacheKey();
    if (!sectionCache.has(cacheKey)) {
        sectionCache.set(cacheKey, new Map());
    }
    sectionCache.get(cacheKey).set(registerKey, entry);
}

function restoreFromCache(sectionCacheKey) {
    var cache = sectionCache.get(sectionCacheKey);
    if (!cache || cache.size === 0) return false;

    var body = $('#content-body');
    cache.forEach(function(entry, registerKey) {
        var el = body.querySelector('[data-register="' + CSS.escape(registerKey) + '"]');
        if (!el) return;
        if (entry.error) {
            el.classList.add('data-row-h__value--stale');
            el.classList.remove('data-row-h__value--pending');
        } else {
            el.textContent = entry.value || '\u2014';
            el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
        }
        // Set tooltip data attributes from cache
        if (entry.registerAddr) el.setAttribute('data-register-addr', entry.registerAddr);
        if (entry.rawValue) el.setAttribute('data-register-raw', entry.rawValue);
        if (entry.timestamp) el.setAttribute('data-register-time', entry.timestamp);
    });

    // Mark section as refreshing (cached values shown dimmed) per D-08
    body.classList.add('content__body--refreshing');
    return true;
}

// Phase 10 D-01, D-04: Apply container-level dim when a read cycle starts
function applyRefreshDimming() {
    var body = $('#content-body');
    // Remove all --fresh classes from previous cycle (Pitfall 2)
    var freshEls = body.querySelectorAll('.data-row-h__value--fresh');
    for (var i = 0; i < freshEls.length; i++) {
        freshEls[i].classList.remove('data-row-h__value--fresh');
    }
    // Apply container-level dim
    body.classList.add('content__body--refreshing');

    // Reset pack streaming counters for new cycle (Phase 11)
    if (packViewState.mode === 'pack_detail') {
        resetPackCellState();
        resetPackTempState();
        resetPackStatusState();
    }
}

// === Pack Detail View State (Phase 5) ===

var packViewState = {
    mode: 'overview',        // 'overview' or 'pack_detail'
    selectedInput: 0,
    selectedTower: 0,
    selectedPack: 0,
    topologyTowers: TOPO_TOWERS,
    topologyPacks: TOPO_PACKS
};

// === DOM Helpers ===

function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

// === WSClient Class ===

class WSClient {
    constructor() {
        this.ws = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.handlers = {};
        this.connected = false;
    }

    connect() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + location.host + '/ws';

        this.ws = new WebSocket(url);

        this.ws.onopen = () => {
            this.connected = true;
            this.reconnectDelay = 1000;
            // Send stored timing config on connect so backend picks up saved values
            var storedTiming = null;
            try { storedTiming = JSON.parse(localStorage.getItem(TIMING_STORAGE_KEY)); } catch(e) {}
            if (storedTiming) {
                this.send({
                    type: 'configure',
                    section: 'timing',
                    timing_config: {
                        read_delay_ms: storedTiming.readDelay || 500,
                        pack_settle_ms: storedTiming.packSettle || 1000
                    }
                });
            }
            // Re-subscribe to active section via navigateToSection so PV config
            // and auto-refresh state are synced with the (possibly restarted) backend
            if (App.activeSection) {
                navigateToSection(App.activeSection);
            }
        };

        this.ws.onmessage = (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch (e) {
                return;
            }
            const handler = this.handlers[msg.type];
            if (handler) {
                handler(msg);
            }
        };

        this.ws.onclose = () => {
            this.connected = false;
            this.scheduleReconnect();
        };

        this.ws.onerror = () => {
            // onclose will handle reconnection
        };
    }

    scheduleReconnect() {
        const jitter = 1 + (Math.random() * 0.3);
        const delay = Math.min(this.reconnectDelay * jitter, this.maxReconnectDelay);
        setTimeout(() => this.connect(), delay);
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    }

    send(msg) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    on(type, handler) {
        this.handlers[type] = handler;
    }
}

// === App State ===

const App = {
    ws: null,
    activeSection: null,
    connectionState: 'disconnected'
};

// === Initialization ===

document.addEventListener('DOMContentLoaded', function () {
    // Create WSClient and register handlers
    App.ws = new WSClient();
    App.ws.on('connection_state', handleConnectionState);
    App.ws.on('section_data', handleSectionData);
    App.ws.on('section_error', handleSectionError);
    App.ws.on('pack_data', handlePackData);
    App.ws.on('pack_error', handlePackError);
    App.ws.on('section_schema', handleSectionSchema);
    App.ws.on('register_value', handleRegisterValue);
    App.ws.on('section_complete', handleSectionComplete);

    // Connect WebSocket to server
    App.ws.connect();

    // Initialize connection form
    initConnectionForm();

    // Setup event listeners
    setupFormHandler();
    setupSectionNav();
    setupSidebarToggle();
    setupAutoRefreshToggle();
    initPVDropdown();
    initPackSelectors();
    initTimingControls();
    initCycleDelayDropdown();
    initTooltip();
});

// === Connection Form (CONN-01, CONN-03) ===

function initConnectionForm() {
    // Priority: localStorage > /api/defaults > hardcoded fallbacks
    const saved = loadConnectionSettings();
    if (saved) {
        $('#input-host').value = saved.host || '';
        $('#input-port').value = saved.port || '';
        $('#input-slave').value = saved.slaveId || '';
        return;
    }

    // Fetch from /api/defaults (D-14)
    fetch('/api/defaults')
        .then(function (res) { return res.json(); })
        .then(function (data) {
            if (!$('#input-host').value) $('#input-host').value = data.host || '';
            if (!$('#input-port').value) $('#input-port').value = data.port || '';
            if (!$('#input-slave').value) $('#input-slave').value = data.slave_id || '';
        })
        .catch(function () {
            // Hardcoded fallbacks
            if (!$('#input-host').value) $('#input-host').value = '10.5.99.29';
            if (!$('#input-port').value) $('#input-port').value = '4192';
            if (!$('#input-slave').value) $('#input-slave').value = '1';
        });
}

// === Form Submission (CONN-02) ===

function setupFormHandler() {
    $('#connection-form').addEventListener('submit', function (e) {
        e.preventDefault();

        if (App.connectionState === 'connected' || App.connectionState === 'reconnecting' || App.connectionState === 'connecting') {
            // Disconnect
            App.ws.send({ type: 'disconnect' });
            return;
        }

        // Validate form (D-34)
        if (!validateForm()) {
            return;
        }

        var host = $('#input-host').value.trim();
        var port = parseInt($('#input-port').value, 10);
        var slaveId = parseInt($('#input-slave').value, 10);

        // Save to localStorage (CONN-03)
        saveConnectionSettings(host, port, slaveId);

        // Send connect command
        App.userInitiatedConnect = true;  // D-23: flag for auto-navigate
        App.ws.send({
            type: 'connect',
            host: host,
            port: port,
            slave_id: slaveId
        });
    });
}

// === Form Validation (D-34) ===

function validateForm() {
    var valid = true;
    var host = $('#input-host').value.trim();
    var port = $('#input-port').value.trim();
    var slave = $('#input-slave').value.trim();

    // Validate host
    var hostError = $('#error-host');
    var hostInput = $('#input-host');
    if (!host) {
        hostError.textContent = 'IP address is required';
        hostInput.classList.add('form-input--error');
        valid = false;
    } else if (!/^(\d{1,3}\.){3}\d{1,3}$|^[a-zA-Z0-9.-]+$/.test(host)) {
        hostError.textContent = 'Enter a valid IP address or hostname';
        hostInput.classList.add('form-input--error');
        valid = false;
    } else {
        hostError.textContent = '';
        hostInput.classList.remove('form-input--error');
    }

    // Validate port
    var portError = $('#error-port');
    var portInput = $('#input-port');
    var portNum = parseInt(port, 10);
    if (!port || isNaN(portNum) || portNum < 1 || portNum > 65535) {
        portError.textContent = 'Port must be between 1 and 65535';
        portInput.classList.add('form-input--error');
        valid = false;
    } else {
        portError.textContent = '';
        portInput.classList.remove('form-input--error');
    }

    // Validate slave ID
    var slaveError = $('#error-slave');
    var slaveInput = $('#input-slave');
    var slaveNum = parseInt(slave, 10);
    if (!slave || isNaN(slaveNum) || slaveNum < 1 || slaveNum > 247) {
        slaveError.textContent = 'Slave ID must be between 1 and 247';
        slaveInput.classList.add('form-input--error');
        valid = false;
    } else {
        slaveError.textContent = '';
        slaveInput.classList.remove('form-input--error');
    }

    return valid;
}

// === Section Navigation (RT-01, D-17, D-18) ===

function setupSectionNav() {
    $$('.section-nav__item:not(.section-nav__item--disabled)').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var section = btn.getAttribute('data-section');
            navigateToSection(section);
        });
    });
}

function navigateToSection(section) {
    // Phase 8: Clear pending cycle delay timer on section switch (Pitfall 4)
    if (refreshState.delayTimer) {
        clearTimeout(refreshState.delayTimer);
        refreshState.delayTimer = null;
    }
    refreshState.cycleCount = 0;
    refreshState.readingInProgress = false;
    updateAutoRefreshButton();

    // Update active nav item highlight
    $$('.section-nav__item').forEach(function (item) {
        item.classList.remove('section-nav__item--active');
    });
    var activeBtn = $('.section-nav__item[data-section="' + section + '"]');
    if (activeBtn) {
        activeBtn.classList.add('section-nav__item--active');
    }

    // Set active section
    App.activeSection = section;

    // Update content title (per UI-SPEC copywriting contract)
    var sectionTitles = {
        system: 'System',
        grid: 'Grid',
        eps: 'EPS',
        pv: 'PV',
        battery: 'Battery',
        bms: 'BMS',
        stats: 'Statistics'
    };
    var title = sectionTitles[section] || section.charAt(0).toUpperCase() + section.slice(1);
    $('#content-title').textContent = title;

    // Show/hide PV dropdown based on active section (per UI-SPEC)
    var pvSelect = $('#pv-channel-select');
    if (section === 'pv') {
        pvSelect.style.display = '';
    } else {
        pvSelect.style.display = 'none';
    }

    // Show/hide pack selector controls based on active section
    var packControls = $('#pack-selector-controls');
    if (section === 'bms') {
        // Reset pack view to overview mode when navigating to BMS
        packViewState.mode = 'overview';
        packViewState.selectedInput = 0;
        packViewState.selectedTower = 0;
        packViewState.selectedPack = 0;
        packViewState.topologyTowers = TOPO_TOWERS;
        packViewState.topologyPacks = TOPO_PACKS;
        if (packControls) packControls.style.display = 'none';
    } else {
        if (packControls) packControls.style.display = 'none';
    }

    // Phase 10: Hide tooltip on section navigation (Pitfall 5)
    hideTooltip();
    clearTimeout(tooltipTimer);

    // Show loading spinner
    showLoading();

    // Show auto-refresh button
    var autoBtn = $('#btn-auto-refresh');
    autoBtn.style.display = '';
    updateAutoRefreshButton();

    // Show cycle delay dropdown when connected and viewing a section
    var cycleDelaySelect = $('#cycle-delay-select');
    if (cycleDelaySelect) {
        cycleDelaySelect.style.display = '';
    }

    // Show/hide manual Refresh button based on auto-refresh state
    updateRefreshButtonVisibility();

    // Hide timestamp
    $('#content-timestamp').style.display = 'none';

    // Send subscribe via WebSocket (D-17; auto-unsubscribes previous per D-18; triggers immediate read per D-20)
    App.ws.send({ type: 'subscribe', section: section });

    // Phase 8: Mark reading in progress since subscribe triggers immediate read (D-01/D-20)
    if (refreshState.active) {
        refreshState.readingInProgress = true;
    }

    // Sync PV channel config with backend
    if (section === 'pv') {
        var pvChannels = loadPVChannels() || PV_DEFAULT_CHANNELS;
        App.ws.send({
            type: 'configure',
            section: 'pv',
            config: { channels: pvChannels }
        });
    }

}

// === Sidebar Toggle ===

function setupSidebarToggle() {
    $('#sidebar-toggle').addEventListener('click', function () {
        var sidebar = $('#sidebar');
        var content = $('#content');
        var icon = $('#sidebar-toggle-icon');

        sidebar.classList.toggle('sidebar--collapsed');
        content.classList.toggle('content--sidebar-collapsed');

        // Use Unicode characters via textContent (safe, no innerHTML)
        if (sidebar.classList.contains('sidebar--collapsed')) {
            icon.textContent = '\u00BB'; // right-pointing double angle quotation mark
        } else {
            icon.textContent = '\u00AB'; // left-pointing double angle quotation mark
        }
    });
}

// === Auto-Refresh Toggle (RT-02, D-35) ===

function setupAutoRefreshToggle() {
    $('#btn-auto-refresh').addEventListener('click', function () {
        refreshState.active = !refreshState.active;

        if (!refreshState.active) {
            // D-13: Stop immediately -- clear pending delay timer
            if (refreshState.delayTimer) {
                clearTimeout(refreshState.delayTimer);
                refreshState.delayTimer = null;
            }
            refreshState.cycleCount = 0;
        } else {
            // Start first cycle immediately
            refreshState.cycleCount = 0;
            if (App.activeSection && App.connectionState === 'connected') {
                applyRefreshDimming();
                refreshState.readingInProgress = true;
                App.ws.send({ type: 'read_cycle', section: App.activeSection });
            }
        }

        updateAutoRefreshButton();
        updateRefreshButtonVisibility();
    });

    // Manual Refresh button (D-11)
    $('#btn-refresh').addEventListener('click', function () {
        if (refreshState.readingInProgress) return;
        if (App.activeSection && App.connectionState === 'connected') {
            applyRefreshDimming();
            refreshState.readingInProgress = true;
            updateRefreshButtonState();
            App.ws.send({ type: 'read_cycle', section: App.activeSection });
        }
    });
}

function updateAutoRefreshButton() {
    var btn = $('#btn-auto-refresh');
    if (refreshState.active) {
        if (refreshState.cycleCount > 0) {
            btn.textContent = 'Auto (#' + refreshState.cycleCount + ')';
        } else {
            btn.textContent = 'Auto';
        }
        btn.classList.add('btn-auto-refresh--active');
        btn.setAttribute('aria-pressed', 'true');
    } else {
        btn.textContent = 'Auto';
        btn.classList.remove('btn-auto-refresh--active');
        btn.setAttribute('aria-pressed', 'false');
    }
}

function updateRefreshButtonVisibility() {
    var refreshBtn = $('#btn-refresh');
    if (!refreshBtn) return;
    if (refreshState.active || !App.activeSection) {
        refreshBtn.style.display = 'none';
    } else {
        refreshBtn.style.display = '';
    }
}

function updateRefreshButtonState() {
    var refreshBtn = $('#btn-refresh');
    if (!refreshBtn) return;
    refreshBtn.disabled = refreshState.readingInProgress;
    if (refreshState.readingInProgress) {
        refreshBtn.setAttribute('aria-busy', 'true');
    } else {
        refreshBtn.removeAttribute('aria-busy');
    }
}

// === Message Handlers ===

function handleConnectionState(msg) {
    App.connectionState = msg.state;
    var dot = $('#status-dot');
    var text = $('#status-text');
    var btn = $('#btn-connect');

    // Remove all state classes from dot
    dot.className = 'status-dot';

    switch (msg.state) {
        case 'connected':
            dot.classList.add('status-dot--connected');
            text.textContent = 'Connected';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            // Show timing controls when connected
            var timingCtrlsConn = $('#timing-controls');
            if (timingCtrlsConn) timingCtrlsConn.style.display = '';
            // D-23: Auto-navigate to System section on user-initiated connect
            if (App.userInitiatedConnect) {
                App.userInitiatedConnect = false;
                navigateToSection('system');
            }
            break;
        case 'connecting':
            dot.classList.add('status-dot--connecting');
            text.textContent = 'Connecting...';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            break;
        case 'reconnecting':
            dot.classList.add('status-dot--reconnecting');
            text.textContent = 'Reconnecting...';
            btn.textContent = 'Disconnect';
            btn.className = 'btn btn--disconnect';
            btn.disabled = false;
            setFormInputsDisabled(true);
            break;
        case 'disconnected':
        case 'dormant':
        default:
            dot.classList.add('status-dot--disconnected');
            text.textContent = 'Disconnected';
            btn.textContent = 'Connect';
            btn.className = 'btn btn--connect';
            btn.disabled = false;
            setFormInputsDisabled(false);
            // Hide timing controls when disconnected
            var timingCtrlsDisc = $('#timing-controls');
            if (timingCtrlsDisc) timingCtrlsDisc.style.display = 'none';
            // Phase 8: Cancel pending refresh on disconnect
            if (refreshState.delayTimer) {
                clearTimeout(refreshState.delayTimer);
                refreshState.delayTimer = null;
            }
            refreshState.readingInProgress = false;
            refreshState.cycleCount = 0;
            updateAutoRefreshButton();
            updateRefreshButtonState();
            // Phase 8: Hide cycle delay dropdown and Refresh button when disconnected
            var cycleDelaySelect = $('#cycle-delay-select');
            if (cycleDelaySelect) cycleDelaySelect.style.display = 'none';
            var refreshBtn = $('#btn-refresh');
            if (refreshBtn) refreshBtn.style.display = 'none';
            // Phase 10 D-12, D-14: Clear section cache and reset all values to em-dash skeletons
            sectionCache.clear();
            var contentBody = $('#content-body');
            contentBody.classList.remove('content__body--refreshing');
            // Reset all value elements to em-dash pending state
            var allValues = contentBody.querySelectorAll('.data-row-h__value');
            for (var vi = 0; vi < allValues.length; vi++) {
                allValues[vi].textContent = '\u2014';
                allValues[vi].className = 'data-row-h__value data-row-h__value--pending';
                allValues[vi].removeAttribute('data-register-addr');
                allValues[vi].removeAttribute('data-register-raw');
                allValues[vi].removeAttribute('data-register-time');
                allValues[vi].removeAttribute('aria-describedby');
            }
            // Hide tooltip if visible (Pitfall 5)
            hideTooltip();
            clearTimeout(tooltipTimer);
            // Show connection error if present (e.g. connection refused)
            if (msg.error) {
                showConnectionError(msg.error);
                triggerFlash('error');
            }
            break;
    }
}

function handleSectionData(msg) {
    if (msg.section !== App.activeSection) return;
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;

    var body = $('#content-body');

    // Check if streaming skeleton is rendered (has data-register elements)
    var hasStreamingSkeleton = body.querySelector('[data-register]') !== null;

    if (hasStreamingSkeleton) {
        // Streaming mode: only render computed groups (bitmap, protection) and faults
        // into their placeholder divs, without clearing the skeleton
        var groups = msg.groups || [];
        for (var i = 0; i < groups.length; i++) {
            var group = groups[i];
            if (group.type === 'bitmap') {
                var bitmapPlaceholder = body.querySelector('[data-computed-group="Battery Topology"]');
                if (bitmapPlaceholder) {
                    var bitmapWidget = renderBitmapGroup(group);
                    bitmapPlaceholder.parentNode.replaceChild(bitmapWidget, bitmapPlaceholder);
                } else {
                    body.appendChild(renderBitmapGroup(group));
                }
            } else if (group.type === 'protection') {
                var protPlaceholder = body.querySelector('[data-computed-group="Protection & Alarms"]');
                if (protPlaceholder) {
                    var protWidget = renderProtectionGroup(group);
                    protPlaceholder.parentNode.replaceChild(protWidget, protPlaceholder);
                } else {
                    body.appendChild(renderProtectionGroup(group));
                }
            }
        }

        // Render faults card if present
        if (Array.isArray(msg.faults)) {
            // Remove existing fault card if any
            var existingFault = body.querySelector('.fault-card');
            if (existingFault) existingFault.remove();
            body.appendChild(renderFaultCard(msg.faults));
        }

        // Update timestamp if present
        if (msg.timestamp) {
            var ts = $('#content-timestamp');
            var d = new Date(msg.timestamp);
            ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
            ts.style.display = '';
        }
        return;
    }

    // Legacy batch mode: full re-render (fallback for pack_data or non-streaming paths)
    body.textContent = '';
    if (msg.groups && msg.groups.length > 0) {
        var container = renderGroupedData(msg);
        body.appendChild(container);
    }
    if (Array.isArray(msg.faults)) {
        body.appendChild(renderFaultCard(msg.faults));
    }
    if (msg.timestamp) {
        var ts2 = $('#content-timestamp');
        var d2 = new Date(msg.timestamp);
        ts2.textContent = 'Last updated: ' + d2.toLocaleTimeString();
        ts2.style.display = '';
    }
    triggerFlash('success');
}

// === Grouped Data Renderer (Phase 3) ===

function renderGroupedData(msg) {
    var container = document.createElement('div');
    var gridContainer = null;

    var groups = msg.groups || [];
    for (var i = 0; i < groups.length; i++) {
        var group = groups[i];

        // Type-based widget dispatch
        if (group.type === 'bitmap') {
            gridContainer = null;
            container.appendChild(renderBitmapGroup(group));
        } else if (group.type === 'protection') {
            gridContainer = null;
            container.appendChild(renderProtectionGroup(group));
        } else if (group.layout === 'column') {
            if (!gridContainer) {
                gridContainer = document.createElement('div');
                gridContainer.className = 'group-grid';
                container.appendChild(gridContainer);
            }
            gridContainer.appendChild(renderGroupCard(group));
        } else {
            gridContainer = null;
            container.appendChild(renderGroupCard(group));
        }
    }

    // Render faults card (system section only, per D-09, D-11)
    if (Array.isArray(msg.faults)) {
        gridContainer = null;
        container.appendChild(renderFaultCard(msg.faults));
    }

    return container;
}

function renderGroupCard(group) {
    var card = document.createElement('div');
    card.className = 'group-card';

    // Group name heading
    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    card.appendChild(heading);

    // Separator
    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    card.appendChild(sep);

    // Data rows
    var body = document.createElement('div');
    body.className = 'group-card__body';

    var items = group.items || {};
    var keys = Object.keys(items);
    for (var i = 0; i < keys.length; i++) {
        var row = document.createElement('div');
        row.className = 'data-row-h';

        var keyEl = document.createElement('span');
        keyEl.className = 'data-row-h__key';
        keyEl.textContent = keys[i].toUpperCase();

        var valEl = document.createElement('span');
        valEl.className = 'data-row-h__value';
        valEl.textContent = items[keys[i]];

        // Phase 10: Set tooltip data attributes from item_meta (D-15)
        var meta = group.item_meta && group.item_meta[keys[i]];
        if (meta) {
            var addrHex = '0x' + meta.register_addr.toString(16).toUpperCase().padStart(4, '0');
            valEl.setAttribute('data-register-addr', addrHex);
            if (meta.raw_value) valEl.setAttribute('data-register-raw', meta.raw_value);
        }

        row.appendChild(keyEl);
        row.appendChild(valEl);
        body.appendChild(row);
    }

    card.appendChild(body);
    return card;
}

function renderFaultCard(faults) {
    var card = document.createElement('div');

    if (faults && faults.length > 0) {
        // Active faults -- amber warning styling
        card.className = 'fault-card fault-card--active';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Faults';  // warning sign
        card.appendChild(heading);

        var list = document.createElement('div');
        list.className = 'fault-card__list';

        for (var i = 0; i < faults.length; i++) {
            var item = document.createElement('div');
            item.className = 'fault-card__item';
            item.textContent = '\u2022 ' + faults[i].name;  // bullet
            list.appendChild(item);
        }

        card.appendChild(list);
    } else {
        // No faults -- green success styling
        card.className = 'fault-card fault-card--clear';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Faults';  // checkmark
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'No active faults';
        card.appendChild(clearText);
    }

    return card;
}

function handleSectionError(msg) {
    // Ignore errors for non-active section
    if (msg.section !== App.activeSection) {
        return;
    }

    // Show error text using textContent (safe)
    var body = $('#content-body');
    body.textContent = '';
    var errEl = document.createElement('p');
    errEl.className = 'error-message';
    errEl.textContent = 'Failed to read ' + msg.section + ' data. Check connection and retry.';
    body.appendChild(errEl);

    // Trigger red flash (RT-04)
    triggerFlash('error');
}

// === UI Helpers ===

function showConnectionError(errMsg) {
    var body = $('#content-body');
    body.textContent = '';
    var errEl = document.createElement('p');
    errEl.className = 'error-message';
    // Show a user-friendly message with the technical detail
    var friendly = 'Connection failed';
    if (errMsg.indexOf('refused') !== -1) {
        friendly = 'Connection refused — inverter not reachable on this port';
    } else if (errMsg.indexOf('timeout') !== -1 || errMsg.indexOf('i/o timeout') !== -1) {
        friendly = 'Connection timed out — check IP address';
    }
    errEl.textContent = friendly;
    body.appendChild(errEl);
}

function showLoading() {
    var body = $('#content-body');
    body.textContent = '';

    var loading = document.createElement('div');
    loading.className = 'loading';

    var spinner = document.createElement('div');
    spinner.className = 'loading__spinner';
    loading.appendChild(spinner);

    var text = document.createElement('p');
    text.className = 'loading__text';
    text.textContent = 'Loading...';
    loading.appendChild(text);

    body.appendChild(loading);
}

function triggerFlash(type) {
    var content = $('#content');
    // Remove existing flash classes
    content.classList.remove('content-flash--success', 'content-flash--error');
    // Force reflow to restart transition
    void content.offsetWidth;
    // Add flash class (instant color via 0ms transition)
    content.classList.add('content-flash--' + type);
    // Use double requestAnimationFrame to ensure the browser has painted the flash color
    // before removing the class, which triggers the 1000ms fade-out transition
    requestAnimationFrame(function () {
        requestAnimationFrame(function () {
            content.classList.remove('content-flash--' + type);
        });
    });
}

function setFormInputsDisabled(disabled) {
    $('#input-host').disabled = disabled;
    $('#input-port').disabled = disabled;
    $('#input-slave').disabled = disabled;
}

// === localStorage (CONN-03) ===

function saveConnectionSettings(host, port, slaveId) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ host: host, port: port, slaveId: slaveId }));
    } catch (e) {
        // Private browsing or storage full; silently ignore
    }
}

function loadConnectionSettings() {
    try {
        var raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return null;
        return JSON.parse(raw);
    } catch (e) {
        return null;
    }
}

// === PV Channel Dropdown (D-14, D-15, D-16) ===

function initPVDropdown() {
    var select = $('#pv-channel-select');
    // Populate options 2-16
    for (var i = 2; i <= 16; i++) {
        var opt = document.createElement('option');
        opt.value = String(i);
        opt.textContent = i + ' channels';
        select.appendChild(opt);
    }

    // Load default: localStorage > /api/defaults > 2
    var stored = loadPVChannels();
    if (stored) {
        select.value = String(stored);
    }

    // Also try /api/defaults if no localStorage value
    if (!stored) {
        fetch('/api/defaults')
            .then(function(res) { return res.json(); })
            .then(function(data) {
                if (data.pv_channels && !loadPVChannels()) {
                    select.value = String(data.pv_channels);
                    PV_DEFAULT_CHANNELS = data.pv_channels;
                }
            })
            .catch(function() {});
    }

    // On change: send configure, save to localStorage
    select.addEventListener('change', function() {
        var channels = parseInt(select.value, 10);
        savePVChannels(channels);
        App.ws.send({
            type: 'configure',
            section: 'pv',
            config: { channels: channels }
        });
    });
}

function loadPVChannels() {
    try {
        var val = localStorage.getItem(PV_STORAGE_KEY);
        if (val) return parseInt(val, 10);
        return null;
    } catch(e) { return null; }
}

function savePVChannels(channels) {
    try {
        localStorage.setItem(PV_STORAGE_KEY, String(channels));
    } catch(e) {}
}


// === Phase 10: Parameter Tooltip (DISP-03, D-15 to D-19) ===
var tooltipEl = null;
var tooltipTimer = null;

function initTooltip() {
    tooltipEl = document.createElement('div');
    tooltipEl.className = 'param-tooltip';
    tooltipEl.style.display = 'none';
    tooltipEl.setAttribute('role', 'tooltip');
    tooltipEl.id = 'param-tooltip';
    document.body.appendChild(tooltipEl);

    var body = $('#content-body');

    // Event delegation with useCapture for dynamically-created elements (D-15)
    body.addEventListener('mouseenter', function(e) {
        var target = e.target.closest('.data-row-h__value');
        if (!target) return;
        clearTimeout(tooltipTimer);
        tooltipTimer = setTimeout(function() {
            showTooltip(target);
        }, 300); // D-18: 300ms hover delay
    }, true);

    body.addEventListener('mouseleave', function(e) {
        var target = e.target.closest('.data-row-h__value');
        if (!target) return;
        clearTimeout(tooltipTimer);
        hideTooltip();
    }, true);
}

function showTooltip(el) {
    var addr = el.getAttribute('data-register-addr');
    var raw = el.getAttribute('data-register-raw');
    var time = el.getAttribute('data-register-time');

    var lines = [];
    // D-19: Omit Register line if addr is 0x0000 (composed values)
    if (addr && addr !== '0x0000') lines.push('Register: ' + addr);
    if (raw) lines.push('Raw: ' + raw);
    if (time) lines.push('Last read: ' + time);

    if (lines.length === 0) return;

    // Build tooltip content using textContent (not innerHTML) for XSS safety
    tooltipEl.textContent = '';
    lines.forEach(function(line) {
        var div = document.createElement('div');
        div.textContent = line;
        tooltipEl.appendChild(div);
    });

    // Remove directional modifier before positioning
    tooltipEl.classList.remove('param-tooltip--below');
    tooltipEl.style.display = '';

    // Position above element (D-18)
    var rect = el.getBoundingClientRect();
    var tipWidth = tooltipEl.offsetWidth;
    var tipHeight = tooltipEl.offsetHeight;
    var left = rect.left + rect.width / 2 - tipWidth / 2;
    var top = rect.top - tipHeight - 8; // 8px gap

    // Clamp horizontal position to viewport
    if (left < 4) left = 4;
    if (left + tipWidth > window.innerWidth - 4) left = window.innerWidth - 4 - tipWidth;

    // If tooltip goes above viewport, flip to below
    if (top < 4) {
        top = rect.bottom + 8;
        tooltipEl.classList.add('param-tooltip--below');
    }

    tooltipEl.style.left = left + 'px';
    tooltipEl.style.top = top + 'px';

    // Set aria-describedby on the value element
    el.setAttribute('aria-describedby', 'param-tooltip');
}

function hideTooltip() {
    if (tooltipEl) {
        tooltipEl.style.display = 'none';
    }
    // Remove aria-describedby from any element that has it
    var described = document.querySelector('[aria-describedby="param-tooltip"]');
    if (described) described.removeAttribute('aria-describedby');
}

// === Streaming Message Handlers (Phase 7) ===

function handleSectionSchema(msg) {
    if (msg.section !== App.activeSection) return;

    // Pack schema detection: pack_context present means pack drill-down
    if (msg.pack_context) {
        packViewState.mode = 'pack_detail';
        packViewState.selectedInput = msg.pack_context.input;
        packViewState.selectedTower = msg.pack_context.tower;
        packViewState.selectedPack = msg.pack_context.pack;
        showPackSelectors();
        syncPackSelectorValues();
        renderPackSkeleton(msg);
        return;
    }

    // BMS overview guard: if in pack_detail mode and no pack_context, ignore overview schema
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') return;

    var body = $('#content-body');
    body.textContent = '';

    var container = document.createElement('div');
    var gridContainer = null;
    var groups = msg.groups || [];

    for (var i = 0; i < groups.length; i++) {
        var group = groups[i];

        // Bitmap and protection groups will be rendered by section_data handler
        if (group.type === 'bitmap' || group.type === 'protection') {
            gridContainer = null;
            // Create a placeholder div for computed groups
            var placeholder = document.createElement('div');
            placeholder.setAttribute('data-computed-group', group.name);
            container.appendChild(placeholder);
            continue;
        }

        if (group.layout === 'column') {
            if (!gridContainer) {
                gridContainer = document.createElement('div');
                gridContainer.className = 'group-grid';
                container.appendChild(gridContainer);
            }
            gridContainer.appendChild(renderSkeletonCard(group));
        } else {
            gridContainer = null;
            container.appendChild(renderSkeletonCard(group));
        }
    }

    body.appendChild(container);

    // Phase 10: Restore cached values after skeleton DOM is built (DISP-02, D-08)
    // Must happen after DOM build to avoid Pitfall 1 (querySelector finds nothing)
    var cacheKey = getCacheKey();
    restoreFromCache(cacheKey);
    // If no cache was restored but auto-refresh is active, the subscribe
    // triggers an immediate read -- skeleton stays as-is (em-dashes, no dim needed per D-10)
}

function renderSkeletonCard(group) {
    var card = document.createElement('div');
    card.className = 'group-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    card.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    card.appendChild(sep);

    var body = document.createElement('div');
    body.className = 'group-card__body';

    var registers = group.registers || [];
    for (var i = 0; i < registers.length; i++) {
        var row = document.createElement('div');
        row.className = 'data-row-h';

        var keyEl = document.createElement('span');
        keyEl.className = 'data-row-h__key';
        keyEl.textContent = registers[i].toUpperCase();

        var valEl = document.createElement('span');
        valEl.className = 'data-row-h__value data-row-h__value--pending';
        valEl.textContent = '\u2014'; // em dash (D-02)
        valEl.setAttribute('data-register', group.name + '::' + registers[i]);

        row.appendChild(keyEl);
        row.appendChild(valEl);
        body.appendChild(row);
    }

    card.appendChild(body);
    return card;
}

// === Phase 11: Pack Skeleton Renderers and Streaming Value Handlers ===

// Track arrived cell values for progressive summary computation (D-08, D-09)
var packCellState = {
    values: {},    // cell name -> millivolt value (integer)
    count: 0,
    totalCells: 16
};

function resetPackCellState() {
    packCellState.values = {};
    packCellState.count = 0;
}

// Track temperature values for progressive summary computation
var packTempState = {
    values: {},  // probe name -> raw value (x10 integer)
    count: 0
};

function resetPackTempState() {
    packTempState.values = {};
    packTempState.count = 0;
}

// Track pack status registers for progressive rendering
var packStatusState = {
    registers: {},  // register name -> raw int value
    count: 0
};

function resetPackStatusState() {
    packStatusState.registers = {};
    packStatusState.count = 0;
}

function renderPackSkeleton(msg) {
    // Reset all progressive state trackers
    resetPackCellState();
    resetPackTempState();
    resetPackStatusState();

    var body = $('#content-body');
    body.textContent = '';

    // Breadcrumb
    body.appendChild(renderBreadcrumb(msg.pack_context.input, msg.pack_context.tower, msg.pack_context.pack));

    var groups = msg.groups || [];
    for (var i = 0; i < groups.length; i++) {
        var group = groups[i];

        if (group.type === 'cell_grid') {
            body.appendChild(renderCellGridSkeleton(group));
        } else if (group.type === 'balance') {
            // D-17: Balance renders inside the cell voltage card, not as standalone
            var cellCard = body.querySelector('.cell-grid-card');
            if (cellCard) {
                cellCard.appendChild(renderBalanceSkeleton(group));
            } else {
                // Fallback: render as standalone card if cell card not found
                body.appendChild(renderBalanceSkeletonStandalone(group));
            }
        } else if (group.type === 'pack_status') {
            body.appendChild(renderPackStatusSkeleton(group));
        } else if (group.name === 'Temperatures') {
            // D-14: Temperatures use 4-column grid layout matching cell voltages
            body.appendChild(renderTempGridSkeleton(group));
        } else {
            // Standard group (Pack Info)
            body.appendChild(renderSkeletonCard(group));
        }
    }

    // Phase 10: Restore cached values after skeleton DOM is built (DISP-02)
    var cacheKey = getCacheKey();
    restoreFromCache(cacheKey);
}

function renderCellGridSkeleton(group) {
    var container = document.createElement('div');
    container.className = 'group-card cell-grid-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    container.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    container.appendChild(sep);

    // Summary row with pending placeholders
    var summary = document.createElement('div');
    summary.className = 'cell-summary';
    summary.setAttribute('data-pack-summary', 'cells');

    var summaryItems = ['Min', 'Max', 'Spread', 'Avg'];
    for (var s = 0; s < summaryItems.length; s++) {
        var item = document.createElement('div');
        item.className = 'cell-summary-item';
        var lbl = document.createElement('span');
        lbl.className = 'cell-summary-label';
        lbl.textContent = summaryItems[s];
        item.appendChild(lbl);
        var val = document.createElement('span');
        val.className = 'cell-summary-value data-row-h__value--pending';
        val.textContent = '\u2014';
        val.setAttribute('data-cell-summary', summaryItems[s].toLowerCase());
        item.appendChild(val);
        summary.appendChild(item);
    }
    container.appendChild(summary);

    // Cell grid with 16 cells
    var cellCount = group.cell_count || 16;
    var grid = document.createElement('div');
    grid.className = 'cell-grid';
    grid.setAttribute('data-pack-grid', 'cells');

    for (var c = 0; c < cellCount; c++) {
        var cellDiv = document.createElement('div');
        cellDiv.className = 'cell-voltage';

        var numSpan = document.createElement('span');
        numSpan.className = 'cell-number';
        numSpan.textContent = 'Cell ' + (c + 1);
        cellDiv.appendChild(numSpan);

        var voltSpan = document.createElement('span');
        voltSpan.className = 'cell-value data-row-h__value data-row-h__value--pending';
        voltSpan.textContent = '\u2014';
        voltSpan.setAttribute('data-register', 'Cell Voltages::Cell ' + (c + 1));
        cellDiv.appendChild(voltSpan);

        grid.appendChild(cellDiv);
    }
    container.appendChild(grid);

    return container;
}

function renderBalanceSkeleton(group) {
    // Balance renders as a sub-section within the cell card, not a separate card (D-17)
    var balanceSection = document.createElement('div');
    balanceSection.className = 'balance-inline';

    var balanceSep = document.createElement('hr');
    balanceSep.className = 'group-card__separator';
    balanceSection.appendChild(balanceSep);

    var balanceLabel = document.createElement('h4');
    balanceLabel.className = 'group-card__name';
    balanceLabel.style.fontSize = '14px';
    balanceLabel.style.marginTop = '8px';
    balanceLabel.textContent = 'Balance State';
    balanceSection.appendChild(balanceLabel);

    var balanceValue = document.createElement('div');
    balanceValue.className = 'balance-status data-row-h__value--pending';
    balanceValue.textContent = '\u2014';
    balanceValue.setAttribute('data-register', 'Balance State::Balance State');
    balanceValue.setAttribute('data-pack-balance', 'true');
    balanceSection.appendChild(balanceValue);

    return balanceSection;
}

function renderBalanceSkeletonStandalone(group) {
    // Fallback: standalone balance card (if cell card not found)
    var container = document.createElement('div');
    container.className = 'group-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    container.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    container.appendChild(sep);

    var balanceValue = document.createElement('div');
    balanceValue.className = 'balance-status data-row-h__value--pending';
    balanceValue.textContent = '\u2014';
    balanceValue.setAttribute('data-register', 'Balance State::Balance State');
    balanceValue.setAttribute('data-pack-balance', 'true');
    container.appendChild(balanceValue);

    return container;
}

function renderPackStatusSkeleton(group) {
    var card = document.createElement('div');
    card.className = 'fault-card fault-card--clear';
    card.setAttribute('data-pack-status', 'true');

    var heading = document.createElement('h3');
    heading.className = 'fault-card__heading';
    heading.textContent = '\u2713 Pack Status';
    card.appendChild(heading);

    var clearText = document.createElement('p');
    clearText.className = 'fault-card__clear-text data-row-h__value--pending';
    clearText.textContent = '\u2014';
    card.appendChild(clearText);

    return card;
}

function renderTempGridSkeleton(group) {
    var container = document.createElement('div');
    container.className = 'group-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    container.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    container.appendChild(sep);

    // Summary row (D-15: Min/Max/Spread from Temp 1-8, excluding zeros and Env/MOS)
    var summary = document.createElement('div');
    summary.className = 'cell-summary';
    summary.setAttribute('data-pack-summary', 'temps');

    var summaryItems = ['Min', 'Max', 'Spread'];
    for (var s = 0; s < summaryItems.length; s++) {
        var item = document.createElement('div');
        item.className = 'cell-summary-item';
        var lbl = document.createElement('span');
        lbl.className = 'cell-summary-label';
        lbl.textContent = summaryItems[s];
        item.appendChild(lbl);
        var val = document.createElement('span');
        val.className = 'cell-summary-value data-row-h__value--pending';
        val.textContent = '\u2014';
        val.setAttribute('data-temp-summary', summaryItems[s].toLowerCase());
        item.appendChild(val);
        summary.appendChild(item);
    }
    container.appendChild(summary);

    // 4-column temperature grid (D-14)
    var grid = document.createElement('div');
    grid.className = 'cell-grid'; // Reuse cell-grid CSS for 4-column layout
    grid.setAttribute('data-pack-grid', 'temps');

    var registers = group.registers || [];
    for (var i = 0; i < registers.length; i++) {
        var tempDiv = document.createElement('div');
        // Env Temp and MOS Temp get visually distinct styling
        var isSpecial = (registers[i] === 'MOS Temp' || registers[i] === 'Env Temp');
        tempDiv.className = 'cell-voltage' + (isSpecial ? ' temp-sensor--special' : '');
        if (isSpecial) {
            tempDiv.style.opacity = '0.8';
            tempDiv.style.borderStyle = 'dashed';
        }

        var labelSpan = document.createElement('span');
        labelSpan.className = 'cell-number';
        labelSpan.textContent = registers[i];
        tempDiv.appendChild(labelSpan);

        var valSpan = document.createElement('span');
        valSpan.className = 'cell-value data-row-h__value data-row-h__value--pending';
        valSpan.textContent = '\u2014';
        valSpan.setAttribute('data-register', group.name + '::' + registers[i]);
        tempDiv.appendChild(valSpan);

        grid.appendChild(tempDiv);
    }
    container.appendChild(grid);

    return container;
}

// === Phase 11: Pack Register Value Handlers ===

function handlePackRegisterValue(msg) {
    if (msg.group === 'Cell Voltages') {
        updateCellValue(msg);
    } else if (msg.group === 'Balance State') {
        updateBalanceValue(msg);
    } else if (msg.group === 'Pack Status') {
        updatePackStatusValue(msg);
    } else if (msg.group === 'Temperatures') {
        updateTemperatureValue(msg);
    } else {
        // Standard group (Pack Info) -- use standard register value update logic
        updateStandardPackValue(msg);
    }
}

function updateStandardPackValue(msg) {
    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
    if (!el) return;

    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8);

    if (msg.error) {
        el.classList.add('data-row-h__value--stale');
        el.classList.remove('data-row-h__value--pending');
        el.classList.add('data-row-h__value--fresh');
    } else {
        el.textContent = msg.value || '\u2014';
        el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
        el.classList.add('data-row-h__value--fresh');
    }

    el.setAttribute('data-register-addr', addrHex);
    if (msg.raw_value) el.setAttribute('data-register-raw', msg.raw_value);
    el.setAttribute('data-register-time', timeStr);

    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}

function updateCellValue(msg) {
    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
    if (!el) return;

    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8);

    if (msg.error) {
        el.classList.add('data-row-h__value--stale');
        el.classList.remove('data-row-h__value--pending');
        el.classList.add('data-row-h__value--fresh');
    } else {
        el.textContent = msg.value || '\u2014';
        el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
        el.classList.add('data-row-h__value--fresh');

        // Track cell value for progressive summary (D-08, D-09)
        // Parse millivolt from raw_value (integer string)
        if (msg.name.indexOf('Cell ') === 0 && msg.raw_value) {
            var cellMv = parseInt(msg.raw_value, 10);
            if (!isNaN(cellMv)) {
                packCellState.values[msg.name] = cellMv;
                packCellState.count = Object.keys(packCellState.values).length;

                // D-08: Recompute summary progressively as each cell arrives
                // D-09: Recompute average and recolor ALL received cells on every arrival
                computeCellSummary();
                applyCellDeviationColors();
            }
        }
        // Max Cell Voltage and Min Cell Voltage are summary probes -- ignore for cell tracking
    }

    el.setAttribute('data-register-addr', addrHex);
    if (msg.raw_value) el.setAttribute('data-register-raw', msg.raw_value);
    el.setAttribute('data-register-time', timeStr);

    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}

function computeCellSummary() {
    var vals = packCellState.values;
    var keys = Object.keys(vals);
    if (keys.length === 0) return;

    var sum = 0, min = Infinity, max = -Infinity, minIdx = 1, maxIdx = 1;
    for (var i = 0; i < keys.length; i++) {
        var cellNum = parseInt(keys[i].replace('Cell ', ''), 10);
        var v = vals[keys[i]];
        sum += v;
        if (v > max) { max = v; maxIdx = cellNum; }
        if (v < min) { min = v; minIdx = cellNum; }
    }
    var avg = sum / keys.length;
    var spread = max - min;

    // Update summary elements
    var minEl = document.querySelector('[data-cell-summary="min"]');
    var maxEl = document.querySelector('[data-cell-summary="max"]');
    var spreadEl = document.querySelector('[data-cell-summary="spread"]');
    var avgEl = document.querySelector('[data-cell-summary="avg"]');

    if (minEl) {
        minEl.textContent = (min / 1000).toFixed(3) + 'V (Cell ' + minIdx + ')';
        minEl.classList.remove('data-row-h__value--pending');
    }
    if (maxEl) {
        maxEl.textContent = (max / 1000).toFixed(3) + 'V (Cell ' + maxIdx + ')';
        maxEl.classList.remove('data-row-h__value--pending');
    }
    if (spreadEl) {
        spreadEl.textContent = spread + 'mV';
        spreadEl.classList.remove('data-row-h__value--pending');
        // Apply spread color class
        spreadEl.classList.remove('spread--ok', 'spread--warn', 'spread--danger');
        if (spread <= 30) spreadEl.classList.add('spread--ok');
        else if (spread <= 50) spreadEl.classList.add('spread--warn');
        else spreadEl.classList.add('spread--danger');
    }
    if (avgEl) {
        avgEl.textContent = (avg / 1000).toFixed(3) + 'V';
        avgEl.classList.remove('data-row-h__value--pending');
    }
}

function applyCellDeviationColors() {
    var vals = packCellState.values;
    var keys = Object.keys(vals);
    if (keys.length === 0) return;

    var sum = 0;
    for (var i = 0; i < keys.length; i++) sum += vals[keys[i]];
    var avg = sum / keys.length;

    // Apply deviation colors to each cell
    for (var j = 0; j < keys.length; j++) {
        var cellName = keys[j];
        var v = vals[cellName];
        var dev = Math.abs(v - avg);
        var cellEl = document.querySelector('[data-register="' + CSS.escape('Cell Voltages::' + cellName) + '"]');
        if (!cellEl || !cellEl.parentElement) continue;

        var cellDiv = cellEl.parentElement; // the cell-voltage div
        cellDiv.classList.remove('cell--good', 'cell--warn', 'cell--danger');
        if (dev <= 5) cellDiv.classList.add('cell--good');
        else if (dev <= 20) cellDiv.classList.add('cell--warn');
        else cellDiv.classList.add('cell--danger');
    }
}

function updateBalanceValue(msg) {
    var el = document.querySelector('[data-pack-balance="true"]');
    if (!el) return;

    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8);

    if (msg.error) {
        el.classList.add('data-row-h__value--stale');
        el.classList.remove('data-row-h__value--pending');
        el.classList.add('data-row-h__value--fresh');
    } else {
        // Parse bitmap from raw_value
        var bitmap = parseInt(msg.raw_value || '0', 10);

        // Clear existing content
        while (el.firstChild) el.removeChild(el.firstChild);

        el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale',
                            'balance-status--ok', 'balance-status--active');
        el.classList.add('data-row-h__value--fresh');

        if (bitmap === 0) {
            el.className = 'balance-status balance-status--ok data-row-h__value--fresh';
            el.textContent = 'Balanced';
        } else {
            el.className = 'balance-status balance-status--active data-row-h__value--fresh';

            var statusText = document.createElement('span');
            statusText.textContent = 'Balancing Active';
            el.appendChild(statusText);

            var pills = document.createElement('div');
            pills.className = 'balance-pills';
            for (var b = 0; b < 24; b++) {
                if (bitmap & (1 << b)) {
                    var pill = document.createElement('span');
                    pill.className = 'balance-pill';
                    pill.textContent = 'Cell ' + (b + 1);
                    pills.appendChild(pill);
                }
            }
            el.appendChild(pills);
        }
    }

    el.setAttribute('data-register-addr', addrHex);
    if (msg.raw_value) el.setAttribute('data-register-raw', msg.raw_value);
    el.setAttribute('data-register-time', timeStr);

    var key = 'Balance State::Balance State';
    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}

function updateTemperatureValue(msg) {
    // Use standard value update first
    updateStandardPackValue(msg);

    // Track temperature for summary and apply color coding
    if (!msg.error && msg.raw_value) {
        var rawVal = parseInt(msg.raw_value, 10);
        if (!isNaN(rawVal)) {
            packTempState.values[msg.name] = rawVal;
            packTempState.count = Object.keys(packTempState.values).length;

            // Apply temperature color coding to this value
            var key = msg.group + '::' + msg.name;
            var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
            if (el) {
                var tempC = rawVal / 10.0;
                el.classList.remove('temp--normal', 'temp--elevated', 'temp--critical');
                if (tempC > 55 || tempC < -10) {
                    el.classList.add('temp--critical');
                } else if ((tempC > 45 && tempC <= 55) || (tempC < 0 && tempC >= -10)) {
                    el.classList.add('temp--elevated');
                } else {
                    el.classList.add('temp--normal');
                }
            }

            // Progressive temperature summary computation
            computeTempSummary();
        }
    }
}

function computeTempSummary() {
    var vals = packTempState.values;
    var min = Infinity, max = -Infinity;
    var count = 0;

    // D-15: Summary from Temp 1-8 only, excluding 0.00C readings
    for (var i = 1; i <= 8; i++) {
        var name = 'Temp ' + i;
        if (vals[name] === undefined) continue;
        var raw = vals[name]; // x10 integer
        var tempC = raw / 10.0;
        if (tempC === 0.0) continue; // exclude zero readings (sensor not connected)
        if (tempC < min) min = tempC;
        if (tempC > max) max = tempC;
        count++;
    }

    if (count === 0) return;

    var spread = max - min;

    var minEl = document.querySelector('[data-temp-summary="min"]');
    var maxEl = document.querySelector('[data-temp-summary="max"]');
    var spreadEl = document.querySelector('[data-temp-summary="spread"]');

    if (minEl) {
        minEl.textContent = min.toFixed(1) + '\u00b0C';
        minEl.classList.remove('data-row-h__value--pending');
    }
    if (maxEl) {
        maxEl.textContent = max.toFixed(1) + '\u00b0C';
        maxEl.classList.remove('data-row-h__value--pending');
    }
    if (spreadEl) {
        spreadEl.textContent = spread.toFixed(1) + '\u00b0C';
        spreadEl.classList.remove('data-row-h__value--pending');
    }
}

function updatePackStatusValue(msg) {
    var card = document.querySelector('[data-pack-status="true"]');
    if (!card) return;

    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8);

    if (!msg.error && msg.raw_value) {
        var rawInt = parseInt(msg.raw_value, 10);
        packStatusState.registers[msg.name] = rawInt;
        packStatusState.count = Object.keys(packStatusState.registers).length;

        // After 3+ status registers arrive, render current state
        // (3 = Alarm, Protection, Fault from RT block always present)
        if (packStatusState.count >= 3) {
            renderPackStatusFromState(card);
        }
    }

    // Cache entry
    var key = msg.group + '::' + msg.name;
    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}

// BMS bitmap decode tables (mirrors backend register.BMSAlarmBits etc.)
var BMS_BITMAP_TABLES = {
    alarm: [
        {bit: 0, desc: 'Cell OV alarm'}, {bit: 1, desc: 'Cell UV alarm'},
        {bit: 2, desc: 'Pack OV alarm'}, {bit: 3, desc: 'Pack UV alarm'},
        {bit: 4, desc: 'Charge over-temperature alarm'}, {bit: 5, desc: 'Charge under-temperature alarm'},
        {bit: 6, desc: 'Discharge over-temperature alarm'}, {bit: 7, desc: 'Discharge under-temperature alarm'},
        {bit: 8, desc: 'Charge overcurrent alarm'}, {bit: 9, desc: 'Discharge overcurrent alarm'}
    ],
    protection: [
        {bit: 0, desc: 'Cell OV protection'}, {bit: 1, desc: 'Cell UV protection'},
        {bit: 2, desc: 'Pack OV protection'}, {bit: 3, desc: 'Pack UV protection'},
        {bit: 4, desc: 'Charge over-temperature protection'}, {bit: 5, desc: 'Charge under-temperature protection'},
        {bit: 6, desc: 'Discharge over-temperature protection'}, {bit: 7, desc: 'Discharge under-temperature protection'},
        {bit: 8, desc: 'Charge overcurrent protection'}, {bit: 9, desc: 'Discharge overcurrent protection'},
        {bit: 10, desc: 'Short circuit protection'}, {bit: 11, desc: 'IC fault protection'},
        {bit: 12, desc: 'MOS over-temperature protection'}
    ],
    fault: [
        {bit: 0, desc: 'Cell voltage diff too large'}, {bit: 1, desc: 'Temperature diff too large'},
        {bit: 2, desc: 'Charging lockout (cell OV)'}, {bit: 3, desc: 'Discharging lockout (cell UV)'}
    ],
    alarm2: [
        {bit: 0, desc: 'Cell OV alarm 2'}, {bit: 1, desc: 'Cell UV alarm 2'},
        {bit: 2, desc: 'Pack OV alarm 2'}, {bit: 3, desc: 'Pack UV alarm 2'}
    ],
    protection2: [
        {bit: 0, desc: 'Cell OV protection 2'}, {bit: 1, desc: 'Cell UV protection 2'},
        {bit: 2, desc: 'Pack OV protection 2'}, {bit: 3, desc: 'Pack UV protection 2'}
    ],
    fault2: [
        {bit: 0, desc: 'Cell voltage diff too large 2'}, {bit: 1, desc: 'Temperature diff too large 2'},
        {bit: 2, desc: 'Charging lockout (cell OV) 2'}, {bit: 3, desc: 'Discharging lockout (cell UV) 2'}
    ]
};

function decodeBMSBits(value, type) {
    var table = BMS_BITMAP_TABLES[type] || [];
    var decoded = [];
    for (var i = 0; i < table.length; i++) {
        if (value & (1 << table[i].bit)) {
            decoded.push(table[i].desc);
        }
    }
    return decoded;
}

function renderPackStatusFromState(card) {
    var regs = packStatusState.registers;
    var alarm = regs['Alarm Status'] || 0;
    var protection = regs['Protection Status'] || 0;
    var fault = regs['Fault Status'] || 0;
    var alarm2 = regs['Alarm Status 2'] || 0;
    var protection2 = regs['Protection Status 2'] || 0;
    var fault2 = regs['Fault Status 2'] || 0;

    var allClear = (alarm === 0 && protection === 0 && fault === 0 &&
                    alarm2 === 0 && protection2 === 0 && fault2 === 0);

    // Clear existing card content
    while (card.firstChild) card.removeChild(card.firstChild);
    card.setAttribute('data-pack-status', 'true');

    if (allClear) {
        card.className = 'fault-card fault-card--clear';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Pack Status';
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'All clear -- no alarms, protections, or faults';
        card.appendChild(clearText);
    } else {
        card.className = 'fault-card fault-card--active';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Pack Status';
        card.appendChild(heading);

        // Decode bitmaps using BMS bitmap tables
        var decoded = [];
        decoded = decoded.concat(decodeBMSBits(alarm, 'alarm'));
        decoded = decoded.concat(decodeBMSBits(protection, 'protection'));
        decoded = decoded.concat(decodeBMSBits(fault, 'fault'));
        decoded = decoded.concat(decodeBMSBits(alarm2, 'alarm2'));
        decoded = decoded.concat(decodeBMSBits(protection2, 'protection2'));
        decoded = decoded.concat(decodeBMSBits(fault2, 'fault2'));

        var list = document.createElement('div');
        list.className = 'fault-card__list';

        if (decoded.length > 0) {
            for (var i = 0; i < decoded.length; i++) {
                var item = document.createElement('div');
                item.className = 'fault-card__item';
                var text = decoded[i].toLowerCase();
                if (text.indexOf('protection') !== -1 || text.indexOf('fault') !== -1) {
                    item.style.color = '#c62828';
                }
                item.textContent = '\u2022 ' + decoded[i];
                list.appendChild(item);
            }
        } else {
            // Hex fallback when no bits match known tables
            var hexItems = [];
            if (alarm !== 0) hexItems.push('Alarm: 0x' + alarm.toString(16).toUpperCase().padStart(4, '0'));
            if (protection !== 0) hexItems.push('Protection: 0x' + protection.toString(16).toUpperCase().padStart(4, '0'));
            if (fault !== 0) hexItems.push('Fault: 0x' + fault.toString(16).toUpperCase().padStart(4, '0'));
            if (alarm2 !== 0) hexItems.push('Alarm 2: 0x' + alarm2.toString(16).toUpperCase().padStart(4, '0'));
            if (protection2 !== 0) hexItems.push('Protection 2: 0x' + protection2.toString(16).toUpperCase().padStart(4, '0'));
            if (fault2 !== 0) hexItems.push('Fault 2: 0x' + fault2.toString(16).toUpperCase().padStart(4, '0'));
            for (var h = 0; h < hexItems.length; h++) {
                var hexItem = document.createElement('div');
                hexItem.className = 'fault-card__item';
                hexItem.textContent = '\u2022 ' + hexItems[h];
                list.appendChild(hexItem);
            }
        }
        card.appendChild(list);
    }
}

function handleRegisterValue(msg) {
    if (msg.section !== App.activeSection) return;

    // Route to pack handler when in pack_detail mode
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') {
        handlePackRegisterValue(msg);
        return;
    }

    var key = msg.group + '::' + msg.name;
    var el = document.querySelector('[data-register="' + CSS.escape(key) + '"]');
    if (!el) return;

    // Format register address as hex for tooltip (D-16)
    var addrHex = '0x' + msg.register_addr.toString(16).toUpperCase().padStart(4, '0');

    // Format timestamp (D-16: HH:MM:SS)
    var now = new Date();
    var timeStr = now.toTimeString().slice(0, 8);

    if (msg.error) {
        // D-03: show last known value dimmed with error icon (Phase 9 behavior preserved)
        el.classList.add('data-row-h__value--stale');
        el.classList.remove('data-row-h__value--pending');
        // Phase 10: Still add fresh class to override container dim (value is visible, just styled as stale)
        el.classList.add('data-row-h__value--fresh');
    } else {
        el.textContent = msg.value || '\u2014';
        el.classList.remove('data-row-h__value--pending', 'data-row-h__value--stale');
        // Phase 10 D-02: Snap value back to full opacity
        el.classList.add('data-row-h__value--fresh');
    }

    // Phase 10: Set tooltip data attributes (D-16, D-19)
    el.setAttribute('data-register-addr', addrHex);
    if (msg.raw_value) el.setAttribute('data-register-raw', msg.raw_value);
    el.setAttribute('data-register-time', timeStr);

    // Phase 10: Update section cache (DISP-02)
    updateCache(key, {
        value: msg.value || '',
        registerAddr: addrHex,
        rawValue: msg.raw_value || '',
        timestamp: timeStr,
        error: !!msg.error
    });
}

function handleSectionComplete(msg) {
    if (msg.section !== App.activeSection) return;

    // Update timestamp display (existing logic preserved)
    if (msg.timestamp) {
        var ts = $('#content-timestamp');
        var d = new Date(msg.timestamp);
        ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
        ts.style.display = '';
    }
    triggerFlash('success');

    // Phase 11: Final summary computation for pack streaming
    if (msg.section === 'bms' && packViewState.mode === 'pack_detail') {
        if (packCellState.count > 0) {
            computeCellSummary();
            applyCellDeviationColors();
        }
        if (packTempState.count > 0) {
            computeTempSummary();
        }
    }

    // Phase 10 D-06: Remove container-level dim on section complete (cleanup sweep)
    var contentBody = $('#content-body');
    contentBody.classList.remove('content__body--refreshing');

    // Phase 8: Browser-driven refresh cycle
    refreshState.readingInProgress = false;
    updateRefreshButtonState();

    if (refreshState.active) {
        refreshState.cycleCount++;
        updateAutoRefreshButton();

        // Schedule next cycle after configured delay (D-04, REFR-02)
        refreshState.delayTimer = setTimeout(function () {
            refreshState.delayTimer = null;
            if (refreshState.active && App.activeSection === msg.section && App.connectionState === 'connected') {
                applyRefreshDimming();
                refreshState.readingInProgress = true;
                App.ws.send({ type: 'read_cycle', section: App.activeSection });
            }
        }, refreshState.cycleDelay);
    }
}

// === Timing Controls (Phase 7, TIMING-01, TIMING-02) ===

function initTimingControls() {
    var readDelayInput = $('#timing-read-delay');
    var packSettleInput = $('#timing-pack-settle');
    if (!readDelayInput || !packSettleInput) return;

    // Load from localStorage
    var stored = null;
    try {
        stored = JSON.parse(localStorage.getItem(TIMING_STORAGE_KEY));
    } catch (e) { /* ignore */ }

    if (stored) {
        if (stored.readDelay) readDelayInput.value = stored.readDelay;
        if (stored.packSettle) packSettleInput.value = stored.packSettle;
    }

    function clamp(val, min, max) {
        var n = parseInt(val, 10);
        if (isNaN(n)) return min;
        if (n < min) return min;
        if (n > max) return max;
        return n;
    }

    function sendTimingConfig() {
        var readDelay = clamp(readDelayInput.value, 50, 5000);
        var packSettle = clamp(packSettleInput.value, 500, 10000);

        // Update inputs to clamped values
        readDelayInput.value = readDelay;
        packSettleInput.value = packSettle;

        // Persist to localStorage
        localStorage.setItem(TIMING_STORAGE_KEY, JSON.stringify({
            readDelay: readDelay,
            packSettle: packSettle
        }));

        // Send to backend
        App.ws.send({
            type: 'configure',
            section: 'timing',
            timing_config: {
                read_delay_ms: readDelay,
                pack_settle_ms: packSettle
            }
        });
    }

    // Send on blur or Enter key
    readDelayInput.addEventListener('change', sendTimingConfig);
    packSettleInput.addEventListener('change', sendTimingConfig);
    readDelayInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') { readDelayInput.blur(); }
    });
    packSettleInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') { packSettleInput.blur(); }
    });
}

// === Cycle Delay Dropdown (Phase 8, D-05, D-06, D-07) ===

function initCycleDelayDropdown() {
    var select = $('#cycle-delay-select');
    if (!select) return;

    // Load from localStorage (D-07)
    var saved = localStorage.getItem(CYCLE_DELAY_KEY);
    if (saved !== null) {
        var val = parseInt(saved, 10);
        if (!isNaN(val)) {
            refreshState.cycleDelay = val;
            select.value = String(val);
        }
    }

    select.addEventListener('change', function () {
        var val = parseInt(select.value, 10);
        if (isNaN(val)) val = 0;
        refreshState.cycleDelay = val;
        localStorage.setItem(CYCLE_DELAY_KEY, String(val));
    });
}

// === Bitmap Grid Renderer (BAT-05, D-06, D-07, D-08) ===

function renderBitmapGroup(group) {
    var card = document.createElement('div');
    card.className = 'group-card';

    // Heading
    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    card.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    card.appendChild(sep);

    var body = document.createElement('div');
    body.className = 'bitmap-group';

    var bm = group.bitmap;
    if (!bm) { card.appendChild(body); return card; }

    // Detected topology label (D-14)
    if (bm.detected_topology) {
        var detLabel = document.createElement('div');
        detLabel.className = 'bitmap-detected';
        detLabel.textContent = 'Detected: ' + bm.detected_topology;

        // Mismatch warning
        if (bm.mismatch) {
            var warn = document.createElement('span');
            warn.className = 'bitmap-mismatch';
            warn.textContent = ' \u26A0 Config mismatch';
            detLabel.appendChild(warn);
        }
        body.appendChild(detLabel);
    }

    // Grid rows (one per tower)
    for (var t = 0; t < bm.towers; t++) {
        var rowWrap = document.createElement('div');
        rowWrap.className = 'bitmap-row';

        var rowLabel = document.createElement('span');
        rowLabel.className = 'bitmap-row__label';
        rowLabel.textContent = 'Tower ' + (t + 1);
        rowWrap.appendChild(rowLabel);

        var grid = document.createElement('div');
        grid.className = 'bitmap-grid';
        grid.style.gridTemplateColumns = 'repeat(' + bm.packs_per_tower + ', 28px)';

        var online = bm.online[t] || 0;
        for (var p = 0; p < bm.packs_per_tower; p++) {
            var cell = document.createElement('div');
            var isOnline = (online >> p) & 1;
            cell.className = 'bitmap-cell ' + (isOnline ? 'bitmap-cell--online' : 'bitmap-cell--offline');
            cell.textContent = String(p + 1);
            cell.style.cursor = 'pointer';

            // Check if this cell is the currently selected pack
            if (packViewState.mode === 'pack_detail') {
                var cellInput = 1;
                var cellTower = t + 1;
                var cellPack = p + 1;
                if (cellInput === packViewState.selectedInput &&
                    cellTower === packViewState.selectedTower &&
                    cellPack === packViewState.selectedPack) {
                    cell.className = 'bitmap-cell bitmap-cell--selected';
                }
            }

            // Click and hover handlers via closure
            (function(towerIdx, packIdx, cellOnline, cellEl) {
                cellEl.addEventListener('click', function() {
                    handleBitmapCellClick(towerIdx, packIdx, cellOnline);
                });
                cellEl.addEventListener('mouseenter', function() {
                    if (cellEl.classList.contains('bitmap-cell--selected')) return;
                    if (cellOnline) {
                        cellEl.style.backgroundColor = 'var(--bitmap-hover, #2e7d32)';
                    } else {
                        cellEl.style.backgroundColor = '#9e9e9e';
                    }
                });
                cellEl.addEventListener('mouseleave', function() {
                    if (cellEl.classList.contains('bitmap-cell--selected')) return;
                    cellEl.style.backgroundColor = '';
                });
            })(t, p, isOnline, cell);

            grid.appendChild(cell);
        }
        rowWrap.appendChild(grid);
        body.appendChild(rowWrap);
    }

    // Legend
    var legend = document.createElement('div');
    legend.className = 'bitmap-legend';

    var onItem = document.createElement('div');
    onItem.className = 'bitmap-legend__item';
    var onSwatch = document.createElement('span');
    onSwatch.className = 'bitmap-legend__swatch bitmap-cell--online';
    onItem.appendChild(onSwatch);
    var onLabel = document.createElement('span');
    onLabel.className = 'bitmap-legend__label';
    onLabel.textContent = 'Online';
    onItem.appendChild(onLabel);
    legend.appendChild(onItem);

    var offItem = document.createElement('div');
    offItem.className = 'bitmap-legend__item';
    var offSwatch = document.createElement('span');
    offSwatch.className = 'bitmap-legend__swatch bitmap-cell--offline';
    offItem.appendChild(offSwatch);
    var offLabel = document.createElement('span');
    offLabel.className = 'bitmap-legend__label';
    offLabel.textContent = 'Offline';
    offItem.appendChild(offLabel);
    legend.appendChild(offItem);

    body.appendChild(legend);
    card.appendChild(body);
    return card;
}

// === Protection & Alarms Renderer (D-04) ===

function renderProtectionGroup(group) {
    var card = document.createElement('div');
    var items = group.items || {};
    var keys = Object.keys(items);

    // Check if any non-zero values
    var hasActive = false;
    for (var i = 0; i < keys.length; i++) {
        if (items[keys[i]] !== '0x0000') {
            hasActive = true;
            break;
        }
    }

    if (hasActive) {
        card.className = 'fault-card fault-card--active';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Protection & Alarms';
        card.appendChild(heading);

        var list = document.createElement('div');
        list.className = 'fault-card__list';
        for (var i = 0; i < keys.length; i++) {
            if (items[keys[i]] !== '0x0000') {
                var item = document.createElement('div');
                item.className = 'fault-card__item';
                item.textContent = '\u2022 ' + keys[i] + ': ' + items[keys[i]];
                list.appendChild(item);
            }
        }
        card.appendChild(list);
    } else {
        card.className = 'fault-card fault-card--clear';
        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Protection & Alarms';
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'No active protections or alarms';
        card.appendChild(clearText);
    }

    return card;
}

// === Pack Detail Sub-View (Phase 5, BAT-07 through BAT-11) ===

function handlePackData(msg) {
    packViewState.mode = 'pack_detail';
    packViewState.selectedInput = msg.input;
    packViewState.selectedTower = msg.tower;
    packViewState.selectedPack = msg.pack;

    // Switch header controls to pack selectors
    showPackSelectors();
    syncPackSelectorValues();

    renderPackDetail(msg);

    // Phase 10: Set timestamps on all tooltip-enabled elements and populate cache (D-15, D-09)
    var timeStr = new Date().toTimeString().slice(0, 8);
    var body = document.getElementById('content-body');
    var tooltipEls = body.querySelectorAll('[data-register-addr]');
    for (var ti = 0; ti < tooltipEls.length; ti++) {
        tooltipEls[ti].setAttribute('data-register-time', timeStr);
    }

    // Populate section cache for pack drill-down if cache is available (DISP-02, D-09)
    if (typeof sectionCache !== 'undefined' && sectionCache) {
        var packCacheKey = 'bms:pack:' + msg.input + ':' + msg.tower + ':' + msg.pack;
        if (!sectionCache.has(packCacheKey)) {
            sectionCache.set(packCacheKey, new Map());
        }
        var packCache = sectionCache.get(packCacheKey);
        for (var ci = 0; ci < tooltipEls.length; ci++) {
            var el = tooltipEls[ci];
            var regKey = el.getAttribute('data-register') || el.getAttribute('data-register-addr') || ('pack-item-' + ci);
            packCache.set(regKey, {
                value: el.textContent,
                registerAddr: el.getAttribute('data-register-addr') || '',
                rawValue: el.getAttribute('data-register-raw') || '',
                timestamp: timeStr,
                error: false
            });
        }
    }

    triggerFlash('success');
}

function handlePackError(msg) {
    packViewState.mode = 'pack_detail';
    packViewState.selectedInput = msg.input;
    packViewState.selectedTower = msg.tower;
    packViewState.selectedPack = msg.pack;

    // Switch header controls to pack selectors
    showPackSelectors();
    syncPackSelectorValues();

    var body = $('#content-body');
    body.textContent = '';

    // Breadcrumb
    body.appendChild(renderBreadcrumb(msg.input, msg.tower, msg.pack));

    // Error message (D-03)
    var errEl = document.createElement('div');
    errEl.className = 'pack-error';
    errEl.textContent = 'Failed to read pack data. Pack may be offline -- check BMS bitmap.';
    body.appendChild(errEl);

    triggerFlash('error');
}

function handleBitmapCellClick(towerIndex, packIndex, isOnline) {
    if (!isOnline) {
        showBitmapWarning('Pack ' + (packIndex + 1) + ' is offline -- check BMS bitmap');
        return;
    }
    // Hardcoded single input; tower maps directly from towerIndex (D-06)
    var input = 1;
    var tower = towerIndex + 1;
    var pack = packIndex + 1;
    sendSelectPack(input, tower, pack);
}

function showBitmapWarning(message) {
    // Remove any existing warning
    var existing = document.querySelector('.bitmap-warning');
    if (existing) existing.parentNode.removeChild(existing);

    // Find the bitmap group card to append warning below
    var bitmapGroups = document.querySelectorAll('.bitmap-group');
    if (bitmapGroups.length === 0) return;
    var target = bitmapGroups[bitmapGroups.length - 1];

    var warn = document.createElement('div');
    warn.className = 'bitmap-warning';
    warn.textContent = message;
    target.appendChild(warn);

    // Auto-remove after 3 seconds
    setTimeout(function() {
        if (warn.parentNode) warn.parentNode.removeChild(warn);
    }, 3000);
}

function sendSelectPack(input, tower, pack) {
    packViewState.selectedInput = input;
    packViewState.selectedTower = tower;
    packViewState.selectedPack = pack;

    // Show loading state
    showPackLoading();

    // Switch header controls
    showPackSelectors();
    syncPackSelectorValues();

    // Send WS message
    var msg = {
        type: 'select_pack',
        section: 'bms',
        input: input,
        tower: tower,
        pack: pack
    };
    if (App.ws && App.ws.ws && App.ws.ws.readyState === WebSocket.OPEN) {
        App.ws.send(msg);
    }
}

function showPackLoading() {
    var body = $('#content-body');
    body.textContent = '';

    // Add breadcrumb even during loading
    body.appendChild(renderBreadcrumb(
        packViewState.selectedInput,
        packViewState.selectedTower,
        packViewState.selectedPack
    ));

    var loading = document.createElement('div');
    loading.className = 'pack-loading';

    var spinner = document.createElement('div');
    spinner.className = 'loading__spinner';
    loading.appendChild(spinner);

    var text = document.createElement('p');
    text.className = 'loading__text';
    text.textContent = 'Reading pack data...';
    loading.appendChild(text);

    body.appendChild(loading);
}

// === Pack Selector Dropdowns (D-07) ===

function initPackSelectors() {
    // Create pack selector controls container in content header
    var headerControls = document.querySelector('.content__header-controls');
    if (!headerControls) return;

    var container = document.createElement('div');
    container.id = 'pack-selector-controls';
    container.className = 'pack-selector-controls';
    container.style.display = 'none';

    // Input dropdown
    var inputLabel = document.createElement('span');
    inputLabel.className = 'pack-selector-label';
    inputLabel.textContent = 'Input:';
    container.appendChild(inputLabel);

    var inputSel = document.createElement('select');
    inputSel.id = 'pack-input-select';
    inputSel.className = 'pv-channel-select';
    container.appendChild(inputSel);

    // Tower dropdown
    var towerLabel = document.createElement('span');
    towerLabel.className = 'pack-selector-label';
    towerLabel.textContent = 'Tower:';
    container.appendChild(towerLabel);

    var towerSel = document.createElement('select');
    towerSel.id = 'pack-tower-select';
    towerSel.className = 'pv-channel-select';
    container.appendChild(towerSel);

    // Pack dropdown
    var packLabel = document.createElement('span');
    packLabel.className = 'pack-selector-label';
    packLabel.textContent = 'Pack:';
    container.appendChild(packLabel);

    var packSel = document.createElement('select');
    packSel.id = 'pack-pack-select';
    packSel.className = 'pv-channel-select';
    container.appendChild(packSel);

    // Insert before auto-refresh button
    var autoBtn = $('#btn-auto-refresh');
    headerControls.insertBefore(container, autoBtn);

    // On change: select new pack
    inputSel.addEventListener('change', onPackSelectorChange);
    towerSel.addEventListener('change', onPackSelectorChange);
    packSel.addEventListener('change', onPackSelectorChange);
}

function populatePackSelectorOptions() {
    var inputSel = $('#pack-input-select');
    var towerSel = $('#pack-tower-select');
    var packSel = $('#pack-pack-select');
    if (!inputSel || !towerSel || !packSel) return;

    // Clear existing options
    inputSel.textContent = '';
    towerSel.textContent = '';
    packSel.textContent = '';

    var inputOpt = document.createElement('option');
    inputOpt.value = '1';
    inputOpt.textContent = '1';
    inputSel.appendChild(inputOpt);
    for (var t = 1; t <= packViewState.topologyTowers; t++) {
        var opt = document.createElement('option');
        opt.value = String(t);
        opt.textContent = String(t);
        towerSel.appendChild(opt);
    }
    for (var p = 1; p <= packViewState.topologyPacks; p++) {
        var opt = document.createElement('option');
        opt.value = String(p);
        opt.textContent = String(p);
        packSel.appendChild(opt);
    }
}

function syncPackSelectorValues() {
    populatePackSelectorOptions();
    var inputSel = $('#pack-input-select');
    var towerSel = $('#pack-tower-select');
    var packSel = $('#pack-pack-select');
    if (!inputSel) return;

    inputSel.value = String(packViewState.selectedInput);
    towerSel.value = String(packViewState.selectedTower);
    packSel.value = String(packViewState.selectedPack);
}

function onPackSelectorChange() {
    var input = parseInt($('#pack-input-select').value, 10);
    var tower = parseInt($('#pack-tower-select').value, 10);
    var pack = parseInt($('#pack-pack-select').value, 10);
    sendSelectPack(input, tower, pack);
}

function showPackSelectors() {
    var packControls = $('#pack-selector-controls');
    if (packControls) packControls.style.display = '';
}

function hidePackSelectors() {
    var packControls = $('#pack-selector-controls');
    if (packControls) packControls.style.display = 'none';
}

// === Breadcrumb Navigation (D-05) ===

function renderBreadcrumb(input, tower, pack) {
    var bar = document.createElement('div');
    bar.className = 'breadcrumb-bar';

    // Left side: breadcrumb segments
    var segments = document.createElement('div');
    segments.className = 'breadcrumb-segments';

    // "Battery" segment - clickable, navigates to Battery section
    var batteryLink = document.createElement('span');
    batteryLink.className = 'breadcrumb-link';
    batteryLink.textContent = 'Battery';
    batteryLink.addEventListener('click', function() { navigateToSection('battery'); });
    segments.appendChild(batteryLink);

    appendBreadcrumbSeparator(segments);

    // "Input N" segment - clickable, returns to BMS overview
    var inputLink = document.createElement('span');
    inputLink.className = 'breadcrumb-link';
    inputLink.textContent = 'Input ' + input;
    inputLink.addEventListener('click', function() { returnToBMSOverview(); });
    segments.appendChild(inputLink);

    appendBreadcrumbSeparator(segments);

    // "Tower M" segment - clickable, returns to BMS overview
    var towerLink = document.createElement('span');
    towerLink.className = 'breadcrumb-link';
    towerLink.textContent = 'Tower ' + tower;
    towerLink.addEventListener('click', function() { returnToBMSOverview(); });
    segments.appendChild(towerLink);

    appendBreadcrumbSeparator(segments);

    // "Pack P" segment - current location, not clickable
    var packText = document.createElement('span');
    packText.className = 'breadcrumb-current';
    packText.textContent = 'Pack ' + pack;
    segments.appendChild(packText);

    bar.appendChild(segments);

    // Right side: "Back to BMS" button
    var backBtn = document.createElement('span');
    backBtn.className = 'breadcrumb-back';
    backBtn.textContent = 'Back to BMS';
    backBtn.addEventListener('click', function() { returnToBMSOverview(); });
    bar.appendChild(backBtn);

    return bar;
}

function appendBreadcrumbSeparator(parent) {
    var sep = document.createElement('span');
    sep.className = 'breadcrumb-separator';
    sep.textContent = '>';
    parent.appendChild(sep);
}

function returnToBMSOverview() {
    packViewState.mode = 'overview';
    packViewState.selectedInput = 0;
    packViewState.selectedTower = 0;
    packViewState.selectedPack = 0;
    // Hide pack selectors
    hidePackSelectors();
    // Re-subscribe to BMS to get overview data
    navigateToSection('bms');
}

// === Pack Detail Renderer (D-06) ===

function renderPackDetail(msg) {
    var body = $('#content-body');
    body.textContent = '';

    // Breadcrumb bar
    body.appendChild(renderBreadcrumb(msg.input, msg.tower, msg.pack));

    // Render each group
    var groups = msg.groups || [];
    for (var i = 0; i < groups.length; i++) {
        var group = groups[i];

        if (group.type === 'cell_grid') {
            body.appendChild(renderCellVoltageGrid(group));
        } else if (group.type === 'pack_status') {
            body.appendChild(renderPackStatusCard(group));
        } else if (group.type === 'balance') {
            body.appendChild(renderBalanceState(group));
        } else if (group.temp_raw && group.temp_raw.length > 0) {
            body.appendChild(renderPackTemperatures(group));
        } else {
            body.appendChild(renderGroupCard(group));
        }
    }

    // Timestamp
    if (msg.timestamp) {
        var ts = $('#content-timestamp');
        var d = new Date(msg.timestamp);
        ts.textContent = 'Last updated: ' + d.toLocaleTimeString();
        ts.style.display = '';
    }
}

// === Cell Voltage Grid (D-08, D-09, D-10, D-11) ===

function renderCellVoltageGrid(group) {
    var container = document.createElement('div');
    container.className = 'group-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    container.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    container.appendChild(sep);

    var cells = group.cells || [];
    if (cells.length === 0) {
        var empty = document.createElement('div');
        empty.textContent = 'No cell voltage data';
        container.appendChild(empty);
        return container;
    }

    // Compute statistics
    var sum = 0;
    for (var i = 0; i < cells.length; i++) { sum += cells[i]; }
    var avg = sum / cells.length;

    var minVal = group.min_cell || cells[0];
    var maxVal = group.max_cell || cells[0];
    var minIdx = group.min_cell_index || 1;
    var maxIdx = group.max_cell_index || 1;
    var spread = maxVal - minVal;

    // Summary row (D-10)
    var summary = document.createElement('div');
    summary.className = 'cell-summary';

    var items = [
        { label: 'Min', value: (minVal / 1000).toFixed(3) + 'V (Cell ' + minIdx + ')' },
        { label: 'Max', value: (maxVal / 1000).toFixed(3) + 'V (Cell ' + maxIdx + ')' },
        { label: 'Spread', value: spread + 'mV', cls: spread <= 30 ? 'spread--ok' : spread <= 50 ? 'spread--warn' : 'spread--danger' },
        { label: 'Avg', value: (avg / 1000).toFixed(3) + 'V' }
    ];

    for (var s = 0; s < items.length; s++) {
        var item = document.createElement('div');
        item.className = 'cell-summary-item';
        var lbl = document.createElement('span');
        lbl.className = 'cell-summary-label';
        lbl.textContent = items[s].label;
        item.appendChild(lbl);
        var val = document.createElement('span');
        val.className = 'cell-summary-value';
        if (items[s].cls) val.className += ' ' + items[s].cls;
        val.textContent = items[s].value;
        item.appendChild(val);
        summary.appendChild(item);
    }
    container.appendChild(summary);

    // Cell voltage grid (D-08, D-09)
    var grid = document.createElement('div');
    grid.className = 'cell-grid';

    for (var c = 0; c < cells.length; c++) {
        var cellDiv = document.createElement('div');
        var dev = Math.abs(cells[c] - avg);
        var cls = 'cell-voltage';
        if (dev <= 5) cls += ' cell--good';
        else if (dev <= 20) cls += ' cell--warn';
        else cls += ' cell--danger';
        cellDiv.className = cls;

        var numSpan = document.createElement('span');
        numSpan.className = 'cell-number';
        numSpan.textContent = 'Cell ' + (c + 1);
        cellDiv.appendChild(numSpan);

        var voltSpan = document.createElement('span');
        voltSpan.className = 'cell-value data-row-h__value';
        voltSpan.textContent = (cells[c] / 1000).toFixed(3) + 'V';

        // Phase 10: Cell voltage tooltip data attributes (D-15)
        if (group.cell_addrs && group.cell_addrs[c]) {
            var cellAddrHex = '0x' + group.cell_addrs[c].toString(16).toUpperCase().padStart(4, '0');
            voltSpan.setAttribute('data-register-addr', cellAddrHex);
            voltSpan.setAttribute('data-register-raw', String(cells[c]));
        }

        cellDiv.appendChild(voltSpan);

        grid.appendChild(cellDiv);
    }
    container.appendChild(grid);

    return container;
}

// === Pack Temperatures (D-16, D-17, D-18) ===

function renderPackTemperatures(group) {
    // Render standard group card first
    var card = renderGroupCard(group);

    // Apply temperature color coding to value elements based on temp_raw
    var tempRaw = group.temp_raw || [];
    var valueEls = card.querySelectorAll('.data-row-h__value');

    for (var i = 0; i < Math.min(tempRaw.length, valueEls.length); i++) {
        var tempC = tempRaw[i] / 10.0;
        if (tempC > 55 || tempC < -10) {
            valueEls[i].classList.add('temp--critical');
        } else if ((tempC > 45 && tempC <= 55) || (tempC < 0 && tempC >= -10)) {
            valueEls[i].classList.add('temp--elevated');
        } else {
            valueEls[i].classList.add('temp--normal');
        }
    }

    return card;
}

// === Pack Status Card (D-12, D-13) ===

function renderPackStatusCard(group) {
    var card = document.createElement('div');

    var alarm = group.alarm || 0;
    var protection = group.protection || 0;
    var fault = group.fault || 0;
    var alarm2 = group.alarm2 || 0;
    var protection2 = group.protection2 || 0;
    var fault2 = group.fault2 || 0;
    var decoded = group.decoded || [];

    var allClear = (alarm === 0 && protection === 0 && fault === 0 &&
                    alarm2 === 0 && protection2 === 0 && fault2 === 0);

    if (allClear) {
        card.className = 'fault-card fault-card--clear';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u2713 Pack Status';
        card.appendChild(heading);

        var clearText = document.createElement('p');
        clearText.className = 'fault-card__clear-text';
        clearText.textContent = 'All clear -- no alarms, protections, or faults';
        card.appendChild(clearText);
    } else {
        card.className = 'fault-card fault-card--active';

        var heading = document.createElement('h3');
        heading.className = 'fault-card__heading';
        heading.textContent = '\u26A0 Pack Status';
        card.appendChild(heading);

        var list = document.createElement('div');
        list.className = 'fault-card__list';

        if (decoded.length > 0) {
            for (var i = 0; i < decoded.length; i++) {
                var item = document.createElement('div');
                item.className = 'fault-card__item';
                var text = decoded[i].toLowerCase();
                if (text.indexOf('protection') !== -1 || text.indexOf('fault') !== -1) {
                    item.style.color = '#c62828';
                }
                item.textContent = '\u2022 ' + decoded[i];
                list.appendChild(item);
            }
        } else {
            // Hex fallback if decoded is empty but bitmaps non-zero
            var hexItems = [];
            if (alarm !== 0) hexItems.push('Alarm: 0x' + alarm.toString(16).toUpperCase().padStart(4, '0'));
            if (protection !== 0) hexItems.push('Protection: 0x' + protection.toString(16).toUpperCase().padStart(4, '0'));
            if (fault !== 0) hexItems.push('Fault: 0x' + fault.toString(16).toUpperCase().padStart(4, '0'));
            if (alarm2 !== 0) hexItems.push('Alarm 2: 0x' + alarm2.toString(16).toUpperCase().padStart(4, '0'));
            if (protection2 !== 0) hexItems.push('Protection 2: 0x' + protection2.toString(16).toUpperCase().padStart(4, '0'));
            if (fault2 !== 0) hexItems.push('Fault 2: 0x' + fault2.toString(16).toUpperCase().padStart(4, '0'));
            for (var h = 0; h < hexItems.length; h++) {
                var hexItem = document.createElement('div');
                hexItem.className = 'fault-card__item';
                hexItem.textContent = '\u2022 ' + hexItems[h];
                list.appendChild(hexItem);
            }
        }
        card.appendChild(list);
    }

    return card;
}

// === Balance State (D-14) ===

function renderBalanceState(group) {
    var container = document.createElement('div');
    container.className = 'group-card';

    var heading = document.createElement('h3');
    heading.className = 'group-card__name';
    heading.textContent = group.name;
    container.appendChild(heading);

    var sep = document.createElement('hr');
    sep.className = 'group-card__separator';
    container.appendChild(sep);

    var bitmap = group.balance_bitmap || 0;
    if (bitmap === 0) {
        var balanced = document.createElement('div');
        balanced.className = 'balance-status balance-status--ok';
        balanced.textContent = 'Balanced';
        container.appendChild(balanced);
    } else {
        var status = document.createElement('div');
        status.className = 'balance-status balance-status--active';
        status.textContent = 'Balancing Active';
        container.appendChild(status);

        var pills = document.createElement('div');
        pills.className = 'balance-pills';
        for (var i = 0; i < 24; i++) {
            if (bitmap & (1 << i)) {
                var pill = document.createElement('span');
                pill.className = 'balance-pill';
                pill.textContent = 'Cell ' + (i + 1);
                pills.appendChild(pill);
            }
        }
        container.appendChild(pills);
    }

    return container;
}
