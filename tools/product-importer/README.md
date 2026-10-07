# Kashi Product Importer

A small Windows desktop app for batch-inserting products into a Kashi POS
SQLite database from a CSV file. Built with [Wails](https://wails.io) (Go +
a plain HTML/JS frontend, no Node build step required).

## Usage

1. Launch `kashi-product-importer.exe`.
2. **Choose database file…** and pick the Kashi `database` SQLite file you
   want to add products to. The Brand, Family, Kind, Season, Color and Size
   fields then suggest the values already used in that database.
3. Fill in the product (Name, Code and Price are required; leave Barcode
   empty to generate a valid EAN-13) and click **Add to list**. The form
   keeps everything except Color, Size and Barcode, so adding the next
   variant only takes those fields.
4. Click **Import**. Each product reports its own result, so one bad
   product doesn't block the rest; failed ones stay in the list to fix.

Prices are stored in cents. On databases with price lists (POS builds from
2026-09-28 on), the till sells only at list prices, so each product's price
is also added to the active default price list, and a discount to the
default discount list. On a till connected to kashi, kashi's next
price-list sync replaces that list and removes these prices.

Databases from POS versions before 0.8 are not supported.

## Building

From this directory:

```sh
wails build -platform windows/amd64
```

The resulting `.exe` is written to `build/bin/`. This also cross-compiles
fine from macOS/Linux since the app uses the pure-Go `modernc.org/sqlite`
driver (no CGO).

To run it locally during development (opens a native window on the host
OS):

```sh
wails dev
```
