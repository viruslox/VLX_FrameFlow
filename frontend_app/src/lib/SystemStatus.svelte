<script>
  export let usage = { cpu: 0, ram: 0, swap: 0, temps: {} };
</script>

<div class="card">
  <h2 style="text-align: center;">System Usage</h2>
  <div class="metrics">
    <div class="metric left">CPU: {usage.cpu.toFixed(1)}%</div>
    <div class="metric center">RAM: {usage.ram.toFixed(1)}%</div>
    <div class="metric right">Swap: {usage.swap.toFixed(1)}%</div>
  </div>
  
  {#if usage.temps && Object.keys(usage.temps).length > 0}
    <div class="metrics temps">
      {#each Object.entries(usage.temps) as [name, temp]}
        <div class="metric temp-item">{name}: {temp.toFixed(1)}°C</div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .card {
    background: #f4f4f4;
    padding: 1rem;
    border-radius: 8px;
    margin-bottom: 1rem;
    text-align: center;
  }
  h2 {
    margin-top: 0;
  }
  .metrics {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .metric {
    flex: 1;
    font-size: 1.1rem;
    font-weight: 500;
  }
  .left {
    text-align: left;
  }
  .center {
    text-align: center;
  }
  .right {
    text-align: right;
  }
  .temps {
    margin-top: 0.8rem;
    padding-top: 0.8rem;
    border-top: 1px solid #ccc;
    font-size: 0.95rem;
    color: #555;
    flex-wrap: wrap;
    justify-content: center;
    gap: 1rem;
  }
  .temp-item {
    flex: 0 0 auto;
    text-align: center;
  }
  @media (prefers-color-scheme: dark) {
    .card {
      background: #333;
    }
    .temps {
      border-top-color: #555;
      color: #aaa;
    }
  }
</style>
