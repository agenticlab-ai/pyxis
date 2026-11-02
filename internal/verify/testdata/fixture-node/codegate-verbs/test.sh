#!/bin/sh
set -e
for f in *.test.js; do
  [ -e "$f" ] || continue
  node "$f"
done
