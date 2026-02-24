import { render as _render } from '@testing-library/svelte';
export { act, fireEvent, screen, cleanup, waitFor } from '@testing-library/svelte';

type R = typeof _render;

export function render(component: unknown, options?: Parameters<R>[1]): ReturnType<R> {
  return _render(component as Parameters<R>[0], options);
}
