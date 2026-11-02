const assert = require('assert');
const { add, padded } = require('./calc');

assert.strictEqual(add(1, 2), 3);
assert.strictEqual(padded('hi'), '   hi');
