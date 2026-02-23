/// <reference types="vitest/config" />
import { getViteConfig } from 'astro/config';

export default getViteConfig({
  test: {
    coverage: {
      provider: 'v8',
      include: ['src/**/*.ts'],
      exclude: ['src/env.d.ts'],
      thresholds: { lines: 30, functions: 30, branches: 30, statements: 30 },
    },
  },
});
