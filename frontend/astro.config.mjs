import { defineConfig } from 'astro/config';

export default defineConfig({
  output: 'static',
  vite: {
    // In dev mode, proxy /api/* from the browser to the Go API
    // so you don't need Caddy running locally
    server: {
      allowedHosts: ['dora-extraneous-stasia.ngrok-free.dev'],
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  },
});
