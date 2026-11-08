#!/usr/bin/env bash
# Builds the frontend image and checks SPA fallback, cache and security headers.
set -euo pipefail
cd "$(dirname "$0")/.."
IMG=esg-frontend:test; NAME=esg-frontend-test; PORT=${PORT:-18080}
docker build -q -f frontend/Dockerfile -t "$IMG" .
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --rm --name "$NAME" --read-only --tmpfs /tmp -p "$PORT:8080" "$IMG" >/dev/null
trap 'docker rm -f "$NAME" >/dev/null 2>&1 || true' EXIT
for _ in $(seq 30); do curl -fs "localhost:$PORT/healthz" >/dev/null && break; sleep 0.5; done

fail=0
hdr() { curl -s -o /dev/null -D - "localhost:$PORT$1" | tr -d '\r'; }
check() { # path, expected-status, header-regex...
  local p=$1 want=$2; shift 2; local h; h=$(hdr "$p")
  grep -q "^HTTP/[0-9.]* $want" <<<"$h" || { echo "FAIL $p: status != $want"; fail=1; }
  for re in "$@"; do grep -qiE "$re" <<<"$h" || { echo "FAIL $p: missing /$re/"; fail=1; }; done
}
CSP="^content-security-policy: default-src 'self'; frame-ancestors 'none'"
REF="^referrer-policy: strict-origin-when-cross-origin"
NSN="^x-content-type-options: nosniff"
asset=$(docker exec "$NAME" sh -c 'ls /usr/share/nginx/html/assets | head -1')
check /                     200 "$CSP" "$REF" "$NSN" "^cache-control: no-cache"
check /ideas/123/deep-link  200 "$CSP" "^cache-control: no-cache"
check "/assets/$asset"      200 "$CSP" "$NSN" "^cache-control: public, max-age=31536000, immutable"
check /assets/missing.js    404 "$CSP" "$NSN"
check /api/v1/nope          404 "$CSP" "^content-type: application/problem\+json"
check /healthz              200 "$CSP"
[ "$(curl -s "localhost:$PORT/ideas/1" | grep -c '<div id="root">')" = 1 ] || { echo "FAIL fallback body"; fail=1; }
[ "$(docker exec "$NAME" id -u)" != 0 ] || { echo "FAIL runs as root"; fail=1; }
[ $fail = 0 ] && echo "frontend image OK"; exit $fail
