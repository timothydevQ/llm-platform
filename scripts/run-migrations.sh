#!/usr/bin/env bash
# run-migrations.sh — apply SQL migrations to PostgreSQL or SQLite
# Usage:
#   DATABASE_URL=postgres://user:pass@host/db bash scripts/run-migrations.sh
#   DB_DRIVER=sqlite DB_PATH=/data/app.db bash scripts/run-migrations.sh
set -euo pipefail
cd "$(dirname "$0")/.."

DB_DRIVER="${DB_DRIVER:-sqlite}"
DB_PATH="${DB_PATH:-/tmp/llm-platform-dev.db}"
DATABASE_URL="${DATABASE_URL:-}"
MIGRATIONS_DIR="sql/migrations"
SEEDS_DIR="sql/seeds"

run_sqlite() {
  local db="$1"
  echo "Running migrations on SQLite: $db"
  for f in "$MIGRATIONS_DIR"/*.sql; do
    echo "  Applying: $f"
    sqlite3 "$db" < "$f"
    echo "  ✓ $f"
  done
}

run_postgres() {
  echo "Running migrations on PostgreSQL: $DATABASE_URL"
  for f in "$MIGRATIONS_DIR"/*.sql; do
    echo "  Applying: $f"
    psql "$DATABASE_URL" -f "$f"
    echo "  ✓ $f"
  done
}

run_seeds() {
  local driver="$1"
  echo "Running seeds..."
  for f in "$SEEDS_DIR"/*.sql; do
    if [[ "$driver" == "sqlite" ]]; then
      sqlite3 "$DB_PATH" < "$f" 2>/dev/null || true
    else
      psql "$DATABASE_URL" -f "$f" 2>/dev/null || true
    fi
    echo "  ✓ $f"
  done
}

case "$DB_DRIVER" in
  sqlite)
    run_sqlite "$DB_PATH"
    run_seeds "sqlite"
    ;;
  postgres|postgresql)
    if [[ -z "$DATABASE_URL" ]]; then
      echo "ERROR: DATABASE_URL must be set for postgres driver"
      exit 1
    fi
    run_postgres
    run_seeds "postgres"
    ;;
  *)
    echo "Unknown DB_DRIVER: $DB_DRIVER (use 'sqlite' or 'postgres')"
    exit 1
    ;;
esac

echo ""
echo "✓ All migrations applied."
