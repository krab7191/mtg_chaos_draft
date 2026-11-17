import { describe, it, expect, vi, afterEach } from 'vitest';
import { render } from '../tests/svelte';
import Toast from './Toast.svelte';

afterEach(() => vi.unstubAllGlobals());

describe('Toast component', () => {
  it('mounts the notifications region', () => {
    // jsdom has no matchMedia; sonner uses it for the theme.
    vi.stubGlobal('matchMedia', () => ({
      matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(),
    }));
    const { container } = render(Toast);
    expect(container.querySelector('section[aria-label*="Notifications"]')).not.toBeNull();
  });
});
