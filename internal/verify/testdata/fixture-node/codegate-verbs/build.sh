#!/bin/sh
set -e
for f in *.js; do
  [ -e "$f" ] || continue
  node --check "$f"
done
