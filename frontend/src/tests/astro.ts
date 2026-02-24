import { experimental_AstroContainer as AstroContainer } from 'astro/container';
import { getContainerRenderer } from '@astrojs/svelte';
import svelteServerRenderer from '@astrojs/svelte/server.js';

/** Create an AstroContainer with the Svelte renderer (ssr module) pre-loaded. */
export async function createContainer() {
  const rendererDef = getContainerRenderer();
  return AstroContainer.create({
    renderers: [{ ...rendererDef, ssr: svelteServerRenderer }],
  });
}

type AstroPage = Parameters<Awaited<ReturnType<typeof AstroContainer.create>>['renderToResponse']>[0];

/** Render a page and return the Response (handles redirects and full renders). */
export async function renderPage(Page: AstroPage, path: string) {
  const container = await createContainer();
  return container.renderToResponse(Page, {
    request: new Request(`http://localhost${path}`),
  });
}
