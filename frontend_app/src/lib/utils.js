export function formatBytes(bytes) {
  if (typeof bytes !== 'number' || isNaN(bytes)) return "0 B";
  if (bytes <= 0) return "0 B"; // Handle negative bytes as 0 B or however appropriate
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const validIndex = Math.max(0, Math.min(i, sizes.length - 1));
  return parseFloat((bytes / Math.pow(k, validIndex)).toFixed(2)) + " " + sizes[validIndex];
}

export function parseServiceStatus(outputString) {
  const output = typeof outputString === "string" ? outputString.toLowerCase() : "";

  // Use regex to find specific status patterns, ignoring subsequent logs
  if (/(?:active: inactive|active: failed|status: inactive|process is dead|service is stopped|state:\s*(?:stopped|inactive|dead))/i.test(output)) {
     return "stopped";
  } else if (/(?:active: active|status: active|service is running|command executed successfully|state:\s*(?:running|active))/i.test(output)) {
     return "running";
  } else {
     // Fallback to word boundaries if specific patterns aren't found,
     // prioritize "stopped" states first.
     if (/\b(?:stopped|inactive|dead)\b/.test(output)) {
         return "stopped";
     } else if (/\b(?:running|active|executed)\b/.test(output)) {
         return "running";
     }

     return "running"; // fallback if command succeeds but output is unknown format
  }
}
