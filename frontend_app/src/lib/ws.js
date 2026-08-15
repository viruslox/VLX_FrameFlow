import { writable } from "svelte/store";

export const telemetryStore = writable({
  networkInterfaces: {},
  systemUsage: { cpu: 0, ram: 0, swap: 0 },
  wifiMode: "Not found",
});

export const connectionStatus = writable("disconnected");

export async function connectWebSocket() {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

  // Resolve direct vs relay mode once. In relay mode the ticket must be
  // obtained from the SBC through the Server relay (api/v1/relay/ws/ticket),
  // since the WebSocket that /ws tunnels to is validated on the SBC. The /ws
  // URL itself is unchanged: the frontend proxy forwards it to the Server,
  // which tunnels it on to the SBC.
  let ticketPath = "api/ws/ticket";
  try {
    const cfgRes = await fetch("config");
    if (cfgRes.ok) {
      const cfg = await cfgRes.json();
      if (cfg.use_relay) {
        ticketPath = "api/v1/relay/ws/ticket";
      }
    }
  } catch (err) {
    // Fall back to direct mode ticket path.
  }

  const connect = async () => {
    connectionStatus.set("connecting");

    let ticket = "";
    try {
      const res = await fetch(ticketPath);
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

    const pathname = window.location.pathname || '/';
    const cleanPath = pathname.endsWith('/') ? pathname.slice(0, -1) : pathname;
    const wsUrl = `${wsProtocol}//${window.location.host}${cleanPath}/ws?ticket=${encodeURIComponent(ticket)}`;

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
