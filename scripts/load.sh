#!/usr/bin/env sh
# Dispara requisicoes sequenciais e mostra o status de cada uma.
# Uso: ./scripts/load.sh [total] [token]

set -eu

TOTAL="${1:-15}"
TOKEN="${2:-}"
URL="${URL:-http://localhost:8080/}"

i=1
while [ "$i" -le "$TOTAL" ]; do
	if [ -n "$TOKEN" ]; then
		status=$(curl -s -o /dev/null -w '%{http_code}' -H "API_KEY: $TOKEN" "$URL")
	else
		status=$(curl -s -o /dev/null -w '%{http_code}' "$URL")
	fi
	echo "$i -> $status"
	i=$((i + 1))
done
