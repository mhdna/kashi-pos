#!/bin/zsh

set -euo pipefail

if (( $# != 2 )); then
  echo "Usage: $0 DB_PATH PRODUCTS_CSV" >&2
  echo "Example: $0 db/kashi.db products.csv" >&2
  exit 2
fi

db_path="$1"
csv_path="$2"

command -v sqlite3 >/dev/null 2>&1 || { echo "Error: sqlite3 is required." >&2; exit 1; }
[[ -f "$db_path" ]] || { echo "Error: database not found: $db_path" >&2; exit 1; }
[[ -f "$csv_path" ]] || { echo "Error: CSV not found: $csv_path" >&2; exit 1; }

for column in color size currency_code; do
  if [[ "$(sqlite3 "$db_path" "SELECT count(*) FROM pragma_table_info('products') WHERE name = '$column';")" != "1" ]]; then
    echo "Error: database schema is not migrated for product variants." >&2
    echo "Start the current app once, then retry the import." >&2
    exit 1
  fi
done

backup_path="${db_path}.backup.$(date +%Y%m%d-%H%M%S)"
cp "$db_path" "$backup_path"

# Expected header:
# Product,Product Name,Unit Price,Currency,Color,Size,Barcode
# Each row is one sellable variant. Product codes may repeat, but barcodes
# must be unique. Unit Price is normalized to the app's accounting USD cents;
# LBP values are converted using settings.exchange_rate (LBP per USD).
sqlite3 -bail "$db_path" \
  "CREATE TEMP TABLE product_import(product TEXT, product_name TEXT, unit_price TEXT, currency TEXT, color TEXT, size TEXT, barcode TEXT);" \
  ".mode csv" \
  ".import --skip 1 \"$csv_path\" product_import" \
  "BEGIN;" \
  "INSERT INTO products (barcode, code, name, description, is_active, price, discount, color, size, currency_code) SELECT trim(i.barcode), trim(i.product), trim(i.product_name), '', 1, CASE WHEN upper(trim(i.currency)) = 'LBP' THEN CAST(round(CAST(replace(trim(i.unit_price), ',', '') AS REAL) * 100.0 / coalesce((SELECT exchange_rate FROM settings LIMIT 1), 90000)) AS INTEGER) ELSE CAST(round(CAST(replace(trim(i.unit_price), ',', '') AS REAL) * 100) AS INTEGER) END, 0, trim(coalesce(i.color, '')), trim(coalesce(i.size, '')), upper(trim(i.currency)) FROM product_import i WHERE trim(i.product) <> '' AND trim(i.product_name) <> '' AND trim(i.unit_price) <> '' AND trim(i.currency) <> '' AND trim(i.barcode) <> '' AND CAST(replace(trim(i.unit_price), ',', '') AS REAL) > 0 AND NOT EXISTS (SELECT 1 FROM products p WHERE p.barcode = trim(i.barcode)) GROUP BY trim(i.barcode);" \
  "SELECT 'Imported product variants: ' || changes();" \
  "COMMIT;"

echo "Backup created: $backup_path"
