import { defineConfig } from 'astro/config';
import node from '@astrojs/node';

import svelte from '@astrojs/svelte';

export default defineConfig({
  output: 'server',

  // No dev toolbar along the bottom of the page in `astro dev`.
  devToolbar: { enabled: false },

  adapter: node({
    mode: 'standalone',
  }),

  vite: {
    // In dev mode, proxy /api/* from the browser to the Go API
    // so you don't need Caddy running locally
    server: {
      allowedHosts: ['dora-extraneous-stasia.ngrok-free.dev'],
      watch: {
        ignored: ['**/coverage/**', '**/dist/**'],
      },
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  },

  integrations: [svelte()],
});
