#!/usr/bin/env bash
# End-to-end check against a running API (default http://localhost:8080), e.g. after
# `docker compose up --build -d`. Posts the FR-01 AC1 example and compares the body.
set -euo pipefail

API_URL=${API_URL:-http://localhost:8080}
expected='{"word_count":5,"character_count":20,"top_words":[{"word":"the","count":2},{"word":"and","count":1},{"word":"cat","count":1},{"word":"hat","count":1}]}'

for _ in $(seq 1 30); do
  curl -fsS "$API_URL/healthz" >/dev/null 2>&1 && break
  sleep 1
done
curl -fsS "$API_URL/healthz" >/dev/null || { echo "smoke: API not healthy at $API_URL" >&2; exit 1; }

# The analyzer may still be starting after the API is up: retry while it answers 502.
for _ in $(seq 1 30); do
  status=$(curl -sS -o /tmp/smoke-body.$$ -w '%{http_code}' "$API_URL/analyze" \
    -H 'Content-Type: application/json' -d '{"text":"The cat and the hat."}')
  [ "$status" != 502 ] && break
  sleep 1
done
body=$(cat /tmp/smoke-body.$$); rm -f /tmp/smoke-body.$$

if [ "$status" = 200 ] && [ "$body" = "$expected" ]; then
  echo "smoke: OK $body"
else
  echo "smoke: FAILED (status $status)" >&2
  echo "  got:      $body" >&2
  echo "  expected: $expected" >&2
  exit 1
fi
