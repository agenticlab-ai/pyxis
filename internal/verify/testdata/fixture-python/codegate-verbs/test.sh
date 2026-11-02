#!/bin/sh
set -e
for f in test_*.py *_test.py; do
  [ -e "$f" ] || continue
  python3 "$f"
done
