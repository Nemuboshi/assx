import assert from 'node:assert/strict';
import test from 'node:test';
import { serverSettings } from './settings.js';

test('sends renderer settings under the server schema', () => {
  const values: Record<string, unknown> = { renderers: ['vsfiltermod', 'libass'] };
  assert.deepEqual(serverSettings(key => values[key]), {
    renderers: ['vsfiltermod', 'libass']
  });
});
