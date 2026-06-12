# bot/

Go module for all Rollton Telegram bots. One module, multiple binaries (`cmd/<bot>/`), shared internals under `internal/`.

## Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `goose` CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- `sqlc`:     `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `air`:      `go install github.com/air-verse/air@latest` (hot reload, dev only)

## First-time setup

```bash
cp .env.example .env                   # fill in TOKEN_ROLLTONCHATBOT etc.
make up                                # start postgres
make migrate-status                    # should report "no migrations" cleanly
make build                             # build all binaries into ./bin/
```

## Running a bot

```bash
make run BOT=rolltonchatbot            # vanilla
make dev BOT=rolltonchatbot            # with hot reload via air
```

## Migrations (goose)

```bash
make migrate-new name=add_users        # creates db/migrations/<timestamp>_add_users.sql
make migrate-up                        # apply
make migrate-status
make migrate-down                      # roll back one step
```

After modifying migrations, regenerate the snapshot schema sqlc reads:

```bash
make schema-dump                       # uses scripts/schema-dump.sh
```

## sqlc

```bash
make sqlc-gen                          # writes to pkg/sqlc/postgres/
```

Note: `sqlc-gen` errors until the first `.sql` file lands in `db/queries/`. Add your first query (and the matching schema in `db/schema/schema.sql` via `make schema-dump`) before running it.

## Tests

```bash
make test                              # unit only (-short)
make test-integration                  # includes testcontainers
make coverage
```
