'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { Queue, priority } = require('../lib/queue.js');

test('queue keeps at most limit items', () => {
  const q = new Queue(2);
  assert.equal(q.push(1), true);
  q.push(2);
  q.push(3);
  assert.equal(q.size, 2);
  assert.equal(q.pop(), 2);
  assert.equal(q.push(null), false);
});

test('priority of urgent tasks', () => {
  assert.equal(priority({ urgent: true }), 10);
  assert.equal(priority({ urgent: false }), 0);
});
