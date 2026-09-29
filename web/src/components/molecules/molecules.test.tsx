import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '../../lib/api';
import type { Transfer } from '../../state/uploads';
import { file } from '../../test/fixtures';
import { DeviceName } from './DeviceName';
import { FileRow } from './FileRow';
import { StorageMeter } from './StorageMeter';
import { Tabs } from './Tabs';
import { TransferCard } from './TransferCard';

const now = Date.parse('2026-09-20T12:00:00Z');

describe('FileRow', () => {
  it('links to the download and asks before deleting', async () => {
    const onDelete = vi.fn();
    const onDownload = vi.fn();
    render(<FileRow file={file()} now={now} layout="compact" onDelete={onDelete} onDownload={onDownload} />);

    const link = screen.getByRole('link', { name: 'Download contract-signed-v2.pdf' });
    expect(link).toHaveAttribute('href', '/api/v1/files/f1/content');
    expect(link).toHaveAttribute('download', 'contract-signed-v2.pdf');
    expect(screen.getByText(/1.4 MB · still-pine · 6m/)).toBeInTheDocument();

    const del = screen.getByRole('button', { name: 'Delete contract-signed-v2.pdf' });
    await userEvent.click(del);
    expect(onDelete).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: 'Confirm: Delete contract-signed-v2.pdf' }));
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it('shows a checkbox instead of actions while selecting', async () => {
    const onToggle = vi.fn();
    render(
      <FileRow
        file={file()}
        now={now}
        layout="wide"
        selecting
        onToggle={onToggle}
        onDelete={vi.fn()}
        onDownload={vi.fn()}
      />,
    );
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('checkbox', { name: 'Select contract-signed-v2.pdf' }));
    expect(onToggle).toHaveBeenCalledWith('f1');
  });
});

describe('DeviceName', () => {
  it('renames inline and shows server errors', async () => {
    const onRename = vi
      .fn<(name: string) => Promise<void>>()
      .mockRejectedValueOnce(new ApiError(409, 'name_taken', 'Another device already uses that name.'));
    render(<DeviceName name="quiet-otter" onRename={onRename} />);
    await userEvent.click(screen.getByRole('button', { name: /rename/i }));
    const input = screen.getByRole('textbox', { name: 'Device name' });
    await userEvent.clear(input);
    await userEvent.type(input, 'Work laptop{Enter}');
    expect(onRename).toHaveBeenCalledWith('Work laptop');
    expect(await screen.findByRole('alert')).toHaveTextContent('Another device already uses that name.');

    onRename.mockResolvedValueOnce(undefined);
    await userEvent.click(screen.getByRole('button', { name: 'Save name' }));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('cancels with Escape', async () => {
    const onRename = vi.fn<(name: string) => Promise<void>>();
    render(<DeviceName name="quiet-otter" onRename={onRename} />);
    await userEvent.click(screen.getByRole('button', { name: /rename/i }));
    await userEvent.keyboard('{Escape}');
    expect(screen.getByRole('button', { name: /rename/i })).toBeInTheDocument();
    expect(onRename).not.toHaveBeenCalled();
  });
});

describe('TransferCard', () => {
  const base: Transfer = {
    id: 't1',
    name: 'IMG_1234.mov',
    size: 1_200_000_000,
    status: 'uploading',
    sent: 412_000_000,
    rate: 58e6,
    message: null,
  };
  const actions = { onResume: vi.fn(), onCancel: vi.fn(), onRetry: vi.fn(), onDismiss: vi.fn() };

  it('shows progress while sending', () => {
    render(<TransferCard transfer={base} showKeepOpen {...actions} />);
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '34');
    expect(screen.getByText(/412 MB of 1.2 GB · 58 MB\/s · 14s left/)).toBeInTheDocument();
    expect(screen.getByText('Keep this tab open until it finishes.')).toBeInTheDocument();
  });

  it('offers to resume when paused', async () => {
    render(<TransferCard transfer={{ ...base, status: 'paused' }} showKeepOpen {...actions} />);
    expect(screen.getByText('Paused — 412 MB is already across.')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Tap to resume' }));
    expect(actions.onResume).toHaveBeenCalledWith('t1');
  });

  it('explains failures', async () => {
    render(
      <TransferCard
        transfer={{ ...base, status: 'failed', message: 'Not enough space left in the tray.' }}
        showKeepOpen={false}
        {...actions}
      />,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Not enough space left in the tray.');
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(actions.onRetry).toHaveBeenCalledWith('t1');
  });
});

describe('StorageMeter', () => {
  it('warns when nearly full', () => {
    render(<StorageMeter storage={{ used: 9.5e9, reserved: 0, limit: 10e9 }} />);
    expect(screen.getByText('Tray nearly full')).toBeInTheDocument();
  });
});

describe('Tabs', () => {
  it('moves with arrow keys', async () => {
    const onChange = vi.fn();
    const items = [
      { key: 'a', label: 'One' },
      { key: 'b', label: 'Two' },
    ] as const;
    render(<Tabs items={items} active="a" onChange={onChange} label="Sections" idPrefix="t" />);
    screen.getByRole('tab', { name: 'One' }).focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(onChange).toHaveBeenCalledWith('b');
    expect(screen.getByRole('tab', { name: 'One' })).toHaveAttribute('aria-selected', 'true');
  });
});
