<script>
  import { onMount, onDestroy } from "svelte";
  import AnsiToHtml from "ansi-to-html";
  import { parseServiceStatus } from "./utils.js";

  const ansiConvert = new AnsiToHtml({ escapeXML: true });
  let consoleOutput = "";

  const modules = [
    { id: 'client', name: "Network Client", endpoints: { start: "/api/v1/client/start", stop: "/api/v1/client/stop", status: "/api/v1/client/status", reset: "/api/v1/client/reset" } },
    { id: 'ap', name: "Access Point", endpoints: { start: "/api/v1/ap/start", stop: "/api/v1/ap/stop", status: "/api/v1/ap/status" } },
    { id: 'bonding', name: "Bonding MPTCP", endpoints: { start: "/api/v1/bonding/start", stop: "/api/v1/bonding/stop", status: "/api/v1/bonding/status" } },
    { id: 'gps', name: "GPS Tracking", endpoints: { start: "/api/v1/gps/start", stop: "/api/v1/gps/stop", status: "/api/v1/gps/status" } },
    { id: 'mediamtx', name: "MediaMTX Core", endpoints: { start: "/api/v1/mediamtx/start", stop: "/api/v1/mediamtx/stop", status: "/api/v1/mediamtx/status" } },
    { id: 'cameraman', name: "Cameraman", endpoints: { start: "/api/v1/stream/start", stop: "/api/v1/stream/stop", status: "/api/v1/stream/status" } },
  ];

  let serviceStates = {};
  let pollingInterval;

  const fetchStatuses = async () => {
    try {
      const results = await Promise.all(
        modules.map(m => fetch(m.endpoints.status).then(res => res.json()).catch(() => ({ status: 'error' })))
      );

      let newStates = {};
      results.forEach((res, index) => {
        const modId = modules[index].id;
        newStates[modId] = parseServiceStatus(res.status || 'unknown');
      });
      serviceStates = newStates;
    } catch (err) {
      console.error("Polling failed", err);
    }
  };

  const execCommand = async (moduleName, endpoint, actionName) => {
    try {
      logToConsole(`[${moduleName}] Initiating ${actionName}...`);
      const res = await fetch(endpoint, { method: "POST" });
      const data = await res.json();
      logToConsole(`[${moduleName}] Success: ${data.status || 'Done'}`);
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

  onMount(() => {
    fetchStatuses();
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
    padding: 0.25rem 0.75rem;
    border-radius: 12px;
    font-weight: bold;
    font-size: 0.85rem;
    text-transform: uppercase;
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
  .console-box {
    background: #000;
    color: #ddd;
    font-family: monospace;
    padding: 1rem;
    height: 250px;
    overflow-y: auto;
    border-radius: 8px;
    margin-top: 2rem;
  }
</style>

<div>
  <h2>Control Panel</h2>
  <div class="module-grid">
    {#each modules as mod}
      <div class="module-card">
        <div class="module-header">
          <h3>{mod.name}</h3>
          {#if serviceStates[mod.id]}
            <span class="status-badge" style="background-color: {serviceStates[mod.id].color}; color: black;">
              {serviceStates[mod.id].label}
            </span>
          {/if}
        </div>

        <div class="btn-group">
          <button on:click={() => execCommand(mod.name, mod.endpoints.start, 'Start')}>Start</button>
          <button on:click={() => execCommand(mod.name, mod.endpoints.stop, 'Stop')}>Stop</button>
          {#if mod.endpoints.reset}
            <button on:click={() => execCommand(mod.name, mod.endpoints.reset, 'Reset')}>Reset</button>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <h2>System Logs</h2>
  <div class="console-box">
    {@html consoleOutput}
  </div>
</div>