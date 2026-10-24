#!/usr/bin/env python3
"""Walk the till through every sale workflow by copying barcodes to the clipboard.

Each workflow is a list of barcodes picked from the till's local database. The
script copies one barcode with pbcopy every second (paste it into the barcode
field with Cmd+V); when a list is done it waits, then press `n` to start the
next workflow. `n` also skips the rest of a list, `r` replays the current list from its
first barcode, `p` pauses/resumes, `q` quits.

    scripts/barcode_workflows.py                 # newest till DB with discounts
    scripts/barcode_workflows.py --db ~/Library/Application\\ Support/kashi-pos/database-tb
    scripts/barcode_workflows.py --central       # real data from kashi over ssh
    scripts/barcode_workflows.py --list          # just print the workflows
"""
import argparse
import glob
import os
import select
import sqlite3
import subprocess
import sys
import termios
import time
import tty

SUPPORT = os.path.expanduser("~/Library/Application Support/kashi-pos")

# Products sellable today: priced by an active price list. `d` is the plain
# list discount (0 when none), `deal` the deal uid the discount row names.
BASE = """
SELECT p.barcode, p.code, p.name, pi.price,
       COALESCE(di.discount, 0), COALESCE(di.deal_uid, ''),
       (SELECT COUNT(*) FROM product_sale_options o WHERE o.product_id = p.id)
FROM products p
JOIN price_list_items pi ON pi.product_uid = p.uid
JOIN price_lists pl ON pl.id = pi.price_list_id AND pl.is_active = 1
LEFT JOIN discount_list_items di ON di.product_uid = p.uid
  AND di.discount_list_id IN (SELECT id FROM discount_lists WHERE is_active = 1)
WHERE p.is_active = 1 AND p.barcode <> ''
GROUP BY p.id
"""


def connect(path):
    return sqlite3.connect(f"file:{path}?mode=ro", uri=True)


def pick_db():
    """The most recently used till database that has any discount rows."""
    paths = [
        p for p in glob.glob(os.path.join(SUPPORT, "database*"))
        if not p.endswith(("-shm", "-wal")) and ".backup" not in p
    ]
    for path in sorted(paths, key=os.path.getmtime, reverse=True):
        try:
            with connect(path) as db:
                if db.execute("SELECT 1 FROM discount_list_items WHERE discount > 0 LIMIT 1").fetchone():
                    return path
        except sqlite3.Error:
            continue
    return max(paths, key=os.path.getmtime)


COLS = ["barcode", "code", "name", "price", "d", "deal", "opts"]

# The same three questions against kashi's Postgres (read-only, over `ssh kashi`).
CENTRAL_PRODUCTS = """
SELECT v.barcode, p.code, p.name, pi.price, COALESCE(di.discount, 0), COALESCE(di.deal_id::text, ''),
       (SELECT count(*) FROM product_sale_options o WHERE o.product_id = v.product_id)
FROM product_variants v JOIN products p ON p.id = v.product_id
JOIN price_list_items pi ON pi.product_variant_id = v.id
JOIN price_lists pl ON pl.id = pi.price_list_id AND pl.is_active
LEFT JOIN discount_list_items di ON di.product_variant_id = v.id
  AND di.discount_list_id IN (SELECT id FROM discount_lists WHERE is_active)
WHERE v.is_active AND v.barcode <> ''
"""
CENTRAL_VARIANTS = """
SELECT v.barcode FROM product_variants v JOIN products p ON p.id = v.product_id
WHERE v.is_active AND v.barcode <> '' AND p.code = (
  SELECT p2.code FROM product_variants v2 JOIN products p2 ON p2.id = v2.product_id
  WHERE v2.is_active AND v2.barcode <> '' AND p2.code <> ''
  GROUP BY p2.code HAVING count(*) >= 3 ORDER BY count(*) LIMIT 1) LIMIT 4
"""
CENTRAL_UNPRICED = """
SELECT v.barcode FROM product_variants v WHERE v.is_active AND v.barcode <> '' AND NOT EXISTS (
  SELECT 1 FROM price_list_items pi JOIN price_lists pl ON pl.id = pi.price_list_id AND pl.is_active
  WHERE pi.product_variant_id = v.id) LIMIT 2
"""


def central_rows(sql):
    out = subprocess.run(
        ["ssh", "kashi", "LC_ALL=C.UTF-8 psql 'postgresql://kashi:kashi@localhost:5432/kashi?sslmode=disable'"
         " -At -F'|' -v ON_ERROR_STOP=1"],
        input=sql, capture_output=True, text=True, check=True,
    ).stdout
    return [line.split("|") for line in out.splitlines() if line]


def from_central():
    products = [
        dict(zip(COLS, [r[0], r[1], r[2], int(r[3]), int(r[4]), r[5], int(r[6])]))
        for r in central_rows(CENTRAL_PRODUCTS)
    ]
    return (products, [r[0] for r in central_rows(CENTRAL_VARIANTS)],
            [r[0] for r in central_rows(CENTRAL_UNPRICED)])


