#!/usr/bin/env bash
# Regenerate db/schema/schema.sql by running every goose migration against a
# throwaway Postgres container and dumping the resulting schema.
set -euo pipefail

CONTAINER=rollton-schema-dump-$$
PORT=55432
PASSWORD=schema_dump_pass

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run --rm -d \
  --name "$CONTAINER" \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB=schema_dump \
  -p "$PORT":5432 \
  postgres:16-alpine >/dev/null

# wait for ready
for _ in {1..30}; do
  if docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done

URL="postgres://postgres:${PASSWORD}@localhost:${PORT}/schema_dump?sslmode=disable"

if compgen -G "db/migrations/*.sql" >/dev/null; then
  goose -dir db/migrations postgres "$URL" up
fi

docker exec "$CONTAINER" pg_dump -U postgres --schema-only --no-owner --no-privileges schema_dump \
  | grep -vE '^\\(restrict|unrestrict) ' \
  > db/schema/schema.sql

echo "✓ db/schema/schema.sql regenerated"
