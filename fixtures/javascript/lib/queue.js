'use strict';

class Queue {
  #items = [];

  constructor(limit) {
    this.limit = limit ?? 10;
  }

  push(item) {
    if (item === undefined || item === null) {
      return false;
    }
    if (this.#items.length >= this.limit) {
      this.#items.shift();
    }
    this.#items.push(item);
    return true;
  }

  pop() {
    return this.#items.length > 0 ? this.#items.shift() : undefined;
  }

  get size() {
    return this.#items.length;
  }

  #compact() {
    this.#items = this.#items.filter((item) => item != null);
  }
}

function priority(task) {
  let score = 0;
  if (task.urgent && !task.blocked) {
    score += 10;
  }
  if (task.owner?.vip || task.tags?.includes('vip')) {
    score += 5;
  }
  return score;
}

module.exports = { Queue, priority };
