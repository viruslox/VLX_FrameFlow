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
    if (!rawStatus) return { state: 'unknown', color: 'gray', label: 'Unknown' };

    const cleanStatus = rawStatus.toString().trim().toLowerCase();

    switch (cleanStatus) {
        case 'active':
            return { state: 'active', color: 'green', label: 'Online' };
        case 'inactive':
            return { state: 'inactive', color: 'gray', label: 'Offline' };
        case 'failed':
            return { state: 'failed', color: 'red', label: 'Error' };
        case 'activating':
        case 'deactivating':
        case 'reloading':
            return { state: 'transitioning', color: 'yellow', label: 'Working...' };
        default:
            return { state: 'unknown', color: 'orange', label: cleanStatus };
    }
}
