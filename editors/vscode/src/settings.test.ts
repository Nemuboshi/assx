import assert from 'node:assert/strict';
import test from 'node:test';
import { serverSettings } from './settings.js';

test('sends renderer settings under the server schema', () => {
  const values: Record<string, unknown> = {
    primaryRenderer: 'vsfiltermod', compatibilityRenderers: ['libass'],
    'vsfilterMod.mod': 'enabled', 'vsfilterMod.lua': 'disabled'
  };
  assert.deepEqual(serverSettings(key => values[key]), {
    primaryRenderer: 'vsfiltermod', compatibilityRenderers: ['libass'],
    vsfilterMod: { mod: 'enabled', lua: 'disabled' }
  });
});
