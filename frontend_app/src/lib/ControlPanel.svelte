<script>
  import { onMount, onDestroy } from "svelte";
  import AnsiToHtml from "ansi-to-html";
  import { parseServiceStatus } from "./utils.js";

  const ansiConvert = new AnsiToHtml({ escapeXML: true });
  let consoleOutput = "";

  // In relay mode (frontend fronting a Server/VM) every module call must go
  // through api/v1/relay/... to reach the remote SBC. In direct mode (frontend
  // on the SBC itself) calls hit the local api/... routes. The prefix is chosen
  // at runtime from the backend's /config, so the same build serves both modes.
  let apiBase = "api";

  function ep(path) {
    // path is the module path WITHOUT the leading "api/", e.g. "frameflow/client/status"
    return `${apiBase}/${path}`;
  }

  const modules = [
    { id: 'client', name: "Network Client", paths: { start: "frameflow/client/start", stop: "frameflow/client/stop", status: "frameflow/client/status", reset: "frameflow/client/reset" } },
    { id: 'ap', name: "Access Point", paths: { start: "frameflow/ap/start", stop: "frameflow/ap/stop", status: "frameflow/ap/status" } },
    { id: 'bonding', name: "Bonding", paths: { start: "frameflow/bonding/start", stop: "frameflow/bonding/stop", status: "frameflow/bonding/status" } },
    { id: 'gps', name: "GPS Tracking", paths: { start: "gps/start", stop: "gps/stop", status: "gps/status" } },
    { id: 'mediamtx', name: "MediaMTX Core", paths: { start: "mediamtx/start", stop: "mediamtx/stop", status: "mediamtx/status" } },
    { id: 'cameraman', name: "Cameraman", paths: { start: "cameraman/start", stop: "cameraman/stop", status: "cameraman/status", listDev: "cameraman/list-dev" } },
  ];

  let serviceStates = {};
  let rawStates = {};
  let pollingInterval;

  let availableDevices = [];
  let selectedDevice = "";

  const loadMode = async () => {
    // Decide direct vs relay base from the frontend's own /config. In relay
    // mode all module calls are prefixed with v1/relay/ so they reach the
    // remote SBC through the Server's transparent proxy.
    try {
      const res = await fetch("config");
      const data = await res.json();
      apiBase = data.use_relay ? "api/v1/relay" : "api";
    } catch (err) {
      // Fall back to direct mode if /config is unavailable.
      apiBase = "api";
    }
  };

  const fetchDevList = async () => {
    try {
      const res = await fetch(ep("cameraman/list-dev"), { method: "POST" });
      const data = await res.json();
      const output = data.output || "";

      const vIds = [];
      const aIds = [];

      const lines = output.split('\n');
      let parsingVideo = false;
      let parsingAudio = false;

      for (const line of lines) {
        if (line.includes('[VIDEO DEVICES]')) {
          parsingVideo = true;
          parsingAudio = false;
          continue;
        }
        if (line.includes('[AUDIO DEVICES]')) {
          parsingVideo = false;
          parsingAudio = true;
          continue;
        }

        if (parsingVideo) {
          const match = line.match(/^\s*(V\d+)\s*\|/);
          if (match) vIds.push(match[1]);
        }
        if (parsingAudio) {
          const match = line.match(/^\s*(A\d+)\s*\|/);
          if (match) aIds.push(match[1]);
        }
      }

      const combos = [...vIds, ...aIds];
      if (vIds.length === 0) combos.push("V0");
      if (aIds.length === 0) combos.push("A0");

      availableDevices = combos;
      if (combos.length > 0 && !selectedDevice) {
        selectedDevice = combos[0];
      }
    } catch (err) {
      console.error("Failed to fetch devlist", err);
      availableDevices = ["V0", "A0"];
      if (!selectedDevice) selectedDevice = "V0";
    }
  };

  const logDevList = async () => {
    try {
      const res = await fetch(ep("cameraman/list-dev"), { method: "POST" });
      const data = await res.json();
      logToConsole(data.output);
      fetchDevList(); // refresh dropdown too
    } catch (err) {
      logToConsole(`[Cameraman] Failed to fetch devlist: ${err.message}`, true);
    }
  };

  const fetchStatuses = async () => {
    try {
      const results = await Promise.all(
        modules.map(m => fetch(ep(m.paths.status), { method: "POST" }).then(res => res.json()).catch(() => ({ status: 'error' })))
      );

      let newStates = {};
      let newRawStates = {};
      results.forEach((res, index) => {
        const modId = modules[index].id;
        newStates[modId] = parseServiceStatus(res.status || 'unknown');
        newRawStates[modId] = res.status || 'unknown';
      });
      serviceStates = newStates;
      rawStates = newRawStates;
    } catch (err) {
      console.error("Polling failed", err);
    }
  };

  const execCommand = async (moduleName, endpoint, actionName, payload = null) => {
    try {
      if (moduleName === "Cameraman") {
        if (actionName === "Start" && payload) {
          logToConsole(`Starting stream ${payload.device}...`);
        } else if (actionName === "Stop" && payload) {
          logToConsole(`Stopping stream ${payload.device}...`);
        }
      } else {
        logToConsole(`[${moduleName}] Initiating ${actionName}...`);
      }

      const options = { method: "POST" };
      if (payload) {
        options.headers = { "Content-Type": "application/json" };
        options.body = JSON.stringify(payload);
      }
      const res = await fetch(endpoint, options);
      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'API request failed');
      }

      if (moduleName === "Cameraman" && (actionName === "Start" || actionName === "Stop" || actionName === "Status")) {
        logToConsole(data.status || 'Done');
      } else {
        logToConsole(`[${moduleName}] Success: ${data.status || 'Done'}`);
      }

      fetchStatuses();
    } catch (err) {
      logToConsole(`[${moduleName}] Failed: ${err.message}`, true);
    }
  };

  function logToConsole(msg, isError = false) {
    const timestamp = new Date().toLocaleTimeString();
    const color = isError ? "red" : "lightgreen";
    consoleOutput += `[${timestamp}] <span style="color:${color}">${ansiConvert.toHtml(msg)}</span><br/>`;
  }

  function clearConsole() {
    consoleOutput = "";
  }

  onMount(async () => {
    await loadMode();
    fetchStatuses();
    fetchDevList();
    pollingInterval = setInterval(fetchStatuses, 5000);
  });

  onDestroy(() => clearInterval(pollingInterval));
