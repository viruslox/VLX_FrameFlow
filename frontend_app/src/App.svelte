<script>
  import { onMount } from "svelte";
  import {
    telemetryStore,
    connectionStatus,
    connectWebSocket,
  } from "./lib/ws.js";
  import SystemStatus from "./lib/SystemStatus.svelte";
  import NetworkStatus from "./lib/NetworkStatus.svelte";
  import ControlPanel from "./lib/ControlPanel.svelte";

  onMount(() => {
    connectWebSocket();
  });
</script>

<main>
  <h1>VLX FrameFlow Dashboard</h1>
  <p class="status">Connection Status: {$connectionStatus}</p>

  <div class="vertical-layout">
    <div class="box">
      <SystemStatus usage={$telemetryStore.systemUsage} />
    </div>
    <div class="box">
      <NetworkStatus interfaces={$telemetryStore.networkInterfaces} />
    </div>
    <ControlPanel />
  </div>
</main>

<style>
  main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2rem;
    font-family: sans-serif;
  }
  h1 {
    text-align: center;
    color: #333;
  }
  .status {
    text-align: center;
    font-weight: bold;
    margin-bottom: 2rem;
  }
  .vertical-layout {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    align-items: stretch;
  }
  .box {
    width: 100%;
  }

  @media (prefers-color-scheme: dark) {
    h1 {
      color: #fff;
    }
  }
</style>
