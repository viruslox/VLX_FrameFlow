<script>
  import { formatBytes } from "./utils.js";

  export let interfaces = {};

  $: sortedInterfaces = Object.entries(interfaces).sort(([a], [b]) => {
    if (a === "lo") return -1;
    if (b === "lo") return 1;
    return a.localeCompare(b);
  });
</script>

<div class="card">
  <h2 style="text-align: center;">Network Interfaces</h2>
  {#if sortedInterfaces.length > 0}
    <table>
      <thead>
        <tr>
          <th>Interface</th>
          <th>IPv4 + gateway</th>
          <th>IPv6 + gateway</th>
          <th>Link</th>
          <th>Rx Bytes</th>
          <th>Tx Bytes</th>
        </tr>
      </thead>
      <tbody>
        {#each sortedInterfaces as [iface, stats] (iface)}
          <tr>
            <td>{iface}</td>
            <td>
              {stats.ipv4 || "-"}
              {#if stats.ipv4_gw}<br /><span class="gw"
                  >gw: {stats.ipv4_gw}</span
                >{/if}
            </td>
            <td>
              {stats.ipv6 || "-"}
              {#if stats.ipv6_gw}<br /><span class="gw"
                  >gw: {stats.ipv6_gw}</span
                >{/if}
            </td>
            <td
              class={stats.operstate === "UP"
                ? "link-up"
                : stats.operstate === "DOWN"
                  ? "link-down"
                  : ""}
            >
              {stats.operstate || "UNKNOWN"}
            </td>
            <td>{formatBytes(stats.rx_bytes)}</td>
            <td>{formatBytes(stats.tx_bytes)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p>Loading...</p>
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
  table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 1rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.75rem 0.5rem;
    border-bottom: 1px solid #ddd;
    vertical-align: top;
  }
  .gw {
    font-size: 0.85em;
    color: #666;
  }
  .link-up {
    color: #00aa00;
    font-weight: bold;
  }
  .link-down {
    color: #aa0000;
    font-weight: bold;
  }
  @media (prefers-color-scheme: dark) {
    .card {
      background: #333;
    }
    th,
    td {
      border-bottom: 1px solid #555;
    }
    .gw {
      color: #aaa;
    }
    .link-up {
      color: #55ff55;
    }
    .link-down {
      color: #ff5555;
    }
  }
</style>
