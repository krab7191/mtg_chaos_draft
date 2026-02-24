/// <reference types="vitest/config" />
import { getViteConfig } from 'astro/config';

export default getViteConfig({
  resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
  test: {
    environment: 'jsdom',
    setupFiles: ['src/tests/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary', 'html'],
      include: ['src/**/*.ts', 'src/**/*.astro', 'src/**/*.svelte'],
      exclude: ['src/**/*.d.ts'],
      thresholds: { lines: 70, functions: 60, branches: 50, statements: 70 },
    },
  },
});