def from_local(db):
    products = [dict(zip(COLS, r)) for r in db.execute(BASE)]
    # Same code, several barcodes: colour/size variants of one model.
    model = db.execute(
        "SELECT code FROM products WHERE is_active = 1 AND barcode <> '' AND code <> '' "
        "GROUP BY code HAVING COUNT(*) >= 3 LIMIT 1"
    ).fetchone()
    variants = [r[0] for r in db.execute(
        "SELECT barcode FROM products WHERE code = ? AND is_active = 1 AND barcode <> '' LIMIT 4",
        (model[0],),
    )] if model else []
    # In no active price list: the till must refuse it as unpriced.
    unpriced = [r[0] for r in db.execute(
        "SELECT barcode FROM products p WHERE is_active = 1 AND barcode <> '' AND NOT EXISTS "
        "(SELECT 1 FROM price_list_items pi JOIN price_lists pl ON pl.id = pi.price_list_id "
        "AND pl.is_active = 1 WHERE pi.product_uid = p.uid) LIMIT 2"
    )]
    return products, variants, unpriced


def build_workflows(products, variants, unpriced):
    plain = [p for p in products if not p["d"] and not p["deal"] and not p["opts"]]
    disc = [p for p in products if p["d"] and not p["deal"] and not p["opts"]]
    deal = [p for p in products if p["deal"]]
    opts = [p for p in products if p["opts"]]
    cheap = sorted(plain, key=lambda p: p["price"])

    def take(lst, n, skip=0):
        return [p["barcode"] for p in lst[skip:skip + n]]

    first_deal = deal[0]["deal"] if deal else None
    same_deal = [p for p in deal if p["deal"] == first_deal]

    flows = [
        ("1. Plain full-price items, no discount", take(plain, 3)),
        ("2. List-discounted items (chip on the line)", take(disc, 3)),
        ("3. Same discounted item scanned 3x (qty 3)", take(disc, 1) * 3),
        ("4. Items of one deal (B2G1 / bundle): fills the deal, then one over",
         take(same_deal, 4) or take(deal, 3)),
        ("5. Sale-option products (Summer/Winter picker)", take(opts, 3)),
        ("6. Variants: one model, several colours/sizes", variants),
        ("7. Mixed cart: discounted + full-price + deal + option",
         take(disc, 2) + take(cheap, 2) + take(deal, 1) + take(opts, 1)),
        ("8. Discounted item alone, then a full-price one (conditional-discount case)",
         take(disc, 1) + take(cheap, 1, skip=5)),
        ("9. Unpriced products (till must refuse)", unpriced),
        ("10. Barcode that does not exist", ["0000000000000", "NOTABARCODE"]),
    ]
    for title, codes in flows:
        if not codes:
            print(f"skipped (nothing in this database): {title}")
    return [(title, codes) for title, codes in flows if codes]


def copy(text):
    subprocess.run(["pbcopy"], input=text.encode(), check=True)


def key_pressed(timeout):
    """One keypress within timeout seconds (forever if None), else None."""
    ready, _, _ = select.select([sys.stdin], [], [], timeout)
    return sys.stdin.read(1).lower() if ready else None


def run(flows):
    """Returns when done or on q. n = next workflow, r = replay this one."""
    paused = False
    number = 0
    while number < len(flows):
        title, codes = flows[number]
        print(f"\n\033[1m{number + 1}/{len(flows)} {title}\033[0m  ({len(codes)} barcodes)\r")
        action = None  # "n" or "r" when a key cuts the list short
        for code in codes:
            copy(code)
            print(f"  copied {code}\r")
            deadline = time.time() + 1
            while True:
                key = key_pressed(0.1 if paused else max(0, deadline - time.time()))
                if key == "q":
                    return
                if key in ("n", "r"):
                    action = key
                    break
                if key == "p":
                    paused = not paused
                    print("  paused\r" if paused else "  resumed\r")
                    deadline = time.time() + 1
                if not paused and time.time() >= deadline:
                    break
            if action:
                break
        if not action:
            if number == len(flows) - 1:
                break
            print("  list done. n = next workflow, r = repeat this one, q = quit\r")
            while (action := key_pressed(None)) not in ("n", "r", "q"):
                pass
            if action == "q":
                return
        if action == "n":
            number += 1
    print("\nall workflows done\r")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--db", help="till database (default: newest one with discounts)")
    parser.add_argument("--central", action="store_true", help="read kashi's database over `ssh kashi` instead")
    parser.add_argument("--list", action="store_true", help="print the workflows and exit")
    args = parser.parse_args()

    if args.central:
        print("source: kashi (ssh kashi, read-only)")
        flows = build_workflows(*from_central())
    else:
        path = os.path.expanduser(args.db) if args.db else pick_db()
        print(f"database: {path}")
        with connect(path) as db:
            flows = build_workflows(*from_local(db))

    if args.list or not sys.stdin.isatty():
        for title, codes in flows:
            print(f"\n{title}\n  " + "\n  ".join(codes))
        return

    print(f"{len(flows)} workflows. n = next, r = repeat, p = pause, q = quit. Starting in 3s: focus the barcode field.")
    time.sleep(3)
    fd = sys.stdin.fileno()
    saved = termios.tcgetattr(fd)
    try:
        tty.setcbreak(fd)
        run(flows)
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, saved)


if __name__ == "__main__":
    main()