</script>

<style>
  .module-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 1.5rem;
    padding: 1rem 0;
  }
  .module-card {
    background: #1e1e2e;
    border-radius: 8px;
    padding: 1.5rem;
    box-shadow: 0 4px 6px rgba(0,0,0,0.3);
  }
  .module-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  .module-header h3 {
    margin: 0;
    font-size: 1.2rem;
    color: #fff;
  }
  .status-badge {
    display: inline-block;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.2);
  }
  .btn-group {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  button {
    background: #3a3a5a;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
  }
  button:hover { background: #5a5a8a; }
  select {
    background: #2a2a4a;
    color: white;
    border: 1px solid #4a4a6a;
    padding: 0.5rem;
    border-radius: 4px;
  }
  .console-box {
    background: #000;
    color: #ddd;
    font-family: monospace;
    padding: 1rem;
    height: 250px;
    overflow-y: auto;
    border-radius: 8px;
    margin-top: 2rem;
    white-space: pre-wrap;
  }
  .lcd-display {
    background: #111;
    color: #0f0;
    font-family: monospace;
    padding: 1rem;
    border-radius: 4px;
    border: 2px solid #333;
    margin-bottom: 1rem;
    white-space: pre-wrap;
    box-shadow: inset 0 0 10px rgba(0,0,0,0.8);
  }
  .logs-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .clear-btn {
    background: transparent;
    color: #888;
    border: 1px solid #555;
    padding: 0.25rem 0.5rem;
    font-size: 0.8rem;
  }
</style>

<div>
  <h2>Control Panel</h2>
  <div class="module-grid">
    {#each modules as mod}
      <div class="module-card">
        <div class="module-header">
          <h3>{mod.name}</h3>
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            {#if mod.id === 'cameraman'}
              <button style="padding: 0.25rem 0.5rem; font-size: 0.8rem;" on:click={logDevList}>Devlist</button>
            {/if}
            {#if serviceStates[mod.id]}
              <div class="status-badge" style="background-color: {serviceStates[mod.id].color === 'gray' ? '#555' : serviceStates[mod.id].color};" title="{serviceStates[mod.id].label}"></div>
            {/if}
          </div>
        </div>

        {#if mod.id === 'bonding' && rawStates[mod.id]}
          <div class="lcd-display">
            {@html ansiConvert.toHtml(rawStates[mod.id].split('\n').filter(line => line.includes('MPTCP Proxy') || line.includes('MLVPN Tunnel')).join('\n'))}
          </div>
        {/if}

        <div class="btn-group">
          {#if mod.id === 'cameraman'}
            <select bind:value={selectedDevice}>
              {#each availableDevices as dev}
                <option value={dev}>{dev}</option>
              {/each}
            </select>
          {/if}

          {#if mod.id !== 'bonding'}
            <button on:click={() => execCommand(mod.name, ep(mod.paths.start), 'Start', mod.id === 'cameraman' ? { device: selectedDevice } : null)}>Start</button>
            <button on:click={() => execCommand(mod.name, ep(mod.paths.stop), 'Stop', mod.id === 'cameraman' ? { device: selectedDevice } : null)}>Stop</button>
          {/if}
          {#if mod.id !== 'bonding'}
            <button on:click={() => execCommand(mod.name, ep(mod.paths.status), 'Status', null)}>Status</button>
          {/if}
          {#if mod.paths.reset}
            <button on:click={() => execCommand(mod.name, ep(mod.paths.reset), 'Reset')}>Reset</button>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div class="logs-header">
    <h2>System Logs</h2>
    <button class="clear-btn" on:click={clearConsole}>Clear</button>
  </div>
  <div class="console-box">
    {@html consoleOutput}
  </div>
</div>