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
});
