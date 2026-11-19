import '@testing-library/jest-dom/vitest';
import '@testing-library/svelte/vitest';

// jsdom has no ResizeObserver (Svelte's bind:clientWidth uses it); every
// browser does. A no-op stand-in: sizes stay 0, so components use defaults.
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
