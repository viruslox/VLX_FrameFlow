import { render, fireEvent, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ControlPanel from './ControlPanel.svelte';

describe('ControlPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  it('handles fetch errors correctly and displays them', async () => {
    global.fetch.mockResolvedValue({
      ok: true,
      json: async () => ({ output: "Success" }),
    });

    render(ControlPanel);

    await waitFor(() => {
        expect(global.fetch).toHaveBeenCalledWith('/api/frameflow/client/status', { method: "POST" });
    });

    global.fetch.mockRejectedValue(new Error('Start client network error'));

    const clientHeader = screen.getByRole('heading', { name: 'Network Client' });
    const clientGroup = clientHeader.closest('.module-card');
    const startButton = Array.from(clientGroup.querySelectorAll('button')).find(btn => btn.textContent === 'Start');

    await fireEvent.click(startButton);

    await waitFor(() => {
        expect(screen.getByText(/Start client network error/)).toBeTruthy();
    });
  });
});
