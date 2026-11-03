import test from 'node:test';
import assert from 'node:assert/strict';
import { plural, clamp, describe, totals } from '../lib/format.mjs';

test('plural', () => {
  assert.equal(plural(1, 'cat'), '1 cat');
  assert.equal(plural(2, 'cat'), '2 cats');
});

test('clamp', () => {
  assert.equal(clamp(5, 0, 3), 3);
  assert.equal(clamp(2, 0, 3), 2);
});

test('describe', () => {
  assert.equal(describe({ name: 'ann', role: 'admin' }), 'ann (admin)');
  assert.equal(describe(undefined), 'anonymous');
});

test('totals', () => {
  assert.deepEqual(totals([{ amount: 2 }, { amount: 4 }, { amount: -1 }]), { sum: 6, count: 2, mean: 3 });
});
