import type { E2EConfig } from 'e2e';

export default {
  targets: [{ name: 'cli', platform: 'cli' }],
  tests: ['tests/e2e/**/*.e2e.ts'],
  workers: 1,
  retries: 0,
  timeout: 30_000,
  reporters: ['list', 'junit', 'markdown'],
} satisfies E2EConfig;
