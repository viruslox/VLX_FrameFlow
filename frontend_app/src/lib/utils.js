export function formatBytes(bytes) {
  if (typeof bytes !== 'number' || isNaN(bytes)) return "0 B";
  if (bytes <= 0) return "0 B"; // Handle negative bytes as 0 B or however appropriate
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const validIndex = Math.max(0, Math.min(i, sizes.length - 1));
  return parseFloat((bytes / Math.pow(k, validIndex)).toFixed(2)) + " " + sizes[validIndex];
}

export function parseServiceStatus(rawStatus) {
    if (!rawStatus) return { state: 'unknown', color: 'orange', label: 'Unknown' };

    const noAnsi = rawStatus.toString().replace(/\x1b\[[0-9;]*m/g, '');
    const cleanStatus = noAnsi.trim().toLowerCase();

    // Bonding specific matches
    if (cleanStatus.includes('mptcp proxy') && cleanStatus.includes('mlvpn tunnel')) {
        const mptcpActive = cleanStatus.includes('mptcp proxy (shadowsocks): active');
        const mlvpnActive = cleanStatus.includes('mlvpn tunnel (mlvpn0): connected');
        if (mptcpActive && mlvpnActive) return { state: 'active', color: 'green', label: 'Online' };
        if (!mptcpActive && !mlvpnActive) return { state: 'inactive', color: 'gray', label: 'Offline' };
        return { state: 'degraded', color: 'yellow', label: 'Degraded' };
    }

    // Partial matches for long systemctl status outputs
    if (cleanStatus.includes('active: active')) return { state: 'active', color: 'green', label: 'Online' };
    if (cleanStatus.includes('active: inactive')) return { state: 'inactive', color: 'gray', label: 'Offline' };
    if (cleanStatus.includes('active: failed')) return { state: 'failed', color: 'orange', label: 'Error' };
    if (cleanStatus.includes('active: activating') || cleanStatus.includes('active: deactivating') || cleanStatus.includes('active: reloading')) return { state: 'transitioning', color: 'yellow', label: 'Working...' };

    // Cameraman matches
    if (cleanStatus.includes('no active cameraman services running')) return { state: 'inactive', color: 'gray', label: 'Offline' };
    if (cleanStatus.includes('api: active')) return { state: 'active', color: 'green', label: 'Online' };
    if (cleanStatus.includes('api: not found')) return { state: 'inactive', color: 'gray', label: 'Offline' };
    if (cleanStatus.includes('running')) return { state: 'active', color: 'green', label: 'Online' };
    if (cleanStatus.includes('db: stopped')) return { state: 'inactive', color: 'gray', label: 'Offline' };


    switch (cleanStatus) {
        case 'active':
            return { state: 'active', color: 'green', label: 'Online' };
        case 'inactive':
            return { state: 'inactive', color: 'gray', label: 'Offline' };
        case 'failed':
            return { state: 'failed', color: 'orange', label: 'Error' };
        case 'activating':
        case 'deactivating':
        case 'reloading':
            return { state: 'transitioning', color: 'yellow', label: 'Working...' };
        default:
            return { state: 'unknown', color: 'orange', label: cleanStatus };
    }
}
