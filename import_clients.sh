#!/bin/zsh

set -euo pipefail

if (( $# != 2 )); then
  echo "Usage: $0 DB_PATH CLIENTS_CSV" >&2
  echo "Example: $0 db/kashi.db clients.csv" >&2
  exit 2
fi

db_path="$1"
csv_path="$2"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "Error: sqlite3 is required but was not found in PATH." >&2
  exit 1
fi

if [[ ! -f "$db_path" ]]; then
  echo "Error: database not found: $db_path" >&2
  exit 1
fi

if [[ ! -f "$csv_path" ]]; then
  echo "Error: CSV not found: $csv_path" >&2
  exit 1
fi

backup_path="${db_path}.backup.$(date +%Y%m%d-%H%M%S)"
cp "$db_path" "$backup_path"

# The CSV must have a header row: name,phone
# Phone is deliberately TEXT so leading zeroes are preserved.
sqlite3 -bail "$db_path" \
  "CREATE TEMP TABLE client_import(name TEXT, phone TEXT);" \
  ".mode csv" \
  ".import --skip 1 \"$csv_path\" client_import" \
  "BEGIN;" \
  "INSERT INTO clients (name, phone, total_loyalty_points, valid_loyalty_points) SELECT trim(min(i.name)), trim(i.phone), 0, 0 FROM client_import i WHERE trim(i.name) <> '' AND trim(i.phone) <> '' AND NOT EXISTS (SELECT 1 FROM clients c WHERE c.phone = trim(i.phone)) GROUP BY trim(i.phone);" \
  "SELECT 'Imported clients: ' || changes();" \
  "COMMIT;"

echo "Backup created: $backup_path"
