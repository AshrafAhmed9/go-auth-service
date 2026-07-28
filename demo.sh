#!/usr/bin/env bash
# Live demo of the Go + Java auth platform: this service plus the Spring Boot
# notes API that authenticates against it.
#
# The two-service compose file lives in the springboot-resource-api repo,
# because that's the side that builds both images. This script just finds
# that repo next to this one and drives it, so the demo starts from here.
#
# Pass --reset to wipe stale Postgres volumes first — do that before a real
# interview so no leftover data from a rehearsal is sitting there.
set -euo pipefail
cd "$(dirname "$0")"

STACK_DIR=$(ls -d ../*springboot-resource-api 2>/dev/null | head -1 || true)
if [[ -z "$STACK_DIR" ]]; then
  echo "Couldn't find the springboot-resource-api repo next to this one."
  echo "Clone it as a sibling directory and try again:"
  echo "  git clone https://github.com/AshrafAhmed9/springboot-resource-api.git"
  exit 1
fi
cd "$STACK_DIR"

BOLD=$(tput bold 2>/dev/null || true)
DIM=$(tput dim 2>/dev/null || true)
RESET=$(tput sgr0 2>/dev/null || true)
GREEN=$(tput setaf 2 2>/dev/null || true)
RED=$(tput setaf 1 2>/dev/null || true)

say() { echo -e "\n${BOLD}>> $1${RESET}"; }
pause() { read -rp "$(echo -e "${DIM}[press enter to continue]${RESET}")" _; }
pretty() { command -v jq >/dev/null 2>&1 && jq . || cat; }

if [[ "${1:-}" == "--reset" ]]; then
  say "Resetting: wiping old containers and volumes for a clean run"
  docker compose down -v
fi

say "Starting the stack (Go auth service + Postgres/Redis, Java notes API + its own Postgres)"
docker compose up -d --build

say "Waiting for both services to be ready..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8080/health >/dev/null 2>&1 && curl -sf http://localhost:8081/actuator/health >/dev/null 2>&1; then
    echo "${GREEN}Both services are up.${RESET}"
    break
  fi
  sleep 1
  if [[ $i -eq 60 ]]; then
    echo "${RED}Timed out waiting for services. Run 'docker compose logs' to see what's wrong.${RESET}"
    exit 1
  fi
done

echo
echo "${BOLD}What you're about to see:${RESET} log into the Go identity service, use that"
echo "token against a separate Java service that trusts Go for identity over gRPC,"
echo "then kill the Go service mid-session to show the Java side deliberately"
echo "refuses to serve data it can't authenticate — and recovers on its own."
pause

say "1. Logging into the Go auth service (seeded admin account)"
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@app.com","password":"admin123"}' | jq -r .access_token)

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "${RED}Login failed — no token came back. Check 'docker logs auth-api'.${RESET}"
  exit 1
fi
echo "Got a token (truncated): ${TOKEN:0:40}..."
pause

say "2. Using that token against the Java service — this is the cross-service proof"
echo "${DIM}GET /api/me${RESET}"
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/me | pretty
echo
echo "${DIM}POST /api/notes${RESET}"
curl -s -X POST http://localhost:8081/api/notes \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"title":"Demo note","body":"Proves the gRPC auth path works"}' | pretty
echo
echo "${DIM}GET /api/notes${RESET}"
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/notes | pretty
echo
echo "The Java service never touched a password or the Go service's database —"
echo "it validated that token over gRPC and cached the result."
pause

say "3. Killing the auth service — watch it fail CLOSED, not open"
docker stop auth-api >/dev/null
echo "${DIM}GET /api/notes with a token that isn't cached${RESET}"
curl -s -i -H "Authorization: Bearer some-token-not-cached-yet" http://localhost:8081/api/notes | head -12
echo
echo "The auth service is down, this token was never cached, so the Java service"
echo "refuses to guess — 503 with Retry-After, instead of letting the request through."
pause

say "4. Bringing the auth service back — automatic recovery, no restart needed"
docker start auth-api >/dev/null
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/health >/dev/null 2>&1; then break; fi
  sleep 1
done
sleep 1
echo "${DIM}GET /api/notes with the original token${RESET}"
curl -s -o /dev/null -w "Status: %{http_code}\n" -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/notes
echo
echo "Back to normal, automatically."
pause

say "Numbers to have ready if asked"
echo "31 Go tests · 30 Java tests (61 total) · cache TTL ≤60s · circuit breaker:"
echo "50% failure threshold over a 10-call window, retries after 10s · warm cache"
echo "~91.8 req/s p50 6.3ms · cold cache ~92.9 req/s p50 6.0ms"

echo
read -rp "${BOLD}Shut the stack down now? [y/N] ${RESET}" answer
if [[ "$answer" == "y" || "$answer" == "Y" ]]; then
  docker compose down
  echo "Stack stopped."
else
  echo "Leaving it running. 'docker compose down' when you're done."
fi
