import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, act } from '../tests/svelte';
import Toast from './Toast.svelte';
import { toast } from '../lib/toast.svelte';

beforeEach(() => {
  toast.dismiss();
  vi.useFakeTimers();
});

describe('Toast component', () => {
  it('renders nothing when toast is not visible', () => {
    const { container } = render(Toast);
    expect(container.querySelector('.toast')).toBeNull();
  });

  it('renders message after show()', async () => {
    const { container } = render(Toast);
    await act(() => { toast.show('hello world'); });
    expect(container.querySelector('.toast__msg')?.textContent).toBe('hello world');
  });

  it('renders confirm buttons after confirm()', async () => {
    const { container } = render(Toast);
    await act(() => { toast.confirm('sure?', vi.fn(), vi.fn()); });
    expect(container.querySelector('.toast__confirm')).not.toBeNull();
    expect(container.querySelector('.toast__cancel')).not.toBeNull();
  });

  it('hides after dismiss()', async () => {
    const { container } = render(Toast);
    await act(() => { toast.show('bye'); });
    expect(container.querySelector('.toast')).not.toBeNull();
    await act(() => { toast.dismiss(); });
    expect(container.querySelector('.toast')).toBeNull();
  });
});
