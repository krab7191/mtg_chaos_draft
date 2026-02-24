import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { toast } from './toast.svelte';

beforeEach(() => {
  toast.dismiss();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('toast store', () => {
  it('show() sets visible, msg, and default type=info', () => {
    toast.show('hello');
    expect(toast.visible).toBe(true);
    expect(toast.msg).toBe('hello');
    expect(toast.type).toBe('info');
    expect(toast.mode).toBe('info');
  });

  it('show() accepts explicit type', () => {
    toast.show('oops', 'error');
    expect(toast.type).toBe('error');
  });

  it('dismiss() clears visible', () => {
    toast.show('hi');
    toast.dismiss();
    expect(toast.visible).toBe(false);
  });

  it('auto-dismisses after 4s', () => {
    toast.show('bye');
    expect(toast.visible).toBe(true);
    vi.advanceTimersByTime(4000);
    expect(toast.visible).toBe(false);
  });

  it('confirm() sets mode=confirm and stores callbacks', () => {
    const onConfirm = vi.fn();
    const onCancel  = vi.fn();
    toast.confirm('sure?', onConfirm, onCancel);
    expect(toast.visible).toBe(true);
    expect(toast.mode).toBe('confirm');
    expect(toast.msg).toBe('sure?');
    expect(toast.onConfirm).toBe(onConfirm);
    expect(toast.onCancel).toBe(onCancel);
  });
});
