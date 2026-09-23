#!/bin/sh
set -eu

base_url="http://127.0.0.1:${GATEWAY_PORT:-8080}"
body=$(curl --fail --silent --show-error --max-time 5 "$base_url/healthz")
test "$body" = '{"status":"ok"}'
status=$(curl --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' \
  -X POST "$base_url/v1/chat/completions")
test "$status" = 404
echo 'Smoke passed: healthy process, proxy API correctly unavailable in M0.'
