// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { createContainer } from '../tests/astro';
import Layout from './Layout.astro';

describe('Layout', () => {
  it('renders title in <title> tag', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test Page' },
    });
    expect(html).toContain('<title>Test Page — MTG Chaos Draft</title>');
  });

  it('renders no nav-menu without user', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
    });
    expect(html).not.toContain('nav-menu__item');
  });

  it('renders nav-menu with user', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: {
        title: 'Test',
        user: { name: 'Alice', email: 'a@b.com', role: 'user', picture: null },
      },
    });
    expect(html).toContain('nav-menu__item');
  });

  it('renders admin links for admin role', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: {
        title: 'Test',
        user: { name: 'Alice', email: 'a@b.com', role: 'admin', picture: null },
      },
    });
    expect(html).toContain('Manage Collection');
  });

  it('renders slot content', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: { title: 'Test' },
      slots: { default: '<p id="slot-content">Hello</p>' },
    });
    expect(html).toContain('id="slot-content"');
  });

  it('pages can opt into the wide layout for big tables', async () => {
    const container = await createContainer();
    const narrow = await container.renderToString(Layout, { props: { title: 'T' } });
    const wide = await container.renderToString(Layout, { props: { title: 'T', wide: true } });
    expect(narrow).not.toContain('main--wide');
    expect(wide).toMatch(/class="main main--wide/);
  });

  it('groups admin pages under headings and marks the current page', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: { title: 'T', user: { name: 'A', email: 'a@b.com', role: 'viewer' } },
      request: new Request('http://localhost/admin/finance'),
    });
    const drafts = html.indexOf('Drafts &amp; collection');
    const prices = html.indexOf('Prices &amp; finance');
    expect(drafts).toBeGreaterThan(-1);
    expect(prices).toBeGreaterThan(drafts);
    // Each page sits in its group.
    for (const label of ['Manage Collection', 'Weights', '>Drafts<']) {
      const at = html.indexOf(label);
      expect(at > drafts && at < prices).toBe(true);
    }
    for (const label of ['>Finance<', 'Price Trends', 'Cheap Packs']) {
      expect(html.indexOf(label)).toBeGreaterThan(prices);
    }
    expect(html).toMatch(/href="\/admin\/finance"[^>]*aria-current="page"/);
    expect(html.match(/aria-current="page"/g)).toHaveLength(1);
  });

  it('players only get Pick a Pack, marked when they are on it', async () => {
    const container = await createContainer();
    const html = await container.renderToString(Layout, {
      props: { title: 'T', user: { name: 'P', email: 'p@b.com', role: 'user' } },
      request: new Request('http://localhost/select/'),
    });
    expect(html).not.toContain('Prices &amp; finance');
    expect(html).not.toContain('/admin/');
    expect(html).toMatch(/href="\/select"[^>]*aria-current="page"/);
  });
});
