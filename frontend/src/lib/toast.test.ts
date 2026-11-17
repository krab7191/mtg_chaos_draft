import { describe, it, expect, vi, beforeEach } from 'vitest';

const sonner = vi.hoisted(() => {
  const fn = vi.fn(() => 'id') as any;
  fn.error = vi.fn(() => 'id');
  fn.success = vi.fn(() => 'id');
  fn.info = vi.fn(() => 'id');
  fn.dismiss = vi.fn();
  return fn;
});
vi.mock('svelte-sonner', () => ({ toast: sonner }));

import { toast, DURATIONS } from './toast.svelte';

beforeEach(() => vi.clearAllMocks());

describe('toast adapter', () => {
  it('routes each type to sonner, keyed by message so repeats collapse', () => {
    toast.show('hi');
    expect(sonner.info).toHaveBeenCalledWith('hi', { id: 'info:hi', duration: DURATIONS.info });
    toast.error('bad');
    expect(sonner.error).toHaveBeenCalledWith('bad', { id: 'error:bad', duration: DURATIONS.error });
    toast.success('yay');
    expect(sonner.success).toHaveBeenCalledWith('yay', { id: 'success:yay', duration: DURATIONS.success });
  });

  it('keeps errors up longer than success messages', () => {
    expect(DURATIONS.error).toBeGreaterThan(DURATIONS.success);
  });

  it('confirm() waits for an answer and wires both buttons', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    toast.confirm('sure?', onConfirm, onCancel, 'Delete');
    const [msg, opts] = sonner.mock.calls[0];
    expect(msg).toBe('sure?');
    expect(opts.duration).toBe(Number.POSITIVE_INFINITY);
    expect(opts.action.label).toBe('Delete');
    opts.action.onClick();
    expect(onConfirm).toHaveBeenCalled();
    opts.cancel.onClick();
    expect(onCancel).toHaveBeenCalled();
  });

  it('confirm() defaults to "Remove" and tolerates no cancel handler', () => {
    toast.confirm('sure?', vi.fn());
    const [, opts] = sonner.mock.calls[0];
    expect(opts.action.label).toBe('Remove');
    expect(() => opts.cancel.onClick()).not.toThrow();
  });

  it('dismiss() passes through', () => {
    toast.dismiss(3);
    expect(sonner.dismiss).toHaveBeenCalledWith(3);
  });
});
