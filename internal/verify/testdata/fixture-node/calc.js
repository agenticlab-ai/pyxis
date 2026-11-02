const leftpad = require('leftpad');

function add(a, b) {
  return a + b;
}

function padded(s) {
  return leftpad(s, 5);
}

module.exports = { add, padded };
