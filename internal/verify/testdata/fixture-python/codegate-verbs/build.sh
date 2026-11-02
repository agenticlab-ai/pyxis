#!/bin/sh
set -e
for f in *.py; do
  [ -e "$f" ] || continue
  python3 -m py_compile "$f"
done
