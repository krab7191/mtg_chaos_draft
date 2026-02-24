// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { createContainer } from '../tests/astro';
import PackSearch from './PackSearch.astro';

describe('PackSearch', () => {
  it('renders pack-search container', async () => {
    const container = await createContainer();
    const result = await container.renderToString(PackSearch);
    expect(result).toContain('id="pack-search"');
  });

  it('renders search input', async () => {
    const container = await createContainer();
    const result = await container.renderToString(PackSearch);
    expect(result).toContain('id="search-input"');
  });
});
