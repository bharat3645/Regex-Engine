import * as runtime from '/wailsjs/runtime/runtime.js';

window.addEventListener('DOMContentLoaded', () => {

    // --- Element Selectors ---
    const scanBtn = document.getElementById('scanBtn');
    const pauseBtn = document.getElementById('pauseBtn');
    const stopBtn = document.getElementById('stopBtn');
    const settingsBtn = document.getElementById('settingsBtn');
    const cpuUsageEl = document.getElementById('cpuUsage');
    const ramUsageEl = document.getElementById('ramUsage');
    const queueDepthEl = document.getElementById('queueDepth');
    const timeElapsedEl = document.getElementById('timeElapsed');
    const logContentEl = document.getElementById('logContent');
    const diskReadEl = document.getElementById('diskRead');
    const diskWriteEl = document.getElementById('diskWrite');

    const progressTextEl = document.getElementById('progressText');
    const progressBarEl = document.getElementById('progressBar');
    const progressPercentEl = document.getElementById('progressPercent');
    const progressFilesEl = document.getElementById('progressFiles');

    const cpuCircle = document.getElementById('cpu-circle');
    const ramCircle = document.getElementById('ram-circle');
    const cpuRadius = cpuCircle.r.baseVal.value;
    const ramRadius = ramCircle.r.baseVal.value;
    const cpuCircumference = cpuRadius * 2 * Math.PI;
    const ramCircumference = ramRadius * 2 * Math.PI;

    cpuCircle.style.strokeDasharray = `${cpuCircumference} ${cpuCircumference}`;
    ramCircle.style.strokeDasharray = `${ramCircumference} ${ramCircumference}`;

    const settingsModal = document.getElementById('settingsModal');
    const closeSettingsBtn = document.getElementById('closeSettingsBtn');
    const saveSettingsBtn = document.getElementById('saveSettingsBtn');
    const rootDirInput = document.getElementById('rootDirInput');
    const rulesDirInput = document.getElementById('rulesDirInput');
    const outputFileInput = document.getElementById('outputFileInput');
    const logLevelInput = document.getElementById('logLevelInput');
    const scannerWorkersInput = document.getElementById('scannerWorkersInput');
    const extractorWorkersInput = document.getElementById('extractorWorkersInput');
    const maxCpuInput = document.getElementById('maxCpuInput');
    const pipelineBufferInput = document.getElementById('pipelineBufferInput');
    const scannerBufferInput = document.getElementById('scannerBufferInput');
    const gcTriggerInput = document.getElementById('gcTriggerInput');
    const discoveryBatchSizeInput = document.getElementById('discoveryBatchSizeInput');
    const maxFileSizeInput = document.getElementById('maxFileSizeInput');
    const ocrToggle = document.getElementById('ocrToggle'); // NEW: OCR Toggle selector

    // --- State & Event Listeners ---
    let startTime, timerInterval;
    let isPaused = false;
    const MAX_LOG_LINES = 200;

    // NEW: Add event listener for the OCR toggle switch
    ocrToggle.addEventListener('change', (event) => {
        const isEnabled = event.target.checked;
        window.go.main.App.SetOCREnabled(isEnabled);
        const status = isEnabled ? 'ENABLED' : 'DISABLED';
        addLogEntry({ Level: "INFO", Msg: `OCR scanning for image-based PDFs has been ${status}.` });
    });

    runtime.EventsOn("scan:progress", (update) => {
        progressTextEl.innerText = `Status: ${update.message}`;
        if (update.totalFiles > 0) {
            const percentage = (update.filesScanned / update.totalFiles) * 100;
            progressBarEl.style.width = `${percentage}%`;
            progressPercentEl.innerText = `${percentage.toFixed(1)}%`;
        }
        progressFilesEl.innerText = `${update.filesScanned.toLocaleString()} / ${update.totalFiles.toLocaleString()} Files`;
    });

    runtime.EventsOn("scan:complete", () => {
        setScanState(false, false);
        progressTextEl.innerText = `Scan complete. Found results in output file.`;
        progressBarEl.style.width = '100%';
        progressPercentEl.innerText = '100.0%';
        clearInterval(timerInterval);
    });

    runtime.EventsOn("scan:starting", () => {
        setScanState(true, false);
        resetProgressUI();
        logContentEl.innerHTML = '';
    });

    runtime.EventsOn("scan:stopped", () => {
        setScanState(false, false);
        progressTextEl.innerText = `Status: Scan stopped by user.`;
        clearInterval(timerInterval);
    });

    runtime.EventsOn("scan:error", (errorMessage) => {
        setScanState(false, false);
        progressTextEl.innerText = `Status: Error! Check logs for details.`;
        addLogEntry({ level: "ERROR", msg: errorMessage });
        clearInterval(timerInterval);
    });

    runtime.EventsOn("scan:paused", () => {
        setScanState(true, true);
        progressTextEl.innerText = `Status: Scan Paused.`;
        addLogEntry({ level: "INFO", msg: "Scan has been paused." });
    });

    runtime.EventsOn("scan:resumed", () => {
        setScanState(true, false);
        progressTextEl.innerText = `Status: Scanning...`;
        addLogEntry({ level: "INFO", msg: "Scan has resumed." });
    });

    runtime.EventsOn("log:message", (log) => { addLogEntry(log); });


    // --- Settings Modal Logic ---
    settingsBtn.addEventListener('click', () => {
        window.go.main.App.GetConfig().then(config => {
            rootDirInput.value = config.root_dir;
            rulesDirInput.value = config.rules_dir;
            outputFileInput.value = config.output_file;
            logLevelInput.value = config.log_level;
            scannerWorkersInput.value = config.scanner_workers;
            extractorWorkersInput.value = config.extractor_workers;
            maxCpuInput.value = config.max_cpu_percentage;
            pipelineBufferInput.value = config.pipeline_buffer_size;
            scannerBufferInput.value = config.scanner_buffer_size_mb;
            gcTriggerInput.value = config.gc_trigger_mb;
            discoveryBatchSizeInput.value = config.discovery_batch_size;
            maxFileSizeInput.value = config.max_file_size_mb;
            settingsModal.style.display = 'flex';
        });
    });

    closeSettingsBtn.addEventListener('click', () => { settingsModal.style.display = 'none'; });
    window.addEventListener('click', (event) => { if (event.target === settingsModal) { settingsModal.style.display = 'none'; } });

    saveSettingsBtn.addEventListener('click', () => {
        const newConfig = {
            root_dir: rootDirInput.value,
            rules_dir: rulesDirInput.value,
            output_file: outputFileInput.value,
            log_level: logLevelInput.value,
            scanner_workers: parseInt(scannerWorkersInput.value, 10),
            extractor_workers: parseInt(extractorWorkersInput.value, 10),
            max_cpu_percentage: parseInt(maxCpuInput.value, 10),
            pipeline_buffer_size: parseInt(pipelineBufferInput.value, 10),
            scanner_buffer_size_mb: parseInt(scannerBufferInput.value, 10),
            gc_trigger_mb: parseInt(gcTriggerInput.value, 10),
            discovery_batch_size: parseInt(discoveryBatchSizeInput.value, 10),
            max_file_size_mb: parseInt(maxFileSizeInput.value, 10),
        };
        window.go.main.App.SaveConfig(newConfig).then(() => {
            addLogEntry({ level: "INFO", msg: "Settings saved successfully." });
            settingsModal.style.display = 'none';
        });
    });

    scanBtn.addEventListener('click', () => {
        startTime = Date.now();
        updateTimer();
        timerInterval = setInterval(updateTimer, 1000);
        window.go.main.App.StartScan();
    });

    stopBtn.addEventListener('click', () => {
        window.go.main.App.StopScan();
    });

    pauseBtn.addEventListener('click', () => {
        if (isPaused) {
            window.go.main.App.ResumeScan();
        } else {
            window.go.main.App.PauseScan();
        }
    });

    function setScanState(isScanning, paused) {
        isPaused = paused;

        scanBtn.style.display = isScanning ? 'none' : 'block';
        stopBtn.style.display = isScanning ? 'block' : 'none';
        pauseBtn.style.display = isScanning ? 'block' : 'none';
        settingsBtn.disabled = isScanning;
        ocrToggle.disabled = isScanning; // Disable OCR toggle during scan

        if (isScanning) {
            if (isPaused) {
                pauseBtn.innerText = 'Resume Scan';
                pauseBtn.classList.remove('warning-btn');
                pauseBtn.classList.add('primary-btn');
            } else {
                pauseBtn.innerText = 'Pause Scan';
                pauseBtn.classList.add('warning-btn');
                pauseBtn.classList.remove('primary-btn');
            }
        }
    }

    function getStats() {
        window.go.main.App.GetSystemStats().then((stats) => {
            cpuUsageEl.innerText = `${stats.cpu_usage.toFixed(1)}%`;
            ramUsageEl.innerText = `${stats.ram_usage_gb.toFixed(2)} GB`;
            setCircularProgress(cpuCircle, cpuCircumference, stats.cpu_usage);
            setCircularProgress(ramCircle, ramCircumference, (stats.ram_usage_gb / 16) * 100); // Assumes 16GB max
            diskReadEl.innerText = `${stats.disk_read_mbps.toFixed(1)} MB/s`;
            diskWriteEl.innerText = `${stats.disk_write_mbps.toFixed(1)} MB/s`;
            queueDepthEl.innerText = `${stats.queue_depth.toLocaleString()}`;
        }).catch(() => { });
    }

    function setCircularProgress(circle, circumference, percent) {
        const offset = circumference - (percent / 100) * circumference;
        circle.style.strokeDashoffset = offset;
    }

    function updateTimer() {
        if (!startTime) return;
        const elapsed = Math.floor((Date.now() - startTime) / 1000);
        timeElapsedEl.innerText = `${elapsed}s`;
    }

    function resetProgressUI() {
        progressTextEl.innerText = 'Status: Initializing...';
        progressBarEl.style.width = '0%';
        progressPercentEl.innerText = '0.0%';
        progressFilesEl.innerText = '0 / 0 Files';
        timeElapsedEl.innerText = '0s';
        getStats();
    }

    function addLogEntry(log) {
        const entry = document.createElement('div');
        entry.className = 'log-entry';
        const time = log.Time ? new Date(log.Time).toLocaleTimeString() : new Date().toLocaleTimeString();
        const level = log.Level || 'INFO';
        entry.innerHTML = `<span class="timestamp">${time}</span> <span class="level-${level.toLowerCase()}">${level}</span> <span class="message">${log.Msg}</span>` +
            (log.Details && Object.keys(log.Details).length > 0 ? ` <span class="details">${JSON.stringify(log.Details)}</span>` : '');
        logContentEl.prepend(entry);
        if (logContentEl.children.length > MAX_LOG_LINES) {
            logContentEl.removeChild(logContentEl.lastChild);
        }
    }

    setInterval(getStats, 1500);
    getStats();
    addLogEntry({ Level: "INFO", Msg: "Application ready. Click 'Start Scan' to begin." });

    // --- Tab Switching Logic ---
    const tabDashboard = document.getElementById('tabDashboard');
    const tabTree = document.getElementById('tabTree');
    const dashboardView = document.getElementById('dashboardView');
    const treeView = document.getElementById('treeView');
    const treeRoot = document.getElementById('treeRoot');

    tabDashboard.addEventListener('click', () => {
        tabDashboard.classList.add('active');
        tabTree.classList.remove('active');
        dashboardView.style.display = 'flex'; // Use flex for view-container
        treeView.style.display = 'none';
    });

    tabTree.addEventListener('click', () => {
        tabTree.classList.add('active');
        tabDashboard.classList.remove('active');
        treeView.style.display = 'flex';
        dashboardView.style.display = 'none';

        // Load tree if empty
        if (treeRoot.children.length <= 1) { // 1 because of loading text
            loadTreeNodes(0, treeRoot);
        }
    });

    // --- Tree View Logic ---
    async function loadTreeNodes(parentId, container) {
        container.innerHTML = '<p class="loading-text" style="font-size:12px; margin-left:20px;">Loading...</p>';
        try {
            // Call Backend API
            const nodes = await window.go.main.App.GetTreeNodes(parentId);
            container.innerHTML = ''; // Clear loading

            if (!nodes || nodes.length === 0) {
                container.innerHTML = '<div class="tree-node" style="margin-left:20px; opacity:0.5;">Empty</div>';
                return;
            }

            nodes.forEach(node => {
                const nodeEl = createTreeNode(node);
                container.appendChild(nodeEl);
            });

        } catch (error) {
            console.error(error);
            container.innerHTML = `<div class="error">Failed to load nodes: ${error}</div>`;
        }
    }

    function createTreeNode(node) {
        const wrapper = document.createElement('div');
        wrapper.className = 'tree-node';
        wrapper.dataset.type = node.type;

        const item = document.createElement('div');
        item.className = 'tree-item';
        if (node.children) item.classList.add('has-children');

        let iconHtml = '<span class="icon"></span>'; // CSS pseudo-element adds the icon

        let riskHtml = '';
        if (node.risk_score > 0) {
            riskHtml = `<span class="risk-badge">Risk: ${node.risk_score.toFixed(1)}</span>`;
        }

        let edgeHtml = '';
        if (node.edges && node.edges.length > 0) {
            // It has lineage edges (e.g. CopyOf)
            const tooltip = node.edges.join('\n');
            edgeHtml = `<span class="edge-badge" title="${tooltip}">🔗 Duplicate</span>`;
        }

        item.innerHTML = `${iconHtml} <span class="name">${node.name}</span> <span style="opacity:0.5; font-size:10px;">(${node.type})</span> ${riskHtml} ${edgeHtml}`;

        wrapper.appendChild(item);

        if (node.children) {
            const childrenContainer = document.createElement('div');
            childrenContainer.className = 'tree-children';
            wrapper.appendChild(childrenContainer);

            let isLoaded = false;
            item.addEventListener('click', (e) => {
                e.stopPropagation();
                item.classList.toggle('expanded');
                childrenContainer.classList.toggle('visible');

                if (node.children && !isLoaded) {
                    loadTreeNodes(node.id, childrenContainer);
                    isLoaded = true;
                }
            });
        }

        return wrapper;
    }
});
