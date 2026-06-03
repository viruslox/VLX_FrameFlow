import { describe, it, expect } from 'vitest';
import { formatBytes, parseServiceStatus } from './utils';

describe('formatBytes', () => {
  it('returns "0 B" for 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('formats bytes correctly', () => {
    expect(formatBytes(500)).toBe('500 B');
    expect(formatBytes(1023)).toBe('1023 B');
  });

  it('formats kilobytes correctly', () => {
    expect(formatBytes(1024)).toBe('1 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1024 * 1024 - 1)).toBe('1024 KB');
  });

  it('formats megabytes correctly', () => {
    expect(formatBytes(1024 * 1024)).toBe('1 MB');
    expect(formatBytes(1024 * 1024 * 2.5)).toBe('2.5 MB');
  });

  it('formats gigabytes correctly', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GB');
    expect(formatBytes(1024 * 1024 * 1024 * 5.123)).toBe('5.12 GB');
  });

  it('formats terabytes correctly', () => {
    expect(formatBytes(1024 * 1024 * 1024 * 1024)).toBe('1 TB');
    expect(formatBytes(1024 * 1024 * 1024 * 1024 * 3.14159)).toBe('3.14 TB');
  });

  it('handles negative bytes by returning "0 B"', () => {
     expect(formatBytes(-1)).toBe('0 B');
     expect(formatBytes(-1024)).toBe('0 B');
  });

  it('handles fractional bytes correctly', () => {
     expect(formatBytes(0.5)).toBe('0.5 B');
     expect(formatBytes(0.99)).toBe('0.99 B');
  });

  it('handles invalid inputs gracefully by returning "0 B"', () => {
     expect(formatBytes(NaN)).toBe('0 B');
     expect(formatBytes(null)).toBe('0 B');
     expect(formatBytes(undefined)).toBe('0 B');
     expect(formatBytes("1024")).toBe('0 B');
     expect(formatBytes({})).toBe('0 B');
  });

  it('formats very large values correctly (PB, EB, etc.)', () => {
     expect(formatBytes(1024 * 1024 * 1024 * 1024 * 1024)).toBe('1 PB');
     expect(formatBytes(1024 * 1024 * 1024 * 1024 * 1024 * 1024)).toBe('1 EB');
     expect(formatBytes(1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024)).toBe('1 ZB');
     expect(formatBytes(1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024)).toBe('1 YB');
     expect(formatBytes(1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024)).toBe('1024 YB');
  });
});

describe('parseServiceStatus', () => {
  it('returns "stopped" for inputs indicating a stopped state', () => {
    expect(parseServiceStatus('Service is stopped.')).toBe('stopped');
    expect(parseServiceStatus('status: INACTIVE')).toBe('stopped');
    expect(parseServiceStatus('process is dead')).toBe('stopped');
  });

  it('returns "running" for inputs indicating an active state', () => {
    expect(parseServiceStatus('Service is running.')).toBe('running');
    expect(parseServiceStatus('Active: active (running)')).toBe('running');
    expect(parseServiceStatus('command executed successfully')).toBe('running');
  });

  it('correctly handles multiline strings with conflicting log words', () => {
    const activeWithStoppedLog = `
● service.service - Some Service
     Loaded: loaded (/lib/systemd/system/service.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2026-05-07 10:00:00 UTC; 1h ago
       Docs: man:service(8)
   Main PID: 1234 (service)
      Tasks: 1 (limit: 4915)
     Memory: 10.0M
        CPU: 50ms
     CGroup: /system.slice/service.service
             └─1234 /usr/bin/service

May 07 10:10:00 host service[1234]: [INFO] worker 1 stopped
May 07 10:10:01 host service[1234]: [INFO] worker 2 started
    `;
    expect(parseServiceStatus(activeWithStoppedLog)).toBe('running');

    const inactiveWithRunningLog = `
● service.service - Some Service
     Loaded: loaded (/lib/systemd/system/service.service; enabled; vendor preset: enabled)
     Active: inactive (dead) since Thu 2026-05-07 10:00:00 UTC; 1h ago
       Docs: man:service(8)

May 07 09:59:00 host service[1234]: [INFO] service was running earlier
May 07 10:00:00 host systemd[1]: service.service: Succeeded.
    `;
    expect(parseServiceStatus(inactiveWithRunningLog)).toBe('stopped');
  });

  it('returns "running" as a fallback for unknown string formats', () => {
    expect(parseServiceStatus('Started successfully')).toBe('running');
    expect(parseServiceStatus('Some random output')).toBe('running');
    expect(parseServiceStatus('')).toBe('running');
  });

  it('returns "running" as a fallback for non-string inputs', () => {
    expect(parseServiceStatus(null)).toBe('running');
    expect(parseServiceStatus(undefined)).toBe('running');
    expect(parseServiceStatus({})).toBe('running');
    expect(parseServiceStatus(123)).toBe('running');
  });
});
