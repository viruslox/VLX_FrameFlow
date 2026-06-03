import { writable } from "svelte/store";

export const telemetryStore = writable({
  networkInterfaces: {},
  systemUsage: { cpu: 0, ram: 0, swap: 0 },
  wifiMode: "Not found",
});

export const connectionStatus = writable("disconnected");

export async function connectWebSocket() {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

  const connect = async () => {
    connectionStatus.set("connecting");

    let ticket = "";
    try {
      const res = await fetch("/api/ws/ticket");
      if (res.ok) {
        const data = await res.json();
        ticket = data.ticket;
      } else {
        console.error("Failed to get WebSocket ticket:", res.status);
        setTimeout(connect, 3000);
        return;
      }
    } catch (err) {
      console.error("Error fetching WebSocket ticket:", err);
      setTimeout(connect, 3000);
      return;
    }

    const wsUrl = `${wsProtocol}//${window.location.host}/ws?ticket=${encodeURIComponent(ticket)}`;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      connectionStatus.set("connected");
      console.log("WebSocket connected");
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === "system_network_stats") {
          telemetryStore.update((store) => {
            return {
              ...store,
              networkInterfaces: data.data.network_interfaces || {},
              systemUsage: data.data.system_usage || { cpu: 0, ram: 0, swap: 0 },
              wifiMode: data.data.wifi_mode || "Not found",
            };
          });
        }
      } catch (err) {
        console.error("Failed to parse WebSocket message:", err);
      }
    };

    ws.onclose = () => {
      connectionStatus.set("disconnected");
      console.log("WebSocket disconnected. Reconnecting in 3s...");
      setTimeout(connect, 3000);
    };

    ws.onerror = (err) => {
      console.error("WebSocket error:", err);
      ws.close();
    };
  };

  connect();
}
