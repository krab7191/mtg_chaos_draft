/// <reference types="vitest/config" />
import { getViteConfig } from 'astro/config';

export default getViteConfig({
  test: {
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/**/*.ts', 'src/**/*.astro', 'src/**/*.svelte'],
      exclude: ['src/**/*.d.ts'],
      thresholds: { lines: 1, functions: 1, branches: 1, statements: 1 },
    },
  },
});
